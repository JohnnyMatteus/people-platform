-- Esquema do learning-service. Banco exclusivo deste serviço (database per service, ADR 0006).

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'learning_app') THEN
    CREATE ROLE learning_app NOLOGIN;
  END IF;
END
$$;

CREATE FUNCTION app_current_tenant() RETURNS uuid
  LANGUAGE sql STABLE
  AS $$ SELECT NULLIF(current_setting('app.tenant_id', true), '')::uuid $$;

CREATE TABLE tracks (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL,
  title       text NOT NULL,
  description text NOT NULL DEFAULT '',
  created_at  timestamptz NOT NULL
);

CREATE TABLE track_items (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL,
  track_id    uuid NOT NULL REFERENCES tracks (id) ON DELETE CASCADE,
  position    integer NOT NULL,
  title       text NOT NULL,
  kind        text NOT NULL,
  minutes     integer NOT NULL,
  competency  text NOT NULL DEFAULT '',
  UNIQUE (track_id, position)
);

CREATE TABLE enrollments (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL,
  track_id    uuid NOT NULL REFERENCES tracks (id) ON DELETE CASCADE,
  employee_id uuid NOT NULL,
  source      text NOT NULL,
  enrolled_at timestamptz NOT NULL,
  UNIQUE (track_id, employee_id)
);
CREATE INDEX enrollments_employee_idx ON enrollments (employee_id);

CREATE TABLE enrollment_progress (
  enrollment_id uuid NOT NULL REFERENCES enrollments (id) ON DELETE CASCADE,
  item_id       uuid NOT NULL REFERENCES track_items (id) ON DELETE CASCADE,
  tenant_id     uuid NOT NULL,
  completed_at  timestamptz NOT NULL,
  PRIMARY KEY (enrollment_id, item_id)
);

-- Contratação que espera a identidade da pessoa (o evento traz a candidatura, não o colaborador).
CREATE TABLE onboarding_assignments (
  id             uuid PRIMARY KEY,
  tenant_id      uuid NOT NULL,
  application_id uuid NOT NULL,
  position_title text NOT NULL,
  unit           text NOT NULL,
  start_date     date NOT NULL,
  employee_id    uuid,
  enrollment_id  uuid REFERENCES enrollments (id) ON DELETE SET NULL,
  created_at     timestamptz NOT NULL,
  UNIQUE (tenant_id, application_id)
);

CREATE TABLE tenant_settings (
  tenant_id                  uuid PRIMARY KEY,
  default_onboarding_track   uuid REFERENCES tracks (id) ON DELETE SET NULL
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
  FOREACH t IN ARRAY ARRAY['tracks', 'track_items', 'enrollments', 'enrollment_progress', 'onboarding_assignments', 'tenant_settings', 'processed_events']
  LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format(
      'CREATE POLICY %I ON %I USING (tenant_id = app_current_tenant()) WITH CHECK (tenant_id = app_current_tenant())',
      t || '_isolation', t);
  END LOOP;
END
$$;

GRANT USAGE ON SCHEMA public TO learning_app;
GRANT SELECT, INSERT, UPDATE ON tracks, track_items, enrollments, enrollment_progress, onboarding_assignments,
  tenant_settings, processed_events TO learning_app;
GRANT EXECUTE ON FUNCTION app_current_tenant() TO learning_app;
