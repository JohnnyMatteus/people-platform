/* eslint-disable @typescript-eslint/no-explicit-any -- linhas cruas do driver pg */
import type { PoolClient } from 'pg';
import type { FeedbackRepository, OneOnOneRepository, PostRepository, PraiseRepository } from '../../app/ports';
import type { OneOnOne } from '../../domain/one-on-one';
import type { Feedback, Post, Praise, ReadReceipt } from '../../domain/social';

// O filtro por tenant fica por conta da RLS: as consultas não repetem tenant_id no WHERE de
// propósito, para que um esquecimento aqui nunca abra uma brecha. As colunas tenant_id só
// aparecem nos INSERTs (WITH CHECK recusa valor diferente do escopo).

const feedback = (r: any): Feedback => ({
  id: r.id, tenantId: r.tenant_id, fromId: r.from_id, toId: r.to_id, body: r.body, createdAt: r.created_at,
});
const praise = (r: any): Praise => ({
  id: r.id, tenantId: r.tenant_id, fromId: r.from_id, toIds: r.to_ids, message: r.message, values: r.praise_values, createdAt: r.created_at,
});
const meeting = (r: any): OneOnOne => ({
  id: r.id, tenantId: r.tenant_id, leaderId: r.leader_id, participantId: r.participant_id, scheduledFor: r.scheduled_for, agenda: r.agenda,
});
const post = (r: any): Post => ({
  id: r.id, tenantId: r.tenant_id, authorId: r.author_id, title: r.title, body: r.body, createdAt: r.created_at,
});

export class PostgresFeedbacks implements FeedbackRepository {
  constructor(private readonly c: PoolClient) {}

  async insert(f: Feedback) {
    await this.c.query('INSERT INTO feedbacks (id, tenant_id, from_id, to_id, body, created_at) VALUES ($1,$2,$3,$4,$5,$6)', [
      f.id, f.tenantId, f.fromId, f.toId, f.body, f.createdAt,
    ]);
  }

  async listFor(employeeId: string, withId?: string) {
    const { rows } = withId
      ? await this.c.query(
          `SELECT * FROM feedbacks WHERE (from_id = $1 AND to_id = $2) OR (from_id = $2 AND to_id = $1) ORDER BY created_at DESC LIMIT 200`,
          [employeeId, withId],
        )
      : await this.c.query('SELECT * FROM feedbacks WHERE from_id = $1 OR to_id = $1 ORDER BY created_at DESC LIMIT 200', [employeeId]);
    return rows.map(feedback);
  }
}

export class PostgresPraises implements PraiseRepository {
  constructor(private readonly c: PoolClient) {}

  async insert(p: Praise) {
    await this.c.query(
      'INSERT INTO praises (id, tenant_id, from_id, to_ids, message, praise_values, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)',
      [p.id, p.tenantId, p.fromId, p.toIds, p.message, p.values, p.createdAt],
    );
  }

  async recent(limit: number) {
    const { rows } = await this.c.query('SELECT * FROM praises ORDER BY created_at DESC LIMIT $1', [limit]);
    return rows.map(praise);
  }

  async recentBetween(a: string, b: string, limit: number) {
    const { rows } = await this.c.query(
      `SELECT * FROM praises WHERE (from_id = $1 AND $2 = ANY(to_ids)) OR (from_id = $2 AND $1 = ANY(to_ids))
       ORDER BY created_at DESC LIMIT $3`,
      [a, b, limit],
    );
    return rows.map(praise);
  }
}

export class PostgresOneOnOnes implements OneOnOneRepository {
  constructor(private readonly c: PoolClient) {}

  async insert(m: OneOnOne) {
    await this.c.query(
      'INSERT INTO one_on_ones (id, tenant_id, leader_id, participant_id, scheduled_for, agenda, created_at) VALUES ($1,$2,$3,$4,$5,$6,now())',
      [m.id, m.tenantId, m.leaderId, m.participantId, m.scheduledFor, JSON.stringify(m.agenda)],
    );
  }

  async get(id: string) {
    const { rows } = await this.c.query('SELECT * FROM one_on_ones WHERE id = $1', [id]);
    return rows[0] ? meeting(rows[0]) : null;
  }

  async save(m: OneOnOne) {
    await this.c.query('UPDATE one_on_ones SET agenda = $2, scheduled_for = $3 WHERE id = $1', [
      m.id, JSON.stringify(m.agenda), m.scheduledFor,
    ]);
  }

  async listFor(employeeId: string) {
    const { rows } = await this.c.query(
      'SELECT * FROM one_on_ones WHERE leader_id = $1 OR participant_id = $1 ORDER BY scheduled_for DESC LIMIT 200',
      [employeeId],
    );
    return rows.map(meeting);
  }
}

export class PostgresPosts implements PostRepository {
  constructor(private readonly c: PoolClient) {}

  async insert(p: Post) {
    await this.c.query('INSERT INTO posts (id, tenant_id, author_id, title, body, created_at) VALUES ($1,$2,$3,$4,$5,$6)', [
      p.id, p.tenantId, p.authorId, p.title, p.body, p.createdAt,
    ]);
  }

  async get(id: string) {
    const { rows } = await this.c.query('SELECT * FROM posts WHERE id = $1', [id]);
    return rows[0] ? post(rows[0]) : null;
  }

  async list(limit: number) {
    const { rows } = await this.c.query('SELECT * FROM posts ORDER BY created_at DESC LIMIT $1', [limit]);
    return rows.map(post);
  }

  async confirmRead(postId: string, employeeId: string, now: Date) {
    const r = await this.c.query(
      `INSERT INTO read_receipts (tenant_id, post_id, employee_id, read_at)
       VALUES (app_current_tenant(), $1, $2, $3) ON CONFLICT DO NOTHING`,
      [postId, employeeId, now],
    );
    return (r.rowCount ?? 0) > 0;
  }

  async receipts(postId: string): Promise<ReadReceipt[]> {
    const { rows } = await this.c.query('SELECT employee_id, read_at FROM read_receipts WHERE post_id = $1 ORDER BY read_at', [postId]);
    return rows.map((r) => ({ employeeId: r.employee_id, readAt: r.read_at }));
  }

  async readCounts(postIds: string[]) {
    const { rows } = await this.c.query(
      'SELECT post_id, count(*)::int AS n FROM read_receipts WHERE post_id = ANY($1) GROUP BY post_id',
      [postIds],
    );
    return new Map<string, number>(rows.map((r) => [r.post_id, r.n]));
  }

  async readBy(employeeId: string, postIds: string[]) {
    const { rows } = await this.c.query('SELECT post_id FROM read_receipts WHERE employee_id = $1 AND post_id = ANY($2)', [
      employeeId, postIds,
    ]);
    return new Set<string>(rows.map((r) => r.post_id));
  }
}
