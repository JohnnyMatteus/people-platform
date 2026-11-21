package com.peopleplatform.performance.domain

import com.peopleplatform.performance.domain.ninebox.Assessment
import com.peopleplatform.performance.domain.ninebox.NineBoxPolicy
import com.peopleplatform.performance.domain.shared.CycleId
import com.peopleplatform.performance.domain.shared.EmployeeId
import com.peopleplatform.performance.domain.shared.TenantId
import java.math.BigDecimal
import java.time.Instant
import java.util.UUID

/** Tenants fictícios usados nos testes. */
val horizonte = TenantId(UUID.fromString("00000000-0000-0000-0000-00000000000a"))
val varejoSol = TenantId(UUID.fromString("00000000-0000-0000-0000-00000000000b"))

val cycle2026 = CycleId(UUID.fromString("00000000-0000-0000-0000-000000002026"))
val cycle2027 = CycleId(UUID.fromString("00000000-0000-0000-0000-000000002027"))

val t0: Instant = Instant.parse("2026-11-01T12:00:00Z")

fun employee(): EmployeeId = EmployeeId(UUID.randomUUID())

fun defaultPolicy(tenant: TenantId = horizonte) = NineBoxPolicy.default(tenant)

fun assessment(
    perf: String,
    pot: String,
    tenant: TenantId = horizonte,
    cycle: CycleId = cycle2026,
    who: EmployeeId = employee(),
) = Assessment(tenant, cycle, who, BigDecimal(perf), BigDecimal(pot))

const val JUSTIFICATION = "Entregas do segundo semestre não refletidas na avaliação do gestor."
