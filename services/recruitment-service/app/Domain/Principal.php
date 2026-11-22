<?php

declare(strict_types=1);

namespace App\Domain;

/** Quem está chamando, extraído do JWT validado. O tenant nunca vem do corpo da requisição. */
final readonly class Principal
{
    /** @param list<string> $roles */
    public function __construct(
        public string $userId,
        public string $tenantId,
        public array $roles,
    ) {}

    public function hasAnyRole(string ...$roles): bool
    {
        return array_intersect($roles, $this->roles) !== [];
    }
}
