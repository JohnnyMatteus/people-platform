<?php

declare(strict_types=1);

namespace App\Application;

use App\Infrastructure\Messaging\EventEnvelope;
use App\Infrastructure\Tenancy\TenantTransaction;
use App\Models\OutboxMessage;
use Throwable;

/** Publica a outbox no Kafka com entrega pelo menos uma vez; consumidores deduplicam por event_id. */
final class OutboxRelay
{
    private const MAX_ERROR_LENGTH = 1000;

    public function __construct(
        private readonly TenantTransaction $transactions,
        private readonly EventPublisher $publisher,
    ) {}

    /**
     * A falha de publicação nunca sai da transação: ela é gravada na linha (attempts, last_error) e a transação
     * confirma o que já foi publicado. Se o erro subisse, o rollback desfaria os published_at e o evento sairia de novo.
     * Depois de uma falha, as demais mensagens do mesmo tenant ficam para a próxima rodada, para preservar a ordem
     * por chave.
     */
    public function drain(int $batchSize): DrainResult
    {
        return $this->transactions->runAsPlatform(function () use ($batchSize): DrainResult {
            $pending = OutboxMessage::query()
                ->whereNull('published_at')
                ->orderBy('occurred_at')->orderBy('id')
                ->limit($batchSize)
                ->lock('for update skip locked')
                ->get();

            $published = 0;
            $failed = 0;
            $blockedTenants = [];

            foreach ($pending as $message) {
                if (isset($blockedTenants[$message->tenant_id])) {
                    continue;
                }

                try {
                    $this->publisher->publish(
                        $message->tenant_id,
                        EventEnvelope::from($message),
                        ['event_type' => $message->event_type, 'event_version' => (string) $message->event_version],
                    );
                } catch (Throwable $e) {
                    $message->update([
                        'attempts' => $message->attempts + 1,
                        'last_error' => mb_substr($e::class.': '.$e->getMessage(), 0, self::MAX_ERROR_LENGTH),
                    ]);
                    $blockedTenants[$message->tenant_id] = true;
                    $failed++;

                    continue;
                }

                $message->update(['published_at' => now()->utc(), 'attempts' => $message->attempts + 1, 'last_error' => null]);
                $published++;
            }

            return new DrainResult($published, $failed);
        });
    }
}
