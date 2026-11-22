<?php

declare(strict_types=1);

namespace App\Infrastructure\Messaging;

use App\Application\EventOutbox;
use App\Infrastructure\Tenancy\TenantContext;
use App\Models\OutboxMessage;
use Illuminate\Support\Str;

final class DatabaseOutbox implements EventOutbox
{
    public function __construct(private readonly TenantContext $tenant) {}

    public function record(string $eventType, string $aggregateId, array $data): void
    {
        OutboxMessage::create([
            'id' => (string) Str::uuid7(),
            'tenant_id' => $this->tenant->id(),
            'aggregate_id' => $aggregateId,
            'event_type' => $eventType,
            'event_version' => 1,
            'payload' => $data,
            'occurred_at' => now()->utc(),
        ]);
    }
}
