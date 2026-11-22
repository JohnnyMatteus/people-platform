package com.peopleplatform.performance.adapters.web

import com.fasterxml.jackson.annotation.JsonInclude
import com.peopleplatform.performance.app.CalibrationService
import com.peopleplatform.performance.app.CycleService
import com.peopleplatform.performance.app.CycleType
import com.peopleplatform.performance.app.CycleView
import com.peopleplatform.performance.app.EmployeeView
import com.peopleplatform.performance.app.EvaluationInput
import com.peopleplatform.performance.app.EvaluationService
import com.peopleplatform.performance.app.EvaluationView
import com.peopleplatform.performance.app.PdiActionView
import com.peopleplatform.performance.app.PdiService
import com.peopleplatform.performance.app.PeopleService
import com.peopleplatform.performance.app.PlanView
import com.peopleplatform.performance.app.PolicyService
import com.peopleplatform.performance.app.ProgressView
import com.peopleplatform.performance.app.SessionView
import com.peopleplatform.performance.app.SuccessionService
import com.peopleplatform.performance.app.parseDecimal
import com.peopleplatform.performance.domain.evaluation.CompetencyScore
import com.peopleplatform.performance.domain.ninebox.AxisThresholds
import com.peopleplatform.performance.domain.ninebox.CalibrationAdjustment
import com.peopleplatform.performance.domain.ninebox.CalibrationWarning
import com.peopleplatform.performance.domain.ninebox.NineBoxPolicy
import com.peopleplatform.performance.domain.ninebox.Quadrant
import com.peopleplatform.performance.domain.ninebox.Scale
import com.peopleplatform.performance.domain.pdi.PdiActionKind
import com.peopleplatform.performance.domain.pdi.PdiActionStatus
import com.peopleplatform.performance.domain.succession.Readiness
import org.springframework.http.HttpStatus
import org.springframework.security.core.annotation.AuthenticationPrincipal
import org.springframework.security.oauth2.jwt.Jwt
import org.springframework.web.bind.annotation.GetMapping
import org.springframework.web.bind.annotation.PatchMapping
import org.springframework.web.bind.annotation.PathVariable
import org.springframework.web.bind.annotation.PostMapping
import org.springframework.web.bind.annotation.PutMapping
import org.springframework.web.bind.annotation.RequestBody
import org.springframework.web.bind.annotation.ResponseStatus
import org.springframework.web.bind.annotation.RestController
import java.time.Instant
import java.time.LocalDate
import java.util.UUID

// ---------------------------------------------------------------- DTOs do contrato (snake_case)

data class AxisBody(val mediumFrom: String, val highFrom: String)

data class PolicyBody(
    val scaleMin: String,
    val scaleMax: String,
    val performance: AxisBody,
    val potential: AxisBody,
    val maxStepsPerAxis: Int = 1,
    val topBoxWarningShare: String = "0.20",
    val minJustificationLength: Int = 20,
)

data class AdjustmentBody(val employeeId: UUID, val to: Quadrant, val justification: String)

data class AdjustmentOut(
    val employeeId: UUID,
    val from: Quadrant,
    val to: Quadrant,
    val calibratedBy: UUID,
    val justification: String,
    val at: Instant,
)

data class PlacementOut(
    val employeeId: UUID,
    val employeeName: String,
    val performanceScore: String,
    val potentialScore: String,
    val original: Quadrant,
    val current: Quadrant,
)

data class WarningOut(val code: String, val share: String, val guideline: String)

data class SessionOut(
    val id: UUID,
    val cycleId: UUID,
    val status: String,
    val policy: PolicyBody,
    val placements: List<PlacementOut>,
    val distribution: Map<String, Int>,
    val warnings: List<WarningOut>,
    val adjustments: List<AdjustmentOut>,
)

data class HistoryEntryOut(val cycleId: UUID, val quadrant: Quadrant, val calibrated: Boolean, val closedAt: Instant)

data class HistoryOut(val entries: List<HistoryEntryOut>, val trend: String)

data class MeOut(val employeeId: UUID, val tenantId: UUID, val name: String?, val roles: List<String>)

data class EmployeeBody(val id: UUID, val name: String, val jobTitle: String, val roleSince: LocalDate)

data class CycleBody(val name: String, val type: String)

data class SelfEvaluationBody(
    val performanceScore: String,
    val competencies: List<CompetencyScore>,
    val comment: String? = null,
)

data class ManagerEvaluationBody(
    val employeeId: UUID,
    val performanceScore: String,
    val potentialScore: String,
    val competencies: List<CompetencyScore>,
    val comment: String? = null,
)

data class PositionBody(val title: String, val incumbentId: UUID? = null)

data class NominationBody(val employeeId: UUID, val readiness: Readiness)

@JsonInclude(JsonInclude.Include.ALWAYS)
data class ReadinessOut(val suggestion: Readiness?)

data class PdiActionBody(val title: String, val kind: PdiActionKind, val dueOn: LocalDate, val competency: String? = null)

data class PdiStatusBody(val status: PdiActionStatus)

// ---------------------------------------------------------------- mapeamento

private fun PolicyBody.toDomain(caller: com.peopleplatform.performance.app.Caller) = NineBoxPolicy(
    tenantId = caller.tenantId,
    scale = Scale(parseDecimal(scaleMin, "scale_min"), parseDecimal(scaleMax, "scale_max")),
    performance = AxisThresholds(parseDecimal(performance.mediumFrom, "medium_from"), parseDecimal(performance.highFrom, "high_from")),
    potential = AxisThresholds(parseDecimal(potential.mediumFrom, "medium_from"), parseDecimal(potential.highFrom, "high_from")),
    maxStepsPerAxis = maxStepsPerAxis,
    topBoxWarningShare = parseDecimal(topBoxWarningShare, "top_box_warning_share"),
    minJustificationLength = minJustificationLength,
)

private fun NineBoxPolicy.toBody() = PolicyBody(
    scale.min.toPlainString(), scale.max.toPlainString(),
    AxisBody(performance.mediumFrom.toPlainString(), performance.highFrom.toPlainString()),
    AxisBody(potential.mediumFrom.toPlainString(), potential.highFrom.toPlainString()),
    maxStepsPerAxis, topBoxWarningShare.toPlainString(), minJustificationLength,
)

private fun CalibrationAdjustment.toOut() =
    AdjustmentOut(employeeId.value, from, to, calibratedBy.value, justification, at)

private fun SessionView.toOut() = SessionOut(
    id, cycleId, status, policy.toBody(),
    placements.map {
        PlacementOut(
            it.employeeId, it.employeeName, it.performanceScore.toPlainString(), it.potentialScore.toPlainString(),
            it.original, it.current,
        )
    },
    distribution.mapKeys { it.key.name },
    warnings.map {
        when (it) {
            is CalibrationWarning.TopBoxAboveGuideline ->
                WarningOut("TOP_BOX_ABOVE_GUIDELINE", it.share.toPlainString(), it.guideline.toPlainString())
        }
    },
    adjustments.map { it.toOut() },
)

private fun Jwt.caller() = toCaller()

// ---------------------------------------------------------------- controllers

@RestController
class NineBoxController(private val policies: PolicyService, private val calibration: CalibrationService) {
    @GetMapping("/v1/nine-box/policy")
    fun policy(@AuthenticationPrincipal jwt: Jwt) = policies.get(jwt.caller()).toBody()

    @PutMapping("/v1/nine-box/policy")
    fun setPolicy(@AuthenticationPrincipal jwt: Jwt, @RequestBody body: PolicyBody): PolicyBody {
        val caller = jwt.caller()
        return policies.set(caller, body.toDomain(caller)).toBody()
    }

    @PostMapping("/v1/cycles/{cycleId}/calibration-sessions")
    @ResponseStatus(HttpStatus.CREATED)
    fun open(@AuthenticationPrincipal jwt: Jwt, @PathVariable cycleId: UUID) =
        calibration.open(jwt.caller(), cycleId).toOut()

    @GetMapping("/v1/cycles/{cycleId}/calibration-sessions/current")
    fun byCycle(@AuthenticationPrincipal jwt: Jwt, @PathVariable cycleId: UUID) =
        calibration.getByCycle(jwt.caller(), cycleId).toOut()

    @GetMapping("/v1/calibration-sessions/{sessionId}")
    fun session(@AuthenticationPrincipal jwt: Jwt, @PathVariable sessionId: UUID) =
        calibration.get(jwt.caller(), sessionId).toOut()

    @PostMapping("/v1/calibration-sessions/{sessionId}/adjustments")
    @ResponseStatus(HttpStatus.CREATED)
    fun adjust(@AuthenticationPrincipal jwt: Jwt, @PathVariable sessionId: UUID, @RequestBody body: AdjustmentBody) =
        calibration.adjust(jwt.caller(), sessionId, body.employeeId, body.to, body.justification).toOut()

    @PostMapping("/v1/calibration-sessions/{sessionId}/close")
    fun close(@AuthenticationPrincipal jwt: Jwt, @PathVariable sessionId: UUID) =
        calibration.close(jwt.caller(), sessionId).toOut()

    @GetMapping("/v1/employees/{employeeId}/nine-box-history")
    fun history(@AuthenticationPrincipal jwt: Jwt, @PathVariable employeeId: UUID): HistoryOut {
        val result = calibration.history(jwt.caller(), employeeId)
        return HistoryOut(
            result.entries.map { HistoryEntryOut(it.cycleId.value, it.quadrant, it.calibrated, it.closedAt) },
            result.trend.name,
        )
    }
}

@RestController
class PeopleController(private val people: PeopleService) {
    @GetMapping("/v1/me")
    fun me(@AuthenticationPrincipal jwt: Jwt): MeOut {
        val c = jwt.caller()
        return MeOut(c.employeeId.value, c.tenantId.value, c.name, c.roles.sorted())
    }

    @GetMapping("/v1/employees")
    fun list(@AuthenticationPrincipal jwt: Jwt): List<EmployeeBody> =
        people.list(jwt.caller()).map { EmployeeBody(it.id, it.name, it.jobTitle, it.roleSince) }

    @PostMapping("/v1/employees")
    fun upsert(@AuthenticationPrincipal jwt: Jwt, @RequestBody body: EmployeeBody): EmployeeBody =
        people.upsert(jwt.caller(), EmployeeView(body.id, body.name, body.jobTitle, body.roleSince))
            .let { EmployeeBody(it.id, it.name, it.jobTitle, it.roleSince) }
}

@RestController
class CycleController(private val cycles: CycleService, private val evaluations: EvaluationService) {
    @GetMapping("/v1/cycles")
    fun list(@AuthenticationPrincipal jwt: Jwt): List<CycleView> = cycles.list(jwt.caller())

    @PostMapping("/v1/cycles")
    @ResponseStatus(HttpStatus.CREATED)
    fun create(@AuthenticationPrincipal jwt: Jwt, @RequestBody body: CycleBody): CycleView =
        cycles.create(jwt.caller(), body.name, CycleType.fromWire(body.type))

    @PostMapping("/v1/cycles/{cycleId}/open")
    fun open(@AuthenticationPrincipal jwt: Jwt, @PathVariable cycleId: UUID): CycleView =
        cycles.open(jwt.caller(), cycleId)

    @GetMapping("/v1/cycles/{cycleId}/evaluations")
    fun progress(@AuthenticationPrincipal jwt: Jwt, @PathVariable cycleId: UUID): List<ProgressView> =
        evaluations.progress(jwt.caller(), cycleId)

    @PostMapping("/v1/cycles/{cycleId}/evaluations/self")
    @ResponseStatus(HttpStatus.CREATED)
    fun self(@AuthenticationPrincipal jwt: Jwt, @PathVariable cycleId: UUID, @RequestBody body: SelfEvaluationBody): EvaluationView =
        evaluations.submitSelf(
            jwt.caller(), cycleId,
            EvaluationInput(null, body.performanceScore, null, body.competencies, body.comment),
        )

    @PostMapping("/v1/cycles/{cycleId}/evaluations/manager")
    @ResponseStatus(HttpStatus.CREATED)
    fun manager(@AuthenticationPrincipal jwt: Jwt, @PathVariable cycleId: UUID, @RequestBody body: ManagerEvaluationBody): EvaluationView =
        evaluations.submitManager(
            jwt.caller(), cycleId,
            EvaluationInput(body.employeeId, body.performanceScore, body.potentialScore, body.competencies, body.comment),
        )
}

@RestController
class SuccessionController(private val succession: SuccessionService) {
    @GetMapping("/v1/critical-positions")
    fun list(@AuthenticationPrincipal jwt: Jwt): List<PlanView> = succession.list(jwt.caller())

    @PostMapping("/v1/critical-positions")
    @ResponseStatus(HttpStatus.CREATED)
    fun create(@AuthenticationPrincipal jwt: Jwt, @RequestBody body: PositionBody): PlanView =
        succession.create(jwt.caller(), body.title, body.incumbentId)

    @PostMapping("/v1/critical-positions/{positionId}/successors")
    @ResponseStatus(HttpStatus.CREATED)
    fun nominate(@AuthenticationPrincipal jwt: Jwt, @PathVariable positionId: UUID, @RequestBody body: NominationBody): PlanView =
        succession.nominate(jwt.caller(), positionId, body.employeeId, body.readiness)

    @GetMapping("/v1/critical-positions/{positionId}/successors/{employeeId}/readiness-suggestion")
    fun suggest(@AuthenticationPrincipal jwt: Jwt, @PathVariable positionId: UUID, @PathVariable employeeId: UUID) =
        ReadinessOut(succession.suggestReadiness(jwt.caller(), positionId, employeeId))
}

@RestController
class PdiController(private val pdi: PdiService) {
    @GetMapping("/v1/employees/{employeeId}/pdi")
    fun get(@AuthenticationPrincipal jwt: Jwt, @PathVariable employeeId: UUID) = pdi.get(jwt.caller(), employeeId)

    @PostMapping("/v1/employees/{employeeId}/pdi/actions")
    @ResponseStatus(HttpStatus.CREATED)
    fun add(@AuthenticationPrincipal jwt: Jwt, @PathVariable employeeId: UUID, @RequestBody body: PdiActionBody): PdiActionView =
        pdi.add(jwt.caller(), employeeId, body.title, body.competency, body.kind, body.dueOn)

    @PatchMapping("/v1/pdi-actions/{actionId}")
    fun status(@AuthenticationPrincipal jwt: Jwt, @PathVariable actionId: UUID, @RequestBody body: PdiStatusBody): PdiActionView =
        pdi.updateStatus(jwt.caller(), actionId, body.status)
}
