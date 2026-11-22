<?php

namespace Tests\Unit;

use App\Domain\DomainRuleViolation;
use App\Domain\Pipeline;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

class PipelineTest extends TestCase
{
    public function test_a_etapa_final_e_a_ultima(): void
    {
        $pipeline = Pipeline::of(['triagem', 'entrevista', 'contratado']);

        $this->assertSame('triagem', $pipeline->first());
        $this->assertTrue($pipeline->isFinal('contratado'));
        $this->assertFalse($pipeline->isFinal('entrevista'));
        $this->assertFalse($pipeline->has('proposta'));
    }

    public function test_o_modelo_padrao_termina_em_contratado(): void
    {
        $this->assertSame('contratado', Pipeline::default()->final());
    }

    /** @param list<string> $stages */
    #[DataProvider('invalidPipelines')]
    public function test_pipelines_invalidos_sao_recusados(array $stages): void
    {
        $this->expectException(DomainRuleViolation::class);

        Pipeline::of($stages);
    }

    /** @return array<string, array{list<string>}> */
    public static function invalidPipelines(): array
    {
        return [
            'uma etapa só' => [['contratado']],
            'repetida' => [['triagem', 'triagem']],
            'com espaço e maiúscula' => [['Triagem Inicial', 'contratado']],
        ];
    }
}
