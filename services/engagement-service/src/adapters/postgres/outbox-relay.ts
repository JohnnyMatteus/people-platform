import { Logger } from '@nestjs/common';
import { toEnvelope, TOPIC_ENGAGEMENT_EVENTS, type OutboxEvent } from '../../domain/events';
import type { PostgresStore } from './store';

export interface EventPublisher {
  publish(messages: { topic: string; key: string; value: string }[]): Promise<void>;
}

/**
 * Publica a outbox no Kafka (entrega at-least-once; os consumidores deduplicam por event_id).
 * Vários relays podem rodar juntos: FOR UPDATE SKIP LOCKED dá cada linha a um só.
 */
export class OutboxRelay {
  private readonly log = new Logger(OutboxRelay.name);
  private timer?: NodeJS.Timeout;

  constructor(
    private readonly store: PostgresStore,
    private readonly publisher: EventPublisher,
    private readonly batchSize = 100,
  ) {}

  async runOnce(): Promise<number> {
    // A falha é gravada dentro da transação (que commita) e o erro é relançado só depois do commit;
    // lançar de dentro desfaria o UPDATE de attempts/last_error. O lock (FOR UPDATE) continua valendo.
    const result = await this.store.asPlatform<{ sent: number } | { failure: unknown }>(async (c) => {
      const { rows } = await c.query(
        `SELECT * FROM outbox WHERE published_at IS NULL ORDER BY occurred_at, id LIMIT $1 FOR UPDATE SKIP LOCKED`,
        [this.batchSize],
      );
      if (rows.length === 0) return { sent: 0 };
      const events: OutboxEvent[] = rows.map((r) => ({
        id: r.id, tenantId: r.tenant_id, aggregateType: r.aggregate_type, aggregateId: r.aggregate_id,
        eventType: r.event_type, eventVersion: r.event_version, payload: r.payload, occurredAt: r.occurred_at,
      }));
      try {
        await this.publisher.publish(
          events.map((e) => ({ topic: TOPIC_ENGAGEMENT_EVENTS, key: e.tenantId, value: JSON.stringify(toEnvelope(e)) })),
        );
      } catch (err) {
        await c.query('UPDATE outbox SET attempts = attempts + 1, last_error = $2 WHERE id = ANY($1)', [
          events.map((e) => e.id), String(err).slice(0, 500),
        ]);
        return { failure: err };
      }
      await c.query('UPDATE outbox SET published_at = now() WHERE id = ANY($1)', [events.map((e) => e.id)]);
      return { sent: events.length };
    });
    if ('failure' in result) throw result.failure;
    return result.sent;
  }

  start(intervalMs: number): void {
    const tick = async () => {
      try {
        while ((await this.runOnce()) > 0);
      } catch (err) {
        this.log.warn(`falha ao publicar a outbox, tentando de novo: ${String(err)}`);
      }
      this.timer = setTimeout(tick, intervalMs);
    };
    this.timer = setTimeout(tick, 0);
  }

  stop(): void {
    clearTimeout(this.timer);
  }
}
