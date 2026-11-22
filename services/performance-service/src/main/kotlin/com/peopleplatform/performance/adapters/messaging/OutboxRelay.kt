package com.peopleplatform.performance.adapters.messaging

import com.peopleplatform.performance.adapters.persistence.OutboxRepository
import com.peopleplatform.performance.adapters.persistence.TenantScope
import org.slf4j.LoggerFactory
import org.springframework.beans.factory.annotation.Value
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty
import org.springframework.kafka.core.KafkaTemplate
import org.springframework.kafka.support.KafkaHeaders
import org.springframework.messaging.support.MessageBuilder
import org.springframework.scheduling.annotation.Scheduled
import org.springframework.stereotype.Component
import java.time.Clock
import java.util.concurrent.TimeUnit

/**
 * Publica no Kafka os eventos gravados na outbox (at-least-once: se o processo cair entre o envio
 * e a marcação, o evento sai de novo; consumidores deduplicam por `event_id`).
 * `FOR UPDATE SKIP LOCKED` deixa várias réplicas drenarem a fila sem se atropelar.
 */
@Component
@ConditionalOnProperty("performance.outbox.relay.enabled", havingValue = "true", matchIfMissing = true)
class OutboxRelay(
    private val scope: TenantScope,
    private val outbox: OutboxRepository,
    private val kafka: KafkaTemplate<String, String>,
    private val clock: Clock,
    @Value("\${performance.outbox.topic}") private val topic: String,
) {
    private val log = LoggerFactory.getLogger(javaClass)

    @Scheduled(fixedDelayString = "\${performance.outbox.relay.poll-interval}")
    fun tick() {
        runCatching { relayOnce() }.onFailure { log.warn("relay da outbox falhou: {}", it.message) }
    }

    fun relayOnce(batch: Int = 100): Int = scope.runAsPlatform {
        var sent = 0
        for (event in outbox.lockPending(batch)) {
            event.attempts += 1
            try {
                val message = MessageBuilder.withPayload(event.payload)
                    .setHeader(KafkaHeaders.TOPIC, topic)
                    .setHeader(KafkaHeaders.KEY, event.aggregateId.toString())
                    .setHeader("event_type", event.eventType)
                    .build()
                kafka.send(message).get(5, TimeUnit.SECONDS)
                event.publishedAt = clock.instant()
                event.lastError = null
                sent++
            } catch (e: Exception) {
                event.lastError = e.message?.take(500)
                // Preserva a ordem por aggregate: não segue para o próximo evento depois de uma falha.
                outbox.save(event)
                break
            }
            outbox.save(event)
        }
        sent
    }
}
