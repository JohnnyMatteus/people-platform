package com.peopleplatform.performance.app

import com.peopleplatform.performance.adapters.persistence.EmployeeRepository
import com.peopleplatform.performance.adapters.persistence.EvaluationRepository
import com.peopleplatform.performance.adapters.persistence.HistoryRepository
import com.peopleplatform.performance.adapters.persistence.PdiActionEntity
import com.peopleplatform.performance.adapters.persistence.PdiActionRepository
import com.peopleplatform.performance.adapters.persistence.PositionEntity
import com.peopleplatform.performance.adapters.persistence.PositionRepository
import com.peopleplatform.performance.adapters.persistence.SuccessorEntity
import com.peopleplatform.performance.adapters.persistence.SuccessorKey
import com.peopleplatform.performance.adapters.persistence.SuccessorRepository
import com.peopleplatform.performance.adapters.persistence.TenantScope
import com.peopleplatform.performance.domain.evaluation.CompetencyGap
import com.peopleplatform.performance.domain.evaluation.EvaluationConsolidator
import com.peopleplatform.performance.domain.ninebox.HistoryEntry
import com.peopleplatform.performance.domain.ninebox.Quadrant
import com.peopleplatform.performance.domain.pdi.PdiActionKind
import com.peopleplatform.performance.domain.pdi.PdiActionStatus
import com.peopleplatform.performance.domain.pdi.PdiAdvisor
import com.peopleplatform.performance.domain.pdi.PdiSuggestion
import com.peopleplatform.performance.domain.shared.CycleId
import com.peopleplatform.performance.domain.shared.DomainException
import com.peopleplatform.performance.domain.shared.EmployeeId
import com.peopleplatform.performance.domain.succession.BenchStrength
import com.peopleplatform.performance.domain.succession.CriticalPosition
import com.peopleplatform.performance.domain.succession.Readiness
import com.peopleplatform.performance.domain.succession.ReadinessAdvisor
import com.peopleplatform.performance.domain.succession.SuccessionPlan
import org.springframework.stereotype.Service
import java.time.Clock
import java.time.LocalDate
import java.time.Period
import java.time.ZoneOffset
import java.util.UUID

data class SuccessorView(val employeeId: UUID, val employeeName: String, val readiness: Readiness)

data class PlanView(
    val positionId: UUID,
    val title: String,
    val incumbentId: UUID?,
    val incumbentName: String?,
    val benchStrength: BenchStrength,
    val successors: List<SuccessorView>,
)

@Service
class SuccessionService(
    private val scope: TenantScope,
    private val positions: PositionRepository,
    private val successors: SuccessorRepository,
    private val employees: EmployeeRepository,
    private val history: HistoryRepository,
    private val clock: Clock,
) {
    fun list(caller: Caller): List<PlanView> = scope.run(caller.tenantId) {
        val names = names()
        positions.findAllByOrderByCreatedAtAsc().map { plan(caller, it, names) }
    }

    fun create(caller: Caller, title: String, incumbent: UUID?): PlanView = scope.run(caller.tenantId) {
        val position = CriticalPosition(UUID.randomUUID(), caller.tenantId, title.trim(), incumbent?.let(::EmployeeId))
        val entity = positions.save(
            PositionEntity(position.id, caller.tenantId.value, position.title, incumbent, clock.instant()),
        )
        plan(caller, entity, names())
    }

    fun nominate(caller: Caller, positionId: UUID, employee: UUID, readiness: Readiness): PlanView =
        scope.run(caller.tenantId) {
            val entity = positions.findById(positionId).orElseThrow { NotFoundException("posição não encontrada") }
            if (!employees.existsById(employee)) throw DomainException("colaborador não cadastrado neste tenant")
            if (successors.existsById(SuccessorKey(positionId, employee))) {
                throw ConflictException("colaborador já é sucessor desta posição")
            }
            val domain = rebuild(caller, entity)
            val nominated = domain.nominate(EmployeeId(employee), readiness, caller.employeeId, clock.instant())
            successors.save(
                SuccessorEntity(
                    positionId,
                    employee,
                    caller.tenantId.value,
                    nominated.readiness,
                    caller.employeeId.value,
                    nominated.at,
                ),
            )
            plan(caller, entity, names())
        }

    /** Null quando o potencial é baixo ou quando a pessoa ainda não tem nine box fechado. */
    fun suggestReadiness(caller: Caller, positionId: UUID, employee: UUID): Readiness? = scope.run(caller.tenantId) {
        positions.findById(positionId).orElseThrow { NotFoundException("posição não encontrada") }
        val person = employees.findById(employee).orElseThrow { NotFoundException("colaborador não encontrado") }
        val latest = history.findByEmployeeIdOrderByClosedAtAsc(employee).lastOrNull() ?: return@run null
        val months = Period.between(person.roleSince, LocalDate.now(clock.withZone(ZoneOffset.UTC))).toTotalMonths()
        ReadinessAdvisor.suggest(
            HistoryEntry(
                CycleId(latest.cycleId),
                Quadrant.valueOf(latest.quadrant),
                latest.calibrated,
                latest.closedAt,
            ),
            months.toInt().coerceAtLeast(0),
        )
    }

    private fun names() = employees.findAll().associate { it.id to it.name }

    private fun rebuild(caller: Caller, entity: PositionEntity): SuccessionPlan {
        val domain = SuccessionPlan(
            CriticalPosition(entity.id, caller.tenantId, entity.title, entity.incumbentId?.let(::EmployeeId)),
        )
        successors.findByPositionIdOrderByAtAsc(entity.id).forEach {
            domain.nominate(EmployeeId(it.employeeId), it.readiness, EmployeeId(it.nominatedBy), it.at)
        }
        return domain
    }

    private fun plan(caller: Caller, entity: PositionEntity, names: Map<UUID, String>): PlanView {
        val domain = rebuild(caller, entity)
        return PlanView(
            entity.id,
            entity.title,
            entity.incumbentId,
            entity.incumbentId?.let { names[it] },
            domain.benchStrength(),
            domain.ranked.map {
                SuccessorView(it.employeeId.value, names[it.employeeId.value] ?: "Colaborador", it.readiness)
            },
        )
    }
}

data class PdiActionView(
    val id: UUID,
    val employeeId: UUID,
    val title: String,
    val competency: String?,
    val kind: PdiActionKind,
    val dueOn: LocalDate,
    val status: PdiActionStatus,
)

data class PdiView(
    val employeeId: UUID,
    val gaps: List<CompetencyGap>,
    val actions: List<PdiActionView>,
    val suggestions: List<PdiSuggestion>,
)

@Service
class PdiService(
    private val scope: TenantScope,
    private val actions: PdiActionRepository,
    private val evaluations: EvaluationRepository,
    private val history: HistoryRepository,
    private val evaluationService: EvaluationService,
    private val clock: Clock,
) {
    fun get(caller: Caller, employee: UUID): PdiView = scope.run(caller.tenantId) {
        authorize(caller, employee)
        PdiView(
            employee,
            latestGaps(employee),
            actions.findByEmployeeIdOrderByDueOnAsc(employee).map { it.view() },
            PdiAdvisor.suggest(
                latestGaps(employee),
                history.findByEmployeeIdOrderByClosedAtAsc(employee).lastOrNull()?.let {
                    Quadrant.valueOf(it.quadrant)
                },
            ),
        )
    }

    fun add(
        caller: Caller,
        employee: UUID,
        title: String,
        competency: String?,
        kind: PdiActionKind,
        dueOn: LocalDate,
    ): PdiActionView =
        scope.run(caller.tenantId) {
            authorize(caller, employee)
            if (title.trim().length < 3) throw DomainException("título da ação precisa de ao menos 3 caracteres")
            if (dueOn.isBefore(LocalDate.now(clock.withZone(ZoneOffset.UTC)))) {
                throw DomainException("prazo da ação não pode estar no passado")
            }
            actions.save(
                PdiActionEntity(
                    UUID.randomUUID(),
                    caller.tenantId.value,
                    employee,
                    title.trim(),
                    competency?.trim()?.ifBlank { null },
                    kind,
                    dueOn,
                    PdiActionStatus.PLANNED,
                    caller.employeeId.value,
                    clock.instant(),
                ),
            ).view()
        }

    fun updateStatus(caller: Caller, id: UUID, status: PdiActionStatus): PdiActionView = scope.run(caller.tenantId) {
        val entity = actions.findById(id).orElseThrow { NotFoundException("ação não encontrada") }
        authorize(caller, entity.employeeId)
        entity.status = entity.status.moveTo(status)
        actions.save(entity).view()
    }

    private fun authorize(caller: Caller, employee: UUID) {
        if (!caller.canSee(EmployeeId(employee))) throw ForbiddenException("sem permissão sobre o PDI desta pessoa")
    }

    /** Gaps do ciclo mais recente em que a pessoa tem autoavaliação e avaliação do gestor. */
    private fun latestGaps(employee: UUID): List<CompetencyGap> {
        val byCycle = evaluations.findByEmployeeIdOrderBySubmittedAtDesc(employee).groupBy { it.cycleId }
        val pair = byCycle.values.firstNotNullOfOrNull { rows ->
            val self = rows.firstOrNull { it.kind == "self" }
            val manager = rows.firstOrNull { it.kind == "manager" }
            if (self != null && manager != null) self to manager else null
        } ?: return emptyList()
        val (self, manager) = pair
        return EvaluationConsolidator.consolidate(
            self.performanceScore,
            evaluationService.competenciesOf(self),
            manager.performanceScore,
            manager.potentialScore ?: manager.performanceScore,
            evaluationService.competenciesOf(manager),
        ).gaps
    }

    private fun PdiActionEntity.view() = PdiActionView(id, employeeId, title, competency, kind, dueOn, status)
}
