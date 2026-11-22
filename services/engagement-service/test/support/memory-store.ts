import type { Store, Tx } from '../../src/app/ports';
import type { OutboxEvent } from '../../src/domain/events';
import type { PointEntry, PointReason } from '../../src/domain/gamification';
import type { OneOnOne } from '../../src/domain/one-on-one';
import type { Feedback, Post, Praise } from '../../src/domain/social';
import type { Survey, SurveyResponse } from '../../src/domain/survey';

interface Ledger extends PointEntry {
  tenantId: string;
  sourceEventId: string;
  createdAt: Date;
}

/** Armazém em memória para testes unitários; como a RLS, só enxerga as linhas do tenant do escopo. */
export class MemoryStore implements Store {
  feedbacks: Feedback[] = [];
  praises: Praise[] = [];
  meetings: OneOnOne[] = [];
  surveys: Survey[] = [];
  participation: { tenantId: string; surveyId: string; employeeId: string; on: string }[] = [];
  responses: SurveyResponse[] = [];
  ledger: Ledger[] = [];
  badges: { tenantId: string; employeeId: string; code: string; awardedAt: Date }[] = [];
  posts: Post[] = [];
  receipts: { tenantId: string; postId: string; employeeId: string; readAt: Date }[] = [];
  outbox: OutboxEvent[] = [];

  async inTenant<T>(t: string, fn: (tx: Tx) => Promise<T>): Promise<T> {
    const mine = <R extends { tenantId: string }>(rows: R[]) => rows.filter((r) => r.tenantId === t);
    // eslint-disable-next-line @typescript-eslint/no-this-alias
    const s = this;
    const tx: Tx = {
      feedbacks: {
        insert: async (f) => void s.feedbacks.push(f),
        listFor: async (e, w) =>
          mine(s.feedbacks)
            .filter((f) => (w ? (f.fromId === e && f.toId === w) || (f.fromId === w && f.toId === e) : f.fromId === e || f.toId === e))
            .sort((a, b) => b.createdAt.getTime() - a.createdAt.getTime()),
      },
      praises: {
        insert: async (p) => void s.praises.push(p),
        recent: async (n) => mine(s.praises).slice(-n).reverse(),
        recentBetween: async (a, b, n) =>
          mine(s.praises).filter((p) => (p.fromId === a && p.toIds.includes(b)) || (p.fromId === b && p.toIds.includes(a))).slice(-n).reverse(),
      },
      oneOnOnes: {
        insert: async (m) => void s.meetings.push(m),
        get: async (id) => mine(s.meetings).find((m) => m.id === id) ?? null,
        save: async (m) => void (s.meetings = s.meetings.map((x) => (x.id === m.id && x.tenantId === t ? m : x))),
        listFor: async (e) => mine(s.meetings).filter((m) => m.leaderId === e || m.participantId === e),
      },
      surveys: {
        insert: async (x) => void s.surveys.push(x),
        get: async (id) => mine(s.surveys).find((x) => x.id === id) ?? null,
        save: async (x) => void (s.surveys = s.surveys.map((y) => (y.id === x.id && y.tenantId === t ? x : y))),
        addParticipation: async (surveyId, employeeId, on) => {
          if (mine(s.participation).some((p) => p.surveyId === surveyId && p.employeeId === employeeId)) return false;
          s.participation.push({ tenantId: t, surveyId, employeeId, on });
          return true;
        },
        hasParticipated: async (id, e) => mine(s.participation).some((p) => p.surveyId === id && p.employeeId === e),
        countParticipation: async (id) => mine(s.participation).filter((p) => p.surveyId === id).length,
        insertResponse: async (r) => void s.responses.push(r),
        listResponses: async (id) => mine(s.responses).filter((r) => r.surveyId === id),
      },
      points: {
        award: async (entries, sourceEventId, now) => {
          const fresh: PointEntry[] = [];
          for (const e of entries) {
            if (mine(s.ledger).some((l) => l.sourceEventId === sourceEventId && l.employeeId === e.employeeId && l.reason === e.reason)) continue;
            s.ledger.push({ ...e, tenantId: t, sourceEventId, createdAt: now });
            fresh.push(e);
          }
          return fresh;
        },
        stats: async (e) => {
          const out: Record<PointReason, number> = { praise_received: 0, praise_given: 0, read_confirmed: 0 };
          mine(s.ledger).filter((l) => l.employeeId === e).forEach((l) => out[l.reason]++);
          return out;
        },
        balance: async (e) => mine(s.ledger).filter((l) => l.employeeId === e).reduce((n, l) => n + l.points, 0),
        ledger: async (e) => mine(s.ledger).filter((l) => l.employeeId === e).map((l) => ({ points: l.points, reason: l.reason, createdAt: l.createdAt })),
        badges: async (e) => mine(s.badges).filter((b) => b.employeeId === e),
        addBadge: async (e, code, now) => {
          if (!mine(s.badges).some((b) => b.employeeId === e && b.code === code)) s.badges.push({ tenantId: t, employeeId: e, code, awardedAt: now });
        },
        leaderboard: async (n) => {
          const sum = new Map<string, number>();
          mine(s.ledger).forEach((l) => sum.set(l.employeeId, (sum.get(l.employeeId) ?? 0) + l.points));
          return [...sum].map(([employeeId, points]) => ({ employeeId, points })).sort((a, b) => b.points - a.points).slice(0, n);
        },
      },
      posts: {
        insert: async (p) => void s.posts.push(p),
        get: async (id) => mine(s.posts).find((p) => p.id === id) ?? null,
        list: async (n) => mine(s.posts).slice(-n).reverse(),
        confirmRead: async (postId, employeeId, readAt) => {
          if (mine(s.receipts).some((r) => r.postId === postId && r.employeeId === employeeId)) return false;
          s.receipts.push({ tenantId: t, postId, employeeId, readAt });
          return true;
        },
        receipts: async (id) => mine(s.receipts).filter((r) => r.postId === id),
        readCounts: async (ids) => new Map(ids.map((id) => [id, mine(s.receipts).filter((r) => r.postId === id).length])),
        readBy: async (e, ids) => new Set(mine(s.receipts).filter((r) => r.employeeId === e && ids.includes(r.postId)).map((r) => r.postId)),
      },
      outbox: { add: async (e) => void s.outbox.push(e) },
    };
    return fn(tx);
  }
}
