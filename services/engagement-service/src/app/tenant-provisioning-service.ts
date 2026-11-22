import { settingsFromTenantCriado, type TenantCriadoEnvelope } from '../domain/tenant-settings';
import type { Deps } from './ports';

export class TenantProvisioningService {
  constructor(private readonly d: Deps) {}

  /** Consome TenantCriado: grava as configurações do módulo. Reentrega é segura (claim por event_id). */
  async onTenantCreated(e: TenantCriadoEnvelope): Promise<boolean> {
    const now = this.d.now();
    return this.d.store.inTenant(e.tenant_id, async (tx) => {
      if (!(await tx.processedEvents.claim(e.event_id, e.event_type, now))) return false;
      await tx.settings.upsert(settingsFromTenantCriado(e), now);
      return true;
    });
  }
}
