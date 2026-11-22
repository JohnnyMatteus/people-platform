<?php

declare(strict_types=1);

namespace App\Application;

/** Registra eventos na mesma transação da mudança de estado (transactional outbox). */
interface EventOutbox
{
    /** @param array<string, mixed> $data */
    public function record(string $eventType, string $aggregateId, array $data): void;
}
