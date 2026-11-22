<?php

declare(strict_types=1);

namespace App\Models;

use App\Domain\Pipeline;
use App\Infrastructure\Tenancy\BelongsToTenant;
use Illuminate\Database\Eloquent\Concerns\HasUuids;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\HasMany;

/**
 * @property string $id
 * @property string $title
 * @property string $unit
 * @property string $status
 * @property list<string> $stages
 */
class Job extends Model
{
    use BelongsToTenant;
    use HasUuids;

    protected $table = 'jobs';

    protected $guarded = [];

    protected function casts(): array
    {
        return ['stages' => 'array'];
    }

    public function pipeline(): Pipeline
    {
        return Pipeline::of($this->stages);
    }

    public function applications(): HasMany
    {
        return $this->hasMany(JobApplication::class, 'job_id');
    }
}
