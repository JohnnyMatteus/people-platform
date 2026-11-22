package com.peopleplatform.performance.adapters.web

import com.peopleplatform.performance.app.ConflictException
import com.peopleplatform.performance.app.ForbiddenException
import com.peopleplatform.performance.app.NotFoundException
import com.peopleplatform.performance.domain.shared.DomainException
import org.springframework.http.HttpStatus
import org.springframework.http.ProblemDetail
import org.springframework.http.converter.HttpMessageNotReadableException
import org.springframework.web.bind.annotation.ExceptionHandler
import org.springframework.web.bind.annotation.RestControllerAdvice
import org.springframework.web.method.annotation.MethodArgumentTypeMismatchException
import java.net.URI

/** Erros no formato RFC 9457 (application/problem+json), como o contrato promete. */
@RestControllerAdvice
class Problems {
    @ExceptionHandler(DomainException::class)
    fun domain(e: DomainException) = problem(HttpStatus.UNPROCESSABLE_ENTITY, "regra-de-negocio", e.message)

    @ExceptionHandler(NotFoundException::class)
    fun notFound(e: NotFoundException) = problem(HttpStatus.NOT_FOUND, "nao-encontrado", e.message)

    @ExceptionHandler(ConflictException::class)
    fun conflict(e: ConflictException) = problem(HttpStatus.CONFLICT, "conflito", e.message)

    @ExceptionHandler(ForbiddenException::class)
    fun forbidden(e: ForbiddenException) = problem(HttpStatus.FORBIDDEN, "proibido", e.message)

    @ExceptionHandler(HttpMessageNotReadableException::class)
    fun unreadable(e: HttpMessageNotReadableException): ProblemDetail {
        // Invariantes do domínio lançadas na construção do DTO chegam embrulhadas pelo Jackson.
        val cause = generateSequence<Throwable>(e) { it.cause }.filterIsInstance<DomainException>().firstOrNull()
        return problem(HttpStatus.UNPROCESSABLE_ENTITY, "corpo-invalido", cause?.message ?: "corpo da requisição inválido")
    }

    @ExceptionHandler(MethodArgumentTypeMismatchException::class)
    fun typeMismatch(e: MethodArgumentTypeMismatchException) =
        problem(HttpStatus.UNPROCESSABLE_ENTITY, "parametro-invalido", "parâmetro '${e.name}' inválido")

    private fun problem(status: HttpStatus, type: String, detail: String?): ProblemDetail =
        ProblemDetail.forStatusAndDetail(status, detail ?: status.reasonPhrase).apply {
            this.type = URI.create("https://people-platform.example.com/problems/$type")
            title = status.reasonPhrase
        }
}
