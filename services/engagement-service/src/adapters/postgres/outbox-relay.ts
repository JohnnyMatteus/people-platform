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

  /** Uma rodada; lança a primeira falha só depois de confirmar o que saiu (ver `round`). */
  async runOnce(): Promise<number> {
    const { sent, failures } = await this.round();
    if (failures.length > 0) throw failures[0];
    return sent;
  }

  /**
   * Os eventos de um tenant saem em ordem. Quando um falha, a falha é gravada só na linha dele (attempts, last_error)
   * dentro da transação, que commita o que já saiu (lançar de dentro desfaria tudo). As mensagens seguintes do
   * mesmo tenant esperam, para manter a ordem por chave, e o tenant sai da busca seguinte, então uma mensagem
   * envenenada não esgota o lote nem impede os outros tenants de publicar.
   */
  async round(): Promise<{ sent: number; failures: unknown[] }> {
    return this.store.asPlatform(async (c) => {
      let sent = 0;
      const failures: unknown[] = [];
      const blocked: string[] = [];
      while (sent + failures.length < this.batchSize) {
        const { rows } = await c.query(
          `SELECT * FROM outbox WHERE published_at IS NULL AND NOT (tenant_id = ANY($2::uuid[]))
           ORDER BY occurred_at, id LIMIT $1 FOR UPDATE SKIP LOCKED`,
          [this.batchSize - sent - failures.length, blocked],
        );
        if (rows.length === 0) break;
        const byTenant = new Map<string, OutboxEvent[]>();
        for (const r of rows) {
          const e: OutboxEvent = {
            id: r.id, tenantId: r.tenant_id, aggregateType: r.aggregate_type, aggregateId: r.aggregate_id,
            eventType: r.event_type, eventVersion: r.event_version, payload: r.payload, occurredAt: r.occurred_at,
          };
          byTenant.set(e.tenantId, [...(byTenant.get(e.tenantId) ?? []), e]);
        }
        for (const [tenantId, events] of byTenant) {
          const { published, failure } = await this.publishTenant(events);
          if (published.length > 0) {
            await c.query('UPDATE outbox SET published_at = now(), last_error = NULL WHERE id = ANY($1)', [published.map((e) => e.id)]);
          }
          sent += published.length;
          if (failure) {
            await c.query('UPDATE outbox SET attempts = attempts + 1, last_error = $2 WHERE id = $1', [
              events[published.length]!.id, String(failure.error).slice(0, 500),
            ]);
            failures.push(failure.error);
            blocked.push(tenantId);
          }
        }
      }
      return { sent, failures };
    });
  }

  /** Tenta o grupo todo; se o broker recusa, repete uma a uma em ordem para achar a linha que falha e para nela. */
  private async publishTenant(events: OutboxEvent[]): Promise<{ published: OutboxEvent[]; failure?: { error: unknown } }> {
    const message = (e: OutboxEvent) => ({ topic: TOPIC_ENGAGEMENT_EVENTS, key: e.tenantId, value: JSON.stringify(toEnvelope(e)) });
    try {
      await this.publisher.publish(events.map(message));
      return { published: events };
    } catch (error) {
      if (events.length === 1) return { published: [], failure: { error } };
      let failure: { error: unknown } = { error };
      const published: OutboxEvent[] = [];
      for (const e of events) {
        try {
          await this.publisher.publish([message(e)]);
          published.push(e);
        } catch (err) {
          failure = { error: err };
          return { published, failure };
        }
      }
      return { published };
    }
  }

  start(intervalMs: number): void {
    const tick = async () => {
      try {
        for (;;) {
          const { sent, failures } = await this.round();
          if (failures.length > 0) this.log.warn(`${failures.length} evento(s) falharam e seguem pendentes: ${String(failures[0])}`);
          if (sent === 0) break;
        }
      } catch (err) {
        this.log.warn(`falha ao ler a outbox, tentando de novo: ${String(err)}`);
      }
      this.timer = setTimeout(tick, intervalMs);
    };
    this.timer = setTimeout(tick, 0);
  }

  stop(): void {
    clearTimeout(this.timer);
  }
}
