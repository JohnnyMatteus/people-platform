<?php

declare(strict_types=1);

namespace App\Models;

use App\Infrastructure\Tenancy\BelongsToTenant;
use Illuminate\Database\Eloquent\Model;

/** Eventos já consumidos; a chave primária é o event_id, que garante a deduplicação. */
class ProcessedEvent extends Model
{
    use BelongsToTenant;

    protected $table = 'processed_events';

    protected $primaryKey = 'event_id';

    public $incrementing = false;

    protected $keyType = 'string';

    public $timestamps = false;

    protected $guarded = [];
}
