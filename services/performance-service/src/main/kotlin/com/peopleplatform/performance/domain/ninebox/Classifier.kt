package com.peopleplatform.performance.domain.ninebox

import com.peopleplatform.performance.domain.shared.CycleId
import com.peopleplatform.performance.domain.shared.EmployeeId
import com.peopleplatform.performance.domain.shared.TenantId
import com.peopleplatform.performance.domain.shared.require
import com.peopleplatform.performance.domain.shared.requireSameTenant
import java.math.BigDecimal

/**
 * Notas consolidadas de uma pessoa num ciclo. O desempenho vem da avaliação (auto, gestor,
 * pares) e o potencial do assessment; a consolidação acontece antes, no ciclo de avaliação.
 */
data class Assessment(
    val tenantId: TenantId,
    val cycleId: CycleId,
    val employeeId: EmployeeId,
    val performanceScore: BigDecimal,
    val potentialScore: BigDecimal,
)

/** Posição de uma pessoa na matriz, com as notas que a originaram. */
data class Placement(
    val employeeId: EmployeeId,
    val performance: Level,
    val potential: Level,
    val performanceScore: BigDecimal,
    val potentialScore: BigDecimal,
) {
    val quadrant: Quadrant get() = Quadrant.of(performance, potential)
}

/** Classificação pura: notas + política do tenant → quadrante. */
object NineBoxClassifier {
    fun classify(assessment: Assessment, policy: NineBoxPolicy): Placement {
        requireSameTenant(policy.tenantId, assessment.tenantId)
        require(assessment.performanceScore in policy.scale) {
            "nota de desempenho ${assessment.performanceScore} fora da escala ${policy.scale.min}..${policy.scale.max}"
        }
        require(assessment.potentialScore in policy.scale) {
            "nota de potencial ${assessment.potentialScore} fora da escala ${policy.scale.min}..${policy.scale.max}"
        }
        return Placement(
            employeeId = assessment.employeeId,
            performance = policy.performance.levelOf(assessment.performanceScore),
            potential = policy.potential.levelOf(assessment.potentialScore),
            performanceScore = assessment.performanceScore,
            potentialScore = assessment.potentialScore,
        )
    }
}
