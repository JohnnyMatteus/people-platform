package com.peopleplatform.performance.integration

import com.fasterxml.jackson.databind.ObjectMapper
import com.peopleplatform.performance.adapters.messaging.OutboxRelay
import com.peopleplatform.performance.adapters.persistence.OutboxRepository
import com.peopleplatform.performance.adapters.persistence.TenantScope
import org.mockito.ArgumentMatchers
import org.mockito.Mockito
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.kafka.core.KafkaTemplate
import org.springframework.messaging.Message
import org.springframework.test.web.servlet.MockMvc
import java.time.Clock
import java.util.UUID
import java.util.concurrent.CompletableFuture
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class OutboxRelayIT : IntegrationTestBase() {
    @Autowired lateinit var mvc: MockMvc

    @Autowired lateinit var json: ObjectMapper

    @Autowired lateinit var scope: TenantScope

    @Autowired lateinit var outbox: OutboxRepository

    @Suppress("UNCHECKED_CAST")
    private fun kafka(failing: Boolean): KafkaTemplate<String, String> {
        val template = Mockito.mock(KafkaTemplate::class.java) as KafkaTemplate<String, String>
        val future: CompletableFuture<Any?> =
            if (failing) {
                CompletableFuture.failedFuture(IllegalStateException("broker fora"))
            } else {
                CompletableFuture.completedFuture(null)
            }
        Mockito.doReturn(future).`when`(template).send(ArgumentMatchers.any<Message<*>>())
        return template
    }

    private fun pending(): Int = scope.runAsPlatform { outbox.findAll().count { it.publishedAt == null } }

    @Test
    fun `relay publica o que esta pendente, marca como publicado e nao repete`() {
        val s = Scenario(Api(mvc, json), UUID.randomUUID())
        s.runUntilCalibrationOpen()
        val before = pending()
        assertTrue(before >= 3)

        val topic = "people.performance.events.v1"
        val broken = OutboxRelay(scope, outbox, kafka(failing = true), Clock.systemUTC(), topic)
        assertEquals(0, broken.relayOnce())
        assertEquals(before, pending())
        assertTrue(scope.runAsPlatform { outbox.findAll().any { it.attempts > 0 && it.lastError != null } })

        val template = kafka(failing = false)
        val relay = OutboxRelay(scope, outbox, template, Clock.systemUTC(), "people.performance.events.v1")
        assertEquals(before, relay.relayOnce(batch = 1000))
        assertEquals(0, pending())
        assertEquals(0, relay.relayOnce())
    }
}
