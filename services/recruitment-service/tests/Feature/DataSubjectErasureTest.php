<?php

namespace Tests\Feature;

use App\Application\ResumeStorage;
use App\Infrastructure\Messaging\PrivacyEventHandler;
use App\Models\Candidate;
use App\Models\Interview;
use App\Models\JobApplication;
use App\Models\Scorecard;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Str;
use RuntimeException;
use Tests\Support\IssuesTokens;
use Tests\Support\RecordingResumeStorage;
use Tests\TestCase;

class DataSubjectErasureTest extends TestCase
{
    use IssuesTokens;
    use RefreshDatabase;

    private const ACME = '0197a000-0000-7000-8000-00000000000a';

    private const SOL = '0197a000-0000-7000-8000-00000000000b';

    private RecordingResumeStorage $storage;

    protected function setUp(): void
    {
        parent::setUp();
        $this->trustTestKey();
        $this->storage = new RecordingResumeStorage;
        $this->app->instance(ResumeStorage::class, $this->storage);
    }

    /** @return array{candidate: string, application: string, interview: string, key: string} */
    private function candidateWithInterview(string $tenant, string $email, string $evaluator): array
    {
        $h = $this->bearer($tenant);
        $job = $this->postJson('/v1/jobs', ['title' => 'Mestre de Obras', 'unit' => 'Obra Norte', 'stages' => ['triagem', 'contratado']], $h)->json();
        $application = $this->postJson("/v1/jobs/{$job['id']}/applications", ['name' => 'Ana Souza', 'email' => $email], $h)->assertCreated()->json();
        $interview = $this->postJson("/v1/applications/{$application['id']}/interviews", [
            'kind' => 'entrevista-rh', 'scheduled_at' => '2026-12-01T10:00:00Z', 'interviewer_id' => $evaluator,
        ], $h)->assertCreated()->json();
        $this->postJson("/v1/interviews/{$interview['id']}/scorecards", [
            'recommendation' => 'yes', 'ratings' => [['criterion' => 'Aderência', 'score' => 4]],
        ], $this->bearer($tenant, ['manager'], $evaluator))->assertCreated();

        return [
            'candidate' => $application['candidate']['id'],
            'application' => $application['id'],
            'interview' => $interview['id'],
            'key' => "tenants/{$tenant}/applications/{$application['id']}/resume.pdf",
        ];
    }

    /** @return array<string, mixed> */
    private function event(string $tenant, string $subject, ?string $eventId = null): array
    {
        return [
            'event_id' => $eventId ?? (string) Str::uuid7(), 'event_type' => 'TitularExclusaoSolicitada', 'event_version' => 1,
            'tenant_id' => $tenant, 'aggregate_id' => $subject, 'occurred_at' => '2026-11-22T12:00:00Z', 'producer' => 'tenant-service',
            'data' => ['request_id' => (string) Str::uuid7(), 'subject_id' => $subject],
        ];
    }

    private function handle(array $event): bool
    {
        return $this->app->make(PrivacyEventHandler::class)->handle(json_encode($event));
    }

    public function test_apagar_um_candidato_remove_cadastro_candidaturas_entrevistas_scorecards_e_o_curriculo(): void
    {
        $evaluator = (string) Str::uuid7();
        $acme = $this->candidateWithInterview(self::ACME, 'ana@example.com', $evaluator);
        $sol = $this->candidateWithInterview(self::SOL, 'ana@example.com', $evaluator);

        $this->assertTrue($this->handle($this->event(self::ACME, $acme['candidate'])));

        $this->assertSame([$acme['key']], $this->storage->deleted);
        $this->assertNull(DB::table('candidates')->where('id', $acme['candidate'])->first());
        $this->assertNull(DB::table('applications')->where('id', $acme['application'])->first());
        $this->assertNull(DB::table('interviews')->where('id', $acme['interview'])->first());
        $this->assertSame(0, DB::table('scorecards')->where('interview_id', $acme['interview'])->count());
        // O outro tenant, com a mesma pessoa por e-mail, não é tocado.
        $this->assertNotNull(DB::table('candidates')->where('id', $sol['candidate'])->first());
        $this->assertSame(1, DB::table('scorecards')->where('interview_id', $sol['interview'])->count());
    }

    public function test_evento_de_um_tenant_nao_alcanca_o_titular_de_outro(): void
    {
        $sol = $this->candidateWithInterview(self::SOL, 'bia@example.com', (string) Str::uuid7());

        $this->assertTrue($this->handle($this->event(self::ACME, $sol['candidate'])));

        $this->assertSame([], $this->storage->deleted);
        $this->assertNotNull(DB::table('candidates')->where('id', $sol['candidate'])->first());
    }

    public function test_reentrega_nao_tem_efeito_e_falha_do_s3_nao_marca_o_evento_como_processado(): void
    {
        $acme = $this->candidateWithInterview(self::ACME, 'ana@example.com', (string) Str::uuid7());
        $event = $this->event(self::ACME, $acme['candidate']);

        $this->storage->failing = true;
        try {
            $this->handle($event);
            $this->fail('a falha do S3 deveria subir para o consumidor repetir');
        } catch (RuntimeException) {
            $this->assertNotNull(DB::table('candidates')->where('id', $acme['candidate'])->first(), 'nada foi apagado do banco');
        }

        $this->storage->failing = false;
        $this->assertTrue($this->handle($event));
        $this->assertFalse($this->handle($event), 'reentrega do mesmo event_id');
        $this->assertSame([$acme['key']], $this->storage->deleted);
    }

    public function test_colaborador_que_entrevistou_perde_a_ligacao_e_a_candidatura_de_outra_pessoa_fica(): void
    {
        $evaluator = (string) Str::uuid7();
        $acme = $this->candidateWithInterview(self::ACME, 'ana@example.com', $evaluator);

        $this->assertTrue($this->handle($this->event(self::ACME, $evaluator)));

        $this->assertNotNull(DB::table('candidates')->where('id', $acme['candidate'])->first());
        $this->assertNull(Interview::withoutGlobalScopes()->find($acme['interview'])->interviewer_id);
        $this->assertSame(0, Scorecard::withoutGlobalScopes()->where('evaluator_id', $evaluator)->count());
        $this->assertSame(1, Scorecard::withoutGlobalScopes()->where('interview_id', $acme['interview'])->count());
    }

    public function test_mensagens_fora_do_contrato_sao_descartadas(): void
    {
        $handler = $this->app->make(PrivacyEventHandler::class);
        $semSujeito = $this->event(self::ACME, (string) Str::uuid7());
        $semSujeito['data'] = ['request_id' => (string) Str::uuid7()];

        $this->assertFalse($handler->handle('{nao-json'));
        $this->assertFalse($handler->handle(json_encode($semSujeito)));
        $this->assertFalse($handler->handle(json_encode([...$this->event(self::ACME, (string) Str::uuid7()), 'event_type' => 'TenantCriado'])));
        $this->assertSame([], $this->storage->deleted);
        $this->assertSame(0, Candidate::withoutGlobalScopes()->count() + JobApplication::withoutGlobalScopes()->count());
    }
}
