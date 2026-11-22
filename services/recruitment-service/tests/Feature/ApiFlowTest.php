<?php

namespace Tests\Feature;

use App\Infrastructure\Messaging\EventEnvelope;
use App\Models\OutboxMessage;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Opis\JsonSchema\Validator;
use Tests\Support\IssuesTokens;
use Tests\TestCase;

class ApiFlowTest extends TestCase
{
    use IssuesTokens;
    use RefreshDatabase;

    private const ACME = '0197a000-0000-7000-8000-00000000000a';

    private const SOL = '0197a000-0000-7000-8000-00000000000b';

    protected function setUp(): void
    {
        parent::setUp();
        $this->trustTestKey();
    }

    public function test_sem_token_ou_com_papel_insuficiente_a_api_recusa(): void
    {
        $this->getJson('/v1/jobs')->assertUnauthorized()->assertHeader('WWW-Authenticate', 'Bearer');
        $this->getJson('/v1/jobs', $this->bearer(self::ACME, ['employee']))->assertForbidden();
    }

    public function test_pipeline_do_tenant_vira_o_padrao_das_vagas_novas(): void
    {
        $h = $this->bearer(self::ACME);

        $this->getJson('/v1/pipeline-template', $h)->assertOk()->assertJsonPath('stages.4', 'contratado');
        $this->putJson('/v1/pipeline-template', ['stages' => ['triagem', 'teste-tecnico', 'admitido']], $h)->assertOk();

        $this->postJson('/v1/jobs', ['title' => 'Engenheiro Civil', 'unit' => 'Obra Norte'], $h)
            ->assertCreated()->assertJsonPath('stages', ['triagem', 'teste-tecnico', 'admitido']);

        $this->getJson('/v1/pipeline-template', $this->bearer(self::SOL))->assertJsonPath('stages.0', 'triagem')
            ->assertJsonCount(5, 'stages');
    }

    public function test_fluxo_completo_da_candidatura_a_contratacao_publica_o_evento_conforme_o_contrato(): void
    {
        $h = $this->bearer(self::ACME);
        $job = $this->postJson('/v1/jobs', ['title' => 'Mestre de Obras', 'unit' => 'Obra Norte', 'stages' => ['triagem', 'entrevista', 'contratado']], $h)
            ->assertCreated()->json();

        $applied = $this->postJson("/v1/jobs/{$job['id']}/applications", ['name' => 'Ana Souza', 'email' => 'Ana@example.com'], $h)
            ->assertCreated()
            ->assertJsonPath('stage', 'triagem')
            ->assertJsonPath('candidate.email', 'ana@example.com')
            ->json();
        $this->assertStringContainsString('tenants/'.self::ACME.'/applications/'.$applied['id'].'/resume.pdf', $applied['resume_upload']['url']);
        $this->postJson("/v1/jobs/{$job['id']}/applications", ['name' => 'Ana', 'email' => 'ana@example.com'], $h)->assertConflict();

        $this->putJson("/v1/applications/{$applied['id']}/stage", ['stage' => 'entrevista'], $h)->assertOk()->assertJsonPath('stage', 'entrevista');
        $this->putJson("/v1/applications/{$applied['id']}/stage", ['stage' => 'inexistente'], $h)->assertUnprocessable();

        $interview = $this->postJson("/v1/applications/{$applied['id']}/interviews", ['kind' => 'gestor', 'scheduled_at' => '2026-12-01T14:00:00Z'], $h)
            ->assertCreated()->json();
        $scorecard = ['recommendation' => 'strong_yes', 'ratings' => [['criterion' => 'comunicação', 'score' => 5]], 'notes' => 'Ótima entrevista.'];
        $this->postJson("/v1/interviews/{$interview['id']}/scorecards", $scorecard, $h)->assertCreated();
        $this->postJson("/v1/interviews/{$interview['id']}/scorecards", $scorecard, $h)->assertConflict();
        $this->postJson("/v1/interviews/{$interview['id']}/scorecards", [...$scorecard, 'ratings' => [['criterion' => 'x', 'score' => 9]]], $h)->assertUnprocessable();

        $this->getJson("/v1/applications/{$applied['id']}", $h)->assertJsonPath('interviews.0.scorecards.0.recommendation', 'strong_yes');

        $this->putJson("/v1/applications/{$applied['id']}/stage", ['stage' => 'contratado'], $h)->assertUnprocessable();
        $this->putJson("/v1/applications/{$applied['id']}/stage", ['stage' => 'contratado', 'start_date' => '2027-01-10'], $h)
            ->assertOk()->assertJsonPath('status', 'hired');
        $this->putJson("/v1/applications/{$applied['id']}/stage", ['stage' => 'triagem'], $h)->assertConflict();

        $event = OutboxMessage::query()->sole();
        $this->assertSame(self::ACME, $event->tenant_id);
        $this->assertEventMatchesContract(EventEnvelope::from($event), 'candidato-contratado');
        $this->assertSame('2027-01-10', $event->payload['start_date']);
        $this->assertSame('Mestre de Obras', $event->payload['position_title']);
    }

    public function test_mover_para_etapa_intermediaria_e_rejeitar_nao_publicam_evento(): void
    {
        $h = $this->bearer(self::ACME);
        $job = $this->postJson('/v1/jobs', ['title' => 'Analista'], $h)->json();
        $app = $this->postJson("/v1/jobs/{$job['id']}/applications", ['name' => 'Bia', 'email' => 'bia@example.com'], $h)->json();

        $this->putJson("/v1/applications/{$app['id']}/stage", ['stage' => 'proposta'], $h)->assertOk();
        $this->postJson("/v1/applications/{$app['id']}/reject", [], $h)->assertOk()->assertJsonPath('status', 'rejected');

        $this->assertSame(0, OutboxMessage::query()->count());
    }

    public function test_um_tenant_nao_enxerga_nem_altera_dados_de_outro(): void
    {
        $acme = $this->bearer(self::ACME);
        $sol = $this->bearer(self::SOL);
        $job = $this->postJson('/v1/jobs', ['title' => 'Vaga da Acme'], $acme)->json();
        $app = $this->postJson("/v1/jobs/{$job['id']}/applications", ['name' => 'Caio', 'email' => 'caio@example.com'], $acme)->json();

        $this->getJson('/v1/jobs', $sol)->assertOk()->assertJsonCount(0, 'items');
        $this->getJson("/v1/jobs/{$job['id']}", $sol)->assertNotFound();
        $this->getJson("/v1/jobs/{$job['id']}/applications", $sol)->assertNotFound();
        $this->getJson("/v1/applications/{$app['id']}", $sol)->assertNotFound();
        $this->putJson("/v1/applications/{$app['id']}/stage", ['stage' => 'proposta'], $sol)->assertNotFound();
        $this->getJson("/v1/applications/{$app['id']}/resume", $sol)->assertNotFound();
        $this->getJson("/v1/applications/{$app['id']}/resume", $acme)->assertOk()->assertJsonStructure(['url']);
    }

    public function test_listagem_de_vagas_pagina_por_cursor(): void
    {
        $h = $this->bearer(self::ACME);
        foreach (range(1, 3) as $i) {
            $this->postJson('/v1/jobs', ['title' => "Vaga {$i}"], $h)->assertCreated();
        }

        $first = $this->getJson('/v1/jobs', $h)->assertOk()->json();
        $this->assertCount(3, $first['items']);
        $this->assertNull($first['next_cursor']);
    }

    /** @param array<string, mixed> $envelope */
    private function assertEventMatchesContract(array $envelope, string $schema): void
    {
        $dir = base_path('../../contracts/asyncapi/schemas');
        $validator = new Validator;
        $validator->resolver()->registerFile('https://people-platform.example.com/schemas/envelope.v1.json', "{$dir}/envelope.v1.json");
        $validator->resolver()->registerFile("https://people-platform.example.com/schemas/{$schema}.v1.json", "{$dir}/{$schema}.v1.json");

        $result = $validator->validate(json_decode(json_encode($envelope)), "https://people-platform.example.com/schemas/{$schema}.v1.json");

        $this->assertTrue($result->isValid(), json_encode($result->error()?->message()));
    }
}
