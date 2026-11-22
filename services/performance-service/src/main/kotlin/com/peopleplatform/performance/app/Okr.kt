package com.peopleplatform.performance.app

import com.peopleplatform.performance.adapters.persistence.CheckInEntity
import com.peopleplatform.performance.adapters.persistence.CheckInRepository
import com.peopleplatform.performance.adapters.persistence.EmployeeRepository
import com.peopleplatform.performance.adapters.persistence.KeyResultEntity
import com.peopleplatform.performance.adapters.persistence.KeyResultRepository
import com.peopleplatform.performance.adapters.persistence.ObjectiveEntity
import com.peopleplatform.performance.adapters.persistence.ObjectiveRepository
import com.peopleplatform.performance.adapters.persistence.TenantScope
import com.peopleplatform.performance.domain.okr.KeyResultSpec
import com.peopleplatform.performance.domain.okr.ObjectiveLevel
import com.peopleplatform.performance.domain.okr.ObjectiveStatus
import com.peopleplatform.performance.domain.okr.OkrProgress
import com.peopleplatform.performance.domain.okr.requireCascade
import com.peopleplatform.performance.domain.shared.DomainException
import com.peopleplatform.performance.domain.shared.EmployeeId
import org.springframework.stereotype.Service
import java.math.BigDecimal
import java.time.Clock
import java.time.Instant
import java.util.UUID

data class KeyResultView(
    val id: UUID,
    val title: String,
    val start: BigDecimal,
    val target: BigDecimal,
    val current: BigDecimal,
    val progress: BigDecimal,
)

data class ObjectiveView(
    val id: UUID,
    val level: ObjectiveLevel,
    val ownerId: UUID?,
    val ownerName: String?,
    val parentId: UUID?,
    val title: String,
    val status: ObjectiveStatus,
    val progress: BigDecimal,
    val keyResults: List<KeyResultView>,
)

data class CheckInView(
    val id: UUID,
    val keyResultId: UUID,
    val value: BigDecimal,
    val comment: String?,
    val createdBy: UUID,
    val createdAt: Instant,
    val keyResult: KeyResultView,
)

@Service
class ObjectiveService(
    private val scope: TenantScope,
    private val objectives: ObjectiveRepository,
    private val keyResults: KeyResultRepository,
    private val checkIns: CheckInRepository,
    private val employees: EmployeeRepository,
    private val clock: Clock,
) {
    fun create(
        caller: Caller,
        level: ObjectiveLevel,
        ownerId: UUID?,
        parentId: UUID?,
        title: String,
        specs: List<KeyResultSpec>,
    ): ObjectiveView = scope.run(caller.tenantId) {
        if (title.trim().length < 3) throw DomainException("título do objetivo precisa de ao menos 3 caracteres")
        if (specs.isEmpty() || specs.size > MAX_KEY_RESULTS) {
            throw DomainException("o objetivo precisa de 1 a $MAX_KEY_RESULTS resultados-chave")
        }
        val owner = authorizeCreation(caller, level, ownerId)
        // Dono de outra pessoa tem de ser colaborador cadastrado neste tenant (a RLS esconde os de outros).
        if (owner != null && owner != caller.employeeId.value && !employees.existsById(owner)) {
            throw NotFoundException("colaborador não encontrado")
        }
        val parent = parentId?.let {
            objectives.findById(it).orElseThrow { NotFoundException("objetivo pai não encontrado") }
        }
        requireCascade(level, parent?.level)
        if (parent != null && parent.status == ObjectiveStatus.CLOSED) {
            throw ConflictException("o objetivo pai está encerrado")
        }
        val objective = objectives.save(
            ObjectiveEntity(
                UUID.randomUUID(),
                caller.tenantId.value,
                level,
                owner,
                parentId,
                title.trim(),
                ObjectiveStatus.ACTIVE,
                caller.employeeId.value,
                clock.instant(),
            ),
        )
        val saved = specs.map {
            keyResults.save(
                KeyResultEntity(
                    UUID.randomUUID(),
                    caller.tenantId.value,
                    objective.id,
                    it.title.trim(),
                    it.start,
                    it.target,
                    it.start,
                ),
            )
        }
        view(objective, saved, names())
    }

    fun list(caller: Caller, level: ObjectiveLevel?, ownerId: UUID?): List<ObjectiveView> = scope.run(caller.tenantId) {
        val visible = objectives.findAllByOrderByCreatedAtAsc()
            .filter { level == null || it.level == level }
            .filter { ownerId == null || it.ownerId == ownerId }
            .filter { canSee(caller, it) }
        val byObjective = keyResults.findByObjectiveIdIn(visible.map { it.id }).groupBy { it.objectiveId }
        val names = names()
        visible.map { view(it, byObjective[it.id].orEmpty(), names) }
    }

    fun get(caller: Caller, id: UUID): ObjectiveView = scope.run(caller.tenantId) {
        val objective = visibleObjective(caller, id)
        view(objective, keyResults.findByObjectiveIdIn(listOf(id)), names())
    }

    fun checkIn(caller: Caller, keyResultId: UUID, value: BigDecimal, comment: String?): CheckInView =
        scope.run(caller.tenantId) {
            val keyResult = keyResults.findById(keyResultId).orElseThrow {
                NotFoundException("resultado-chave não encontrado")
            }
            val objective = visibleObjective(caller, keyResult.objectiveId)
            if (!canWrite(caller, objective)) throw ForbiddenException("sem permissão para atualizar este objetivo")
            if (objective.status == ObjectiveStatus.CLOSED) throw ConflictException("o objetivo está encerrado")
            val saved = checkIns.save(
                CheckInEntity(
                    UUID.randomUUID(),
                    caller.tenantId.value,
                    keyResultId,
                    value,
                    comment?.trim()?.ifBlank { null },
                    caller.employeeId.value,
                    clock.instant(),
                ),
            )
            keyResult.currentValue = value
            keyResults.save(keyResult)
            CheckInView(
                saved.id,
                keyResultId,
                value,
                saved.comment,
                saved.createdBy,
                saved.createdAt,
                keyResultView(keyResult),
            )
        }

    fun close(caller: Caller, id: UUID): ObjectiveView = scope.run(caller.tenantId) {
        val objective = visibleObjective(caller, id)
        if (!canWrite(caller, objective)) throw ForbiddenException("sem permissão para encerrar este objetivo")
        objective.status = ObjectiveStatus.CLOSED
        view(objectives.save(objective), keyResults.findByObjectiveIdIn(listOf(id)), names())
    }

    /** Devolve o dono: empresa não tem; equipe é de quem lidera; individual é da pessoa ou de quem gere. */
    private fun authorizeCreation(caller: Caller, level: ObjectiveLevel, ownerId: UUID?): UUID? = when (level) {
        ObjectiveLevel.COMPANY -> {
            if (!caller.isAdmin) throw ForbiddenException("só o admin do tenant define objetivos da empresa")
            if (ownerId != null) throw DomainException("objetivo da empresa não tem dono")
            null
        }
        ObjectiveLevel.TEAM -> {
            if (!caller.isManager) throw ForbiddenException("só gestor ou admin define objetivos de equipe")
            ownerId ?: caller.employeeId.value
        }
        ObjectiveLevel.INDIVIDUAL -> {
            val owner = ownerId ?: caller.employeeId.value
            if (owner != caller.employeeId.value && !caller.isManager) {
                throw ForbiddenException("só gestor ou admin define objetivos de outra pessoa")
            }
            owner
        }
    }

    /** Empresa e equipe são transparentes ao tenant; individual só à própria pessoa e a quem gere. */
    private fun canSee(caller: Caller, o: ObjectiveEntity): Boolean =
        o.level != ObjectiveLevel.INDIVIDUAL || caller.canSee(EmployeeId(o.ownerId ?: caller.employeeId.value))

    private fun canWrite(caller: Caller, o: ObjectiveEntity): Boolean = when (o.level) {
        ObjectiveLevel.COMPANY -> caller.isAdmin
        else -> caller.isManager || o.ownerId == caller.employeeId.value
    }

    private fun visibleObjective(caller: Caller, id: UUID): ObjectiveEntity {
        val objective = objectives.findById(id).orElseThrow { NotFoundException("objetivo não encontrado") }
        if (!canSee(caller, objective)) throw NotFoundException("objetivo não encontrado")
        return objective
    }

    private fun names(): Map<UUID, String> = employees.findAll().associate { it.id to it.name }

    private fun keyResultView(k: KeyResultEntity) = KeyResultView(
        k.id,
        k.title,
        k.startValue,
        k.targetValue,
        k.currentValue,
        OkrProgress.keyResult(k.startValue, k.targetValue, k.currentValue),
    )

    private fun view(o: ObjectiveEntity, krs: List<KeyResultEntity>, names: Map<UUID, String>): ObjectiveView {
        val views = krs.map(::keyResultView)
        return ObjectiveView(
            o.id,
            o.level,
            o.ownerId,
            o.ownerId?.let(names::get),
            o.parentId,
            o.title,
            o.status,
            OkrProgress.objective(views.map { it.progress }),
            views,
        )
    }

    companion object {
        const val MAX_KEY_RESULTS = 5
    }
}
