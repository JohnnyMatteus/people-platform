import type { PoolClient } from 'pg';
import type { PrivacyRepository } from '../../app/ports';

export class PostgresPrivacy implements PrivacyRepository {
  constructor(private readonly c: PoolClient) {}

  async eraseSubject(subjectId: string): Promise<number> {
    let changed = 0;
    const run = async (sql: string) => {
      const r = await this.c.query(sql, [subjectId]);
      changed += r.rowCount ?? 0;
    };
    // A RLS limita cada comando ao tenant do escopo; nenhum filtro de tenant é necessário (nem confiável) aqui.
    await run('DELETE FROM feedbacks WHERE from_id = $1::uuid OR to_id = $1::uuid');
    await run('DELETE FROM one_on_ones WHERE leader_id = $1::uuid OR participant_id = $1::uuid');
    // Elogio dado pela pessoa, ou recebido só por ela, sai inteiro; o recebido junto com outros só perde o nome dela.
    await run('DELETE FROM praises WHERE from_id = $1::uuid OR to_ids = ARRAY[$1::uuid]');
    await run('UPDATE praises SET to_ids = array_remove(to_ids, $1::uuid) WHERE $1::uuid = ANY(to_ids)');
    await run('DELETE FROM survey_participation WHERE employee_id = $1::uuid');
    await run('DELETE FROM points_ledger WHERE employee_id = $1::uuid');
    await run('DELETE FROM badges WHERE employee_id = $1::uuid');
    await run('DELETE FROM read_receipts WHERE employee_id = $1::uuid OR post_id IN (SELECT id FROM posts WHERE author_id = $1::uuid)');
    await run('DELETE FROM posts WHERE author_id = $1::uuid');
    return changed;
  }
}
