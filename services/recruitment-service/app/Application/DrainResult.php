<?php

declare(strict_types=1);

namespace App\Application;

final class DrainResult
{
    public function __construct(public readonly int $published, public readonly int $failed) {}
}
