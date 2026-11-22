<?php

declare(strict_types=1);

namespace App\Application;

use App\Domain\Pipeline;
use App\Infrastructure\Tenancy\TenantTransaction;
use App\Models\PipelineTemplate;
use App\Models\ProcessedEvent;
use App\Models\TenantSettings;

/** Prepara os dados do tenant a partir de TenantCriado: configurações e pipeline padrão. Idempotente por event_id. */
final class TenantProvisioner
{
    public function __construct(private readonly TenantTransaction $transactions) {}

    /**
     * @param  array<string, mixed>  $envelope  evento já validado como TenantCriado v1
     * @return bool false se o evento já tinha sido processado
     */
    public function provision(array $envelope): bool
    {
        $tenantId = strtolower((string) $envelope['tenant_id']);
        $data = $envelope['data'];

        return (bool) $this->transactions->run($tenantId, function () use ($envelope, $data): bool {
            if (ProcessedEvent::query()->whereKey($envelope['event_id'])->exists()) {
                return false;
            }

            TenantSettings::query()->updateOrCreate([], [
                'slug' => $data['slug'],
                'name' => $data['name'],
                'plan' => $data['plan'],
                'module_enabled' => in_array('recruitment', $data['features'] ?? [], true),
            ]);

            if (! PipelineTemplate::query()->exists()) {
                PipelineTemplate::query()->create(['stages' => Pipeline::default()->stages()]);
            }

            ProcessedEvent::query()->create([
                'event_id' => $envelope['event_id'],
                'event_type' => $envelope['event_type'],
                'processed_at' => now()->utc(),
            ]);

            return true;
        });
    }
}
