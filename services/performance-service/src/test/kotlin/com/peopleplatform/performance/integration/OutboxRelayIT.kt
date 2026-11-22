package com.peopleplatform.performance.integration

import com.fasterxml.jackson.databind.ObjectMapper
import com.peopleplatform.performance.adapters.messaging.OutboxRelay
import com.peopleplatform.performance.adapters.persistence.OutboxEntity
import com.peopleplatform.performance.adapters.persistence.OutboxRepository
import com.peopleplatform.performance.adapters.persistence.TenantScope
import org.mockito.ArgumentMatchers
import org.mockito.Mockito
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.kafka.core.KafkaTemplate
import org.springframework.messaging.Message
import org.springframework.test.web.servlet.MockMvc
import java.time.Clock
import java.time.Instant
import java.util.UUID
import java.util.concurrent.CompletableFuture
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
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

    private fun seed(
        tenant: UUID,
        count: Int,
        start: Instant,
    ): List<UUID> = scope.runAsPlatform {
        List(count) { i ->
            val id = UUID.randomUUID()
            outbox.save(
                OutboxEntity(
                    id = id,
                    tenantId = tenant,
                    aggregateType = "cycle",
                    aggregateId = id,
                    eventType = "AvaliacaoConcluida",
                    eventVersion = 1,
                    payload = """{"id":"$id"}""",
                    occurredAt = start.plusSeconds(i.toLong()),
                    publishedAt = null,
                    attempts = 0,
                    lastError = null,
                ),
            )
            id
        }
    }

    @Suppress("UNCHECKED_CAST")
    private fun selectiveKafka(
        reject: Set<UUID>,
        delivered: MutableList<UUID>,
    ): KafkaTemplate<String, String> {
        val template = Mockito.mock(KafkaTemplate::class.java) as KafkaTemplate<String, String>
        Mockito.doAnswer { call ->
            val payload = call.getArgument<Message<*>>(0).payload.toString()
            val id = UUID.fromString(json.readTree(payload).get("id").asText())
            if (id in reject) {
                CompletableFuture.failedFuture<Any?>(IllegalStateException("recusada pelo broker"))
            } else {
                delivered += id
                CompletableFuture.completedFuture<Any?>(null)
            }
        }.`when`(template).send(ArgumentMatchers.any<Message<*>>())
        return template
    }

    @Test
    fun `tenant com mensagem envenenada nao esgota o lote nem fura a ordem, e attempts sobe so na linha recusada`() {
        scope.runAsPlatform {
            outbox.findAll().filter { it.publishedAt == null }.forEach {
                it.publishedAt = Instant.now()
                outbox.save(it)
            }
        }
        val start = Instant.now().minusSeconds(3600)
        val tenantA = UUID.randomUUID()
        val tenantB = UUID.randomUUID()
        // A é a mais antiga e tem mais pendentes que o lote (5): sozinha, esgotaria todas as buscas.
        val a = seed(tenantA, 7, start)
        val b = seed(tenantB, 4, start.plusSeconds(60))
        val reject = mutableSetOf(a[1], b[2])
        val delivered = mutableListOf<UUID>()
        val relay = OutboxRelay(scope, outbox, selectiveKafka(reject, delivered), Clock.systemUTC(), "t")

        assertEquals(3, relay.relayOnce(batch = 5))
        assertEquals(listOf(a[0], b[0], b[1]), delivered)

        val rows = scope.runAsPlatform { outbox.findAllById(a + b).associateBy { it.id } }
        (a + b).forEach { assertEquals(if (it in reject) 1 else 0, rows.getValue(it).attempts) }
        assertNull(rows.getValue(a[1]).publishedAt)
        assertNotNull(rows.getValue(a[1]).lastError)
        assertNull(rows.getValue(a[2]).publishedAt)
        assertNull(rows.getValue(b[3]).publishedAt)

        reject.clear()
        delivered.clear()
        while (relay.relayOnce(batch = 5) > 0) {
            // esvazia a fila
        }
        assertEquals(a.drop(1), delivered.filter { it in a })
        assertEquals(b.drop(2), delivered.filter { it in b })
    }
}
