<?php

declare(strict_types=1);

namespace App\Application;

/** Porta para o tenant da operação corrente. */
interface TenantIdentity
{
    public function id(): string;
}
