<?php

declare(strict_types=1);

namespace App\Models;

use Illuminate\Database\Eloquent\Concerns\HasUuids;
use Illuminate\Database\Eloquent\Model;

/** Sem escopo de tenant: o relay roda com escopo de plataforma (ADR 0002). */
class OutboxMessage extends Model
{
    use HasUuids;

    protected $table = 'outbox';

    public $timestamps = false;

    protected $guarded = [];

    protected function casts(): array
    {
        return ['payload' => 'array', 'occurred_at' => 'datetime', 'published_at' => 'datetime'];
    }
}
