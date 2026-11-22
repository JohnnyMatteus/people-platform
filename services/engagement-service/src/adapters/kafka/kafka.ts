import { Logger } from '@nestjs/common';
import { Kafka, logLevel } from 'kafkajs';
import type { GamificationService, ElogioPublicadoEnvelope } from '../../app/gamification-service';
import { TOPIC_ENGAGEMENT_EVENTS } from '../../domain/events';
import type { EventPublisher } from '../postgres/outbox-relay';

export const newKafka = (brokers: string[]) => new Kafka({ clientId: 'engagement-service', brokers, logLevel: logLevel.WARN });

export class KafkaPublisher implements EventPublisher {
  private readonly producer;
  private connected = false;

  constructor(kafka: Kafka) {
    this.producer = kafka.producer({ idempotent: true });
  }

  async publish(messages: { topic: string; key: string; value: string }[]) {
    if (!this.connected) {
      await this.producer.connect();
      this.connected = true;
    }
    const byTopic = Map.groupBy(messages, (m) => m.topic);
    await this.producer.sendBatch({
      topicMessages: [...byTopic].map(([topic, ms]) => ({ topic, messages: ms.map(({ key, value }) => ({ key, value })) })),
    });
  }

  disconnect() {
    return this.producer.disconnect();
  }
}

/** Gamificação: lê os próprios eventos de elogio e credita pontos, com deduplicação por event_id. */
export class GamificationConsumer {
  private readonly log = new Logger(GamificationConsumer.name);
  private readonly consumer;

  constructor(
    kafka: Kafka,
    private readonly gamification: GamificationService,
  ) {
    this.consumer = kafka.consumer({ groupId: 'engagement-gamification' });
  }

  async start() {
    await this.consumer.connect();
    await this.consumer.subscribe({ topic: TOPIC_ENGAGEMENT_EVENTS, fromBeginning: true });
    await this.consumer.run({
      eachMessage: async ({ message }) => {
        const env = JSON.parse(message.value?.toString() ?? '{}') as ElogioPublicadoEnvelope;
        if (env.event_type !== 'ElogioPublicado') return;
        // Se lançar, o kafkajs reentrega; o razão de pontos é idempotente.
        await this.gamification.onPraisePublished(env);
      },
    });
    this.log.log('consumidor de gamificação ativo');
  }

  stop() {
    return this.consumer.disconnect();
  }
}
