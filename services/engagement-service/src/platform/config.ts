import { z } from 'zod';

const schema = z.object({
  HTTP_PORT: z.coerce.number().int().default(8080),
  DATABASE_URL: z.string().min(1, 'DATABASE_URL é obrigatória'),
  DATABASE_MIGRATION_URL: z.string().optional(),
  KAFKA_BROKERS: z.string().min(1, 'KAFKA_BROKERS é obrigatória'),
  OIDC_ISSUER: z.string().url(),
  OIDC_JWKS_URL: z.string().url(),
  OIDC_AUDIENCE: z.string().default('people-api'),
  OUTBOX_INTERVAL_MS: z.coerce.number().int().default(1000),
});

export type Config = ReturnType<typeof loadConfig>;

/** Falha cedo, listando tudo que falta, em vez de quebrar na primeira requisição. */
export function loadConfig(env: NodeJS.ProcessEnv = process.env) {
  const r = schema.safeParse(env);
  if (!r.success) {
    throw new Error(`configuração inválida: ${r.error.issues.map((i) => `${i.path.join('.')}: ${i.message}`).join('; ')}`);
  }
  const c = r.data;
  return { ...c, kafkaBrokers: c.KAFKA_BROKERS.split(',').map((b) => b.trim()) };
}
