package com.peopleplatform.performance.integration

import com.fasterxml.jackson.databind.ObjectMapper
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.jdbc.core.JdbcTemplate
import org.springframework.test.web.servlet.MockMvc
import java.time.LocalDate
import java.util.UUID
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class CalibrationFlowIT : IntegrationTestBase() {
    @Autowired lateinit var mvc: MockMvc

    @Autowired lateinit var json: ObjectMapper

    @Autowired lateinit var jdbc: JdbcTemplate

    private val api by lazy { Api(mvc, json) }

    private fun newTenant() = UUID.randomUUID()

    private fun outboxOf(tenant: UUID, type: String): List<String> = jdbc.execute { c: java.sql.Connection ->
        c.autoCommit = false
        c.prepareStatement("select set_config('app.tenant_id', ?, true)").use { it.setString(1, tenant.toString()); it.execute() }
        val rows = c.prepareStatement("select payload::text from outbox where event_type = ? order by occurred_at, id").use {
            it.setString(1, type)
            it.executeQuery().use { rs -> generateSequence { if (rs.next()) rs.getString(1) else null }.toList() }
        }
        c.rollback()
        rows
    }!!

    @Test
    fun `fluxo completo - avaliacao, calibracao, historico e eventos`() {
        val s = Scenario(api, newTenant())
        s.runUntilCalibrationOpen()

        val bia = s.staff[0]
        val dora = s.staff[2]
        val session = api.get("/v1/calibration-sessions/${s.sessionId}", s.manager)
        assertEquals("open", session.get("status").asText())
        val placements = session.get("placements").associate { it.get("employee_id").asText() to it.get("original").asText() }
        assertEquals("Q9", placements[bia.id.toString()])
        assertEquals("Q5", placements[s.staff[1].id.toString()])
        assertEquals("Q3", placements[dora.id.toString()])
        assertEquals("TOP_BOX_ABOVE_GUIDELINE", session.get("warnings")[0].get("code").asText())
        assertEquals(1, session.get("distribution").get("Q9").asInt())

        // AvaliacaoConcluida: uma por pessoa, só depois que gestor e pessoa enviaram.
        assertEquals(3, outboxOf(s.tenant, "AvaliacaoConcluida").size)

        val justification = "Entregas do segundo semestre não refletidas na avaliação do gestor."
        // Dora: Q3 (alto/baixo) para Q6 (alto/médio) = 1 faixa de potencial, dentro do limite.
        val adjusted = api.post(
            "/v1/calibration-sessions/${s.sessionId}/adjustments", s.manager,
            mapOf("employee_id" to dora.id, "to" to "Q6", "justification" to justification),
        )
        assertEquals("Q3", adjusted.get("from").asText())
        // Dois saltos de potencial (baixo para alto) excedem o limite padrão de 1 faixa por eixo.
        api.post(
            "/v1/calibration-sessions/${s.sessionId}/adjustments", s.manager,
            mapOf("employee_id" to dora.id, "to" to "Q9", "justification" to justification), expect = 422,
        )
        api.post(
            "/v1/calibration-sessions/${s.sessionId}/adjustments", s.manager,
            mapOf("employee_id" to s.staff[1].id, "to" to "Q6", "justification" to "curta"), expect = 422,
        )
        // Quem não está na sessão não pode ser calibrado.
        api.post(
            "/v1/calibration-sessions/${s.sessionId}/adjustments", s.manager,
            mapOf("employee_id" to UUID.randomUUID(), "to" to "Q6", "justification" to justification), expect = 422,
        )

        val closed = api.post("/v1/calibration-sessions/${s.sessionId}/close", s.manager, expect = 200)
        assertEquals("closed", closed.get("status").asText())
        api.post("/v1/calibration-sessions/${s.sessionId}/close", s.manager, expect = 409)
        api.post(
            "/v1/calibration-sessions/${s.sessionId}/adjustments", s.manager,
            mapOf("employee_id" to dora.id, "to" to "Q3", "justification" to justification), expect = 409,
        )

        val events = outboxOf(s.tenant, "NineBoxCalibrado")
        assertEquals(3, events.size)
        events.forEach { ContractSchemas.assertValid("nine-box-calibrado.v1.json", it) }
        outboxOf(s.tenant, "AvaliacaoConcluida").forEach { ContractSchemas.assertValid("avaliacao-concluida.v1.json", it) }
        val doraEvent = events.map(json::readTree).first { it.get("data").get("employee_id").asText() == dora.id.toString() }
        assertEquals("Q6", doraEvent.get("data").get("quadrant").asText())
        assertTrue(doraEvent.get("data").get("calibrated").asBoolean())

        // Histórico: a própria pessoa e o gestor veem; outra pessoa não.
        val history = api.get("/v1/employees/${dora.id}/nine-box-history", dora)
        assertEquals("Q6", history.get("entries")[0].get("quadrant").asText())
        assertEquals("INSUFFICIENT_DATA", history.get("trend").asText())
        api.get("/v1/employees/${dora.id}/nine-box-history", bia, expect = 403)
        api.get("/v1/employees/${dora.id}/nine-box-history", s.manager)

        // Ciclo fechado não recebe avaliação nova.
        api.post(
            "/v1/cycles/${s.cycleId}/evaluations/self", bia,
            mapOf("performance_score" to "3", "competencies" to emptyList<Any>()), expect = 409,
        )
        assertEquals("closed", api.get("/v1/cycles", s.admin)[0].get("status").asText())
    }

    @Test
    fun `avaliacao duplicada, fora da escala e autoavaliacao de gestor sao recusadas`() {
        val tenant = newTenant()
        val admin = Person(tenant, ADMIN)
        val manager = Person(tenant, MANAGER)
        val worker = Person(tenant, EMPLOYEE, name = "Eva Exemplo")
        val cycle = api.post("/v1/cycles", admin, mapOf("name" to "Ciclo 2027", "type" to "self")).get("id").asText()
        api.post(
            "/v1/cycles/$cycle/evaluations/self", worker,
            mapOf("performance_score" to "3", "competencies" to emptyList<Any>()), expect = 409,
        )
        api.post("/v1/cycles/$cycle/open", admin, expect = 200)
        api.post("/v1/cycles/$cycle/open", admin, expect = 409)
        api.post(
            "/v1/cycles/$cycle/evaluations/self", worker,
            mapOf("performance_score" to "9", "competencies" to emptyList<Any>()), expect = 422,
        )
        val body = mapOf("performance_score" to "3", "competencies" to listOf(mapOf("competency" to "Entrega", "score" to 4)))
        api.post("/v1/cycles/$cycle/evaluations/self", worker, body)
        api.post("/v1/cycles/$cycle/evaluations/self", worker, body, expect = 409)
        api.post(
            "/v1/cycles/$cycle/evaluations/manager", manager,
            mapOf("employee_id" to manager.id, "performance_score" to "3", "potential_score" to "3", "competencies" to emptyList<Any>()),
            expect = 422,
        )
        val progress = api.get("/v1/cycles/$cycle/evaluations", manager)
        assertEquals(false, progress[0].get("consolidated").asBoolean())
        // Sessão sem avaliações consolidadas não abre.
        api.post("/v1/cycles/$cycle/calibration-sessions", manager, expect = 409)
    }

    @Test
    fun `papeis - colaborador nao administra ciclo nem calibra, e sem token e 401`() {
        val tenant = newTenant()
        val worker = Person(tenant, EMPLOYEE)
        val manager = Person(tenant, MANAGER)
        api.post("/v1/cycles", worker, mapOf("name" to "Ciclo X", "type" to "self"), expect = 403)
        api.post("/v1/cycles", manager, mapOf("name" to "Ciclo X", "type" to "self"), expect = 403)
        api.get("/v1/cycles", null, expect = 401)
        api.get("/v1/critical-positions", worker, expect = 403)
        api.get("/v1/employees", worker, expect = 403)
        api.put("/v1/nine-box/policy", manager, policyBody(), expect = 403)
        api.get("/v1/me", worker)
    }

    private fun policyBody(medium: String = "2.5", high: String = "3.75") = mapOf(
        "scale_min" to "1", "scale_max" to "5",
        "performance" to mapOf("medium_from" to medium, "high_from" to high),
        "potential" to mapOf("medium_from" to medium, "high_from" to high),
        "max_steps_per_axis" to 2, "top_box_warning_share" to "0.15", "min_justification_length" to 10,
    )

    @Test
    fun `politica do tenant vale para sessoes abertas depois e nao altera sessao em andamento`() {
        val tenant = newTenant()
        val admin = Person(tenant, ADMIN)
        assertEquals("3.75", api.get("/v1/nine-box/policy", admin).get("performance").get("high_from").asText())
        api.put("/v1/nine-box/policy", admin, policyBody("2", "3"))
        assertEquals("3", api.get("/v1/nine-box/policy", admin).get("performance").get("high_from").asText())
        api.put("/v1/nine-box/policy", admin, policyBody("4", "3"), expect = 422)
        // Outro tenant continua com o padrão.
        assertEquals("3.75", api.get("/v1/nine-box/policy", Person(newTenant(), ADMIN)).get("performance").get("high_from").asText())
    }

    @Test
    fun `sucessao, prontidao sugerida e pdi`() {
        val s = Scenario(api, newTenant())
        s.runUntilCalibrationOpen()
        api.post("/v1/calibration-sessions/${s.sessionId}/close", s.manager, expect = 200)
        val bia = s.staff[0]
        val caio = s.staff[1]

        val position = api.post("/v1/critical-positions", s.manager, mapOf("title" to "Gerente de Obras", "incumbent_id" to caio.id))
        val positionId = position.get("position_id").asText()
        assertEquals("NONE", position.get("bench_strength").asText())
        api.post("/v1/critical-positions/$positionId/successors", s.manager, mapOf("employee_id" to caio.id, "readiness" to "READY_NOW"), expect = 422)
        val plan = api.post("/v1/critical-positions/$positionId/successors", s.manager, mapOf("employee_id" to bia.id, "readiness" to "READY_NOW"))
        assertEquals("ADEQUATE", plan.get("bench_strength").asText())
        assertEquals("Bia Exemplo", plan.get("successors")[0].get("employee_name").asText())
        api.post("/v1/critical-positions/$positionId/successors", s.manager, mapOf("employee_id" to bia.id, "readiness" to "READY_NOW"), expect = 409)
        // Bia: Q9 e 2 anos na função, pronta agora. Dora (Q3, potencial baixo) não é indicada.
        assertEquals("READY_NOW", api.get("/v1/critical-positions/$positionId/successors/${bia.id}/readiness-suggestion", s.manager).get("suggestion").asText())
        val dora = s.staff[2]
        assertTrue(api.get("/v1/critical-positions/$positionId/successors/${dora.id}/readiness-suggestion", s.manager).get("suggestion").isNull)

        // PDI da Bia: gap de 3 pontos em Entrega vira sugestão; nada entra no plano sem POST.
        val pdi = api.get("/v1/employees/${bia.id}/pdi", bia)
        assertEquals("Entrega", pdi.get("gaps").first { it.get("gap").asInt() == 3 }.get("competency").asText())
        assertTrue(pdi.get("suggestions").size() >= 2)
        assertEquals(0, pdi.get("actions").size())
        api.get("/v1/employees/${bia.id}/pdi", caio, expect = 403)
        val due = LocalDate.now().plusMonths(3).toString()
        api.post("/v1/employees/${bia.id}/pdi/actions", bia, mapOf("title" to "x", "kind" to "COURSE", "due_on" to due), expect = 422)
        api.post("/v1/employees/${bia.id}/pdi/actions", bia, mapOf("title" to "Curso", "kind" to "COURSE", "due_on" to "2020-01-01"), expect = 422)
        val action = api.post("/v1/employees/${bia.id}/pdi/actions", s.manager, mapOf("title" to "Mentoria de entrega", "kind" to "MENTORING", "competency" to "Entrega", "due_on" to due))
        val id = action.get("id").asText()
        api.patch("/v1/pdi-actions/$id", bia, mapOf("status" to "DONE"))
        api.patch("/v1/pdi-actions/$id", bia, mapOf("status" to "PLANNED"), expect = 422)
        assertEquals(1, api.get("/v1/employees/${bia.id}/pdi", bia).get("actions").size())
    }
}
