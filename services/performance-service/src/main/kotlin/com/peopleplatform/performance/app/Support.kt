package com.peopleplatform.performance.app

import com.peopleplatform.performance.domain.shared.DomainException
import com.peopleplatform.performance.domain.shared.EmployeeId
import com.peopleplatform.performance.domain.shared.TenantId
import java.math.BigDecimal
import java.time.Instant
import java.util.UUID

/** Quem chama, lido do token. O tenant nunca vem da requisição. */
data class Caller(val tenantId: TenantId, val employeeId: EmployeeId, val name: String?, val roles: Set<String>) {
    val isAdmin: Boolean get() = ROLE_ADMIN in roles
    val isManager: Boolean get() = ROLE_MANAGER in roles || isAdmin

    fun canSee(employee: EmployeeId): Boolean = employee == employeeId || isManager

    companion object {
        const val ROLE_ADMIN = "tenant-admin"
        const val ROLE_MANAGER = "manager"
    }
}

class NotFoundException(message: String) : RuntimeException(message)

class ConflictException(message: String) : RuntimeException(message)

class ForbiddenException(message: String) : RuntimeException(message)

/** Decimais chegam como string no contrato para não perder precisão. */
fun parseDecimal(raw: String, label: String): BigDecimal =
    raw.toBigDecimalOrNull() ?: throw DomainException("$label '$raw' não é um número")

/** Limite das colunas numeric(18, 4): até 14 dígitos inteiros e 4 decimais. Acima disso o banco responderia 500. */
private const val MAX_INTEGER_DIGITS = 14
private const val MAX_FRACTION_DIGITS = 4

/** Como [parseDecimal], mas recusa o que não cabe em numeric(18, 4) (inclusive notação como 1e100000). */
fun parseBoundedDecimal(raw: String, label: String): BigDecimal {
    val value = parseDecimal(raw, label)
    val normalized = value.stripTrailingZeros()
    val integerDigits = normalized.precision() - normalized.scale()
    val fractionDigits = maxOf(normalized.scale(), 0)
    if (value.signum() != 0 && (integerDigits > MAX_INTEGER_DIGITS || fractionDigits > MAX_FRACTION_DIGITS)) {
        throw DomainException(
            "$label '$raw' fora do limite: até $MAX_INTEGER_DIGITS dígitos inteiros e $MAX_FRACTION_DIGITS decimais",
        )
    }
    return value
}

/** Porta de saída de eventos: grava na outbox, na mesma transação da mudança de estado. */
interface DomainEvents {
    fun publish(
        tenantId: TenantId,
        aggregateType: String,
        aggregateId: UUID,
        eventType: String,
        data: Map<String, Any?>,
        occurredAt: Instant,
    )
}
