<?php

declare(strict_types=1);

namespace App\Infrastructure\Messaging;

use App\Application\EventPublisher;
use longlang\phpkafka\Producer\Producer;
use longlang\phpkafka\Producer\ProducerConfig;

final class KafkaEventPublisher implements EventPublisher
{
    private ?Producer $producer = null;

    public function __construct(private readonly string $brokers, private readonly string $topic) {}

    public function publish(string $key, array $envelope, array $headers): void
    {
        $this->producer ??= new Producer(new ProducerConfig([
            'bootstrapServers' => $this->brokers,
            'acks' => -1,
            'autoCreateTopic' => false,
        ]));

        $this->producer->send($this->topic, json_encode($envelope, JSON_THROW_ON_ERROR), $key, $headers);
    }
}
