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
}
