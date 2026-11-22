import 'reflect-metadata';
import { randomUUID } from 'node:crypto';
import { Logger } from '@nestjs/common';
import { NestFactory } from '@nestjs/core';
import { Pool } from 'pg';
import { GamificationService } from './app/gamification-service';
import { AppModule } from './app.module';
import { TenantProvisioningService } from './app/tenant-provisioning-service';
import { GamificationConsumer, KafkaPublisher, newKafka, TenantEventsConsumer } from './adapters/kafka/kafka';
import { OutboxRelay } from './adapters/postgres/outbox-relay';
import { PostgresStore } from './adapters/postgres/store';
import { TokenVerifier } from './platform/auth';
import { loadConfig } from './platform/config';

async function bootstrap() {
  const cfg = loadConfig();
  const pool = new Pool({ connectionString: cfg.DATABASE_URL, max: 10 });
  const store = new PostgresStore(pool);
  const deps = { store, now: () => new Date(), newId: randomUUID };
  const verifier = TokenVerifier.remote(cfg.OIDC_ISSUER, cfg.OIDC_AUDIENCE, cfg.OIDC_JWKS_URL);

  const app = await NestFactory.create(
    AppModule.register({ deps, verifier, ready: async () => void (await pool.query('SELECT 1')) }),
    { logger: ['log', 'warn', 'error'] },
  );
  app.enableShutdownHooks();

  const kafka = newKafka(cfg.kafkaBrokers);
  const publisher = new KafkaPublisher(kafka);
  const relay = new OutboxRelay(store, publisher);
  const consumer = new GamificationConsumer(kafka, new GamificationService(deps));
  const tenantConsumer = new TenantEventsConsumer(kafka, new TenantProvisioningService(deps));
  relay.start(cfg.OUTBOX_INTERVAL_MS);
  consumer.start().catch((e) => Logger.error(`consumidor de gamificação falhou: ${String(e)}`, 'kafka'));

  tenantConsumer.start().catch((e) => Logger.error(`consumidor de TenantCriado falhou: ${String(e)}`, 'kafka'));

  const stop = async () => {
    relay.stop();
    await Promise.allSettled([consumer.stop(), tenantConsumer.stop(), publisher.disconnect(), app.close()]);
    await pool.end();
  };
  process.once('SIGTERM', () => void stop());
  process.once('SIGINT', () => void stop());

  await app.listen(cfg.HTTP_PORT, '0.0.0.0');
}

bootstrap().catch((e) => {
  console.error(e);
  process.exit(1);
});
