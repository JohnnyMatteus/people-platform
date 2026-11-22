import { DEFAULT_MIN_GROUP_SIZE } from './survey';

export const TOPIC_TENANT_EVENTS = 'people.tenant.events.v1';

export interface TenantSettings {
  tenantId: string;
  slug: string;
  name: string;
  plan: string;
  climateEnabled: boolean;
  minGroupSize: number;
}

export interface TenantCriadoEnvelope {
  event_id: string;
  event_type: string;
  event_version: number;
  tenant_id: string;
  data: { slug: string; name: string; plan: string; features: string[] };
}

export const settingsFromTenantCriado = (e: TenantCriadoEnvelope): TenantSettings => ({
  tenantId: e.tenant_id,
  slug: e.data.slug,
  name: e.data.name,
  plan: e.data.plan,
  climateEnabled: e.data.features.includes('climate_survey'),
  minGroupSize: DEFAULT_MIN_GROUP_SIZE,
});
