import { addAgendaItem, assertParticipant, completeAgendaItem, scheduleOneOnOne, type OneOnOne } from '../domain/one-on-one';
import { notFound } from '../domain/errors';
import type { Feedback, Praise } from '../domain/social';
import type { AgendaItem } from '../domain/one-on-one';
import type { Deps } from './ports';
import type { Principal } from './principal';

export interface OneOnOneContext {
  meeting: OneOnOne;
  recentFeedbacks: Feedback[];
  recentPraises: Praise[];
  openAgenda: AgendaItem[];
}

export class OneOnOneService {
  constructor(private readonly d: Deps) {}

  async schedule(p: Principal, participantId: string, scheduledFor: Date): Promise<OneOnOne> {
    const m = scheduleOneOnOne(this.d.newId(), p.tenantId, p.employeeId, participantId, scheduledFor);
    return this.d.store.inTenant(p.tenantId, async (tx) => {
      await tx.oneOnOnes.insert(m);
      return m;
    });
  }

  list(p: Principal): Promise<OneOnOne[]> {
    return this.d.store.inTenant(p.tenantId, (tx) => tx.oneOnOnes.listFor(p.employeeId));
  }

  addItem(p: Principal, meetingId: string, text: string): Promise<OneOnOne> {
    return this.mutate(p, meetingId, (m) => addAgendaItem(m, this.d.newId(), p.employeeId, text));
  }

  completeItem(p: Principal, meetingId: string, itemId: string): Promise<OneOnOne> {
    return this.mutate(p, meetingId, (m) => completeAgendaItem(m, itemId, p.employeeId));
  }

  /** Feedbacks trocados entre as duas pessoas (só o que o chamador pode ver) e elogios recentes entre elas. */
  context(p: Principal, meetingId: string): Promise<OneOnOneContext> {
    return this.d.store.inTenant(p.tenantId, async (tx) => {
      const meeting = await this.load(tx.oneOnOnes, meetingId, p);
      const other = meeting.leaderId === p.employeeId ? meeting.participantId : meeting.leaderId;
      const [recentFeedbacks, recentPraises] = await Promise.all([
        tx.feedbacks.listFor(p.employeeId, other),
        tx.praises.recentBetween(p.employeeId, other, 10),
      ]);
      return {
        meeting,
        recentFeedbacks: recentFeedbacks.slice(0, 10),
        recentPraises,
        openAgenda: meeting.agenda.filter((i) => !i.done),
      };
    });
  }

  private mutate(p: Principal, meetingId: string, change: (m: OneOnOne) => OneOnOne): Promise<OneOnOne> {
    return this.d.store.inTenant(p.tenantId, async (tx) => {
      const next = change(await this.load(tx.oneOnOnes, meetingId, p));
      await tx.oneOnOnes.save(next);
      return next;
    });
  }

  private async load(repo: { get(id: string): Promise<OneOnOne | null> }, id: string, p: Principal) {
    const m = await repo.get(id);
    if (!m) throw notFound('1:1 não encontrada');
    assertParticipant(m, p.employeeId);
    return m;
  }
}
