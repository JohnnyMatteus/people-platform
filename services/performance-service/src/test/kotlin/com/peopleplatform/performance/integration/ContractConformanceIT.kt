package com.peopleplatform.performance.integration

import org.springframework.beans.factory.annotation.Autowired
import org.springframework.beans.factory.annotation.Qualifier
import org.springframework.http.HttpMethod
import org.springframework.web.servlet.mvc.method.RequestMappingInfo
import org.springframework.web.servlet.mvc.method.annotation.RequestMappingHandlerMapping
import kotlin.test.Test
import kotlin.test.assertEquals

/** Toda operação do OpenAPI tem handler, e todo handler de /v1 está no OpenAPI. */
class ContractConformanceIT : IntegrationTestBase() {
    // O actuator registra um segundo RequestMappingHandlerMapping (controllerEndpointHandlerMapping).
    @Autowired
    @Qualifier("requestMappingHandlerMapping")
    lateinit var mappings: RequestMappingHandlerMapping

    private fun implemented(): Set<String> = mappings.handlerMethods.keys.flatMap { info: RequestMappingInfo ->
        val patterns = info.pathPatternsCondition?.patternValues ?: emptySet()
        val methods = info.methodsCondition.methods.map { it.name.lowercase() }
        patterns.filter { it.startsWith("/v1") }.flatMap { p -> methods.map { "$it $p" } }
    }.toSet()

    @Test
    fun `rotas implementadas sao exatamente as do contrato OpenAPI`() {
        val paths = ContractSchemas.openApi()["paths"] as Map<*, *>
        val methods = HttpMethod.values().map { it.name().lowercase() }
        val declared = paths.flatMap { (path, item) ->
            (item as Map<*, *>).keys.filter { it in methods }.map { "$it $path" }
        }.toSet()
        assertEquals(declared.sorted(), implemented().sorted())
    }
}
