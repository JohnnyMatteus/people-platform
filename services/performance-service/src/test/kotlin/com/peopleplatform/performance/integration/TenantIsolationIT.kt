package com.peopleplatform.performance.integration

import com.fasterxml.jackson.databind.ObjectMapper
import com.peopleplatform.performance.adapters.persistence.CycleEntity
import com.peopleplatform.performance.adapters.persistence.CycleRepository
import com.peopleplatform.performance.adapters.persistence.TenantScope
import com.peopleplatform.performance.domain.shared.TenantId
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.dao.DataAccessException
import org.springframework.jdbc.core.JdbcTemplate
import org.springframework.test.web.servlet.MockMvc
import java.time.Instant
import java.util.UUID
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertTrue

/** Vazamento entre tenants: o teste que o CLAUDE.md exige para toda persistência nova. */
class TenantIsolationIT : IntegrationTestBase() {
    @Autowired lateinit var mvc: MockMvc

    @Autowired lateinit var json: ObjectMapper

    @Autowired lateinit var jdbc: JdbcTemplate

    @Autowired lateinit var scope: TenantScope

    @Autowired lateinit var cycles: CycleRepository

    private val api by lazy { Api(mvc, json) }

    private val rlsTables = listOf(
        "nine_box_policies", "employees", "cycles", "evaluations", "calibration_sessions", "calibration_placements",
        "calibration_adjustments", "nine_box_history", "critical_positions", "successors", "pdi_actions", "outbox",
    )

    private fun countAs(tenant: UUID?, table: String): Long = jdbc.execute { c: java.sql.Connection ->
        c.autoCommit = false
        if (tenant != null) {
            c.prepareStatement("select set_config('app.tenant_id', ?, true)").use {
                it.setString(1, tenant.toString())
                it.execute()
            }
        }
        val n = c.createStatement().use { st ->
            st.executeQuery("select count(*) from $table").use { rs ->
                rs.next()
                rs.getLong(1)
            }
        }
        c.rollback()
        n
    }!!

    @Test
    fun `tenant B nao enxerga nenhuma linha de nenhuma tabela do tenant A`() {
        val a = Scenario(api, UUID.randomUUID())
        a.runUntilCalibrationOpen()
        val dora = a.staff[2]
        api.post(
            "/v1/calibration-sessions/${a.sessionId}/adjustments", a.manager,
            mapOf(
                "employee_id" to dora.id,
                "to" to "Q6",
                "justification" to "Entregas do segundo semestre não refletidas.",
            ),
        )
        api.post("/v1/calibration-sessions/${a.sessionId}/close", a.manager, expect = 200)
        val position = api.post("/v1/critical-positions", a.manager, mapOf("title" to "Gerente de Obras"))
            .get("position_id").asText()
        api.post(
            "/v1/critical-positions/$position/successors",
            a.manager,
            mapOf("employee_id" to a.staff[0].id, "readiness" to "READY_NOW"),
        )
        api.post(
            "/v1/employees/${a.staff[0].id}/pdi/actions", a.manager,
            mapOf(
                "title" to "Mentoria",
                "kind" to "MENTORING",
                "due_on" to java.time.LocalDate.now().plusMonths(2).toString(),
            ),
        )
        api.put(
            "/v1/nine-box/policy", a.admin,
            mapOf(
                "scale_min" to "1", "scale_max" to "5",
                "performance" to mapOf("medium_from" to "2.5", "high_from" to "3.75"),
                "potential" to mapOf("medium_from" to "2.5", "high_from" to "3.75"),
            ),
            expect = 200,
        )

        val b = UUID.randomUUID()
        rlsTables.forEach { table ->
            assertTrue(countAs(a.tenant, table) > 0, "tenant A deveria ter linhas em $table")
            assertEquals(0, countAs(b, table), "tenant B vazou leitura em $table")
            assertEquals(0, countAs(null, table), "sessão sem escopo de tenant vazou leitura em $table")
        }

        // Pela API: B não alcança nada de A, nem por id.
        val other = Scenario(api, b)
        assertEquals(0, api.get("/v1/cycles", other.admin).size())
        assertEquals(0, api.get("/v1/employees", other.manager).size())
        assertEquals(0, api.get("/v1/critical-positions", other.manager).size())
        api.get("/v1/calibration-sessions/${a.sessionId}", other.manager, expect = 404)
        api.post("/v1/calibration-sessions/${a.sessionId}/close", other.manager, expect = 404)
        api.get("/v1/cycles/${a.cycleId}/calibration-sessions/current", other.manager, expect = 404)
        api.get("/v1/employees/${a.staff[0].id}/nine-box-history", other.manager, expect = 404)
        api.get("/v1/employees/${a.staff[0].id}/pdi", other.manager).let { assertEquals(0, it.get("actions").size()) }
    }

    @Test
    fun `escrever com tenant_id de outro tenant e barrado pela politica de RLS`() {
        val mine = TenantId(UUID.randomUUID())
        val foreign = UUID.randomUUID()
        assertFailsWith<DataAccessException> {
            scope.run(mine) {
                cycles.saveAndFlush(
                    CycleEntity(UUID.randomUUID(), foreign, "Ciclo alheio", "self", "draft", Instant.now()),
                )
            }
        }
        assertFailsWith<DataAccessException> {
            // Sem escopo nenhum, a escrita também falha (falha fechada).
            jdbc.update(
                "insert into cycles (id, tenant_id, name, type, status, created_at) " +
                    "values (?, ?, 'x', 'self', 'draft', now())",
                UUID.randomUUID(),
                foreign,
            )
        }
    }
}
