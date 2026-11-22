<?php

declare(strict_types=1);

namespace App\Application;

interface EventPublisher
{
    /**
     * @param  array<string, mixed>  $envelope
     * @param  array<string, string>  $headers
     */
    public function publish(string $key, array $envelope, array $headers): void;
}
