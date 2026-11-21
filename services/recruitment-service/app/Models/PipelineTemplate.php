<?php

declare(strict_types=1);

namespace App\Models;

use App\Infrastructure\Tenancy\BelongsToTenant;
use Illuminate\Database\Eloquent\Model;

/** Modelo de etapas do tenant, usado quando a vaga não traz etapas próprias. */
class PipelineTemplate extends Model
{
    use BelongsToTenant;

    protected $table = 'pipeline_templates';

    protected $primaryKey = 'tenant_id';

    public $incrementing = false;

    protected $keyType = 'string';

    protected $guarded = [];

    protected function casts(): array
    {
        return ['stages' => 'array'];
    }
}
