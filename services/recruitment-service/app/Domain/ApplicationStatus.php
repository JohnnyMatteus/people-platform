<?php

declare(strict_types=1);

namespace App\Domain;

enum ApplicationStatus: string
{
    case Active = 'active';
    case Hired = 'hired';
    case Rejected = 'rejected';

    public function isClosed(): bool
    {
        return $this !== self::Active;
    }
}
