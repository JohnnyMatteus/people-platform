package com.peopleplatform.performance.app

import com.fasterxml.jackson.core.type.TypeReference
import com.fasterxml.jackson.databind.ObjectMapper
import com.peopleplatform.performance.adapters.persistence.CycleEntity
import com.peopleplatform.performance.adapters.persistence.CycleRepository
import com.peopleplatform.performance.adapters.persistence.EmployeeEntity
import com.peopleplatform.performance.adapters.persistence.EmployeeRepository
import com.peopleplatform.performance.adapters.persistence.EvaluationEntity
import com.peopleplatform.performance.adapters.persistence.EvaluationRepository
import com.peopleplatform.performance.adapters.persistence.PolicyEntity
import com.peopleplatform.performance.adapters.persistence.PolicyRepository
import com.peopleplatform.performance.adapters.persistence.TenantScope
import com.peopleplatform.performance.adapters.persistence.copyFrom
import com.peopleplatform.performance.adapters.persistence.toDomain
import com.peopleplatform.performance.domain.evaluation.CompetencyScore
import com.peopleplatform.performance.domain.evaluation.CycleStatus
import com.peopleplatform.performance.domain.evaluation.EvaluationConsolidator
import com.peopleplatform.performance.domain.evaluation.EvaluationKind
import com.peopleplatform.performance.domain.evaluation.requireScoreInRange
import com.peopleplatform.performance.domain.ninebox.NineBoxPolicy
import com.peopleplatform.performance.domain.shared.DomainException
import org.springframework.stereotype.Service
import java.math.BigDecimal
import java.time.Clock
import java.time.LocalDate
import java.util.UUID

enum class CycleType(val wire: String) {
    SELF("self"),
    MANAGER("manager"),
    PEERS("peers"),
    D180("180"),
    D360("360"),
    ;

    companion object {
        fun fromWire(raw: String): CycleType =
            entries.firstOrNull { it.wire == raw } ?: throw DomainException("tipo de ciclo '$raw' inválido")
    }
}

data class CycleView(val id: UUID, val name: String, val type: String, val status: String)

data class EmployeeView(val id: UUID, val name: String, val jobTitle: String, val roleSince: LocalDate)

data class EvaluationInput(
    val employeeId: UUID?,
    val performanceScore: String,
    val potentialScore: String?,
    val competencies: List<CompetencyScore>,
    val comment: String?,
)

data class EvaluationView(
    val id: UUID,
    val cycleId: UUID,
    val employeeId: UUID,
    val kind: String,
    val performanceScore: String,
    val potentialScore: String?,
    val competencies: List<CompetencyScore>,
    val comment: String?,
    val submittedAt: java.time.Instant,
)

data class ProgressView(
    val employeeId: UUID,
    val employeeName: String,
    val selfSubmitted: Boolean,
    val managerSubmitted: Boolean,
    val consolidated: Boolean,
)

@Service
class PolicyService(private val scope: TenantScope, private val policies: PolicyRepository, private val clock: Clock) {
    fun get(caller: Caller): NineBoxPolicy = scope.run(caller.tenantId) { current(caller) }

    fun set(caller: Caller, policy: NineBoxPolicy): NineBoxPolicy = scope.run(caller.tenantId) {
        val entity = policies.findById(caller.tenantId.value).orElse(null)
            ?: PolicyEntity(
                caller.tenantId.value, BigDecimal.ONE, BigDecimal.ONE, BigDecimal.ONE, BigDecimal.ONE,
                BigDecimal.ONE, BigDecimal.ONE, 1, BigDecimal.ONE, 0, clock.instant(),
            )
        entity.copyFrom(policy.copy(tenantId = caller.tenantId))
        entity.updatedAt = clock.instant()
        policies.save(entity).toDomain(caller.tenantId)
    }

    /** Sem política gravada vale o padrão da plataforma. Chamar dentro de uma transação de tenant. */
    fun current(caller: Caller): NineBoxPolicy =
        policies.findById(caller.tenantId.value).map { it.toDomain(caller.tenantId) }
            .orElseGet { NineBoxPolicy.default(caller.tenantId) }
}

@Service
class PeopleService(
    private val scope: TenantScope,
    private val employees: EmployeeRepository,
) {
    fun list(caller: Caller): List<EmployeeView> = scope.run(caller.tenantId) {
        employees.findAllByOrderByName().map { it.view() }
    }

    fun upsert(caller: Caller, view: EmployeeView): EmployeeView = scope.run(caller.tenantId) {
        if (view.name.trim().length < 2) throw DomainException("nome do colaborador é obrigatório")
        if (view.jobTitle.isBlank()) throw DomainException("cargo é obrigatório")
        val entity = employees.findById(view.id).orElse(null)
            ?: EmployeeEntity(view.id, caller.tenantId.value, view.name.trim(), view.jobTitle.trim(), view.roleSince)
        entity.name = view.name.trim()
        entity.jobTitle = view.jobTitle.trim()
        entity.roleSince = view.roleSince
        employees.save(entity).view()
    }

    private fun EmployeeEntity.view() = EmployeeView(id, name, jobTitle, roleSince)
}

@Service
class CycleService(
    private val scope: TenantScope,
    private val cycles: CycleRepository,
    private val clock: Clock,
) {
    fun list(caller: Caller): List<CycleView> = scope.run(caller.tenantId) {
        cycles.findAllByOrderByCreatedAtDesc().map { it.view() }
    }

    fun create(caller: Caller, name: String, type: CycleType): CycleView = scope.run(caller.tenantId) {
        if (name.trim().length < 3) throw DomainException("nome do ciclo precisa de ao menos 3 caracteres")
        cycles.save(
            CycleEntity(UUID.randomUUID(), caller.tenantId.value, name.trim(), type.wire, "draft", clock.instant()),
        ).view()
    }

    fun open(caller: Caller, id: UUID): CycleView = scope.run(caller.tenantId) {
        val cycle = cycles.findById(id).orElseThrow { NotFoundException("ciclo não encontrado") }
        if (CycleStatus.valueOf(cycle.status.uppercase()) != CycleStatus.DRAFT) {
            throw ConflictException("ciclo já foi aberto")
        }
        cycle.status = "open"
        cycles.save(cycle).view()
    }

    private fun CycleEntity.view() = CycleView(id, name, type, status)
}

@Service
class EvaluationService(
    private val scope: TenantScope,
    private val cycles: CycleRepository,
    private val evaluations: EvaluationRepository,
    private val employees: EmployeeRepository,
    private val policies: PolicyService,
    private val events: DomainEvents,
    private val json: ObjectMapper,
    private val clock: Clock,
) {
    fun submitSelf(caller: Caller, cycleId: UUID, input: EvaluationInput): EvaluationView =
        scope.run(caller.tenantId) {
            val policy = policies.current(caller)
            ensureOpen(cycleId)
            val me = caller.employeeId.value
            if (!employees.existsById(me)) {
                employees.save(
                    EmployeeEntity(me, caller.tenantId.value, caller.name ?: "Colaborador", "Cargo não informado", LocalDate.now(clock)),
                )
            }
            val performance = parseDecimal(input.performanceScore, "nota de desempenho")
            requireScoreInRange(performance, policy.scale.min, policy.scale.max, "desempenho")
            val saved = save(caller, cycleId, me, EvaluationKind.SELF, performance, null, input)
            consolidateIfComplete(caller, cycleId, me)
            saved
        }

    fun submitManager(caller: Caller, cycleId: UUID, input: EvaluationInput): EvaluationView =
        scope.run(caller.tenantId) {
            val policy = policies.current(caller)
            ensureOpen(cycleId)
            val target = input.employeeId ?: throw DomainException("employee_id é obrigatório")
            if (target == caller.employeeId.value) throw DomainException("ninguém avalia a si mesmo como gestor")
            if (!employees.existsById(target)) throw DomainException("colaborador não cadastrado neste tenant")
            val performance = parseDecimal(input.performanceScore, "nota de desempenho")
            val potential = parseDecimal(input.potentialScore ?: throw DomainException("nota de potencial é obrigatória"), "nota de potencial")
            requireScoreInRange(performance, policy.scale.min, policy.scale.max, "desempenho")
            requireScoreInRange(potential, policy.scale.min, policy.scale.max, "potencial")
            val saved = save(caller, cycleId, target, EvaluationKind.MANAGER, performance, potential, input)
            consolidateIfComplete(caller, cycleId, target)
            saved
        }

    fun progress(caller: Caller, cycleId: UUID): List<ProgressView> = scope.run(caller.tenantId) {
        cycles.findById(cycleId).orElseThrow { NotFoundException("ciclo não encontrado") }
        val names = employees.findAll().associate { it.id to it.name }
        evaluations.findByCycleId(cycleId).groupBy { it.employeeId }.map { (employee, rows) ->
            val self = rows.any { it.kind == "self" }
            val manager = rows.any { it.kind == "manager" }
            ProgressView(employee, names[employee] ?: "Colaborador", self, manager, self && manager)
        }.sortedBy { it.employeeName }
    }

    private fun ensureOpen(cycleId: UUID) {
        val cycle = cycles.findById(cycleId).orElseThrow { NotFoundException("ciclo não encontrado") }
        if (cycle.status != "open") throw ConflictException("ciclo não está aberto para avaliações")
    }

    private fun save(
        caller: Caller,
        cycleId: UUID,
        employee: UUID,
        kind: EvaluationKind,
        performance: BigDecimal,
        potential: BigDecimal?,
        input: EvaluationInput,
    ): EvaluationView {
        val wire = kind.name.lowercase()
        if (evaluations.findByCycleIdAndEmployeeIdAndKind(cycleId, employee, wire) != null) {
            throw ConflictException("avaliação ($wire) já enviada para esta pessoa neste ciclo")
        }
        val entity = evaluations.save(
            EvaluationEntity(
                UUID.randomUUID(), caller.tenantId.value, cycleId, employee, wire, caller.employeeId.value,
                performance, potential, json.writeValueAsString(input.competencies), input.comment?.trim(), clock.instant(),
            ),
        )
        return EvaluationView(
            entity.id, cycleId, employee, wire, performance.toPlainString(), potential?.toPlainString(),
            input.competencies, entity.comment, entity.submittedAt,
        )
    }

    /** Com as duas pontas enviadas, o ciclo daquela pessoa está consolidado e o resto da plataforma é avisado. */
    private fun consolidateIfComplete(caller: Caller, cycleId: UUID, employee: UUID) {
        val self = evaluations.findByCycleIdAndEmployeeIdAndKind(cycleId, employee, "self") ?: return
        val manager = evaluations.findByCycleIdAndEmployeeIdAndKind(cycleId, employee, "manager") ?: return
        val result = EvaluationConsolidator.consolidate(
            self.performanceScore, competenciesOf(self), manager.performanceScore,
            manager.potentialScore ?: return, competenciesOf(manager),
        )
        events.publish(
            caller.tenantId, "evaluation", employee, "AvaliacaoConcluida",
            mapOf(
                "cycle_id" to cycleId.toString(),
                "employee_id" to employee.toString(),
                "performance_score" to result.performance.toPlainString(),
                "potential_score" to result.potential.toPlainString(),
                "competency_gaps" to result.gaps.map { mapOf("competency" to it.competency, "gap" to it.gap) },
            ),
            clock.instant(),
        )
    }

    fun competenciesOf(e: EvaluationEntity): List<CompetencyScore> =
        json.readValue(e.competencies, object : TypeReference<List<CompetencyScore>>() {})
}
