<?php

declare(strict_types=1);

namespace App\Infrastructure\Messaging;

use longlang\phpkafka\Consumer\Consumer;
use longlang\phpkafka\Consumer\ConsumerConfig;

/** Lê um tópico (tenants ou privacidade). O offset só avança depois de processar (pelo menos uma vez). */
final class KafkaTenantEventConsumer
{
    public function __construct(
        private readonly string $brokers,
        private readonly string $topic,
        private readonly string $groupId,
        private readonly MessageHandler $handler,
    ) {}

    public function run(): void
    {
        $consumer = new Consumer(new ConsumerConfig([
            'bootstrapServers' => $this->brokers,
            'groupId' => $this->groupId,
            'topic' => $this->topic,
            'autoCommit' => false,
            'interval' => 0.5,
        ]));

        while (true) {
            $message = $consumer->consume();
            if ($message === null) {
                usleep(500_000);

                continue;
            }
            $this->handler->handle((string) $message->getValue());
            $consumer->ack($message);
        }
    }
}
