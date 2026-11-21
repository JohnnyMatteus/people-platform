<?php

declare(strict_types=1);

namespace App\Domain;

/** Etapas ordenadas de uma vaga. A última é sempre a de contratação. */
final class Pipeline
{
    public const DEFAULT_STAGES = ['triagem', 'entrevista-rh', 'entrevista-gestor', 'proposta', 'contratado'];

    /** @param list<string> $stages */
    private function __construct(private readonly array $stages) {}

    /** @param list<string> $stages */
    public static function of(array $stages): self
    {
        $stages = array_values(array_map(static fn ($s) => trim((string) $s), $stages));

        if (count($stages) < 2) {
            throw new DomainRuleViolation('O pipeline precisa de pelo menos duas etapas.');
        }
        if (count(array_unique($stages)) !== count($stages)) {
            throw new DomainRuleViolation('As etapas do pipeline não podem se repetir.');
        }
        foreach ($stages as $stage) {
            if (! preg_match('/^[a-z0-9]+(-[a-z0-9]+)*$/', $stage)) {
                throw new DomainRuleViolation("Etapa inválida: '{$stage}'. Use minúsculas, números e hífen.");
            }
        }

        return new self($stages);
    }

    public static function default(): self
    {
        return new self(self::DEFAULT_STAGES);
    }

    /** @return list<string> */
    public function stages(): array
    {
        return $this->stages;
    }

    public function first(): string
    {
        return $this->stages[0];
    }

    public function final(): string
    {
        return $this->stages[array_key_last($this->stages)];
    }

    public function has(string $stage): bool
    {
        return in_array($stage, $this->stages, true);
    }

    public function isFinal(string $stage): bool
    {
        return $stage === $this->final();
    }
}
