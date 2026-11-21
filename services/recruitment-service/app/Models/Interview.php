<?php

declare(strict_types=1);

namespace App\Models;

use App\Infrastructure\Tenancy\BelongsToTenant;
use Illuminate\Database\Eloquent\Concerns\HasUuids;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\HasMany;

class Interview extends Model
{
    use BelongsToTenant;
    use HasUuids;

    protected $table = 'interviews';

    protected $guarded = [];

    protected function casts(): array
    {
        return ['scheduled_at' => 'datetime'];
    }

    public function scorecards(): HasMany
    {
        return $this->hasMany(Scorecard::class, 'interview_id');
    }
}
