import { pointsForPraise } from '../domain/gamification';
import type { Deps } from './ports';
import type { Principal } from './principal';
import { awardAndBadge } from './social-service';

export interface ElogioPublicadoEnvelope {
  event_id: string;
  event_type: string;
  tenant_id: string;
  data: { from_employee_id: string; to_employee_ids: string[] };
}

export class GamificationService {
  constructor(private readonly d: Deps) {}

  /** Consome ElogioPublicado. Reentrega do Kafka é segura: o razão é único por (evento, pessoa, motivo). */
  async onPraisePublished(e: ElogioPublicadoEnvelope): Promise<void> {
    const entries = pointsForPraise(e.data.from_employee_id, e.data.to_employee_ids);
    await this.d.store.inTenant(e.tenant_id, (tx) => awardAndBadge(tx, entries, e.event_id, this.d.now(), this.d.newId));
  }

  mine(p: Principal) {
    return this.d.store.inTenant(p.tenantId, async (tx) => ({
      balance: await tx.points.balance(p.employeeId),
      ledger: await tx.points.ledger(p.employeeId, 50),
      badges: await tx.points.badges(p.employeeId),
    }));
  }

  leaderboard(p: Principal) {
    return this.d.store.inTenant(p.tenantId, (tx) => tx.points.leaderboard(10));
  }
}
