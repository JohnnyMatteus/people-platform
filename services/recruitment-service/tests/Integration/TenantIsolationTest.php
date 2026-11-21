<?php

namespace Tests\Integration;

use App\Application\EventPublisher;
use App\Application\OutboxRelay;
use App\Infrastructure\Tenancy\TenantTransaction;
use App\Models\Job;
use App\Models\OutboxMessage;
use Illuminate\Database\QueryException;
use Illuminate\Support\Facades\Artisan;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Str;
use PHPUnit\Framework\Attributes\Group;
use Tests\Support\IssuesTokens;
use Tests\TestCase;

/**
 * Vazamento entre tenants contra um Postgres real (ADR 0002). Exige RECRUITMENT_PG_TESTS=1 e as conexões
 * `pgsql` (usuário da aplicação, sem BYPASSRLS) e `migrate` (dono do esquema); roda no CI.
 */
#[Group('integration')]
class TenantIsolationTest extends TestCase
{
    use IssuesTokens;

    private const A = '0197a000-0000-7000-8000-00000000000a';

    private const B = '0197a000-0000-7000-8000-00000000000b';

    protected function setUp(): void
    {
        parent::setUp();

        if (! getenv('RECRUITMENT_PG_TESTS')) {
            $this->markTestSkipped('Defina RECRUITMENT_PG_TESTS=1 com um Postgres disponível.');
        }

        config(['database.default' => 'pgsql']);
        Artisan::call('migrate:fresh', ['--database' => 'migrate', '--force' => true]);
        $this->trustTestKey();

        $tx = app(TenantTransaction::class);
        $tx->run(self::A, fn () => Job::create(['title' => 'Vaga A', 'unit' => '', 'status' => 'open', 'stages' => ['a', 'b']]));
        $tx->run(self::B, fn () => Job::create(['title' => 'Vaga B', 'unit' => '', 'status' => 'open', 'stages' => ['a', 'b']]));
    }

    public function test_consulta_sem_where_no_escopo_de_um_tenant_so_devolve_as_linhas_dele(): void
    {
        $titles = app(TenantTransaction::class)->run(self::A, fn () => DB::table('jobs')->pluck('title')->all());

        $this->assertSame(['Vaga A'], $titles);
    }

    public function test_sem_escopo_nenhuma_linha_e_visivel(): void
    {
        $this->assertSame(0, DB::table('jobs')->count());
    }

    public function test_escrita_em_linha_de_outro_tenant_e_bloqueada(): void
    {
        $tx = app(TenantTransaction::class);

        $this->expectException(QueryException::class);
        $tx->run(self::A, fn () => DB::table('jobs')->insert([
            'id' => (string) Str::uuid7(), 'tenant_id' => self::B, 'title' => 'Intrusa', 'stages' => '[]',
        ]));
    }

    public function test_update_e_delete_de_linhas_de_outro_tenant_nao_afetam_nada(): void
    {
        $affected = app(TenantTransaction::class)->run(self::A, fn () => [
            DB::table('jobs')->where('title', 'Vaga B')->update(['title' => 'Hackeada']),
            DB::table('jobs')->where('title', 'Vaga B')->delete(),
        ]);

        $this->assertSame([0, 0], $affected);
        $this->assertSame(['Vaga B'], app(TenantTransaction::class)->run(self::B, fn () => DB::table('jobs')->pluck('title')->all()));
    }

    public function test_o_escopo_nao_sobrevive_ao_fim_da_transacao_na_mesma_conexao(): void
    {
        app(TenantTransaction::class)->run(self::A, fn () => DB::table('jobs')->count());

        $this->assertSame(0, DB::table('jobs')->count());
    }

    public function test_a_api_nao_mistura_tenants(): void
    {
        $this->getJson('/v1/jobs', $this->bearer(self::A))->assertOk()
            ->assertJsonCount(1, 'items')->assertJsonPath('items.0.title', 'Vaga A');
        $jobB = app(TenantTransaction::class)->run(self::B, fn () => Job::query()->value('id'));

        $this->getJson("/v1/jobs/{$jobB}", $this->bearer(self::A))->assertNotFound();
    }

    public function test_outbox_e_isolada_e_o_relay_com_escopo_de_plataforma_ve_todos(): void
    {
        $tx = app(TenantTransaction::class);
        foreach ([self::A, self::B] as $tenant) {
            $tx->run($tenant, fn () => OutboxMessage::create([
                'tenant_id' => $tenant, 'aggregate_id' => (string) Str::uuid7(), 'event_type' => 'CandidatoContratado',
                'event_version' => 1, 'payload' => [], 'occurred_at' => now(),
            ]));
        }

        $this->assertSame(1, $tx->run(self::A, fn () => OutboxMessage::query()->count()));

        $published = [];
        $publisher = new class($published) implements EventPublisher
        {
            public function __construct(public array &$sent) {}

            public function publish(string $key, array $envelope, array $headers): void
            {
                $this->sent[] = [$key, $envelope['tenant_id'], $headers['event_type']];
            }
        };

        $this->assertSame(2, (new OutboxRelay($tx, $publisher))->drain(10));
        $this->assertEqualsCanonicalizing([[self::A, self::A, 'CandidatoContratado'], [self::B, self::B, 'CandidatoContratado']], $published);
        $this->assertSame(0, (new OutboxRelay($tx, $publisher))->drain(10));
    }
}
