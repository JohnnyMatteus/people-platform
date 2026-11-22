<?php

declare(strict_types=1);

namespace App\Infrastructure\Auth;

use Firebase\JWT\Key;

interface KeySetProvider
{
    /**
     * @param  bool  $refresh  ignora o cache (kid desconhecido pode ser rotação de chaves)
     * @return array<string, Key>
     */
    public function keys(bool $refresh = false): array;
}
