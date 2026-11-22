package com.peopleplatform.performance.adapters.persistence

import com.peopleplatform.performance.domain.ninebox.AxisThresholds
import com.peopleplatform.performance.domain.ninebox.NineBoxPolicy
import com.peopleplatform.performance.domain.ninebox.Scale
import com.peopleplatform.performance.domain.pdi.PdiActionKind
import com.peopleplatform.performance.domain.pdi.PdiActionStatus
import com.peopleplatform.performance.domain.shared.TenantId
import com.peopleplatform.performance.domain.succession.Readiness
import jakarta.persistence.Column
import jakarta.persistence.Entity
import jakarta.persistence.EnumType
import jakarta.persistence.Enumerated
import jakarta.persistence.Id
import jakarta.persistence.IdClass
import jakarta.persistence.Table
import org.hibernate.annotations.ColumnTransformer
import java.io.Serializable
import java.math.BigDecimal
import java.time.Instant
import java.time.LocalDate
import java.util.UUID

/** Colunas de política copiadas em `calibration_sessions` na abertura. */
interface PolicyColumns {
    var scaleMin: BigDecimal
    var scaleMax: BigDecimal
    var perfMediumFrom: BigDecimal
    var perfHighFrom: BigDecimal
    var potMediumFrom: BigDecimal
    var potHighFrom: BigDecimal
    var maxStepsPerAxis: Int
    var topBoxWarningShare: BigDecimal
    var minJustificationLength: Int
}

fun PolicyColumns.toDomain(tenantId: TenantId) = NineBoxPolicy(
    tenantId = tenantId,
    scale = Scale(scaleMin, scaleMax),
    performance = AxisThresholds(perfMediumFrom, perfHighFrom),
    potential = AxisThresholds(potMediumFrom, potHighFrom),
    maxStepsPerAxis = maxStepsPerAxis,
    topBoxWarningShare = topBoxWarningShare,
    minJustificationLength = minJustificationLength,
)

fun PolicyColumns.copyFrom(p: NineBoxPolicy) {
    scaleMin = p.scale.min
    scaleMax = p.scale.max
    perfMediumFrom = p.performance.mediumFrom
    perfHighFrom = p.performance.highFrom
    potMediumFrom = p.potential.mediumFrom
    potHighFrom = p.potential.highFrom
    maxStepsPerAxis = p.maxStepsPerAxis
    topBoxWarningShare = p.topBoxWarningShare
    minJustificationLength = p.minJustificationLength
}

@Entity
@Table(name = "nine_box_policies")
class PolicyEntity(
    @Id @Column(name = "tenant_id") var tenantId: UUID,
    override var scaleMin: BigDecimal,
    override var scaleMax: BigDecimal,
    override var perfMediumFrom: BigDecimal,
    override var perfHighFrom: BigDecimal,
    override var potMediumFrom: BigDecimal,
    override var potHighFrom: BigDecimal,
    override var maxStepsPerAxis: Int,
    override var topBoxWarningShare: BigDecimal,
    override var minJustificationLength: Int,
    var updatedAt: Instant,
) : PolicyColumns

@Entity
@Table(name = "employees")
class EmployeeEntity(
    @Id var id: UUID,
    var tenantId: UUID,
    var name: String,
    var jobTitle: String,
    var roleSince: LocalDate,
)

@Entity
@Table(name = "cycles")
class CycleEntity(
    @Id var id: UUID,
    var tenantId: UUID,
    var name: String,
    var type: String,
    var status: String,
    var createdAt: Instant,
)

@Entity
@Table(name = "evaluations")
class EvaluationEntity(
    @Id var id: UUID,
    var tenantId: UUID,
    var cycleId: UUID,
    var employeeId: UUID,
    var kind: String,
    var evaluatorId: UUID,
    var performanceScore: BigDecimal,
    var potentialScore: BigDecimal?,
    @Column(columnDefinition = "jsonb") @ColumnTransformer(write = "?::jsonb") var competencies: String,
    var comment: String?,
    var submittedAt: Instant,
)

@Entity
@Table(name = "calibration_sessions")
class SessionEntity(
    @Id var id: UUID,
    var tenantId: UUID,
    var cycleId: UUID,
    var status: String,
    override var scaleMin: BigDecimal,
    override var scaleMax: BigDecimal,
    override var perfMediumFrom: BigDecimal,
    override var perfHighFrom: BigDecimal,
    override var potMediumFrom: BigDecimal,
    override var potHighFrom: BigDecimal,
    override var maxStepsPerAxis: Int,
    override var topBoxWarningShare: BigDecimal,
    override var minJustificationLength: Int,
    var openedAt: Instant,
    var closedAt: Instant?,
) : PolicyColumns

data class PlacementKey(var sessionId: UUID? = null, var employeeId: UUID? = null) : Serializable

@Entity
@Table(name = "calibration_placements")
@IdClass(PlacementKey::class)
class PlacementEntity(
    @Id var sessionId: UUID,
    @Id var employeeId: UUID,
    var tenantId: UUID,
    var performanceScore: BigDecimal,
    var potentialScore: BigDecimal,
)

@Entity
@Table(name = "calibration_adjustments")
class AdjustmentEntity(
    @Id var id: UUID,
    var tenantId: UUID,
    var sessionId: UUID,
    var employeeId: UUID,
    var fromQuadrant: String,
    var toQuadrant: String,
    var calibratedBy: UUID,
    var justification: String,
    var at: Instant,
)

data class HistoryKey(var employeeId: UUID? = null, var cycleId: UUID? = null) : Serializable

@Entity
@Table(name = "nine_box_history")
@IdClass(HistoryKey::class)
class HistoryEntity(
    @Id var employeeId: UUID,
    @Id var cycleId: UUID,
    var tenantId: UUID,
    var quadrant: String,
    var calibrated: Boolean,
    var closedAt: Instant,
)

@Entity
@Table(name = "critical_positions")
class PositionEntity(
    @Id var id: UUID,
    var tenantId: UUID,
    var title: String,
    var incumbentId: UUID?,
    var createdAt: Instant,
)

data class SuccessorKey(var positionId: UUID? = null, var employeeId: UUID? = null) : Serializable

@Entity
@Table(name = "successors")
@IdClass(SuccessorKey::class)
class SuccessorEntity(
    @Id var positionId: UUID,
    @Id var employeeId: UUID,
    var tenantId: UUID,
    @Enumerated(EnumType.STRING) var readiness: Readiness,
    var nominatedBy: UUID,
    var at: Instant,
)

@Entity
@Table(name = "pdi_actions")
class PdiActionEntity(
    @Id var id: UUID,
    var tenantId: UUID,
    var employeeId: UUID,
    var title: String,
    var competency: String?,
    @Enumerated(EnumType.STRING) var kind: PdiActionKind,
    var dueOn: LocalDate,
    @Enumerated(EnumType.STRING) var status: PdiActionStatus,
    var createdBy: UUID,
    var createdAt: Instant,
)

@Entity
@Table(name = "outbox")
class OutboxEntity(
    @Id var id: UUID,
    var tenantId: UUID,
    var aggregateType: String,
    var aggregateId: UUID,
    var eventType: String,
    var eventVersion: Int,
    @Column(columnDefinition = "jsonb") @ColumnTransformer(write = "?::jsonb") var payload: String,
    var occurredAt: Instant,
    var publishedAt: Instant?,
    var attempts: Int,
    var lastError: String?,
)
