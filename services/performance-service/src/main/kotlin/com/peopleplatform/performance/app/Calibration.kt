package com.peopleplatform.performance.app

import com.peopleplatform.performance.adapters.persistence.AdjustmentEntity
import com.peopleplatform.performance.adapters.persistence.AdjustmentRepository
import com.peopleplatform.performance.adapters.persistence.CycleRepository
import com.peopleplatform.performance.adapters.persistence.EmployeeRepository
import com.peopleplatform.performance.adapters.persistence.EvaluationEntity
import com.peopleplatform.performance.adapters.persistence.EvaluationRepository
import com.peopleplatform.performance.adapters.persistence.HistoryEntity
import com.peopleplatform.performance.adapters.persistence.HistoryRepository
import com.peopleplatform.performance.adapters.persistence.PlacementEntity
import com.peopleplatform.performance.adapters.persistence.PlacementRepository
import com.peopleplatform.performance.adapters.persistence.SessionEntity
import com.peopleplatform.performance.adapters.persistence.SessionRepository
import com.peopleplatform.performance.adapters.persistence.TenantScope
import com.peopleplatform.performance.adapters.persistence.copyFrom
import com.peopleplatform.performance.adapters.persistence.toDomain
import com.peopleplatform.performance.domain.evaluation.EvaluationConsolidator
import com.peopleplatform.performance.domain.ninebox.Assessment
import com.peopleplatform.performance.domain.ninebox.CalibrationAdjustment
import com.peopleplatform.performance.domain.ninebox.CalibrationSession
import com.peopleplatform.performance.domain.ninebox.CalibrationWarning
import com.peopleplatform.performance.domain.ninebox.HistoryEntry
import com.peopleplatform.performance.domain.ninebox.NineBoxHistory
import com.peopleplatform.performance.domain.ninebox.NineBoxPolicy
import com.peopleplatform.performance.domain.ninebox.Quadrant
import com.peopleplatform.performance.domain.ninebox.Trend
import com.peopleplatform.performance.domain.ninebox.events
import com.peopleplatform.performance.domain.shared.CycleId
import com.peopleplatform.performance.domain.shared.EmployeeId
import org.springframework.stereotype.Service
import java.math.BigDecimal
import java.time.Clock
import java.util.UUID

data class PlacementView(
    val employeeId: UUID,
    val employeeName: String,
    val performanceScore: BigDecimal,
    val potentialScore: BigDecimal,
    val original: Quadrant,
    val current: Quadrant,
)

data class SessionView(
    val id: UUID,
    val cycleId: UUID,
    val status: String,
    val policy: NineBoxPolicy,
    val placements: List<PlacementView>,
    val distribution: Map<Quadrant, Int>,
    val warnings: List<CalibrationWarning>,
    val adjustments: List<CalibrationAdjustment>,
)

data class HistoryView(val entries: List<HistoryEntry>, val trend: Trend)

/**
 * Sessões de calibração. O estado vivo é o domínio (`CalibrationSession`), reconstruído a cada
 * operação a partir das posições originais e do log de ajustes, sob lock da linha da sessão:
 * o banco guarda fatos (notas, ajustes) e nunca uma "posição atual" que possa divergir do log.
 */
@Service
class CalibrationService(
    private val scope: TenantScope,
    private val policies: PolicyService,
    private val cycles: CycleRepository,
    private val evaluations: EvaluationRepository,
    private val employees: EmployeeRepository,
    private val sessions: SessionRepository,
    private val placements: PlacementRepository,
    private val adjustments: AdjustmentRepository,
    private val history: HistoryRepository,
    private val events: DomainEvents,
    private val evaluationService: EvaluationService,
    private val clock: Clock,
) {
    fun open(caller: Caller, cycleId: UUID): SessionView = scope.run(caller.tenantId) {
        val cycle = cycles.findById(cycleId).orElseThrow { NotFoundException("ciclo não encontrado") }
        if (sessions.findByCycleId(cycleId) != null) throw ConflictException("ciclo já tem sessão de calibração")
        if (cycle.status != "open") throw ConflictException("ciclo precisa estar aberto para calibrar")
        val assessments = consolidated(cycleId)
        if (assessments.isEmpty()) throw ConflictException("nenhuma avaliação consolidada neste ciclo")

        val session = SessionEntity(
            UUID.randomUUID(),
            caller.tenantId.value,
            cycleId,
            "open",
            BigDecimal.ONE,
            BigDecimal.ONE,
            BigDecimal.ONE,
            BigDecimal.ONE,
            BigDecimal.ONE,
            BigDecimal.ONE,
            1,
            BigDecimal.ONE,
            0,
            clock.instant(),
            null,
        )
        session.copyFrom(policies.current(caller))
        sessions.save(session)
        assessments.forEach { (employee, scores) ->
            placements.save(PlacementEntity(session.id, employee, caller.tenantId.value, scores.first, scores.second))
        }
        cycle.status = "calibration"
        cycles.save(cycle)
        view(caller, session)
    }

    fun get(caller: Caller, id: UUID): SessionView = scope.run(caller.tenantId) {
        view(caller, sessions.findById(id).orElseThrow { NotFoundException("sessão não encontrada") })
    }

    fun getByCycle(caller: Caller, cycleId: UUID): SessionView = scope.run(caller.tenantId) {
        val session = sessions.findByCycleId(cycleId)
            ?: throw NotFoundException("ciclo ainda não tem sessão de calibração")
        view(caller, session)
    }

    fun adjust(caller: Caller, id: UUID, employee: UUID, to: Quadrant, justification: String): CalibrationAdjustment =
        scope.run(caller.tenantId) {
            val entity = lockOpen(id)
            val domain = rebuild(caller, entity)
            val adjustment = domain.adjust(EmployeeId(employee), to, caller.employeeId, justification, clock.instant())
            adjustments.save(
                AdjustmentEntity(
                    UUID.randomUUID(),
                    caller.tenantId.value,
                    id,
                    employee,
                    adjustment.from.name,
                    to.name,
                    caller.employeeId.value,
                    adjustment.justification,
                    adjustment.at,
                ),
            )
            adjustment
        }

    fun close(caller: Caller, id: UUID): SessionView = scope.run(caller.tenantId) {
        val entity = lockOpen(id)
        val outcome = rebuild(caller, entity).close(clock.instant())
        outcome.events().forEach { e ->
            history.save(
                HistoryEntity(
                    e.employeeId.value,
                    e.cycleId.value,
                    caller.tenantId.value,
                    e.quadrant.name,
                    e.calibrated,
                    e.occurredAt,
                ),
            )
            events.publish(
                caller.tenantId,
                "calibration",
                e.employeeId.value,
                "NineBoxCalibrado",
                mapOf(
                    "cycle_id" to e.cycleId.value.toString(),
                    "employee_id" to e.employeeId.value.toString(),
                    "quadrant" to e.quadrant.name,
                    "calibrated" to e.calibrated,
                ),
                e.occurredAt,
            )
        }
        entity.status = "closed"
        entity.closedAt = outcome.closedAt
        sessions.save(entity)
        cycles.findById(entity.cycleId).ifPresent {
            it.status = "closed"
            cycles.save(it)
        }
        view(caller, entity)
    }

    fun history(caller: Caller, employee: UUID): HistoryView = scope.run(caller.tenantId) {
        if (!caller.canSee(EmployeeId(employee))) throw ForbiddenException("sem permissão para ver este histórico")
        if (!employees.existsById(employee)) throw NotFoundException("colaborador não encontrado")
        val domain = NineBoxHistory(caller.tenantId, EmployeeId(employee))
        domain.restore(
            history.findByEmployeeIdOrderByClosedAtAsc(employee).map {
                HistoryEntry(CycleId(it.cycleId), Quadrant.valueOf(it.quadrant), it.calibrated, it.closedAt)
            },
        )
        HistoryView(domain.all, domain.trend())
    }

    private fun lockOpen(id: UUID): SessionEntity {
        val entity = sessions.lockById(id) ?: throw NotFoundException("sessão não encontrada")
        if (entity.status != "open") throw ConflictException("sessão de calibração já foi fechada")
        return entity
    }

    /** Pessoas com autoavaliação e avaliação do gestor: nota consolidada de desempenho e potencial. */
    private fun consolidated(cycleId: UUID): Map<UUID, Pair<BigDecimal, BigDecimal>> =
        evaluations.findByCycleId(cycleId).groupBy { it.employeeId }.mapNotNull { (employee, rows) ->
            val self = rows.firstOrNull { it.kind == "self" }
            val manager = rows.firstOrNull { it.kind == "manager" }
            if (self == null || manager == null) {
                null
            } else {
                employee to scores(self, manager)
            }
        }.toMap()

    private fun scores(self: EvaluationEntity, manager: EvaluationEntity): Pair<BigDecimal, BigDecimal> {
        val result = EvaluationConsolidator.consolidate(
            self.performanceScore,
            evaluationService.competenciesOf(self),
            manager.performanceScore,
            manager.potentialScore ?: BigDecimal.ZERO,
            evaluationService.competenciesOf(manager),
        )
        return result.performance to result.potential
    }

    private fun rebuild(caller: Caller, entity: SessionEntity): CalibrationSession {
        val domain = CalibrationSession(caller.tenantId, CycleId(entity.cycleId), entity.toDomain(caller.tenantId))
        placements.findBySessionId(entity.id).forEach {
            domain.add(
                Assessment(
                    caller.tenantId,
                    CycleId(entity.cycleId),
                    EmployeeId(it.employeeId),
                    it.performanceScore,
                    it.potentialScore,
                ),
            )
        }
        adjustments.findBySessionIdOrderByAtAscIdAsc(entity.id).forEach {
            domain.adjust(
                EmployeeId(it.employeeId),
                Quadrant.valueOf(it.toQuadrant),
                EmployeeId(it.calibratedBy),
                it.justification,
                it.at,
            )
        }
        return domain
    }

    private fun view(caller: Caller, entity: SessionEntity): SessionView {
        val domain = rebuild(caller, entity)
        val names = employees.findAll().associate { it.id to it.name }
        val rows = placements.findBySessionId(entity.id).map {
            val id = EmployeeId(it.employeeId)
            PlacementView(
                it.employeeId,
                names[it.employeeId] ?: "Colaborador",
                it.performanceScore,
                it.potentialScore,
                domain.originalOf(id),
                domain.currentOf(id),
            )
        }.sortedBy { it.employeeName }
        return SessionView(
            entity.id,
            entity.cycleId,
            entity.status,
            entity.toDomain(caller.tenantId),
            rows,
            domain.distribution(),
            domain.warnings(),
            domain.adjustments,
        )
    }
}
