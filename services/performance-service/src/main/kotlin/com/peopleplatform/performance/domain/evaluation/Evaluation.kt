package com.peopleplatform.performance.domain.evaluation

import com.peopleplatform.performance.domain.shared.DomainException
import com.peopleplatform.performance.domain.shared.require
import java.math.BigDecimal
import java.math.RoundingMode
import kotlin.math.abs

enum class EvaluationKind { SELF, MANAGER }

/** Nota de uma competência, de 1 a 5. */
data class CompetencyScore(val competency: String, val score: Int) {
    init {
        require(competency.isNotBlank()) { "nome da competência é obrigatório" }
        require(score in 1..5) { "nota de competência deve estar entre 1 e 5" }
    }
}

/** Distância entre o que a pessoa se atribuiu e o que o gestor viu: é onde o PDI começa. */
data class CompetencyGap(val competency: String, val gap: Int)

/** Resultado da consolidação: entrada do nine box e dos gaps do PDI. */
data class ConsolidatedEvaluation(
    val performance: BigDecimal,
    val potential: BigDecimal,
    val gaps: List<CompetencyGap>,
)

/**
 * Consolida autoavaliação e avaliação do gestor.
 *
 * O desempenho é a média ponderada (gestor 70%, pessoa 30%): a visão do gestor pesa mais para
 * não premiar autoavaliação inflada, mas a da pessoa não é descartada. O potencial vem só do gestor
 * (a pessoa não se mede em potencial). O gap de cada competência é a diferença absoluta entre as
 * duas notas, arredondada para inteiro; competência avaliada só por um lado não gera gap.
 */
object EvaluationConsolidator {
    val MANAGER_WEIGHT = BigDecimal("0.7")
    val SELF_WEIGHT = BigDecimal("0.3")

    fun consolidate(
        selfPerformance: BigDecimal,
        selfCompetencies: List<CompetencyScore>,
        managerPerformance: BigDecimal,
        managerPotential: BigDecimal,
        managerCompetencies: List<CompetencyScore>,
    ): ConsolidatedEvaluation {
        val performance = (managerPerformance * MANAGER_WEIGHT + selfPerformance * SELF_WEIGHT)
            .setScale(2, RoundingMode.HALF_UP)
        val own = selfCompetencies.associate { it.competency to it.score }
        val gaps = managerCompetencies.mapNotNull { m ->
            own[m.competency]?.let { CompetencyGap(m.competency, abs(it - m.score)) }
        }
        return ConsolidatedEvaluation(performance, managerPotential, gaps)
    }
}

/** Ciclo de avaliação: draft → open → calibration → closed, sem voltar. */
enum class CycleStatus {
    DRAFT,
    OPEN,
    CALIBRATION,
    CLOSED,
    ;

    fun next(): CycleStatus = entries.getOrNull(ordinal + 1) ?: throw DomainException("ciclo já está fechado")

    fun moveTo(target: CycleStatus): CycleStatus {
        if (target != next()) throw DomainException("ciclo não pode ir de $this para $target")
        return target
    }
}

fun requireScoreInRange(value: BigDecimal, min: BigDecimal, max: BigDecimal, label: String) {
    require(value >= min && value <= max) { "nota de $label $value fora da escala $min..$max" }
}
