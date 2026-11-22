import { Logger } from '@nestjs/common';
import { Kafka, logLevel } from 'kafkajs';
import type { GamificationService, ElogioPublicadoEnvelope } from '../../app/gamification-service';
import type { PrivacyService } from '../../app/privacy-service';
import type { TenantProvisioningService } from '../../app/tenant-provisioning-service';
import { TOPIC_ENGAGEMENT_EVENTS } from '../../domain/events';
import { TOPIC_PRIVACY_EVENTS, type ErasureRequestedEnvelope } from '../../domain/privacy';
import { TOPIC_TENANT_EVENTS, type TenantCriadoEnvelope } from '../../domain/tenant-settings';
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
        let env: ElogioPublicadoEnvelope;
        try {
          env = JSON.parse(message.value?.toString() ?? '{}') as ElogioPublicadoEnvelope;
        } catch {
          this.log.warn('mensagem de gamificação com JSON inválido descartada');
          return;
        }
        // `null` é JSON válido e não tem campos.
        if (env?.event_type !== 'ElogioPublicado') return;
        if (!env.event_id || !env.tenant_id || !env.data?.from_employee_id || !Array.isArray(env.data.to_employee_ids)) {
          // Fora do contrato: repetir não adianta e travaria a partição (como o consumidor de tenants, descarta com aviso).
          this.log.warn('ElogioPublicado fora do contrato descartado');
          return;
        }
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

/** Lê TenantCriado do tenant-service e prepara as configurações do tenant. */
export class TenantEventsConsumer {
  private readonly log = new Logger(TenantEventsConsumer.name);
  private readonly consumer;

  constructor(
    kafka: Kafka,
    private readonly provisioning: TenantProvisioningService,
  ) {
    this.consumer = kafka.consumer({ groupId: 'engagement-tenant-provisioning' });
  }

  async start() {
    await this.consumer.connect();
    await this.consumer.subscribe({ topic: TOPIC_TENANT_EVENTS, fromBeginning: true });
    await this.consumer.run({
      eachMessage: async ({ message }) => {
        let env: TenantCriadoEnvelope;
        try {
          env = JSON.parse(message.value?.toString() ?? '{}') as TenantCriadoEnvelope;
        } catch {
          this.log.warn('mensagem de tenant com JSON inválido descartada');
          return;
        }
        if (env.event_type !== 'TenantCriado' || env.event_version !== 1) return;
        if (!Array.isArray(env.data?.features) || !env.data.slug || !env.tenant_id) {
          this.log.warn('TenantCriado fora do contrato descartado');
          return;
        }
        // Se lançar, o kafkajs reentrega; o claim por event_id torna o reprocessamento seguro.
        await this.provisioning.onTenantCreated(env);
      },
    });
    this.log.log('consumidor de TenantCriado ativo');
  }

  stop() {
    return this.consumer.disconnect();
  }
}

/** Lê TitularExclusaoSolicitada (LGPD) e apaga os dados do titular no tenant do evento. */
export class PrivacyEventsConsumer {
  private readonly log = new Logger(PrivacyEventsConsumer.name);
  private readonly consumer;

  constructor(
    kafka: Kafka,
    private readonly privacy: PrivacyService,
  ) {
    this.consumer = kafka.consumer({ groupId: 'engagement-privacy' });
  }

  async start() {
    await this.consumer.connect();
    await this.consumer.subscribe({ topic: TOPIC_PRIVACY_EVENTS, fromBeginning: true });
    await this.consumer.run({
      eachMessage: async ({ message }) => {
        let env: ErasureRequestedEnvelope;
        try {
          env = JSON.parse(message.value?.toString() ?? '{}') as ErasureRequestedEnvelope;
        } catch {
          this.log.warn('mensagem de privacidade com JSON inválido descartada');
          return;
        }
        if (env.event_type !== 'TitularExclusaoSolicitada') return;
        if (env.event_version !== 1 || !env.tenant_id || !env.event_id || !env.data?.subject_id) {
          this.log.warn('TitularExclusaoSolicitada fora do contrato descartado');
          return;
        }
        // Se lançar, o kafkajs reentrega; claim e apagamento na mesma transação tornam o reprocessamento seguro.
        const applied = await this.privacy.onErasureRequested(env);
        if (applied) this.log.log(`exclusão de titular aplicada (pedido ${env.data.request_id})`);
      },
    });
    this.log.log('consumidor de exclusão de titular ativo');
  }

  stop() {
    return this.consumer.disconnect();
  }
}
