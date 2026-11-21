package com.peopleplatform.performance.domain.shared

import java.util.UUID

/** Identificador do tenant. Todo agregado do serviço pertence a exatamente um tenant. */
@JvmInline
value class TenantId(val value: UUID)

/** Identificador de colaborador (pessoa avaliada, gestor ou calibrador). */
@JvmInline
value class EmployeeId(val value: UUID)

/** Identificador de ciclo de avaliação (ex.: "ciclo 2026"). */
@JvmInline
value class CycleId(val value: UUID)

/** Erro de regra de negócio: a operação foi recusada por violar uma invariante do domínio. */
class DomainException(message: String) : RuntimeException(message)

internal fun require(condition: Boolean, message: () -> String) {
    if (!condition) throw DomainException(message())
}

/** Garante que um agregado só receba dados do próprio tenant (defesa contra vazamento). */
internal fun requireSameTenant(expected: TenantId, actual: TenantId) {
    require(expected == actual) { "operação entre tenants diferentes não é permitida" }
}
