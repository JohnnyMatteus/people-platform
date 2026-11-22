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
     * Depois de uma falha, o tenant fica bloqueado nesta rodada (preserva a ordem por chave) e é excluído da busca
     * seguinte, de modo que um tenant com mensagem envenenada não esgota o lote e os demais continuam publicando.
     */
    public function drain(int $batchSize): DrainResult
    {
        return $this->transactions->runAsPlatform(function () use ($batchSize): DrainResult {
            $published = 0;
            $failed = 0;
            $blockedTenants = [];

            while ($published + $failed < $batchSize) {
                $pending = OutboxMessage::query()
                    ->whereNull('published_at')
                    ->whereNotIn('tenant_id', array_keys($blockedTenants))
                    ->orderBy('occurred_at')->orderBy('id')
                    ->limit($batchSize - $published - $failed)
                    ->lock('for update skip locked')
                    ->get();

                if ($pending->isEmpty()) {
                    break;
                }

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

                    $message->update(['published_at' => now()->utc(), 'last_error' => null]);
                    $published++;
                }
            }

            return new DrainResult($published, $failed);
        });
    }
}
