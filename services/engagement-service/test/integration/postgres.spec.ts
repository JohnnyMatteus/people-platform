// Roda com Postgres real via testcontainers (precisa de Docker); no CI, não localmente.
import { TenantProvisioningService } from '../../src/app/tenant-provisioning-service';
import { tenantCriado } from '../support/fixtures';
import { randomUUID } from 'node:crypto';
import { join } from 'node:path';
import { PostgreSqlContainer, type StartedPostgreSqlContainer } from '@testcontainers/postgresql';
import { Client, Pool } from 'pg';
import { GamificationService } from '../../src/app/gamification-service';
import { SocialService } from '../../src/app/social-service';
import { SurveyService } from '../../src/app/survey-service';
import { migrate } from '../../src/adapters/postgres/migrate';
import { OutboxRelay } from '../../src/adapters/postgres/outbox-relay';
import { PostgresStore } from '../../src/adapters/postgres/store';
import { ANA, BRUNO, CARLA, depsFor, principal, TENANT_A, TENANT_B } from '../support/fixtures';

let pg: StartedPostgreSqlContainer;
let pool: Pool;
let store: PostgresStore;

beforeAll(async () => {
  pg = await new PostgreSqlContainer('postgres:17-alpine').start();
  await migrate(pg.getConnectionUri(), join(__dirname, '..', '..', 'migrations'));
  const admin = new Client({ connectionString: pg.getConnectionUri() });
  await admin.connect();
  // Usuário de login sem BYPASSRLS e que não é dono das tabelas: a RLS vale para ele.
  await admin.query(`CREATE ROLE app_login LOGIN PASSWORD 'dev-only-change-me' IN ROLE engagement_app`);
  await admin.end();
  const url = new URL(pg.getConnectionUri());
  url.username = 'app_login';
  url.password = 'dev-only-change-me';
  pool = new Pool({ connectionString: url.toString() });
  store = new PostgresStore(pool);
});

afterAll(async () => {
  await pool?.end();
  await pg?.stop();
});

const scoped = async <T>(tenant: string | null, fn: (c: import('pg').PoolClient) => Promise<T>): Promise<T> => {
  const c = await pool.connect();
  try {
    await c.query('BEGIN');
    if (tenant) await c.query("SELECT set_config('app.tenant_id', $1, true)", [tenant]);
    return await fn(c);
  } finally {
    await c.query('ROLLBACK');
    c.release();
  }
};

const TABLES = ['feedbacks', 'praises', 'one_on_ones', 'surveys', 'survey_participation', 'survey_response', 'points_ledger', 'badges', 'posts', 'read_receipts', 'outbox', 'tenant_settings', 'processed_events'];

describe('vazamento entre tenants (RLS)', () => {
  beforeAll(async () => {
    const a = principal(ANA, TENANT_A, ['tenant-admin', 'manager'], 'vendas');
    const d = depsFor(store);
    await new TenantProvisioningService(d).onTenantCreated(tenantCriado(TENANT_A, ['climate_survey']));
    const social = new SocialService(d);
    const survey = new SurveyService(d);
    await social.giveFeedback(a, BRUNO, 'privado');
    await social.praise(a, [BRUNO], 'parabéns', []);
    const post = await social.publishPost(a, 'Aviso', 'texto');
    await social.confirmRead(principal(BRUNO, TENANT_A), post.id);
    const s = await survey.create(a, { title: 'Clima', questions: [{ id: 'q1', text: 'ok' }], segments: ['vendas'] });
    await survey.answer(a, s.id, { q1: 4 }, 'comentário');
    await new GamificationService(d).onPraisePublished({ event_id: randomUUID(), event_type: 'ElogioPublicado', tenant_id: TENANT_A, data: { from_employee_id: ANA, to_employee_ids: [BRUNO] } });
    await store.inTenant(TENANT_A, (tx) => tx.oneOnOnes.insert({ id: randomUUID(), tenantId: TENANT_A, leaderId: ANA, participantId: BRUNO, scheduledFor: new Date(), agenda: [] }));
  });

  it.each(TABLES)('%s: o tenant dono vê, outro tenant e a sessão sem escopo não veem nada', async (table) => {
    const own = await scoped(TENANT_A, (c) => c.query(`SELECT count(*)::int AS n FROM ${table}`));
    expect(own.rows[0].n).toBeGreaterThan(0);
    const other = await scoped(TENANT_B, (c) => c.query(`SELECT count(*)::int AS n FROM ${table}`));
    expect(other.rows[0].n).toBe(0);
    const none = await scoped(null, (c) => c.query(`SELECT count(*)::int AS n FROM ${table}`));
    expect(none.rows[0].n).toBe(0);
  });

  it('um tenant não consegue gravar linha em nome de outro (WITH CHECK)', async () => {
    await expect(
      scoped(TENANT_B, (c) =>
        c.query(`INSERT INTO feedbacks (id, tenant_id, from_id, to_id, body, created_at) VALUES ($1,$2,$3,$4,'x',now())`, [randomUUID(), TENANT_A, ANA, BRUNO]),
      ),
    ).rejects.toThrow(/row-level security/);
  });

  it('o escopo não vaza para a próxima transação da mesma conexão', async () => {
    const small = new Pool({ connectionString: pool.options.connectionString, max: 1 });
    const one = new PostgresStore(small);
    await one.inTenant(TENANT_A, async () => undefined);
    const c = await small.connect();
    const r = await c.query(`SELECT count(*)::int AS n FROM feedbacks`);
    c.release();
    await small.end();
    expect(r.rows[0].n).toBe(0);
  });

  it('feedback de outro tenant é invisível pela API de aplicação, mesmo com os mesmos colaboradores', async () => {
    const social = new SocialService(depsFor(store));
    expect(await social.listFeedbacks(principal(BRUNO, TENANT_B))).toEqual([]);
    expect(await social.listFeedbacks(principal(BRUNO, TENANT_A))).toHaveLength(1);
    expect(await social.listFeedbacks(principal(CARLA, TENANT_A))).toHaveLength(0);
  });
});

describe('anonimato do clima no banco (ADR 0007)', () => {
  const cols = async (table: string) =>
    (await pool.query(`SELECT column_name FROM information_schema.columns WHERE table_name = $1`, [table])).rows.map((r) => r.column_name);

  it('survey_response não tem coluna de identidade nem horário preciso', async () => {
    const c = await cols('survey_response');
    expect(c.sort()).toEqual(['answers', 'comment', 'id', 'segment_key', 'submitted_on', 'survey_id', 'tenant_id']);
  });

  it('survey_participation não guarda resposta nem segmento, só a data', async () => {
    expect((await cols('survey_participation')).sort()).toEqual(['employee_id', 'participated_on', 'survey_id', 'tenant_id']);
  });

  it('nenhuma chave estrangeira ou índice liga participação e resposta', async () => {
    const fks = await pool.query(
      `SELECT conrelid::regclass::text AS t, confrelid::regclass::text AS r FROM pg_constraint
       WHERE contype = 'f' AND (conrelid::regclass::text IN ('survey_response','survey_participation') OR confrelid::regclass::text IN ('survey_response','survey_participation'))`,
    );
    expect(fks.rows).toEqual([]);
    const idx = await pool.query(`SELECT indexdef FROM pg_indexes WHERE tablename = 'survey_response'`);
    expect(idx.rows.map((r) => r.indexdef).join('\n')).not.toMatch(/employee/);
  });

  it('a resposta gravada não contém o employee_id em nenhum lugar (nem no comentário)', async () => {
    const survey = new SurveyService(depsFor(store));
    const me = principal(randomUUID(), TENANT_A, ['employee'], 'vendas');
    const s = await survey.create(principal(ANA, TENANT_A, ['tenant-admin']), { title: 't', questions: [{ id: 'q1', text: 'ok' }], segments: ['vendas'] });
    await survey.answer(me, s.id, { q1: 3 }, `escreva para ${me.employeeId}@example.com`);
    const dump = await scoped(TENANT_A, (c) => c.query(`SELECT to_jsonb(r)::text AS j FROM survey_response r WHERE survey_id = $1`, [s.id]));
    expect(dump.rows[0].j).not.toContain(me.employeeId);
    await expect(survey.answer(me, s.id, { q1: 1 })).rejects.toMatchObject({ code: 'conflict' });
  });

  it('o usuário da aplicação não consegue alterar nem apagar respostas (imutáveis)', async () => {
    await expect(scoped(TENANT_A, (c) => c.query(`UPDATE survey_response SET segment_key = 'x'`))).rejects.toThrow(/permission denied/);
    await expect(scoped(TENANT_A, (c) => c.query(`DELETE FROM survey_response`))).rejects.toThrow(/permission denied/);
  });

  it('resultados com k respondentes saem agregados; com menos, nada', async () => {
    const survey = new SurveyService(depsFor(store));
    const admin = principal(ANA, TENANT_A, ['tenant-admin']);
    const s = await survey.create(admin, { title: 'k', questions: [{ id: 'q1', text: 'ok' }], segments: ['rh'] });
    for (let i = 0; i < 4; i++) await survey.answer(principal(randomUUID(), TENANT_A, [], 'rh'), s.id, { q1: 5 });
    expect((await survey.results(admin, s.id)).segments).toEqual([]);
    await survey.answer(principal(randomUUID(), TENANT_A, [], 'rh'), s.id, { q1: 5 });
    expect((await survey.results(admin, s.id)).segments.map((x) => x.segmentKey)).toEqual(['_all', 'rh']);
    expect((await survey.results(principal(ANA, TENANT_B, ['tenant-admin']), s.id).catch((e) => e.code))).toBe('not_found');
  });
});

describe('outbox e gamificação idempotente', () => {
  it('o relay publica pendentes uma vez, de todos os tenants, e marca como publicados', async () => {
    const sent: { key: string; value: string }[] = [];
    const relay = new OutboxRelay(store, { publish: async (m) => void sent.push(...m) });
    const social = new SocialService(depsFor(store));
    await social.praise(principal(ANA, TENANT_B), [BRUNO], 'oi', []);
    const n = await relay.runOnce();
    expect(n).toBeGreaterThanOrEqual(2);
    expect(new Set(sent.map((s) => s.key))).toEqual(new Set([TENANT_A, TENANT_B]));
    expect(await relay.runOnce()).toBe(0);
    expect(JSON.parse(sent[0]!.value)).toMatchObject({ producer: 'engagement-service', event_version: 1 });
  });

  it('falha do Kafka mantém o evento pendente e conta a tentativa', async () => {
    await new SocialService(depsFor(store)).praise(principal(ANA, TENANT_A), [CARLA], 'oi', []);
    const relay = new OutboxRelay(store, { publish: async () => Promise.reject(new Error('kafka fora')) });
    await expect(relay.runOnce()).rejects.toThrow('kafka fora');
    const pending = await store.asPlatform((c) => c.query('SELECT attempts, last_error FROM outbox WHERE published_at IS NULL'));
    expect(pending.rows[0]).toMatchObject({ attempts: 1, last_error: expect.stringContaining('kafka fora') });
  });

  it('reentregar o mesmo evento não duplica pontos', async () => {
    const g = new GamificationService(depsFor(store));
    const e = { event_id: randomUUID(), event_type: 'ElogioPublicado', tenant_id: TENANT_B, data: { from_employee_id: ANA, to_employee_ids: [CARLA] } };
    await g.onPraisePublished(e);
    await g.onPraisePublished(e);
    expect((await g.mine(principal(CARLA, TENANT_B))).balance).toBe(10);
  });
});
