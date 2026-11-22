<?php

declare(strict_types=1);

namespace App\Infrastructure\Messaging;

use App\Models\OutboxMessage;

/** Monta o envelope do ADR 0005 (contracts/asyncapi/schemas/envelope.v1.json). */
final class EventEnvelope
{
    public const PRODUCER = 'recruitment-service';

    /** @return array<string, mixed> */
    public static function from(OutboxMessage $message): array
    {
        return [
            'event_id' => $message->id,
            'event_type' => $message->event_type,
            'event_version' => (int) $message->event_version,
            'tenant_id' => $message->tenant_id,
            'aggregate_id' => $message->aggregate_id,
            'occurred_at' => $message->occurred_at->utc()->format('Y-m-d\TH:i:s.v\Z'),
            'producer' => self::PRODUCER,
            'data' => $message->payload,
        ];
    }
}
