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
import java.util.UUID
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

    /**
     * Os eventos de um tenant saem em ordem. Quando um falha, a falha é gravada só na linha dele
     * (attempts, lastError) e o tenant fica bloqueado na rodada: as mensagens seguintes dele esperam, para manter
     * a ordem, e ele sai da busca seguinte, então uma mensagem envenenada não esgota o lote nem impede os outros
     * tenants de publicar.
     * O erro nunca sai da transação, porque o rollback desfaria o que já foi publicado.
     */
    fun relayOnce(batch: Int = 100): Int = scope.runAsPlatform {
        var sent = 0
        var processed = 0
        // A lista nunca fica vazia (NOT IN () é inválido em SQL): o UUID nulo não é tenant de ninguém.
        val blocked = mutableSetOf(UUID(0, 0))
        while (processed < batch) {
            val pending = outbox.lockPending(batch - processed, blocked)
            if (pending.isEmpty()) break
            for (event in pending) {
                if (event.tenantId in blocked) continue
                processed++
                try {
                    val message = MessageBuilder.withPayload(event.payload)
                        .setHeader(KafkaHeaders.TOPIC, topic)
                        .setHeader(KafkaHeaders.KEY, event.tenantId.toString())
                        .setHeader("event_type", event.eventType)
                        .build()
                    kafka.send(message).get(5, TimeUnit.SECONDS)
                    event.publishedAt = clock.instant()
                    event.lastError = null
                    sent++
                } catch (e: Exception) {
                    event.attempts += 1
                    event.lastError = (e.message ?: e.javaClass.simpleName).take(500)
                    blocked += event.tenantId
                }
                outbox.save(event)
            }
        }
        sent
    }
}
