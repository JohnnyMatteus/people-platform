package com.peopleplatform.performance.integration

import com.fasterxml.jackson.databind.ObjectMapper
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.test.web.servlet.MockMvc
import java.util.UUID
import kotlin.test.Test
import kotlin.test.assertEquals

class OkrIT : IntegrationTestBase() {
    @Autowired lateinit var mvc: MockMvc

    @Autowired lateinit var json: ObjectMapper

    private val api by lazy { Api(mvc, json) }

    private fun keyResults(vararg specs: Triple<String, String, String>) = specs.map {
        mapOf("title" to it.first, "start" to it.second, "target" to it.third)
    }

    private fun objective(
        level: String,
        title: String,
        krs: List<Map<String, String>>,
        extra: Map<String, Any> = emptyMap(),
    ): Map<String, Any> = mapOf("level" to level, "title" to title, "key_results" to krs) + extra

    @Test
    fun `desdobramento da empresa ate a pessoa com check-in que move o progresso`() {
        val s = Scenario(api, UUID.randomUUID())
        val company = api.post(
            "/v1/objectives",
            s.admin,
            mapOf(
                "level" to "COMPANY",
                "title" to "Ser referência em segurança",
                "key_results" to keyResults(Triple("Acidentes com afastamento", "10", "0")),
            ),
        )
        val team = api.post(
            "/v1/objectives",
            s.manager,
            mapOf(
                "level" to "TEAM",
                "title" to "Zero acidentes na obra norte",
                "parent_id" to company.get("id").asText(),
                "key_results" to keyResults(Triple("Inspeções feitas", "0", "40"), Triple("Treinamentos", "0", "10")),
            ),
        )
        val mine = api.post(
            "/v1/objectives",
            s.staff[0],
            mapOf(
                "level" to "INDIVIDUAL",
                "title" to "Concluir o treinamento de segurança",
                "parent_id" to team.get("id").asText(),
                "key_results" to keyResults(Triple("Módulos concluídos", "0", "4")),
            ),
        )
        assertEquals(s.staff[0].id.toString(), mine.get("owner_id").asText())
        assertEquals("0", mine.get("progress").asText())

        val keyResult = mine.get("key_results").get(0).get("id").asText()
        val checkIn = api.post("/v1/key-results/$keyResult/check-ins", s.staff[0], mapOf("value" to "1"))
        assertEquals("0.25", checkIn.get("key_result").get("progress").asText())
        assertEquals("0.25", api.get("/v1/objectives/${mine.get("id").asText()}", s.staff[0]).get("progress").asText())

        // Decrescente: de 10 acidentes para 0, estando em 4, falta 60%.
        val companyKr = company.get("key_results").get(0).get("id").asText()
        val decreasing = api.post("/v1/key-results/$companyKr/check-ins", s.admin, mapOf("value" to "4"))
        assertEquals("0.6", decreasing.get("key_result").get("progress").asText())
    }

    @Test
    fun `quem pode criar e quem enxerga cada nivel`() {
        val s = Scenario(api, UUID.randomUUID())
        val krs = keyResults(Triple("Meta", "0", "10"))
        api.post("/v1/objectives", s.manager, objective("COMPANY", "Empresa", krs), expect = 403)
        api.post("/v1/objectives", s.staff[0], objective("TEAM", "Equipe", krs), expect = 403)
        api.post(
            "/v1/objectives",
            s.staff[0],
            objective("INDIVIDUAL", "De outra", krs, mapOf("owner_id" to s.staff[1].id)),
            expect = 403,
        )

        val individual = api.post("/v1/objectives", s.staff[0], objective("INDIVIDUAL", "Só da Bia", krs))
        val id = individual.get("id").asText()
        api.get("/v1/objectives/$id", s.staff[1], expect = 404)
        api.get("/v1/objectives/$id", s.manager)
        assertEquals(0, api.get("/v1/objectives?level=INDIVIDUAL", s.staff[1]).size())
        assertEquals(1, api.get("/v1/objectives?level=INDIVIDUAL", s.staff[0]).size())

        val keyResult = individual.get("key_results").get(0).get("id").asText()
        api.post("/v1/key-results/$keyResult/check-ins", s.staff[1], mapOf("value" to "5"), expect = 404)
    }

    @Test
    fun `regras de desdobramento, de resultado-chave e de objetivo encerrado`() {
        val s = Scenario(api, UUID.randomUUID())
        val krs = keyResults(Triple("Meta", "0", "10"))
        val team = api.post("/v1/objectives", s.manager, objective("TEAM", "Equipe", krs))
        val teamId = team.get("id").asText()

        // Empresa não se desdobra de equipe, e o pai tem de existir.
        api.post(
            "/v1/objectives",
            s.admin,
            objective("COMPANY", "Empresa", krs, mapOf("parent_id" to teamId)),
            expect = 422,
        )
        api.post(
            "/v1/objectives",
            s.manager,
            objective("TEAM", "Equipe", krs, mapOf("parent_id" to UUID.randomUUID())),
            expect = 404,
        )
        // Meta igual à partida, sem resultado-chave e título curto.
        val flat = keyResults(Triple("Meta", "5", "5"))
        api.post("/v1/objectives", s.manager, objective("TEAM", "Equipe", flat), expect = 422)
        api.post("/v1/objectives", s.manager, objective("TEAM", "Equipe", emptyList()), expect = 422)
        api.post("/v1/objectives", s.manager, objective("TEAM", "ab", krs), expect = 422)

        val keyResult = team.get("key_results").get(0).get("id").asText()
        api.post("/v1/objectives/$teamId/close", s.manager, expect = 200)
        api.post("/v1/key-results/$keyResult/check-ins", s.manager, mapOf("value" to "5"), expect = 409)
        api.post(
            "/v1/objectives",
            s.manager,
            objective("INDIVIDUAL", "Filho", krs, mapOf("parent_id" to teamId)),
            expect = 409,
        )
    }

    @Test
    fun `dono de outra pessoa precisa ser colaborador do mesmo tenant`() {
        val a = Scenario(api, UUID.randomUUID())
        a.runUntilCalibrationOpen()
        val b = Scenario(api, UUID.randomUUID())
        b.runUntilCalibrationOpen()
        val krs = keyResults(Triple("Meta", "0", "10"))

        // Colaborador cadastrado no próprio tenant: aceito. De outro tenant ou inexistente: 404, sem revelar qual.
        val ok = api.post(
            "/v1/objectives",
            a.manager,
            objective("INDIVIDUAL", "Da Bia", krs, mapOf("owner_id" to a.staff[0].id)),
        )
        assertEquals(a.staff[0].id.toString(), ok.get("owner_id").asText())
        api.post(
            "/v1/objectives",
            a.manager,
            objective("INDIVIDUAL", "Do outro tenant", krs, mapOf("owner_id" to b.staff[0].id)),
            expect = 404,
        )
        api.post(
            "/v1/objectives",
            a.manager,
            objective("TEAM", "Sem dono real", krs, mapOf("owner_id" to UUID.randomUUID())),
            expect = 404,
        )
    }

    @Test
    fun `valores que nao cabem em numeric 18 4 sao recusados com 422 e nao viram 500`() {
        val s = Scenario(api, UUID.randomUUID())
        listOf("1e100000", "100000000000000", "0.00001", "abc").forEach { bad ->
            api.post(
                "/v1/objectives",
                s.manager,
                objective("TEAM", "Equipe", keyResults(Triple("Meta", "0", bad))),
                expect = 422,
            )
        }
        val valid = keyResults(Triple("Meta", "0", "10"))
        val team = api.post("/v1/objectives", s.manager, objective("TEAM", "Equipe", valid))
        val keyResult = team.get("key_results").get(0).get("id").asText()
        listOf("1e100000", "99999999999999999999", "0.12345").forEach { bad ->
            api.post("/v1/key-results/$keyResult/check-ins", s.manager, mapOf("value" to bad), expect = 422)
        }
        // O limite exato cabe.
        api.post("/v1/key-results/$keyResult/check-ins", s.manager, mapOf("value" to "99999999999999.9999"))
    }
}
