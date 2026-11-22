<?php

namespace Tests\Support;

use App\Infrastructure\Auth\KeySetProvider;
use Firebase\JWT\Key;

final class FakeKeySetProvider implements KeySetProvider
{
    /** @param array<string, Key> $keys */
    public function __construct(private array $keys, public int $refreshes = 0) {}

    public function keys(bool $refresh = false): array
    {
        if ($refresh) {
            $this->refreshes++;
        }

        return $this->keys;
    }
}
