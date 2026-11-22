package com.peopleplatform.performance.domain.evaluation

import com.peopleplatform.performance.domain.shared.DomainException
import java.math.BigDecimal
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith

class EvaluationTest {
    private fun scores(vararg pairs: Pair<String, Int>) = pairs.map { CompetencyScore(it.first, it.second) }

    @Test
    fun `desempenho consolidado pesa 70 por cento o gestor e 30 por cento a pessoa`() {
        val result = EvaluationConsolidator.consolidate(
            selfPerformance = BigDecimal("5"),
            selfCompetencies = emptyList(),
            managerPerformance = BigDecimal("3"),
            managerPotential = BigDecimal("4.2"),
            managerCompetencies = emptyList(),
        )
        assertEquals(BigDecimal("3.60"), result.performance)
        assertEquals(BigDecimal("4.2"), result.potential)
    }

    @Test
    fun `gap e a diferenca absoluta e competencia avaliada so por um lado nao gera gap`() {
        val result = EvaluationConsolidator.consolidate(
            BigDecimal("4"), scores("Comunicação" to 5, "Entrega" to 3, "Só da pessoa" to 4),
            BigDecimal("4"), BigDecimal("4"), scores("Comunicação" to 2, "Entrega" to 4, "Só do gestor" to 1),
        )
        assertEquals(listOf(CompetencyGap("Comunicação", 3), CompetencyGap("Entrega", 1)), result.gaps)
    }

    @Test
    fun `nota de competencia fora de 1 a 5 e recusada`() {
        assertFailsWith<DomainException> { CompetencyScore("Entrega", 6) }
        assertFailsWith<DomainException> { CompetencyScore(" ", 3) }
    }

    @Test
    fun `ciclo so avanca para o proximo estado`() {
        assertEquals(CycleStatus.OPEN, CycleStatus.DRAFT.moveTo(CycleStatus.OPEN))
        assertFailsWith<DomainException> { CycleStatus.DRAFT.moveTo(CycleStatus.CALIBRATION) }
        assertFailsWith<DomainException> { CycleStatus.CLOSED.next() }
    }
}
