package com.peopleplatform.performance.domain.ninebox

import com.peopleplatform.performance.domain.JUSTIFICATION
import com.peopleplatform.performance.domain.assessment
import com.peopleplatform.performance.domain.cycle2026
import com.peopleplatform.performance.domain.cycle2027
import com.peopleplatform.performance.domain.defaultPolicy
import com.peopleplatform.performance.domain.employee
import com.peopleplatform.performance.domain.horizonte
import com.peopleplatform.performance.domain.shared.DomainException
import com.peopleplatform.performance.domain.t0
import com.peopleplatform.performance.domain.varejoSol
import org.junit.jupiter.api.Test
import java.math.BigDecimal
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertIs
import kotlin.test.assertTrue

class CalibrationTest {
    private val calibrator = employee()

    private fun session(policy: NineBoxPolicy = defaultPolicy()) = CalibrationSession(horizonte, cycle2026, policy)

    @Test
    fun `ajuste de uma faixa com justificativa preserva a posicao original`() {
        val s = session()
        val ana = employee()
        s.add(assessment("3.0", "3.0", who = ana))

        val adj = s.adjust(ana, Quadrant.Q6, calibrator, JUSTIFICATION, t0)

        assertEquals(Quadrant.Q5, adj.from)
        assertEquals(Quadrant.Q6, s.currentOf(ana))
        assertEquals(Quadrant.Q5, s.originalOf(ana))
        assertEquals(listOf(adj), s.adjustments)
    }

    @Test
    fun `limite de faixas e medido contra a posicao original`() {
        val s = session()
        val bia = employee()
        s.add(assessment("1.0", "3.0", who = bia)) // Q4: desempenho LOW
        s.adjust(bia, Quadrant.Q5, calibrator, JUSTIFICATION, t0) // LOW → MEDIUM: ok

        // MEDIUM → HIGH seria um passo a partir da posição atual, mas são dois a partir da original.
        val error = assertFailsWith<DomainException> { s.adjust(bia, Quadrant.Q6, calibrator, JUSTIFICATION, t0) }
        assertTrue(error.message!!.contains("excede"))
    }

    @Test
    fun `politica com dois passos permite saltos maiores`() {
        val s = session(defaultPolicy().copy(maxStepsPerAxis = 2))
        val caio = employee()
        s.add(assessment("1.0", "1.0", who = caio))
        s.adjust(caio, Quadrant.Q9, calibrator, JUSTIFICATION, t0)
        assertEquals(Quadrant.Q9, s.currentOf(caio))
    }

    @Test
    fun `recusa ajustes invalidos`() {
        val s = session()
        val dani = employee()
        s.add(assessment("3.0", "3.0", who = dani))

        val stranger = employee()
        assertFailsWith<DomainException>("justificativa curta") {
            s.adjust(dani, Quadrant.Q6, calibrator, "porque sim", t0)
        }
        assertFailsWith<DomainException>("autocalibração") {
            s.adjust(dani, Quadrant.Q6, dani, JUSTIFICATION, t0)
        }
        assertFailsWith<DomainException>("mesmo quadrante") {
            s.adjust(dani, Quadrant.Q5, calibrator, JUSTIFICATION, t0)
        }
        assertFailsWith<DomainException>("fora da sessão") {
            s.adjust(stranger, Quadrant.Q5, calibrator, JUSTIFICATION, t0)
        }
    }

    @Test
    fun `recusa avaliacoes de outro tenant, de outro ciclo ou duplicadas`() {
        val s = session()
        val eva = employee()
        s.add(assessment("3", "3", who = eva))
        assertFailsWith<DomainException> { s.add(assessment("3", "3", who = eva)) }
        assertFailsWith<DomainException> { s.add(assessment("3", "3", tenant = varejoSol)) }
        assertFailsWith<DomainException> { s.add(assessment("3", "3", cycle = cycle2027)) }
        assertFailsWith<DomainException> { CalibrationSession(varejoSol, cycle2026, defaultPolicy(horizonte)) }
    }

    @Test
    fun `alerta quando o quadrante de topo passa da diretriz, sem bloquear`() {
        val s = session()
        repeat(3) { s.add(assessment("4.5", "4.5")) }
        repeat(7) { s.add(assessment("3.0", "3.0")) }

        val warning = assertIs<CalibrationWarning.TopBoxAboveGuideline>(s.warnings().single())
        assertEquals(BigDecimal("0.3000"), warning.share)
        assertEquals(3, s.distribution().getValue(Quadrant.Q9))
        assertEquals(0, s.distribution().getValue(Quadrant.Q1))
        s.close(t0) // alerta não impede o fechamento
    }

    @Test
    fun `sem alerta quando a distribuicao esta dentro da diretriz`() {
        val s = session()
        s.add(assessment("4.5", "4.5"))
        repeat(4) { s.add(assessment("3.0", "3.0")) }
        assertTrue(s.warnings().isEmpty())
    }

    @Test
    fun `sessao fechada e imutavel e gera eventos por pessoa`() {
        val s = session()
        val fabi = employee()
        val gil = employee()
        s.add(assessment("3.0", "3.0", who = fabi))
        s.add(assessment("4.0", "4.0", who = gil))
        s.adjust(fabi, Quadrant.Q8, calibrator, JUSTIFICATION, t0)

        val outcome = s.close(t0.plusSeconds(60))

        assertEquals(mapOf(fabi to Quadrant.Q8, gil to Quadrant.Q9), outcome.finalPlacements)
        assertFailsWith<DomainException> { s.adjust(gil, Quadrant.Q8, calibrator, JUSTIFICATION, t0) }
        assertFailsWith<DomainException> { s.add(assessment("3", "3")) }
        assertFailsWith<DomainException> { s.close(t0) }

        val events = outcome.events().associateBy { it.employeeId }
        assertTrue(events.getValue(fabi).calibrated)
        assertEquals(false, events.getValue(gil).calibrated)
        assertEquals(Quadrant.Q9, events.getValue(gil).quadrant)
    }

    @Test
    fun `sessao vazia nao fecha`() {
        assertFailsWith<DomainException> { session().close(t0) }
    }
}
