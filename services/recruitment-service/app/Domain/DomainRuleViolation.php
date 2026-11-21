<?php

declare(strict_types=1);

namespace App\Domain;

use RuntimeException;

/** Regra de negócio violada: vira HTTP 422 (problem+json). */
class DomainRuleViolation extends RuntimeException {}
