package com.peopleplatform.performance.domain.okr

import com.peopleplatform.performance.domain.shared.DomainException
import org.junit.jupiter.api.Test
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.CsvSource
import java.math.BigDecimal
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith

class OkrTest {
    @ParameterizedTest
    @CsvSource(
        "0,100,0,0.0000",
        "0,100,25,0.2500",
        "0,100,100,1.0000",
        "0,100,150,1.0000",
        "0,100,-10,0.0000",
        "20,5,20,0.0000",
        "20,5,12.5,0.5000",
        "20,5,5,1.0000",
        "20,5,3,1.0000",
    )
    fun `progresso do resultado-chave fica entre zero e um, inclusive em metas decrescentes`(
        start: String,
        target: String,
        current: String,
        expected: String,
    ) {
        val progress = OkrProgress.keyResult(BigDecimal(start), BigDecimal(target), BigDecimal(current))
        assertEquals(BigDecimal(expected), progress)
    }

    @Test
    fun `progresso do objetivo e a media dos resultados-chave e zero quando nao ha nenhum`() {
        assertEquals(BigDecimal("0.5000"), OkrProgress.objective(listOf(BigDecimal("0.2500"), BigDecimal("0.7500"))))
        assertEquals(BigDecimal("0.0000"), OkrProgress.objective(emptyList()))
    }

    @Test
    fun `resultado-chave sem distancia entre partida e meta e recusado`() {
        assertFailsWith<DomainException> { KeyResultSpec("Reduzir acidentes", BigDecimal.TEN, BigDecimal.TEN) }
        assertFailsWith<DomainException> { KeyResultSpec("ab", BigDecimal.ZERO, BigDecimal.TEN) }
    }

    @Test
    fun `desdobramento so vai de um nivel mais alto para um mais baixo`() {
        requireCascade(ObjectiveLevel.COMPANY, null)
        requireCascade(ObjectiveLevel.TEAM, ObjectiveLevel.COMPANY)
        requireCascade(ObjectiveLevel.INDIVIDUAL, ObjectiveLevel.TEAM)
        requireCascade(ObjectiveLevel.INDIVIDUAL, ObjectiveLevel.COMPANY)
        requireCascade(ObjectiveLevel.TEAM, null)
        assertFailsWith<DomainException> { requireCascade(ObjectiveLevel.COMPANY, ObjectiveLevel.COMPANY) }
        assertFailsWith<DomainException> { requireCascade(ObjectiveLevel.TEAM, ObjectiveLevel.INDIVIDUAL) }
        assertFailsWith<DomainException> { requireCascade(ObjectiveLevel.TEAM, ObjectiveLevel.TEAM) }
    }
}
