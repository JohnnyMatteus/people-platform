import type { PoolClient } from 'pg';
import type { PrivacyRepository } from '../../app/ports';

export class PostgresPrivacy implements PrivacyRepository {
  constructor(private readonly c: PoolClient) {}

  /** Única via de exclusão: o papel da aplicação não tem DELETE; erase_subject age só no tenant do escopo. */
  async eraseSubject(subjectId: string, requestId: string): Promise<number> {
    const { rows } = await this.c.query('SELECT erase_subject($1::uuid, $2::uuid) AS removed', [subjectId, requestId]);
    return rows[0].removed as number;
  }

  async isErased(subjectId: string): Promise<boolean> {
    const { rowCount } = await this.c.query('SELECT 1 FROM erased_subjects WHERE subject_id = $1::uuid', [subjectId]);
    return (rowCount ?? 0) > 0;
  }
}
