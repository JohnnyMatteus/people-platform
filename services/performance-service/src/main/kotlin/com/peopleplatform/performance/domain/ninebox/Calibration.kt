package com.peopleplatform.performance.domain.ninebox

import com.peopleplatform.performance.domain.shared.CycleId
import com.peopleplatform.performance.domain.shared.DomainException
import com.peopleplatform.performance.domain.shared.EmployeeId
import com.peopleplatform.performance.domain.shared.TenantId
import com.peopleplatform.performance.domain.shared.require
import com.peopleplatform.performance.domain.shared.requireSameTenant
import java.math.BigDecimal
import java.math.RoundingMode
import java.time.Instant
import kotlin.math.abs

/** Um ajuste feito pelo comitê de calibração. Fica no histórico para auditoria. */
data class CalibrationAdjustment(
    val employeeId: EmployeeId,
    val from: Quadrant,
    val to: Quadrant,
    val calibratedBy: EmployeeId,
    val justification: String,
    val at: Instant,
)

/** Alertas não bloqueantes que o comitê vê antes de fechar a sessão. */
sealed interface CalibrationWarning {
    data class TopBoxAboveGuideline(val share: BigDecimal, val guideline: BigDecimal) : CalibrationWarning
}

/** Resultado de uma sessão fechada: a posição final de cada pessoa, pronta para gerar eventos. */
data class CalibrationOutcome(
    val tenantId: TenantId,
    val cycleId: CycleId,
    val finalPlacements: Map<EmployeeId, Quadrant>,
    val adjustments: List<CalibrationAdjustment>,
    val closedAt: Instant,
)

/**
 * Sessão de calibração de um ciclo. Recebe as posições calculadas pelo classificador e
 * permite que o comitê as mova, com justificativa e dentro do limite de faixas da política.
 *
 * Invariantes:
 * - só aceita avaliações do tenant e do ciclo da sessão;
 * - quem é calibrado não calibra a si mesmo;
 * - ajustes só com a sessão aberta; depois de fechada, a sessão é imutável;
 * - a posição original nunca se perde: o ajuste é um registro novo, não uma sobrescrita.
 */
class CalibrationSession(
    val tenantId: TenantId,
    val cycleId: CycleId,
    private val policy: NineBoxPolicy,
) {
    private val original = linkedMapOf<EmployeeId, Placement>()
    private val current = linkedMapOf<EmployeeId, Quadrant>()
    private val log = mutableListOf<CalibrationAdjustment>()
    private var closedAt: Instant? = null

    init {
        requireSameTenant(tenantId, policy.tenantId)
    }

    val isClosed: Boolean get() = closedAt != null
    val adjustments: List<CalibrationAdjustment> get() = log.toList()

    fun add(assessment: Assessment) {
        ensureOpen()
        requireSameTenant(tenantId, assessment.tenantId)
        require(assessment.cycleId == cycleId) { "avaliação de outro ciclo não entra nesta sessão" }
        require(assessment.employeeId !in original) { "colaborador ${assessment.employeeId.value} já está na sessão" }
        val placement = NineBoxClassifier.classify(assessment, policy)
        original[assessment.employeeId] = placement
        current[assessment.employeeId] = placement.quadrant
    }

    fun originalOf(employee: EmployeeId): Quadrant = placementOf(employee).quadrant

    fun currentOf(employee: EmployeeId): Quadrant {
        placementOf(employee)
        return current.getValue(employee)
    }

    fun adjust(
        employee: EmployeeId,
        to: Quadrant,
        calibratedBy: EmployeeId,
        justification: String,
        at: Instant,
    ): CalibrationAdjustment {
        ensureOpen()
        val base = placementOf(employee).quadrant
        require(calibratedBy != employee) { "ninguém calibra a própria posição" }
        val text = justification.trim()
        require(text.length >= policy.minJustificationLength) {
            "justificativa precisa de ao menos ${policy.minJustificationLength} caracteres"
        }
        // O limite é medido contra a posição ORIGINAL, não a atual: assim uma sequência de
        // ajustes pequenos não consegue somar um salto maior que o permitido.
        val perfSteps = abs(to.performance.ordinal - base.performance.ordinal)
        val potSteps = abs(to.potential.ordinal - base.potential.ordinal)
        require(perfSteps <= policy.maxStepsPerAxis && potSteps <= policy.maxStepsPerAxis) {
            "ajuste de $base para $to excede ${policy.maxStepsPerAxis} faixa(s) por eixo"
        }
        val from = current.getValue(employee)
        require(from != to) { "colaborador já está em $to" }
        val adjustment = CalibrationAdjustment(employee, from, to, calibratedBy, text, at)
        current[employee] = to
        log += adjustment
        return adjustment
    }

    /** Quantidade de pessoas por quadrante, na posição atual. Quadrantes vazios aparecem com 0. */
    fun distribution(): Map<Quadrant, Int> {
        val counts = current.values.groupingBy { it }.eachCount()
        return Quadrant.entries.associateWith { counts[it] ?: 0 }
    }

    fun warnings(): List<CalibrationWarning> {
        if (current.isEmpty()) return emptyList()
        val share = BigDecimal(distribution().getValue(Quadrant.Q9))
            .divide(BigDecimal(current.size), 4, RoundingMode.HALF_UP)
        return if (share > policy.topBoxWarningShare) {
            listOf(CalibrationWarning.TopBoxAboveGuideline(share, policy.topBoxWarningShare))
        } else {
            emptyList()
        }
    }

    fun close(at: Instant): CalibrationOutcome {
        ensureOpen()
        require(current.isNotEmpty()) { "sessão sem colaboradores não pode ser fechada" }
        closedAt = at
        return CalibrationOutcome(tenantId, cycleId, current.toMap(), log.toList(), at)
    }

    private fun placementOf(employee: EmployeeId): Placement =
        original[employee] ?: throw DomainException(
            "colaborador ${employee.value} não está na sessão",
        )

    private fun ensureOpen() = require(!isClosed) { "sessão de calibração já foi fechada" }
}

/**
 * Evento de domínio `NineBoxCalibrado` (ver contracts/asyncapi): uma posição final por pessoa.
 * Consumido pelo PDI (sugestões a partir do quadrante) e pelo analytics (sempre agregado).
 */
data class NineBoxCalibrated(
    val tenantId: TenantId,
    val cycleId: CycleId,
    val employeeId: EmployeeId,
    val quadrant: Quadrant,
    val calibrated: Boolean,
    val occurredAt: Instant,
)

fun CalibrationOutcome.events(): List<NineBoxCalibrated> {
    val adjusted = adjustments.map { it.employeeId }.toSet()
    return finalPlacements.map { (employee, quadrant) ->
        NineBoxCalibrated(tenantId, cycleId, employee, quadrant, employee in adjusted, closedAt)
    }
}
