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
