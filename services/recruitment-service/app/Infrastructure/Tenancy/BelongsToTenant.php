<?php

declare(strict_types=1);

namespace App\Infrastructure\Tenancy;

use Illuminate\Database\Eloquent\Builder;
use Illuminate\Database\Eloquent\Model;

/** Camada de aplicação do isolamento; a camada de banco é o RLS (ADR 0002). */
trait BelongsToTenant
{
    public static function bootBelongsToTenant(): void
    {
        static::addGlobalScope('tenant', static function (Builder $query): void {
            $query->where($query->getModel()->getTable().'.tenant_id', app(TenantContext::class)->id());
        });

        static::creating(static function (Model $model): void {
            $model->setAttribute('tenant_id', app(TenantContext::class)->id());
        });
    }
}
