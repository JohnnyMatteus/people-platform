package com.peopleplatform.performance.domain.succession

import com.peopleplatform.performance.domain.ninebox.HistoryEntry
import com.peopleplatform.performance.domain.ninebox.Level
import com.peopleplatform.performance.domain.shared.DomainException
import com.peopleplatform.performance.domain.shared.EmployeeId
import com.peopleplatform.performance.domain.shared.TenantId
import com.peopleplatform.performance.domain.shared.require
import java.time.Instant
import java.util.UUID

/** Prontidão de um sucessor para assumir a posição. A ordem do enum vai do mais pronto ao menos. */
enum class Readiness { READY_NOW, ONE_TO_TWO_YEARS, THREE_PLUS_YEARS }

/** Força do banco de sucessores de uma posição. */
enum class BenchStrength { NONE, WEAK, ADEQUATE, STRONG }

/** Posição crítica: aquela cuja vacância sem sucessor gera risco ao negócio. */
data class CriticalPosition(val id: UUID, val tenantId: TenantId, val title: String, val incumbent: EmployeeId?) {
    init {
        require(title.isNotBlank()) { "título da posição é obrigatório" }
    }
}

data class Successor(val employeeId: EmployeeId, val readiness: Readiness, val nominatedBy: EmployeeId, val at: Instant)

/**
 * Mapa de sucessão de uma posição crítica.
 *
 * Invariantes: o ocupante não é sucessor de si mesmo; cada pessoa aparece uma vez; há um teto
 * de sucessores (para o mapa continuar sendo uma decisão, não uma lista de todo o time).
 */
class SuccessionPlan(val position: CriticalPosition, private val maxSuccessors: Int = 5) {
    private val successors = linkedMapOf<EmployeeId, Successor>()

    init {
        require(maxSuccessors in 1..10) { "maxSuccessors deve estar entre 1 e 10" }
    }

    val tenantId: TenantId get() = position.tenantId

    /** Sucessores ordenados por prontidão (mais prontos primeiro) e, no empate, por indicação. */
    val ranked: List<Successor>
        get() = successors.values.sortedWith(compareBy<Successor> { it.readiness.ordinal }.thenBy { it.at })

    fun nominate(employee: EmployeeId, readiness: Readiness, nominatedBy: EmployeeId, at: Instant): Successor {
        require(employee != position.incumbent) { "o ocupante não pode ser sucessor da própria posição" }
        require(employee !in successors) { "colaborador já é sucessor desta posição" }
        require(successors.size < maxSuccessors) { "limite de $maxSuccessors sucessores atingido" }
        return Successor(employee, readiness, nominatedBy, at).also { successors[employee] = it }
    }

    fun changeReadiness(employee: EmployeeId, readiness: Readiness, at: Instant): Successor {
        val existing = successors[employee] ?: throw DomainException("colaborador não é sucessor desta posição")
        return existing.copy(readiness = readiness, at = at).also { successors[employee] = it }
    }

    fun remove(employee: EmployeeId) {
        successors.remove(employee) ?: throw DomainException("colaborador não é sucessor desta posição")
    }

    /**
     * - STRONG: ao menos um pronto agora e ao menos dois sucessores no total;
     * - ADEQUATE: ao menos um pronto agora ou em 1–2 anos;
     * - WEAK: só sucessores de 3+ anos;
     * - NONE: sem sucessores (risco máximo se a posição vagar).
     */
    fun benchStrength(): BenchStrength {
        val all = successors.values
        return when {
            all.isEmpty() -> BenchStrength.NONE
            all.any { it.readiness == Readiness.READY_NOW } && all.size >= 2 -> BenchStrength.STRONG
            all.any { it.readiness != Readiness.THREE_PLUS_YEARS } -> BenchStrength.ADEQUATE
            else -> BenchStrength.WEAK
        }
    }
}

/**
 * Sugere a prontidão inicial a partir do nine box e do tempo na função. É só sugestão: quem
 * decide é o comitê, e a sugestão nunca é gravada como decisão sem confirmação humana.
 */
object ReadinessAdvisor {
    /** Tempo mínimo na função atual para sugerir "pronto agora". */
    const val MIN_MONTHS_FOR_READY_NOW = 12

    /** Devolve null quando o potencial é baixo: a pessoa não é indicada como sucessora. */
    fun suggest(latest: HistoryEntry, monthsInRole: Int): Readiness? {
        require(monthsInRole >= 0) { "meses na função não pode ser negativo" }
        val q = latest.quadrant
        return when {
            q.potential == Level.LOW -> null
            q.potential == Level.HIGH && q.performance == Level.HIGH && monthsInRole >= MIN_MONTHS_FOR_READY_NOW ->
                Readiness.READY_NOW
            q.potential == Level.HIGH && q.performance != Level.LOW -> Readiness.ONE_TO_TWO_YEARS
            q.performance == Level.HIGH -> Readiness.ONE_TO_TWO_YEARS
            else -> Readiness.THREE_PLUS_YEARS
        }
    }
}
