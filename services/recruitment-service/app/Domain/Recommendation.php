<?php

declare(strict_types=1);

namespace App\Domain;

enum Recommendation: string
{
    case StrongYes = 'strong_yes';
    case Yes = 'yes';
    case No = 'no';
    case StrongNo = 'strong_no';
}
