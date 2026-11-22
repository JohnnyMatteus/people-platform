export const TOPIC_PRIVACY_EVENTS = 'people.privacy.events.v1';

/** TitularExclusaoSolicitada v1 (contracts/asyncapi/schemas/titular-exclusao-solicitada.v1.json). */
export interface ErasureRequestedEnvelope {
  event_id: string;
  event_type: string;
  event_version: number;
  tenant_id: string;
  data: { request_id: string; subject_id: string };
}
