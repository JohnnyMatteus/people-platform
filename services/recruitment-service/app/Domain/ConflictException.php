<?php

declare(strict_types=1);

namespace App\Domain;

use RuntimeException;

/** Estado atual não permite a operação: vira HTTP 409. */
final class ConflictException extends RuntimeException {}
