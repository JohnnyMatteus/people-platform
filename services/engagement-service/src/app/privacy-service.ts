import type { ErasureRequestedEnvelope } from '../domain/privacy';
import type { Deps } from './ports';

export class PrivacyService {
  constructor(private readonly d: Deps) {}

  /**
   * LGPD: apaga o que o serviço guarda sobre o titular, só no tenant do evento. O claim por event_id e o apagamento saem
   * na mesma transação, então a reentrega do Kafka é segura e uma falha no meio não deixa o evento "processado".
   * subject_id é o colaborador (o `sub` do token, employee_id aqui); candidatos não existem neste serviço. Devolve false para evento repetido. As respostas de clima não têm identidade (ADR 0007), então não há o que apagar.
   */
  async onErasureRequested(e: ErasureRequestedEnvelope): Promise<boolean> {
    return this.d.store.inTenant(e.tenant_id, async (tx) => {
      if (!(await tx.processedEvents.claim(e.event_id, e.event_type, this.d.now()))) return false;
      await tx.privacy.eraseSubject(e.data.subject_id, e.data.request_id);
      return true;
    });
  }
}
