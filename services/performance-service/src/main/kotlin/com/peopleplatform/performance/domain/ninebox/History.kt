package com.peopleplatform.performance.domain.ninebox

import com.peopleplatform.performance.domain.shared.CycleId
import com.peopleplatform.performance.domain.shared.DomainException
import com.peopleplatform.performance.domain.shared.EmployeeId
import com.peopleplatform.performance.domain.shared.TenantId
import com.peopleplatform.performance.domain.shared.require
import com.peopleplatform.performance.domain.shared.requireSameTenant
import java.time.Instant

/** Posição final de uma pessoa num ciclo fechado. */
data class HistoryEntry(val cycleId: CycleId, val quadrant: Quadrant, val calibrated: Boolean, val closedAt: Instant)

/** Direção da trajetória entre os dois últimos ciclos. */
enum class Trend { UP, DOWN, STABLE, LATERAL, INSUFFICIENT_DATA }

/**
 * Histórico de nine box de uma pessoa, em ordem cronológica. É o que alimenta conversas de
 * carreira, o PDI e as sugestões de prontidão no mapa de sucessão.
 */
class NineBoxHistory(val tenantId: TenantId, val employeeId: EmployeeId) {
    private val entries = mutableListOf<HistoryEntry>()

    val all: List<HistoryEntry> get() = entries.toList()
    val latest: HistoryEntry? get() = entries.lastOrNull()

    /** Registra o resultado de uma sessão fechada. Cada ciclo entra uma única vez. */
    fun record(outcome: CalibrationOutcome) {
        requireSameTenant(tenantId, outcome.tenantId)
        val quadrant = outcome.finalPlacements[employeeId]
            ?: throw DomainException(
                "colaborador ${employeeId.value} não participou do ciclo",
            )
        require(entries.none { it.cycleId == outcome.cycleId }) { "ciclo já registrado no histórico" }
        require(entries.isEmpty() || outcome.closedAt.isAfter(entries.last().closedAt)) {
            "ciclos devem ser registrados em ordem cronológica"
        }
        val calibrated = outcome.adjustments.any { it.employeeId == employeeId }
        entries += HistoryEntry(outcome.cycleId, quadrant, calibrated, outcome.closedAt)
    }

    /**
     * Compara os dois últimos ciclos. Subir num eixo e descer no outro é LATERAL, porque não
     * há como dizer que trocar desempenho por potencial é melhora ou piora.
     */
    fun trend(): Trend {
        if (entries.size < 2) return Trend.INSUFFICIENT_DATA
        val (prev, last) = entries.takeLast(2).map { it.quadrant }
        val dPerf = last.performance.ordinal - prev.performance.ordinal
        val dPot = last.potential.ordinal - prev.potential.ordinal
        return when {
            dPerf == 0 && dPot == 0 -> Trend.STABLE
            dPerf >= 0 && dPot >= 0 -> Trend.UP
            dPerf <= 0 && dPot <= 0 -> Trend.DOWN
            else -> Trend.LATERAL
        }
    }
}
