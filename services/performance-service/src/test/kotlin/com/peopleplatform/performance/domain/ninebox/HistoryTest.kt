package com.peopleplatform.performance.domain.ninebox

import com.peopleplatform.performance.domain.cycle2026
import com.peopleplatform.performance.domain.cycle2027
import com.peopleplatform.performance.domain.employee
import com.peopleplatform.performance.domain.horizonte
import com.peopleplatform.performance.domain.shared.CycleId
import com.peopleplatform.performance.domain.shared.DomainException
import com.peopleplatform.performance.domain.shared.EmployeeId
import com.peopleplatform.performance.domain.t0
import com.peopleplatform.performance.domain.varejoSol
import org.junit.jupiter.api.Test
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.CsvSource
import java.time.Duration
import java.util.UUID
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNull

class HistoryTest {
    private val who = employee()

    private fun outcome(
        cycle: CycleId,
        q: Quadrant,
        daysAfter: Long,
        adjusted: Boolean = false,
        person: EmployeeId = who,
    ): CalibrationOutcome {
        val adjustments = if (adjusted) {
            listOf(CalibrationAdjustment(person, Quadrant.Q5, q, employee(), "x", t0))
        } else {
            emptyList()
        }
        return CalibrationOutcome(
            tenantId = horizonte,
            cycleId = cycle,
            finalPlacements = mapOf(person to q),
            adjustments = adjustments,
            closedAt = t0.plus(Duration.ofDays(daysAfter)),
        )
    }

    @Test
    fun `registra ciclos em ordem e marca os calibrados`() {
        val h = NineBoxHistory(horizonte, who)
        assertNull(h.latest)
        h.record(outcome(cycle2026, Quadrant.Q5, 0))
        h.record(outcome(cycle2027, Quadrant.Q8, 365, adjusted = true))
        assertEquals(listOf(Quadrant.Q5, Quadrant.Q8), h.all.map { it.quadrant })
        assertEquals(true, h.latest!!.calibrated)
    }

    @ParameterizedTest(name = "{0} → {1} é {2}")
    @CsvSource("Q5, Q9, UP", "Q5, Q6, UP", "Q9, Q5, DOWN", "Q5, Q5, STABLE", "Q3, Q7, LATERAL", "Q8, Q6, LATERAL")
    fun `tendencia entre os dois ultimos ciclos`(from: Quadrant, to: Quadrant, expected: Trend) {
        val h = NineBoxHistory(horizonte, who)
        h.record(outcome(cycle2026, from, 0))
        h.record(outcome(cycle2027, to, 365))
        assertEquals(expected, h.trend())
    }

    @Test
    fun `tendencia exige dois ciclos`() {
        val h = NineBoxHistory(horizonte, who)
        h.record(outcome(cycle2026, Quadrant.Q5, 0))
        assertEquals(Trend.INSUFFICIENT_DATA, h.trend())
    }

    @Test
    fun `recusa ciclo repetido, fora de ordem, de outro tenant ou sem a pessoa`() {
        val h = NineBoxHistory(horizonte, who)
        h.record(outcome(cycle2027, Quadrant.Q5, 365))
        assertFailsWith<DomainException> { h.record(outcome(cycle2027, Quadrant.Q6, 400)) }
        assertFailsWith<DomainException> { h.record(outcome(CycleId(UUID.randomUUID()), Quadrant.Q6, 10)) }
        assertFailsWith<DomainException> { h.record(outcome(cycle2026, Quadrant.Q6, 500, person = employee())) }
        assertFailsWith<DomainException> {
            NineBoxHistory(varejoSol, who).record(outcome(cycle2026, Quadrant.Q5, 0))
        }
    }
}
