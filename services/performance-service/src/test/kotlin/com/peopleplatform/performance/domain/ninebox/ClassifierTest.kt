package com.peopleplatform.performance.domain.ninebox

import com.peopleplatform.performance.domain.assessment
import com.peopleplatform.performance.domain.defaultPolicy
import com.peopleplatform.performance.domain.horizonte
import com.peopleplatform.performance.domain.shared.DomainException
import com.peopleplatform.performance.domain.varejoSol
import org.junit.jupiter.api.Test
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.CsvSource
import java.math.BigDecimal
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith

class ClassifierTest {
    @ParameterizedTest(name = "desempenho {0} e potencial {1} → {2}")
    @CsvSource(
        "1.0, 1.0, Q1",
        "2.49, 1.0, Q1",
        // Limiar inferior é inclusivo: exatamente 2.5 já é MEDIUM.
        "2.5, 1.0, Q2",
        "3.74, 2.0, Q2",
        "3.75, 2.0, Q3",
        "1.0, 2.5, Q4",
        "3.0, 3.0, Q5",
        "5.0, 3.0, Q6",
        "2.0, 3.75, Q7",
        "3.0, 4.0, Q8",
        "3.75, 3.75, Q9",
        "5.0, 5.0, Q9",
    )
    fun `classifica nos nove quadrantes com limiares inclusivos`(perf: String, pot: String, expected: Quadrant) {
        assertEquals(expected, NineBoxClassifier.classify(assessment(perf, pot), defaultPolicy()).quadrant)
    }

    @Test
    fun `limiares configurados pelo tenant mudam a classificacao`() {
        val strict = defaultPolicy().copy(
            performance = AxisThresholds(BigDecimal("3.0"), BigDecimal("4.5")),
            potential = AxisThresholds(BigDecimal("3.0"), BigDecimal("4.5")),
        )
        val a = assessment("4.0", "4.0")
        assertEquals(Quadrant.Q9, NineBoxClassifier.classify(a, defaultPolicy()).quadrant)
        assertEquals(Quadrant.Q5, NineBoxClassifier.classify(a, strict).quadrant)
    }

    @Test
    fun `escala de 0 a 10 funciona com limiares proprios`() {
        val tenScale = NineBoxPolicy(
            tenantId = horizonte,
            scale = Scale(BigDecimal.ZERO, BigDecimal.TEN),
            performance = AxisThresholds(BigDecimal("4"), BigDecimal("7")),
            potential = AxisThresholds(BigDecimal("5"), BigDecimal("8")),
        )
        assertEquals(Quadrant.Q6, NineBoxClassifier.classify(assessment("9", "6"), tenScale).quadrant)
    }

    @Test
    fun `rejeita nota fora da escala`() {
        assertFailsWith<DomainException> { NineBoxClassifier.classify(assessment("5.01", "3"), defaultPolicy()) }
        assertFailsWith<DomainException> { NineBoxClassifier.classify(assessment("3", "0.99"), defaultPolicy()) }
    }

    @Test
    fun `rejeita avaliacao de outro tenant`() {
        assertFailsWith<DomainException> {
            NineBoxClassifier.classify(assessment("3", "3", tenant = varejoSol), defaultPolicy(horizonte))
        }
    }

    @Test
    fun `politica rejeita limiares incoerentes`() {
        val base = defaultPolicy()
        assertFailsWith<DomainException> {
            base.copy(performance = AxisThresholds(BigDecimal("4"), BigDecimal("3")))
        }
        assertFailsWith<DomainException> {
            base.copy(potential = AxisThresholds(BigDecimal("1"), BigDecimal("3")))
        }
        assertFailsWith<DomainException> {
            base.copy(potential = AxisThresholds(BigDecimal("2"), BigDecimal("5.5")))
        }
        assertFailsWith<DomainException> { base.copy(maxStepsPerAxis = 0) }
        assertFailsWith<DomainException> { Scale(BigDecimal.TEN, BigDecimal.ONE) }
    }

    @Test
    fun `quadrantes cobrem todas as combinacoes uma unica vez`() {
        val combos = Quadrant.entries.map { it.performance to it.potential }.toSet()
        assertEquals(9, combos.size)
        assertEquals((1..9).toList(), Quadrant.entries.map { it.number })
    }
}
