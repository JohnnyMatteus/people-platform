<?php

declare(strict_types=1);

namespace App\Infrastructure\Tenancy;

use Closure;
use Illuminate\Support\Facades\DB;

/** Executa fn numa transação com o escopo de RLS do tenant (equivale a SET LOCAL). */
final class TenantTransaction
{
    public function __construct(private readonly TenantContext $context) {}

    public function run(string $tenantId, Closure $fn): mixed
    {
        $this->context->set($tenantId);

        try {
            return DB::transaction(function () use ($tenantId, $fn) {
                $this->scope('app.tenant_id', $tenantId);

                return $fn();
            });
        } finally {
            $this->context->clear();
        }
    }

    /** Escopo de plataforma, só para o relay da outbox (ADR 0002). */
    public function runAsPlatform(Closure $fn): mixed
    {
        return DB::transaction(function () use ($fn) {
            $this->scope('app.platform', 'on');

            return $fn();
        });
    }

    private function scope(string $name, string $value): void
    {
        if (DB::getDriverName() === 'pgsql') {
            DB::select('SELECT set_config(?, ?, true)', [$name, $value]);
        }
    }
}
