-- Registro dos pedidos de exclusão de titular (LGPD). O evento que sai para os outros serviços carrega só
-- identificadores; quem pediu e quando ficam aqui, no escopo do tenant, com RLS como o resto do esquema.
CREATE TABLE erasure_requests (
  id           uuid PRIMARY KEY,
  tenant_id    uuid NOT NULL,
  subject_id   uuid NOT NULL,
  requested_by text NOT NULL,
  requested_at timestamptz NOT NULL
);
CREATE INDEX erasure_requests_tenant_idx ON erasure_requests (tenant_id, requested_at);

ALTER TABLE erasure_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE erasure_requests FORCE ROW LEVEL SECURITY;
CREATE POLICY erasure_requests_isolation ON erasure_requests
  USING (app_is_platform() OR tenant_id = app_current_tenant())
  WITH CHECK (app_is_platform() OR tenant_id = app_current_tenant());

GRANT SELECT, INSERT ON erasure_requests TO tenant_app;
