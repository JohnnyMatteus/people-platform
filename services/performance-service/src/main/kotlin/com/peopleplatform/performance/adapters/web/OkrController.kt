package com.peopleplatform.performance.adapters.web

import com.peopleplatform.performance.app.CheckInView
import com.peopleplatform.performance.app.KeyResultView
import com.peopleplatform.performance.app.ObjectiveService
import com.peopleplatform.performance.app.ObjectiveView
import com.peopleplatform.performance.app.parseDecimal
import com.peopleplatform.performance.domain.okr.KeyResultSpec
import com.peopleplatform.performance.domain.okr.ObjectiveLevel
import org.springframework.http.HttpStatus
import org.springframework.security.core.annotation.AuthenticationPrincipal
import org.springframework.security.oauth2.jwt.Jwt
import org.springframework.web.bind.annotation.GetMapping
import org.springframework.web.bind.annotation.PathVariable
import org.springframework.web.bind.annotation.PostMapping
import org.springframework.web.bind.annotation.RequestBody
import org.springframework.web.bind.annotation.RequestParam
import org.springframework.web.bind.annotation.ResponseStatus
import org.springframework.web.bind.annotation.RestController
import java.math.BigDecimal
import java.time.Instant
import java.util.UUID

// ---------------------------------------------------------------- DTOs do contrato (snake_case)

data class KeyResultBody(val title: String, val start: String, val target: String)

data class ObjectiveBody(
    val level: ObjectiveLevel,
    val title: String,
    val keyResults: List<KeyResultBody>,
    val ownerId: UUID? = null,
    val parentId: UUID? = null,
)

data class CheckInBody(val value: String, val comment: String? = null)

data class KeyResultOut(
    val id: UUID,
    val title: String,
    val start: String,
    val target: String,
    val current: String,
    val progress: String,
)

data class ObjectiveOut(
    val id: UUID,
    val level: ObjectiveLevel,
    val ownerId: UUID?,
    val ownerName: String?,
    val parentId: UUID?,
    val title: String,
    val status: String,
    val progress: String,
    val keyResults: List<KeyResultOut>,
)

data class CheckInOut(
    val id: UUID,
    val keyResultId: UUID,
    val value: String,
    val comment: String?,
    val createdBy: UUID,
    val createdAt: Instant,
    val keyResult: KeyResultOut,
)

// Decimais saem como string, na forma canônica (sem zeros à direita), como no restante do contrato.
private fun BigDecimal.canonical(): String = stripTrailingZeros().toPlainString()

private fun KeyResultView.toOut() =
    KeyResultOut(id, title, start.canonical(), target.canonical(), current.canonical(), progress.canonical())

private fun ObjectiveView.toOut() = ObjectiveOut(
    id,
    level,
    ownerId,
    ownerName,
    parentId,
    title,
    status.name,
    progress.canonical(),
    keyResults.map { it.toOut() },
)

private fun CheckInView.toOut() =
    CheckInOut(id, keyResultId, value.canonical(), comment, createdBy, createdAt, keyResult.toOut())

@RestController
class ObjectiveController(private val objectives: ObjectiveService) {
    @PostMapping("/v1/objectives")
    @ResponseStatus(HttpStatus.CREATED)
    fun create(@AuthenticationPrincipal jwt: Jwt, @RequestBody body: ObjectiveBody): ObjectiveOut =
        objectives.create(
            jwt.toCaller(),
            body.level,
            body.ownerId,
            body.parentId,
            body.title,
            body.keyResults.map {
                KeyResultSpec(it.title, parseDecimal(it.start, "start"), parseDecimal(it.target, "target"))
            },
        ).toOut()

    @GetMapping("/v1/objectives")
    fun list(
        @AuthenticationPrincipal jwt: Jwt,
        @RequestParam(required = false) level: ObjectiveLevel?,
        @RequestParam(required = false) ownerId: UUID?,
    ): List<ObjectiveOut> = objectives.list(jwt.toCaller(), level, ownerId).map { it.toOut() }

    @GetMapping("/v1/objectives/{objectiveId}")
    fun get(@AuthenticationPrincipal jwt: Jwt, @PathVariable objectiveId: UUID): ObjectiveOut =
        objectives.get(jwt.toCaller(), objectiveId).toOut()

    @PostMapping("/v1/objectives/{objectiveId}/close")
    fun close(@AuthenticationPrincipal jwt: Jwt, @PathVariable objectiveId: UUID): ObjectiveOut =
        objectives.close(jwt.toCaller(), objectiveId).toOut()

    @PostMapping("/v1/key-results/{keyResultId}/check-ins")
    @ResponseStatus(HttpStatus.CREATED)
    fun checkIn(
        @AuthenticationPrincipal jwt: Jwt,
        @PathVariable keyResultId: UUID,
        @RequestBody body: CheckInBody,
    ): CheckInOut =
        objectives.checkIn(jwt.toCaller(), keyResultId, parseDecimal(body.value, "value"), body.comment).toOut()
}
