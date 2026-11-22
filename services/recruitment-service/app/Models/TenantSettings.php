<?php

declare(strict_types=1);

namespace App\Models;

use App\Infrastructure\Tenancy\BelongsToTenant;
use Illuminate\Database\Eloquent\Model;

class TenantSettings extends Model
{
    use BelongsToTenant;

    protected $table = 'tenant_settings';

    protected $primaryKey = 'tenant_id';

    public $incrementing = false;

    protected $keyType = 'string';

    protected $guarded = [];
}
