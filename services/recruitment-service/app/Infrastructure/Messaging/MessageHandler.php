<?php

declare(strict_types=1);

namespace App\Infrastructure\Messaging;

/** Trata o valor de uma mensagem Kafka. */
interface MessageHandler
{
    /** @return bool true se a mensagem alterou o estado; false se foi ignorada, repetida ou descartada */
    public function handle(string $raw): bool;
}
