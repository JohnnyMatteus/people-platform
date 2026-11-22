<?php

declare(strict_types=1);

namespace App\Infrastructure\Messaging;

use App\Application\TenantProvisioner;
use Illuminate\Support\Facades\Log;
use JsonException;

/** Traduz a mensagem do tópico de tenants em ações do serviço. Mensagem fora do contrato é descartada. */
final class TenantEventHandler implements MessageHandler
{
    private const UUID = '/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i';

    public function __construct(private readonly TenantProvisioner $provisioner) {}

    /** @return bool true se o evento alterou o estado */
    public function handle(string $raw): bool
    {
        try {
            $envelope = json_decode($raw, true, 16, JSON_THROW_ON_ERROR);
        } catch (JsonException) {
            Log::warning('tenant-events: JSON inválido descartado');

            return false;
        }

        if (($envelope['event_type'] ?? null) !== 'TenantCriado' || ($envelope['event_version'] ?? null) !== 1) {
            return false;
        }
        if (! $this->valid($envelope)) {
            Log::warning('tenant-events: TenantCriado fora do contrato descartado');

            return false;
        }

        return $this->provisioner->provision($envelope);
    }

    /** @param array<string, mixed> $e */
    private function valid(array $e): bool
    {
        $d = $e['data'] ?? null;

        return is_array($d)
            && is_string($e['event_id'] ?? null) && preg_match(self::UUID, $e['event_id']) === 1
            && is_string($e['tenant_id'] ?? null) && preg_match(self::UUID, $e['tenant_id']) === 1
            && is_string($d['slug'] ?? null) && is_string($d['name'] ?? null) && is_string($d['plan'] ?? null)
            && is_array($d['features'] ?? null);
    }
}
