import { Pool, type PoolClient } from 'pg';
import type { Store, Tx } from '../../app/ports';
import { invalid } from '../../domain/errors';
import { PostgresFeedbacks, PostgresOneOnOnes, PostgresPosts, PostgresPraises } from './social-repositories';
import { PostgresOutbox, PostgresPoints, PostgresSurveys } from './engagement-repositories';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

async function inTransaction<T>(pool: Pool, scope: [string, string], fn: (c: PoolClient) => Promise<T>): Promise<T> {
  const client = await pool.connect();
  try {
    await client.query('BEGIN');
    // is_local=true equivale a SET LOCAL: o escopo morre com a transação e não vaza para a
    // próxima requisição que reutilizar a conexão do pool.
    await client.query('SELECT set_config($1, $2, true)', scope);
    const out = await fn(client);
    await client.query('COMMIT');
    return out;
  } catch (e) {
    await client.query('ROLLBACK').catch(() => undefined);
    throw e;
  } finally {
    client.release();
  }
}

export class PostgresStore implements Store {
  constructor(readonly pool: Pool) {}

  inTenant<T>(tenantId: string, fn: (tx: Tx) => Promise<T>): Promise<T> {
    if (!UUID.test(tenantId)) throw invalid('escopo de tenant vazio ou inválido');
    return inTransaction(this.pool, ['app.tenant_id', tenantId], (c) => fn(txFor(c)));
  }

  /** Visão de todos os tenants. Só o relay da outbox usa. */
  asPlatform<T>(fn: (c: PoolClient) => Promise<T>): Promise<T> {
    return inTransaction(this.pool, ['app.platform', 'on'], fn);
  }
}

const txFor = (c: PoolClient): Tx => ({
  feedbacks: new PostgresFeedbacks(c),
  praises: new PostgresPraises(c),
  oneOnOnes: new PostgresOneOnOnes(c),
  posts: new PostgresPosts(c),
  surveys: new PostgresSurveys(c),
  points: new PostgresPoints(c),
  outbox: new PostgresOutbox(c),
});
