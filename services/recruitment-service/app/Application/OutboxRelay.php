<?php

declare(strict_types=1);

namespace App\Application;

use App\Infrastructure\Messaging\EventEnvelope;
use App\Infrastructure\Tenancy\TenantTransaction;
use App\Models\OutboxMessage;

/** Publica a outbox no Kafka com entrega pelo menos uma vez; consumidores deduplicam por event_id. */
final class OutboxRelay
{
    public function __construct(
        private readonly TenantTransaction $transactions,
        private readonly EventPublisher $publisher,
    ) {}

    /** @return int quantidade de eventos publicados */
    public function drain(int $batchSize): int
    {
        return $this->transactions->runAsPlatform(function () use ($batchSize): int {
            $pending = OutboxMessage::query()
                ->whereNull('published_at')
                ->orderBy('occurred_at')->orderBy('id')
                ->limit($batchSize)
                ->lock('for update skip locked')
                ->get();

            foreach ($pending as $message) {
                $this->publisher->publish(
                    $message->tenant_id,
                    EventEnvelope::from($message),
                    ['event_type' => $message->event_type, 'event_version' => (string) $message->event_version],
                );
                $message->update(['published_at' => now()->utc()]);
            }

            return $pending->count();
        });
    }
}
