-- Esquema do analytics-ai-service. Banco exclusivo deste serviço (database per service, ADR 0006).
-- Read models projetados de eventos: nada aqui é fonte de verdade, e tudo tem tenant_id e RLS (ENABLE e FORCE).

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'analytics_app') THEN
    CREATE ROLE analytics_app NOLOGIN;
  END IF;
END
$$;

CREATE FUNCTION app_current_tenant() RETURNS uuid
  LANGUAGE sql STABLE
  AS $$ SELECT NULLIF(current_setting('app.tenant_id', true), '')::uuid $$;

CREATE TABLE processed_events (
  tenant_id    uuid NOT NULL,
  event_id     uuid NOT NULL,
  processed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, event_id)
);

CREATE TABLE tenant_settings (
  tenant_id         uuid PRIMARY KEY,
  slug              text NOT NULL,
  plan              text NOT NULL,
  analytics_enabled boolean NOT NULL,
  ai_enabled        boolean NOT NULL
);

CREATE TABLE employment_records (
  tenant_id     uuid NOT NULL,
  employee_id   uuid NOT NULL,
  hired_on      date NOT NULL,
  area          text NOT NULL,
  unit          text NOT NULL,
  birth_year    integer,
  gender        text NOT NULL,
  terminated_on date,
  age_band      text NOT NULL,
  PRIMARY KEY (tenant_id, employee_id)
);

CREATE TABLE assessment_facts (
  tenant_id   uuid NOT NULL,
  employee_id uuid NOT NULL,
  cycle_id    uuid NOT NULL,
  gaps        jsonb NOT NULL,
  PRIMARY KEY (tenant_id, employee_id)
);

CREATE TABLE nine_box_facts (
  tenant_id   uuid NOT NULL,
  cycle_id    uuid NOT NULL,
  employee_id uuid NOT NULL,
  quadrant    text NOT NULL,
  PRIMARY KEY (tenant_id, cycle_id, employee_id)
);

CREATE TABLE survey_facts (
  tenant_id      uuid NOT NULL,
  survey_id      uuid NOT NULL,
  min_group_size integer NOT NULL,
  segments       jsonb NOT NULL,
  PRIMARY KEY (tenant_id, survey_id)
);

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['processed_events', 'tenant_settings', 'employment_records', 'assessment_facts',
                           'nine_box_facts', 'survey_facts']
  LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format(
      'CREATE POLICY %I ON %I USING (tenant_id = app_current_tenant()) WITH CHECK (tenant_id = app_current_tenant())',
      t || '_isolation', t);
  END LOOP;
END
$$;

GRANT USAGE ON SCHEMA public TO analytics_app;
GRANT SELECT, INSERT, UPDATE ON processed_events, tenant_settings, employment_records, assessment_facts,
  nine_box_facts, survey_facts TO analytics_app;
GRANT EXECUTE ON FUNCTION app_current_tenant() TO analytics_app;
