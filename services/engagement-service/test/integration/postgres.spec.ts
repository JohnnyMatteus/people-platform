// Roda com Postgres real via testcontainers (precisa de Docker); no CI, não localmente.
import { TenantProvisioningService } from '../../src/app/tenant-provisioning-service';
import { tenantCriado } from '../support/fixtures';
import { randomUUID } from 'node:crypto';
import { join } from 'node:path';
import { PostgreSqlContainer, type StartedPostgreSqlContainer } from '@testcontainers/postgresql';
import { Client, Pool } from 'pg';
import { GamificationService } from '../../src/app/gamification-service';
import { OneOnOneService } from '../../src/app/one-on-one-service';
import { PrivacyService } from '../../src/app/privacy-service';
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

  it('tenant com mensagem envenenada não esgota o lote nem fura a ordem dos outros, e attempts sobe só na linha recusada', async () => {
    await store.asPlatform((c) => c.query("UPDATE outbox SET published_at = now() WHERE published_at IS NULL"));
    const seed = async (tenant: string, n: number, startMs: number) => {
      const ids: string[] = [];
      for (let i = 0; i < n; i++) {
        const id = randomUUID();
        ids.push(id);
        await store.asPlatform((c) =>
          c.query(
            `INSERT INTO outbox (id, tenant_id, aggregate_type, aggregate_id, event_type, event_version, payload, occurred_at)
             VALUES ($1, $2, 'praise', $1, 'ElogioPublicado', 1, '{}', $3)`,
            [id, tenant, new Date(startMs + i * 1000)],
          ),
        );
      }
      return ids;
    };
    const t0 = Date.now() - 3_600_000;
    // A é a mais antiga e tem mais pendentes que o lote (5): sozinha, esgotaria todas as buscas.
    const a = await seed(TENANT_A, 7, t0);
    const b = await seed(TENANT_B, 4, t0 + 60_000);
    const reject = new Set([a[1], b[2]]);
    const sent: { key: string; id: string }[] = [];
    const relay = new OutboxRelay(
      store,
      {
        publish: async (msgs) => {
          const parsed = msgs.map((m) => ({ key: m.key, id: JSON.parse(m.value).event_id as string }));
          if (parsed.some((m) => reject.has(m.id))) throw new Error('mensagem recusada pelo broker');
          sent.push(...parsed);
        },
      },
      5,
    );

    const round = await relay.round();
    expect(round.sent).toBe(3);
    expect(round.failures).toHaveLength(2);
    expect(sent.filter((m) => m.key === TENANT_A).map((m) => m.id)).toEqual([a[0]]);
    expect(sent.filter((m) => m.key === TENANT_B).map((m) => m.id)).toEqual([b[0], b[1]]);

    const state = await store.asPlatform((c) => c.query('SELECT id, attempts, published_at, last_error FROM outbox WHERE id = ANY($1)', [[...a, ...b]]));
    const byId = new Map(state.rows.map((r) => [r.id, r]));
    for (const id of [...a, ...b]) {
      expect(byId.get(id)!.attempts).toBe(reject.has(id) ? 1 : 0);
    }
    expect(byId.get(a[1]!)).toMatchObject({ published_at: null, last_error: expect.stringContaining('recusada') });
    expect(byId.get(a[2]!)!.published_at).toBeNull();
    expect(byId.get(b[3]!)!.published_at).toBeNull();

    reject.clear();
    sent.length = 0;
    while ((await relay.round()).sent > 0);
    expect(sent.filter((m) => m.key === TENANT_A).map((m) => m.id)).toEqual(a.slice(1));
    expect(sent.filter((m) => m.key === TENANT_B).map((m) => m.id)).toEqual(b.slice(2));
  });

  it('reentregar o mesmo evento não duplica pontos', async () => {
    const g = new GamificationService(depsFor(store));
    const e = { event_id: randomUUID(), event_type: 'ElogioPublicado', tenant_id: TENANT_B, data: { from_employee_id: ANA, to_employee_ids: [CARLA] } };
    await g.onPraisePublished(e);
    await g.onPraisePublished(e);
    expect((await g.mine(principal(CARLA, TENANT_B))).balance).toBe(10);
  });
});

describe('exclusão de titular no Postgres real, com o papel da aplicação', () => {
  const asAppRole = async (tenant: string, sql: string, params: unknown[] = []) => scoped(tenant, (c) => c.query(sql, params));
  const erasure = (tenant: string, subject: string, eventId = randomUUID()) => ({
    event_id: eventId,
    event_type: 'TitularExclusaoSolicitada',
    event_version: 1,
    tenant_id: tenant,
    data: { request_id: randomUUID(), subject_id: subject },
  });
  const count = async (tenant: string, table: string, column: string, id: string) =>
    Number((await asAppRole(tenant, `SELECT count(*)::int AS n FROM ${table} WHERE ${column} = $1`, [id])).rows[0].n);

  it('o papel da aplicação não apaga direto; a função exige escopo e não é chamável por PUBLIC', async () => {
    for (const table of ['feedbacks', 'praises', 'one_on_ones', 'points_ledger', 'badges', 'posts', 'read_receipts', 'survey_participation', 'erased_subjects']) {
      await expect(asAppRole(TENANT_A, `DELETE FROM ${table}`)).rejects.toThrow(/permission denied/);
    }
    await expect(asAppRole(TENANT_A, 'INSERT INTO erased_subjects (tenant_id, subject_id, request_id) VALUES ($1, $2, $3)', [TENANT_A, randomUUID(), randomUUID()])).rejects.toThrow(
      /permission denied/,
    );
    await expect(scoped(null, (c) => c.query('SELECT erase_subject($1::uuid, $2::uuid)', [randomUUID(), randomUUID()]))).rejects.toThrow(/escopo de tenant/);
    const { rows } = await pool.query(
      "SELECT EXISTS (SELECT 1 FROM pg_proc p, aclexplode(p.proacl) a WHERE p.proname = 'erase_subject' AND a.grantee = 0 AND a.privilege_type = 'EXECUTE') AS public_can",
    );
    expect(rows[0].public_can).toBe(false);
  });

  it('apaga o titular, preserva outro titular e o mesmo id em outro tenant, reentrega é no-op e a lápide impede a ressurreição', async () => {
    const d = depsFor(store);
    const social = new SocialService(d);
    const gamification = new GamificationService(d);
    const subject = randomUUID();
    const bystander = randomUUID();
    for (const tenant of [TENANT_A, TENANT_B]) {
      await social.giveFeedback(principal(subject, tenant), bystander, 'bom trabalho');
      await social.giveFeedback(principal(bystander, tenant), subject, 'obrigado');
      // Sem o titular em nenhuma ponta: é o que precisa sobreviver à exclusão.
      await social.giveFeedback(principal(bystander, tenant), CARLA, 'boa entrega');
      await social.praise(principal(subject, tenant), [bystander], 'valeu', []);
      await social.praise(principal(bystander, tenant), [subject, CARLA], 'ótimo time', []);
      await new OneOnOneService(d).schedule(principal(subject, tenant, ['manager']), bystander, new Date('2026-12-01T10:00:00Z'));
      const post = await social.publishPost(principal(subject, tenant, ['manager']), 'Aviso', 'texto');
      await social.confirmRead(principal(bystander, tenant), post.id);
    }
    const outbox = await store.asPlatform((c) =>
      // Outros testes deste arquivo gravam ElogioPublicado com payload vazio; só os de elogio de verdade entram aqui.
      c.query("SELECT id, tenant_id, payload FROM outbox WHERE event_type = 'ElogioPublicado' AND payload ? 'to_employee_ids'"),
    );
    for (const e of outbox.rows) {
      await gamification.onPraisePublished({ event_id: e.id, event_type: 'ElogioPublicado', tenant_id: e.tenant_id, data: e.payload });
    }
    expect(await count(TENANT_A, 'feedbacks', 'from_id', subject)).toBeGreaterThan(0);

    const privacy = new PrivacyService(d);
    const event = erasure(TENANT_A, subject);
    expect(await privacy.onErasureRequested(event)).toBe(true);

    for (const [table, column] of [['feedbacks', 'from_id'], ['feedbacks', 'to_id'], ['praises', 'from_id'], ['one_on_ones', 'leader_id'], ['points_ledger', 'employee_id'], ['posts', 'author_id']] as const) {
      expect(await count(TENANT_A, table, column, subject)).toBe(0);
    }
    expect(Number((await asAppRole(TENANT_A, 'SELECT count(*)::int AS n FROM praises WHERE $1 = ANY(to_ids)', [subject])).rows[0].n)).toBe(0);
    // O elogio recebido junto com a Carla continua, só sem o nome do titular.
    expect((await asAppRole(TENANT_A, 'SELECT to_ids FROM praises WHERE from_id = $1', [bystander])).rows[0].to_ids).toEqual([CARLA]);
    // Outro titular do mesmo tenant e o mesmo id no outro tenant ficam intactos.
    expect(await count(TENANT_A, 'feedbacks', 'from_id', bystander)).toBeGreaterThan(0);
    expect(await count(TENANT_B, 'feedbacks', 'from_id', subject)).toBeGreaterThan(0);
    expect(await count(TENANT_B, 'points_ledger', 'employee_id', subject)).toBeGreaterThan(0);

    expect(await privacy.onErasureRequested(event)).toBe(false);

    // Ressurreição: elogio e leitura que chegam depois do pedido não recriam pontos do titular apagado.
    await social.praise(principal(bystander, TENANT_A), [subject], 'tarde demais', []);
    const late = await store.asPlatform((c) =>
      c.query("SELECT id, tenant_id, payload FROM outbox WHERE event_type = 'ElogioPublicado' AND tenant_id = $1 AND payload->'to_employee_ids' = to_jsonb(ARRAY[$2::text])", [TENANT_A, subject]),
    );
    await gamification.onPraisePublished({ event_id: late.rows[0].id, event_type: 'ElogioPublicado', tenant_id: TENANT_A, data: late.rows[0].payload });
    expect(await count(TENANT_A, 'points_ledger', 'employee_id', subject)).toBe(0);
    expect(await count(TENANT_A, 'badges', 'employee_id', subject)).toBe(0);
  });
});
