<?php

namespace Tests\Integration;

use App\Application\DataSubjectEraser;
use App\Application\EventPublisher;
use App\Application\OutboxRelay;
use App\Infrastructure\Messaging\PrivacyEventHandler;
use App\Infrastructure\Tenancy\TenantTransaction;
use App\Models\Candidate;
use App\Models\Interview;
use App\Models\Job;
use App\Models\JobApplication;
use App\Models\OutboxMessage;
use App\Models\Scorecard;
use Illuminate\Database\QueryException;
use Illuminate\Support\Facades\Artisan;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Str;
use PHPUnit\Framework\Attributes\Group;
use Tests\Support\IssuesTokens;
use Tests\Support\RecordingResumeStorage;
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

        $this->assertSame(2, (new OutboxRelay($tx, $publisher))->drain(10)->published);
        $this->assertEqualsCanonicalizing([[self::A, self::A, 'CandidatoContratado'], [self::B, self::B, 'CandidatoContratado']], $published);
        $this->assertSame(0, (new OutboxRelay($tx, $publisher))->drain(10)->published);
    }

    public function test_relay_com_mensagem_recusada_isola_o_tenant_e_so_a_linha_recusada_conta_tentativa(): void
    {
        $tx = app(TenantTransaction::class);
        $enqueue = function (string $tenant, int $count, string $start) use ($tx): array {
            $ids = [];
            for ($i = 0; $i < $count; $i++) {
                $ids[] = $tx->run($tenant, fn () => OutboxMessage::create([
                    'tenant_id' => $tenant, 'aggregate_id' => (string) Str::uuid7(), 'event_type' => 'CandidatoContratado',
                    'event_version' => 1, 'payload' => [], 'occurred_at' => date('Y-m-d H:i:s', strtotime($start) + $i),
                ])->id);
            }

            return $ids;
        };
        // A é a mais antiga e tem mais pendentes que o lote (5): sozinha, esgotaria todas as buscas.
        $a = $enqueue(self::A, 7, '2026-11-22 09:00:00');
        $b = $enqueue(self::B, 4, '2026-11-22 10:00:00');
        $reject = [$a[1], $b[2]];

        $sent = [];
        $publisher = new class($sent, $reject) implements EventPublisher
        {
            public function __construct(public array &$sent, public array &$reject) {}

            public function publish(string $key, array $envelope, array $headers): void
            {
                if (in_array($envelope['event_id'], $this->reject, true)) {
                    throw new \RuntimeException('broker recusou');
                }
                $this->sent[] = $envelope['event_id'];
            }
        };
        $relay = new OutboxRelay($tx, $publisher);

        $result = $relay->drain(5);

        $this->assertSame(3, $result->published);
        $this->assertSame(2, $result->failed);
        $this->assertSame([$a[0], $b[0], $b[1]], $sent);

        // Com o papel da aplicação (RLS), cada tenant vê as próprias linhas: attempts sobe só na recusada.
        foreach ([[self::A, $a], [self::B, $b]] as [$tenant, $ids]) {
            $rows = $tx->run($tenant, fn () => OutboxMessage::query()->whereIn('id', $ids)->get()->keyBy('id'));
            foreach ($ids as $id) {
                $this->assertSame(in_array($id, $reject, true) ? 1 : 0, $rows[$id]->attempts, "attempts de {$id}");
            }
        }
        $recusadaA = $tx->run(self::A, fn () => OutboxMessage::find($a[1]));
        $this->assertNull($recusadaA->published_at);
        $this->assertStringContainsString('broker recusou', $recusadaA->last_error);
        $this->assertNull($tx->run(self::A, fn () => OutboxMessage::find($a[2]))->published_at, 'ordem por tenant');
        $this->assertNull($tx->run(self::B, fn () => OutboxMessage::find($b[3]))->published_at, 'ordem por tenant');

        $reject = [];
        $sent = [];
        do {
            $published = $relay->drain(5)->published;
        } while ($published > 0);
        $this->assertSame([...array_slice($a, 1), ...array_slice($b, 2)], $this->byTenantOrder($sent, $a, $b));
    }

    /**
     * @param  list<string>  $sent
     * @return list<string> os ids de A e depois os de B, cada tenant na ordem em que saiu
     */
    private function byTenantOrder(array $sent, array $a, array $b): array
    {
        return [...array_values(array_filter($sent, fn ($id) => in_array($id, $a, true))), ...array_values(array_filter($sent, fn ($id) => in_array($id, $b, true)))];
    }

    /** @return array{candidate: string, application: string, interview: string, key: string} */
    private function seedCandidate(string $tenant, string $email, string $evaluator): array
    {
        $tx = app(TenantTransaction::class);

        return $tx->run($tenant, function () use ($tenant, $email, $evaluator): array {
            $job = Job::first();
            $candidate = Candidate::create(['name' => 'Ana Souza', 'email' => $email]);
            $application = JobApplication::create([
                'job_id' => $job->id, 'candidate_id' => $candidate->id, 'stage' => 'triagem', 'status' => 'active',
            ]);
            $key = "tenants/{$tenant}/applications/{$application->id}/resume.pdf";
            $application->update(['resume_key' => $key]);
            $interview = Interview::create([
                'application_id' => $application->id, 'kind' => 'entrevista-rh', 'interviewer_id' => $evaluator,
                'scheduled_at' => '2026-12-01 10:00:00',
            ]);
            Scorecard::create([
                'interview_id' => $interview->id, 'evaluator_id' => $evaluator, 'recommendation' => 'yes',
                'ratings' => [['criterion' => 'Aderência', 'score' => 4]],
            ]);

            return ['candidate' => $candidate->id, 'application' => $application->id, 'interview' => $interview->id, 'key' => $key];
        });
    }

    public function test_exclusao_de_titular_no_postgres_real_com_o_papel_da_aplicacao(): void
    {
        $tx = app(TenantTransaction::class);
        $evaluator = (string) Str::uuid7();
        $acme = $this->seedCandidate(self::A, 'ana@example.com', $evaluator);
        $outro = $this->seedCandidate(self::A, 'bia@example.com', $evaluator);
        $sol = $this->seedCandidate(self::B, 'ana@example.com', $evaluator);

        $storage = new RecordingResumeStorage;
        $handler = new PrivacyEventHandler(new DataSubjectEraser($tx, $storage));
        $event = [
            'event_id' => (string) Str::uuid7(), 'event_type' => 'TitularExclusaoSolicitada', 'event_version' => 1,
            'tenant_id' => self::A, 'aggregate_id' => $acme['candidate'], 'occurred_at' => '2026-11-22T12:00:00Z',
            'producer' => 'tenant-service',
            'data' => ['request_id' => (string) Str::uuid7(), 'subject_id' => $acme['candidate']],
        ];

        $this->assertTrue($handler->handle(json_encode($event)));

        $count = fn (string $tenant, string $table, string $column, string $id) => $tx->run(
            $tenant,
            fn () => DB::table($table)->where($column, $id)->count(),
        );
        $this->assertSame(0, $count(self::A, 'candidates', 'id', $acme['candidate']));
        $this->assertSame(0, $count(self::A, 'applications', 'id', $acme['application']));
        $this->assertSame(0, $count(self::A, 'scorecards', 'interview_id', $acme['interview']));
        $this->assertSame([$acme['key']], $storage->deleted);
        // Outro titular do mesmo tenant e o mesmo e-mail no outro tenant ficam intactos.
        $this->assertSame(1, $count(self::A, 'candidates', 'id', $outro['candidate']));
        $this->assertSame(1, $count(self::B, 'candidates', 'id', $sol['candidate']));
        $this->assertSame(1, $count(self::B, 'scorecards', 'interview_id', $sol['interview']));
        // Lápide só com identificadores, só no tenant do pedido.
        $this->assertSame(1, $count(self::A, 'erased_subjects', 'subject_id', $acme['candidate']));
        $this->assertSame(0, $count(self::B, 'erased_subjects', 'subject_id', $acme['candidate']));

        $this->assertFalse($handler->handle(json_encode($event)), 'reentrega do mesmo event_id é no-op');
        $this->assertSame([$acme['key']], $storage->deleted);

        // A lápide é imutável para o papel da aplicação (só SELECT e INSERT): ele não consegue desfazer a exclusão.
        try {
            $tx->run(self::A, fn () => DB::table('erased_subjects')->delete());
            $this->fail('o papel da aplicação não pode apagar a lápide');
        } catch (QueryException $e) {
            $this->assertStringContainsString('permission denied', $e->getMessage());
        }
        $this->assertSame(1, $count(self::A, 'erased_subjects', 'subject_id', $acme['candidate']));
    }
}
