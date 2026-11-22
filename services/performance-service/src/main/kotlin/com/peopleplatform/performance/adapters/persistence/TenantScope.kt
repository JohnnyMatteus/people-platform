package com.peopleplatform.performance.adapters.persistence

import com.fasterxml.jackson.databind.ObjectMapper
import com.peopleplatform.performance.app.DomainEvents
import com.peopleplatform.performance.domain.shared.TenantId
import jakarta.persistence.EntityManager
import jakarta.persistence.PersistenceContext
import org.springframework.stereotype.Component
import org.springframework.transaction.PlatformTransactionManager
import org.springframework.transaction.support.TransactionTemplate
import java.time.Instant
import java.util.UUID

/**
 * Abre uma transação com o escopo de tenant da sessão do Postgres (`app.tenant_id`, via
 * `set_config(..., true)`, equivalente a SET LOCAL). Sem escopo, a RLS não devolve linha alguma.
 */
@Component
class TenantScope(txManager: PlatformTransactionManager) {
    private val tx = TransactionTemplate(txManager)

    @PersistenceContext
    private lateinit var em: EntityManager

    fun <T> run(tenantId: TenantId, block: () -> T): T = inTransaction("app.tenant_id", tenantId.value.toString(), block)

    /** Só para o relay da outbox, que lê eventos de todos os tenants. */
    fun <T> runAsPlatform(block: () -> T): T = inTransaction("app.platform", "on", block)

    private fun <T> inTransaction(setting: String, value: String, block: () -> T): T {
        @Suppress("UNCHECKED_CAST")
        return tx.execute {
            em.createNativeQuery("select set_config(:name, :value, true)")
                .setParameter("name", setting)
                .setParameter("value", value)
                .singleResult
            block()
        } as T
    }
}

/** Grava o evento (envelope completo, ADR 0005) na outbox, dentro da transação em curso. */
@Component
class OutboxEvents(private val outbox: OutboxRepository, private val json: ObjectMapper) : DomainEvents {
    override fun publish(
        tenantId: TenantId,
        aggregateType: String,
        aggregateId: UUID,
        eventType: String,
        data: Map<String, Any?>,
        occurredAt: Instant,
    ) {
        val id = UUID.randomUUID()
        val envelope = linkedMapOf(
            "event_id" to id.toString(),
            "event_type" to eventType,
            "event_version" to 1,
            "tenant_id" to tenantId.value.toString(),
            "aggregate_id" to aggregateId.toString(),
            "occurred_at" to occurredAt.toString(),
            "producer" to "performance-service",
            "data" to data,
        )
        outbox.save(
            OutboxEntity(
                id, tenantId.value, aggregateType, aggregateId, eventType, 1,
                json.writeValueAsString(envelope), occurredAt, null, 0, null,
            ),
        )
    }
}
