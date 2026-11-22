export const TOPIC_ENGAGEMENT_EVENTS = 'people.engagement.events.v1';
export const PRODUCER = 'engagement-service';

export interface OutboxEvent {
  id: string;
  tenantId: string;
  aggregateType: string;
  aggregateId: string;
  eventType: 'ElogioPublicado' | 'PesquisaClimaEncerrada';
  eventVersion: 1;
  payload: Record<string, unknown>;
  occurredAt: Date;
}

/** Formato comum dos eventos da plataforma (contracts/asyncapi/schemas/envelope.v1.json). */
export function toEnvelope(e: OutboxEvent) {
  return {
    event_id: e.id,
    event_type: e.eventType,
    event_version: e.eventVersion,
    tenant_id: e.tenantId,
    aggregate_id: e.aggregateId,
    occurred_at: e.occurredAt.toISOString(),
    producer: PRODUCER,
    data: e.payload,
  };
}
