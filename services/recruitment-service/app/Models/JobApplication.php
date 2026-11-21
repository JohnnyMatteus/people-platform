<?php

declare(strict_types=1);

namespace App\Models;

use App\Domain\ApplicationStatus;
use App\Infrastructure\Tenancy\BelongsToTenant;
use Illuminate\Database\Eloquent\Concerns\HasUuids;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;
use Illuminate\Database\Eloquent\Relations\HasMany;

/**
 * @property string $id
 * @property string $job_id
 * @property string $stage
 * @property ApplicationStatus $status
 * @property ?string $resume_key
 */
class JobApplication extends Model
{
    use BelongsToTenant;
    use HasUuids;

    protected $table = 'applications';

    protected $guarded = [];

    protected function casts(): array
    {
        return ['status' => ApplicationStatus::class, 'start_date' => 'date:Y-m-d', 'hired_at' => 'datetime'];
    }

    public function job(): BelongsTo
    {
        return $this->belongsTo(Job::class, 'job_id');
    }

    public function candidate(): BelongsTo
    {
        return $this->belongsTo(Candidate::class, 'candidate_id');
    }

    public function interviews(): HasMany
    {
        return $this->hasMany(Interview::class, 'application_id');
    }
}
