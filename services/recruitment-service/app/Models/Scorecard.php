<?php

declare(strict_types=1);

namespace App\Models;

use App\Domain\Recommendation;
use App\Infrastructure\Tenancy\BelongsToTenant;
use Illuminate\Database\Eloquent\Concerns\HasUuids;
use Illuminate\Database\Eloquent\Model;

class Scorecard extends Model
{
    use BelongsToTenant;
    use HasUuids;

    protected $table = 'scorecards';

    protected $guarded = [];

    protected function casts(): array
    {
        return ['ratings' => 'array', 'recommendation' => Recommendation::class];
    }
}
