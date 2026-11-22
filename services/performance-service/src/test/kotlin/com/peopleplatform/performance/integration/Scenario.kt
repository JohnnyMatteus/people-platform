package com.peopleplatform.performance.integration

import com.fasterxml.jackson.databind.JsonNode
import com.fasterxml.jackson.databind.ObjectMapper
import org.springframework.http.MediaType
import org.springframework.security.core.authority.SimpleGrantedAuthority
import org.springframework.test.web.servlet.MockMvc
import org.springframework.test.web.servlet.request.MockHttpServletRequestBuilder
import org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get
import org.springframework.test.web.servlet.request.MockMvcRequestBuilders.patch
import org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post
import org.springframework.test.web.servlet.request.MockMvcRequestBuilders.put
import org.springframework.test.web.servlet.result.MockMvcResultMatchers.status
import java.time.LocalDate
import java.util.UUID
import org.springframework.security.test.web.servlet.request.SecurityMockMvcRequestPostProcessors.jwt as jwtPost

/** Uma pessoa de um tenant, com o token que o Keycloak emitiria para ela. */
class Person(
    val tenant: UUID,
    val roles: Set<String>,
    val id: UUID = UUID.randomUUID(),
    val name: String = "Pessoa Exemplo",
)

class Api(private val mvc: MockMvc, private val json: ObjectMapper) {
    fun <B : MockHttpServletRequestBuilder> B.asPerson(p: Person?): B {
        if (p != null) {
            with(
                jwtPost().jwt {
                    it.subject(p.id.toString()).claim("tenant_id", p.tenant.toString()).claim("name", p.name)
                        .claim("realm_access", mapOf("roles" to p.roles.toList()))
                }.authorities(p.roles.map { SimpleGrantedAuthority("ROLE_$it") }),
            )
        }
        return this
    }

    fun call(builder: MockHttpServletRequestBuilder, who: Person?, body: Any? = null, expect: Int): JsonNode {
        if (body != null) builder.contentType(MediaType.APPLICATION_JSON).content(json.writeValueAsString(body))
        val result = mvc.perform(builder.asPerson(who)).andExpect(status().`is`(expect)).andReturn()
        val text = result.response.contentAsString
        return if (text.isBlank()) json.nullNode() else json.readTree(text)
    }

    fun get(path: String, who: Person?, expect: Int = 200) = call(get(path), who, null, expect)

    fun post(path: String, who: Person?, body: Any? = null, expect: Int = 201) = call(post(path), who, body, expect)

    fun put(path: String, who: Person?, body: Any, expect: Int = 200) = call(put(path), who, body, expect)

    fun patch(path: String, who: Person?, body: Any, expect: Int = 200) = call(patch(path), who, body, expect)
}

val ADMIN = setOf("tenant-admin", "manager", "employee")
val MANAGER = setOf("manager", "employee")
val EMPLOYEE = setOf("employee")

/**
 * Ciclo completo de um tenant: três colaboradoras avaliadas, uma sessão de calibração aberta.
 * Notas pensadas para cair em quadrantes distintos na política padrão (1 a 5; 2.5 e 3.75).
 */
class Scenario(private val api: Api, val tenant: UUID) {
    val admin = Person(tenant, ADMIN, name = "Ana Admin")
    val manager = Person(tenant, MANAGER, name = "Marcos Gestor")
    val staff = listOf("Bia Exemplo", "Caio Exemplo", "Dora Exemplo").map { Person(tenant, EMPLOYEE, name = it) }
    lateinit var cycleId: String
    lateinit var sessionId: String

    private fun competencies(self: Int, manager: Int) =
        listOf(
            mapOf("competency" to "Comunicação", "score" to self),
            mapOf("competency" to "Entrega", "score" to manager),
        )

    fun runUntilCalibrationOpen() {
        cycleId = api.post("/v1/cycles", admin, mapOf("name" to "Ciclo 2026", "type" to "360")).get("id").asText()
        api.post("/v1/cycles/$cycleId/open", admin, expect = 200)
        staff.forEach {
            api.post(
                "/v1/employees",
                manager,
                mapOf(
                    "id" to it.id,
                    "name" to it.name,
                    "job_title" to "Analista",
                    "role_since" to LocalDate.now().minusYears(2).toString(),
                ),
                expect = 200,
            )
        }
        // Bia: topo (4.8/4.6). Caio: miolo (3.0/3.0). Dora: desempenho alto, potencial baixo (4.5/2.0).
        val scores = listOf(Triple("4.8", "4.6", 5), Triple("3.0", "3.0", 3), Triple("4.5", "2.0", 4))
        staff.zip(scores).forEach { (person, s) ->
            api.post(
                "/v1/cycles/$cycleId/evaluations/self",
                person,
                mapOf("performance_score" to s.first, "competencies" to competencies(s.third, s.third)),
            )
            api.post(
                "/v1/cycles/$cycleId/evaluations/manager",
                manager,
                mapOf(
                    "employee_id" to person.id,
                    "performance_score" to s.first,
                    "potential_score" to s.second,
                    "competencies" to competencies(s.third, 2),
                ),
            )
        }
        sessionId = api.post("/v1/cycles/$cycleId/calibration-sessions", manager).get("id").asText()
    }
}
