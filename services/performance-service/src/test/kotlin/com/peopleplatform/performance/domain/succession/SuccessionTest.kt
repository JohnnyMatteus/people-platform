package com.peopleplatform.performance.domain.succession

import com.peopleplatform.performance.domain.cycle2026
import com.peopleplatform.performance.domain.employee
import com.peopleplatform.performance.domain.horizonte
import com.peopleplatform.performance.domain.ninebox.HistoryEntry
import com.peopleplatform.performance.domain.ninebox.Quadrant
import com.peopleplatform.performance.domain.shared.DomainException
import com.peopleplatform.performance.domain.t0
import org.junit.jupiter.api.Test
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.CsvSource
import java.util.UUID
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith

class SuccessionTest {
    private val incumbent = employee()
    private val hrbp = employee()
    private val position = CriticalPosition(UUID.randomUUID(), horizonte, "Gerente de Obras Regional", incumbent)

    @Test
    fun `forca do banco evolui com as indicacoes`() {
        val plan = SuccessionPlan(position)
        assertEquals(BenchStrength.NONE, plan.benchStrength())

        val a = employee()
        plan.nominate(a, Readiness.THREE_PLUS_YEARS, hrbp, t0)
        assertEquals(BenchStrength.WEAK, plan.benchStrength())

        plan.changeReadiness(a, Readiness.READY_NOW, t0.plusSeconds(10))
        assertEquals(BenchStrength.ADEQUATE, plan.benchStrength(), "um só pronto agora ainda é adequado")

        plan.nominate(employee(), Readiness.THREE_PLUS_YEARS, hrbp, t0)
        assertEquals(BenchStrength.STRONG, plan.benchStrength())

        plan.remove(a)
        assertEquals(BenchStrength.WEAK, plan.benchStrength())
    }

    @Test
    fun `ranking ordena por prontidao e depois por data`() {
        val plan = SuccessionPlan(position)
        val late = employee()
        val early = employee()
        val ready = employee()
        plan.nominate(late, Readiness.ONE_TO_TWO_YEARS, hrbp, t0.plusSeconds(100))
        plan.nominate(early, Readiness.ONE_TO_TWO_YEARS, hrbp, t0)
        plan.nominate(ready, Readiness.READY_NOW, hrbp, t0.plusSeconds(200))
        assertEquals(listOf(ready, early, late), plan.ranked.map { it.employeeId })
    }

    @Test
    fun `invariantes do mapa`() {
        val plan = SuccessionPlan(position, maxSuccessors = 2)
        assertFailsWith<DomainException>("ocupante") { plan.nominate(incumbent, Readiness.READY_NOW, hrbp, t0) }
        val a = employee()
        plan.nominate(a, Readiness.READY_NOW, hrbp, t0)
        assertFailsWith<DomainException>("duplicado") { plan.nominate(a, Readiness.READY_NOW, hrbp, t0) }
        plan.nominate(employee(), Readiness.READY_NOW, hrbp, t0)
        assertFailsWith<DomainException>("limite") { plan.nominate(employee(), Readiness.READY_NOW, hrbp, t0) }
        assertFailsWith<DomainException> { plan.changeReadiness(employee(), Readiness.READY_NOW, t0) }
        assertFailsWith<DomainException> { plan.remove(employee()) }
        assertFailsWith<DomainException> { CriticalPosition(UUID.randomUUID(), horizonte, " ", null) }
    }

    @ParameterizedTest(name = "{0} com {1} meses → {2}")
    @CsvSource(
        "Q9, 12, READY_NOW",
        "Q9, 11, ONE_TO_TWO_YEARS",
        "Q8, 36, ONE_TO_TWO_YEARS",
        "Q6, 24, ONE_TO_TWO_YEARS",
        "Q5, 24, THREE_PLUS_YEARS",
        "Q7, 24, THREE_PLUS_YEARS",
        "Q4, 24, THREE_PLUS_YEARS",
    )
    fun `sugestao de prontidao a partir do nine box`(q: Quadrant, months: Int, expected: Readiness) {
        assertEquals(expected, ReadinessAdvisor.suggest(HistoryEntry(cycle2026, q, false, t0), months))
    }

    @ParameterizedTest
    @CsvSource("Q1", "Q2", "Q3")
    fun `potencial baixo nao gera sugestao de sucessao`(q: Quadrant) {
        assertEquals(null, ReadinessAdvisor.suggest(HistoryEntry(cycle2026, q, false, t0), 60))
    }

    @Test
    fun `meses negativos sao recusados`() {
        val entry = HistoryEntry(cycle2026, Quadrant.Q9, false, t0)
        assertFailsWith<DomainException> { ReadinessAdvisor.suggest(entry, -1) }
    }
}
