-- Preparo do tenant a partir de TenantCriado: configurações do módulo e deduplicação de eventos consumidos.

CREATE TABLE tenant_settings (
  tenant_id       uuid PRIMARY KEY,
  slug            text NOT NULL,
  name            text NOT NULL,
  plan            text NOT NULL,
  climate_enabled boolean NOT NULL,
  min_group_size  integer NOT NULL CHECK (min_group_size >= 3),
  updated_at      timestamptz NOT NULL
);

CREATE TABLE processed_events (
  tenant_id    uuid NOT NULL,
  event_id     uuid NOT NULL,
  event_type   text NOT NULL,
  processed_at timestamptz NOT NULL,
  PRIMARY KEY (tenant_id, event_id)
);

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['tenant_settings', 'processed_events']
  LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY %I ON %I USING (app_is_platform() OR tenant_id = app_current_tenant())
      WITH CHECK (app_is_platform() OR tenant_id = app_current_tenant())', t || '_isolation', t);
  END LOOP;
END
$$;

GRANT SELECT, INSERT, UPDATE ON tenant_settings TO engagement_app;
GRANT SELECT, INSERT ON processed_events TO engagement_app;
