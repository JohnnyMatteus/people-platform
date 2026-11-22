import { forbidden, notFound } from '../domain/errors';
import { badgesEarned, POINTS, type PointEntry } from '../domain/gamification';
import { newFeedback, newPraise, type Feedback, type Post, type Praise, type ReadReceipt } from '../domain/social';
import type { Deps, Tx } from './ports';
import { hasRole, requireRole, type Principal } from './principal';

export interface PostView extends Post {
  readCount: number;
  readByMe: boolean;
}

export class SocialService {
  constructor(private readonly d: Deps) {}

  async giveFeedback(p: Principal, toId: string, body: string): Promise<Feedback> {
    const f = newFeedback(this.d.newId(), p.tenantId, p.employeeId, toId, body, this.d.now());
    return this.d.store.inTenant(p.tenantId, async (tx) => {
      await tx.feedbacks.insert(f);
      return f;
    });
  }

  listFeedbacks(p: Principal, withId?: string): Promise<Feedback[]> {
    return this.d.store.inTenant(p.tenantId, (tx) => tx.feedbacks.listFor(p.employeeId, withId));
  }

  /** O elogio e o evento ElogioPublicado entram na mesma transação (outbox). */
  async praise(p: Principal, toIds: string[], message: string, values: string[]): Promise<Praise> {
    const pr = newPraise(this.d.newId(), p.tenantId, p.employeeId, toIds, message, values, this.d.now());
    return this.d.store.inTenant(p.tenantId, async (tx) => {
      await tx.praises.insert(pr);
      await tx.outbox.add({
        id: this.d.newId(),
        tenantId: p.tenantId,
        aggregateType: 'praise',
        aggregateId: pr.id,
        eventType: 'ElogioPublicado',
        eventVersion: 1,
        payload: {
          praise_id: pr.id,
          from_employee_id: pr.fromId,
          to_employee_ids: pr.toIds,
          values: pr.values,
        },
        occurredAt: pr.createdAt,
      });
      return pr;
    });
  }

  feed(p: Principal): Promise<Praise[]> {
    return this.d.store.inTenant(p.tenantId, (tx) => tx.praises.recent(50));
  }

  async publishPost(p: Principal, title: string, body: string): Promise<Post> {
    requireRole(p, 'manager', 'tenant-admin');
    const post: Post = {
      id: this.d.newId(),
      tenantId: p.tenantId,
      authorId: p.employeeId,
      title: title.trim(),
      body: body.trim(),
      createdAt: this.d.now(),
    };
    return this.d.store.inTenant(p.tenantId, async (tx) => {
      await tx.posts.insert(post);
      return post;
    });
  }

  listPosts(p: Principal): Promise<PostView[]> {
    return this.d.store.inTenant(p.tenantId, async (tx) => {
      const posts = await tx.posts.list(50);
      const ids = posts.map((x) => x.id);
      const [counts, mine] = await Promise.all([tx.posts.readCounts(ids), tx.posts.readBy(p.employeeId, ids)]);
      return posts.map((x) => ({ ...x, readCount: counts.get(x.id) ?? 0, readByMe: mine.has(x.id) }));
    });
  }

  /** Idempotente: só a primeira confirmação pontua, e na mesma transação. */
  confirmRead(p: Principal, postId: string): Promise<void> {
    return this.d.store.inTenant(p.tenantId, async (tx) => {
      if (!(await tx.posts.get(postId))) throw notFound('comunicado não encontrado');
      const now = this.d.now();
      if (!(await tx.posts.confirmRead(postId, p.employeeId, now))) return;
      await awardAndBadge(tx, [{ employeeId: p.employeeId, points: POINTS.read_confirmed, reason: 'read_confirmed' }], postId, now, this.d.newId);
    });
  }

  receipts(p: Principal, postId: string): Promise<ReadReceipt[]> {
    return this.d.store.inTenant(p.tenantId, async (tx) => {
      const post = await tx.posts.get(postId);
      if (!post) throw notFound('comunicado não encontrado');
      if (post.authorId !== p.employeeId && !hasRole(p, 'tenant-admin')) {
        throw forbidden('só o autor ou o tenant-admin vê quem leu');
      }
      return tx.posts.receipts(postId);
    });
  }
}

/** Credita pontos (idempotente) e concede os badges que passaram a valer. */
export async function awardAndBadge(
  tx: Tx,
  entries: PointEntry[],
  sourceEventId: string,
  now: Date,
  newId: () => string,
): Promise<void> {
  // Titular com lápide não volta a ter pontos: um elogio ou uma leitura que chegue depois do pedido não recria o dado.
  const eligible: PointEntry[] = [];
  for (const e of entries) if (!(await tx.privacy.isErased(e.employeeId))) eligible.push(e);
  const fresh = await tx.points.award(eligible, sourceEventId, now, newId);
  for (const id of new Set(fresh.map((e) => e.employeeId))) {
    const has = (await tx.points.badges(id)).map((b) => b.code);
    for (const code of badgesEarned(await tx.points.stats(id), has)) await tx.points.addBadge(id, code, now);
  }
}
