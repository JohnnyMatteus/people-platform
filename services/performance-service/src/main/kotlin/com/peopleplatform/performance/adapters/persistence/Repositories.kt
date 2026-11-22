package com.peopleplatform.performance.adapters.persistence

import jakarta.persistence.LockModeType
import org.springframework.data.jpa.repository.JpaRepository
import org.springframework.data.jpa.repository.Lock
import org.springframework.data.jpa.repository.Query
import org.springframework.data.repository.query.Param
import java.util.UUID

interface PolicyRepository : JpaRepository<PolicyEntity, UUID>

interface EmployeeRepository : JpaRepository<EmployeeEntity, UUID> {
    fun findAllByOrderByName(): List<EmployeeEntity>
}

interface CycleRepository : JpaRepository<CycleEntity, UUID> {
    fun findAllByOrderByCreatedAtDesc(): List<CycleEntity>
}

interface EvaluationRepository : JpaRepository<EvaluationEntity, UUID> {
    fun findByCycleId(cycleId: UUID): List<EvaluationEntity>

    fun findByCycleIdAndEmployeeIdAndKind(cycleId: UUID, employeeId: UUID, kind: String): EvaluationEntity?

    fun findByEmployeeIdOrderBySubmittedAtDesc(employeeId: UUID): List<EvaluationEntity>
}

interface SessionRepository : JpaRepository<SessionEntity, UUID> {
    fun findByCycleId(cycleId: UUID): SessionEntity?

    // Serializa ajustes e fechamento da mesma sessão: o estado é reconstruído a partir do log.
    @Lock(LockModeType.PESSIMISTIC_WRITE)
    @Query("select s from SessionEntity s where s.id = :id")
    fun lockById(@Param("id") id: UUID): SessionEntity?
}

interface PlacementRepository : JpaRepository<PlacementEntity, PlacementKey> {
    fun findBySessionId(sessionId: UUID): List<PlacementEntity>
}

interface AdjustmentRepository : JpaRepository<AdjustmentEntity, UUID> {
    fun findBySessionIdOrderByAtAscIdAsc(sessionId: UUID): List<AdjustmentEntity>
}

interface HistoryRepository : JpaRepository<HistoryEntity, HistoryKey> {
    fun findByEmployeeIdOrderByClosedAtAsc(employeeId: UUID): List<HistoryEntity>
}

interface PositionRepository : JpaRepository<PositionEntity, UUID> {
    fun findAllByOrderByCreatedAtAsc(): List<PositionEntity>
}

interface SuccessorRepository : JpaRepository<SuccessorEntity, SuccessorKey> {
    fun findByPositionIdOrderByAtAsc(positionId: UUID): List<SuccessorEntity>
}

interface PdiActionRepository : JpaRepository<PdiActionEntity, UUID> {
    fun findByEmployeeIdOrderByDueOnAsc(employeeId: UUID): List<PdiActionEntity>
}

interface OutboxRepository : JpaRepository<OutboxEntity, UUID> {
    @Query(
        """
        select * from outbox
        where published_at is null and tenant_id not in (:blocked)
        order by occurred_at, id
        limit :n
        for update skip locked
        """,
        nativeQuery = true,
    )
    fun lockPending(
        @Param("n") n: Int,
        @Param("blocked") blocked: Collection<UUID>,
    ): List<OutboxEntity>
}
