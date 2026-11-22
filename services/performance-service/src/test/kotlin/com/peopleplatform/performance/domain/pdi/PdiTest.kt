package com.peopleplatform.performance.domain.pdi

import com.peopleplatform.performance.domain.evaluation.CompetencyGap
import com.peopleplatform.performance.domain.ninebox.Quadrant
import com.peopleplatform.performance.domain.shared.DomainException
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNull

class PdiTest {
    @Test
    fun `acao concluida nao reabre e status nao retrocede`() {
        assertEquals(PdiActionStatus.IN_PROGRESS, PdiActionStatus.PLANNED.moveTo(PdiActionStatus.IN_PROGRESS))
        assertFailsWith<DomainException> { PdiActionStatus.DONE.moveTo(PdiActionStatus.PLANNED) }
        assertFailsWith<DomainException> { PdiActionStatus.IN_PROGRESS.moveTo(PdiActionStatus.PLANNED) }
    }

    @Test
    fun `so gaps de 2 pontos ou mais viram sugestao, do maior para o menor`() {
        val suggestions = PdiAdvisor.suggest(
            listOf(CompetencyGap("Entrega", 1), CompetencyGap("Comunicação", 2), CompetencyGap("Liderança", 3)),
            null,
        )
        assertEquals(listOf("Liderança", "Comunicação"), suggestions.map { it.competency })
    }

    @Test
    fun `quadrante de desempenho baixo sugere recuperacao e potencial alto sugere desafio`() {
        assertEquals(PdiActionKind.PROJECT, PdiAdvisor.suggest(emptyList(), Quadrant.Q1).single().kind)
        assertEquals(PdiActionKind.PROJECT, PdiAdvisor.suggest(emptyList(), Quadrant.Q9).single().kind)
        assertNull(PdiAdvisor.suggest(emptyList(), Quadrant.Q5).firstOrNull())
    }
}
