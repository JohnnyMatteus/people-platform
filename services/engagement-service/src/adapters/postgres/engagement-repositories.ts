/* eslint-disable @typescript-eslint/no-explicit-any -- linhas cruas do driver pg */
import type { PoolClient } from 'pg';
import type {
  OutboxRepository,
  PointsRepository,
  ProcessedEventRepository,
  SurveyRepository,
  TenantSettingsRepository,
} from '../../app/ports';
import type { OutboxEvent } from '../../domain/events';
import type { PointEntry, PointReason } from '../../domain/gamification';
import type { Survey, SurveyResponse } from '../../domain/survey';
import type { TenantSettings } from '../../domain/tenant-settings';

const survey = (r: any): Survey => ({
  id: r.id, tenantId: r.tenant_id, title: r.title, status: r.status, questions: r.questions,
  segments: r.segments, minGroupSize: r.min_group_size, createdAt: r.created_at,
});

export class PostgresSurveys implements SurveyRepository {
  constructor(private readonly c: PoolClient) {}

  async insert(s: Survey) {
    await this.c.query(
      'INSERT INTO surveys (id, tenant_id, title, status, questions, segments, min_group_size, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)',
      [s.id, s.tenantId, s.title, s.status, JSON.stringify(s.questions), s.segments, s.minGroupSize, s.createdAt],
    );
  }

  async get(id: string) {
    const { rows } = await this.c.query('SELECT * FROM surveys WHERE id = $1', [id]);
    return rows[0] ? survey(rows[0]) : null;
  }

  async save(s: Survey) {
    await this.c.query('UPDATE surveys SET status = $2 WHERE id = $1', [s.id, s.status]);
  }

  async addParticipation(surveyId: string, employeeId: string, on: string) {
    const r = await this.c.query(
      `INSERT INTO survey_participation (tenant_id, survey_id, employee_id, participated_on)
       VALUES (app_current_tenant(), $1, $2, $3) ON CONFLICT DO NOTHING`,
      [surveyId, employeeId, on],
    );
    return (r.rowCount ?? 0) > 0;
  }

  async hasParticipated(surveyId: string, employeeId: string) {
    const { rowCount } = await this.c.query('SELECT 1 FROM survey_participation WHERE survey_id = $1 AND employee_id = $2', [
      surveyId, employeeId,
    ]);
    return (rowCount ?? 0) > 0;
  }

  async countParticipation(surveyId: string) {
    const { rows } = await this.c.query('SELECT count(*)::int AS n FROM survey_participation WHERE survey_id = $1', [surveyId]);
    return rows[0].n;
  }

  async insertResponse(r: SurveyResponse) {
    await this.c.query(
      'INSERT INTO survey_response (id, tenant_id, survey_id, segment_key, answers, comment, submitted_on) VALUES ($1,$2,$3,$4,$5,$6,$7)',
      [r.id, r.tenantId, r.surveyId, r.segmentKey, JSON.stringify(r.scores), r.comment, r.submittedOn],
    );
  }

  async listResponses(surveyId: string): Promise<SurveyResponse[]> {
    // Ordena pelo id (UUID v4): nenhuma ordem natural da tabela revela a ordem de envio.
    const { rows } = await this.c.query(
      `SELECT id, tenant_id, survey_id, segment_key, answers, comment, to_char(submitted_on, 'YYYY-MM-DD') AS submitted_on
       FROM survey_response WHERE survey_id = $1 ORDER BY id`,
      [surveyId],
    );
    return rows.map((r) => ({
      id: r.id, tenantId: r.tenant_id, surveyId: r.survey_id, segmentKey: r.segment_key,
      scores: r.answers, comment: r.comment, submittedOn: r.submitted_on,
    }));
  }
}

export class PostgresPoints implements PointsRepository {
  constructor(private readonly c: PoolClient) {}

  async award(entries: PointEntry[], sourceEventId: string, now: Date, newId: () => string) {
    const fresh: PointEntry[] = [];
    for (const e of entries) {
      const r = await this.c.query(
        `INSERT INTO points_ledger (id, tenant_id, employee_id, points, reason, source_event_id, created_at)
         VALUES ($1, app_current_tenant(), $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING`,
        [newId(), e.employeeId, e.points, e.reason, sourceEventId, now],
      );
      if ((r.rowCount ?? 0) > 0) fresh.push(e);
    }
    return fresh;
  }

  async stats(employeeId: string) {
    const { rows } = await this.c.query('SELECT reason, count(*)::int AS n FROM points_ledger WHERE employee_id = $1 GROUP BY reason', [
      employeeId,
    ]);
    const out: Record<PointReason, number> = { praise_received: 0, praise_given: 0, read_confirmed: 0 };
    for (const r of rows) out[r.reason as PointReason] = r.n;
    return out;
  }

  async balance(employeeId: string) {
    const { rows } = await this.c.query('SELECT coalesce(sum(points), 0)::int AS n FROM points_ledger WHERE employee_id = $1', [employeeId]);
    return rows[0].n;
  }

  async ledger(employeeId: string, limit: number) {
    const { rows } = await this.c.query(
      'SELECT points, reason, created_at FROM points_ledger WHERE employee_id = $1 ORDER BY created_at DESC LIMIT $2',
      [employeeId, limit],
    );
    return rows.map((r) => ({ points: r.points, reason: r.reason, createdAt: r.created_at }));
  }

  async badges(employeeId: string) {
    const { rows } = await this.c.query('SELECT code, awarded_at FROM badges WHERE employee_id = $1 ORDER BY awarded_at', [employeeId]);
    return rows.map((r) => ({ code: r.code, awardedAt: r.awarded_at }));
  }

  async addBadge(employeeId: string, code: string, now: Date) {
    await this.c.query(
      'INSERT INTO badges (tenant_id, employee_id, code, awarded_at) VALUES (app_current_tenant(), $1, $2, $3) ON CONFLICT DO NOTHING',
      [employeeId, code, now],
    );
  }

  async leaderboard(limit: number) {
    const { rows } = await this.c.query(
      'SELECT employee_id, sum(points)::int AS points FROM points_ledger GROUP BY employee_id ORDER BY points DESC, employee_id LIMIT $1',
      [limit],
    );
    return rows.map((r) => ({ employeeId: r.employee_id, points: r.points }));
  }
}

export class PostgresOutbox implements OutboxRepository {
  constructor(private readonly c: PoolClient) {}

  async add(e: OutboxEvent) {
    await this.c.query(
      `INSERT INTO outbox (id, tenant_id, aggregate_type, aggregate_id, event_type, event_version, payload, occurred_at)
       VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
      [e.id, e.tenantId, e.aggregateType, e.aggregateId, e.eventType, e.eventVersion, JSON.stringify(e.payload), e.occurredAt],
    );
  }
}

export class PostgresTenantSettings implements TenantSettingsRepository {
  constructor(private readonly c: PoolClient) {}

  async get() {
    const { rows } = await this.c.query(
      'SELECT tenant_id, slug, name, plan, climate_enabled, min_group_size FROM tenant_settings',
    );
    const r = rows[0];
    return r
      ? { tenantId: r.tenant_id, slug: r.slug, name: r.name, plan: r.plan, climateEnabled: r.climate_enabled, minGroupSize: r.min_group_size }
      : null;
  }

  async upsert(s: TenantSettings, now: Date) {
    await this.c.query(
      `INSERT INTO tenant_settings (tenant_id, slug, name, plan, climate_enabled, min_group_size, updated_at)
       VALUES ($1,$2,$3,$4,$5,$6,$7)
       ON CONFLICT (tenant_id) DO UPDATE SET slug = $2, name = $3, plan = $4, climate_enabled = $5, updated_at = $7`,
      [s.tenantId, s.slug, s.name, s.plan, s.climateEnabled, s.minGroupSize, now],
    );
  }
}

export class PostgresProcessedEvents implements ProcessedEventRepository {
  constructor(private readonly c: PoolClient) {}

  async claim(eventId: string, eventType: string, now: Date) {
    const r = await this.c.query(
      'INSERT INTO processed_events (tenant_id, event_id, event_type, processed_at) VALUES (app_current_tenant(), $1, $2, $3) ON CONFLICT DO NOTHING',
      [eventId, eventType, now],
    );
    return (r.rowCount ?? 0) > 0;
  }
}
