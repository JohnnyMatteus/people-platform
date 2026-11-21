<?php

declare(strict_types=1);

namespace App\Infrastructure\Tenancy;

use App\Application\TenantIdentity;
use LogicException;

/** Tenant da requisição corrente. Sem tenant definido, qualquer consulta falha (falha fechada). */
final class TenantContext implements TenantIdentity
{
    private ?string $tenantId = null;

    public function set(string $tenantId): void
    {
        $this->tenantId = $tenantId;
    }

    public function id(): string
    {
        return $this->tenantId ?? throw new LogicException('Tenant não definido para esta operação.');
    }

    public function clear(): void
    {
        $this->tenantId = null;
    }
}
