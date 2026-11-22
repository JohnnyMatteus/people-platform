<?php

namespace Tests\Feature;

use App\Infrastructure\Messaging\TenantEventHandler;
use App\Infrastructure\Tenancy\TenantTransaction;
use App\Models\PipelineTemplate;
use App\Models\TenantSettings;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Opis\JsonSchema\Validator;
use Tests\TestCase;

class TenantProvisioningTest extends TestCase
{
    use RefreshDatabase;

    private const TENANT = '0197a000-0000-7000-8000-00000000000c';

    public function test_tenant_criado_prepara_configuracao_e_pipeline_padrao_uma_unica_vez(): void
    {
        $event = $this->tenantCriado(['recruitment', 'performance']);
        $this->assertMatchesContract($event);
        $handler = $this->app->make(TenantEventHandler::class);

        $this->assertTrue($handler->handle(json_encode($event)));
        $this->assertFalse($handler->handle(json_encode($event)), 'evento duplicado');

        $this->inTenant(function () {
            $settings = TenantSettings::query()->sole();
            $this->assertSame('acme', $settings->slug);
            $this->assertTrue((bool) $settings->module_enabled);
            $this->assertSame(['triagem', 'entrevista-rh', 'entrevista-gestor', 'proposta', 'contratado'], PipelineTemplate::query()->sole()->stages);
        });
    }

    public function test_sem_a_feature_de_recrutamento_o_modulo_fica_desligado(): void
    {
        $this->app->make(TenantEventHandler::class)->handle(json_encode($this->tenantCriado(['performance'])));

        $this->inTenant(fn () => $this->assertFalse((bool) TenantSettings::query()->sole()->module_enabled));
    }

    public function test_mensagens_fora_do_contrato_ou_de_outro_tipo_sao_ignoradas(): void
    {
        $handler = $this->app->make(TenantEventHandler::class);
        $outro = $this->tenantCriado([]);
        $outro['event_type'] = 'TenantPlanoAlterado';
        $semSlug = $this->tenantCriado([]);
        unset($semSlug['data']['slug']);

        $this->assertFalse($handler->handle('{nao-json'));
        $this->assertFalse($handler->handle(json_encode($outro)));
        $this->assertFalse($handler->handle(json_encode($semSlug)));
    }

    private function inTenant(callable $fn): void
    {
        $this->app->make(TenantTransaction::class)->run(self::TENANT, $fn);
    }

    /**
     * @param  list<string>  $features
     * @return array<string, mixed>
     */
    private function tenantCriado(array $features): array
    {
        return [
            'event_id' => '0197a000-0000-7000-8000-0000000000e1',
            'event_type' => 'TenantCriado',
            'event_version' => 1,
            'tenant_id' => self::TENANT,
            'aggregate_id' => self::TENANT,
            'occurred_at' => '2026-11-22T10:00:00Z',
            'producer' => 'tenant-service',
            'data' => ['slug' => 'acme', 'name' => 'Acme', 'plan' => 'crescimento', 'features' => $features],
        ];
    }

    /** @param array<string, mixed> $envelope */
    private function assertMatchesContract(array $envelope): void
    {
        $dir = base_path('../../contracts/asyncapi/schemas');
        $validator = new Validator;
        $validator->resolver()->registerFile('https://people-platform.example.com/schemas/envelope.v1.json', "{$dir}/envelope.v1.json");
        $validator->resolver()->registerFile('https://people-platform.example.com/schemas/tenant-criado.v1.json', "{$dir}/tenant-criado.v1.json");
        $result = $validator->validate(json_decode(json_encode($envelope)), 'https://people-platform.example.com/schemas/tenant-criado.v1.json');

        $this->assertTrue($result->isValid(), json_encode($result->error()?->message()));
    }
}
