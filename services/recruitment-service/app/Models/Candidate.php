<?php

declare(strict_types=1);

namespace App\Models;

use App\Infrastructure\Tenancy\BelongsToTenant;
use Illuminate\Database\Eloquent\Concerns\HasUuids;
use Illuminate\Database\Eloquent\Model;

class Candidate extends Model
{
    use BelongsToTenant;
    use HasUuids;

    protected $table = 'candidates';

    protected $guarded = [];
}
