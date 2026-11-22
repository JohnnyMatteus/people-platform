-- Esquema do performance-service. Banco exclusivo deste serviço (database per service).
-- Toda tabela tem tenant_id NOT NULL e RLS com FORCE: nem o dono das tabelas escapa da política.

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'performance_app') THEN
    CREATE ROLE performance_app NOLOGIN;
  END IF;
END
$$;

CREATE FUNCTION app_current_tenant() RETURNS uuid
  LANGUAGE sql STABLE
  AS $$ SELECT NULLIF(current_setting('app.tenant_id', true), '')::uuid $$;

CREATE FUNCTION app_is_platform() RETURNS boolean
  LANGUAGE sql STABLE
  AS $$ SELECT coalesce(current_setting('app.platform', true), '') = 'on' $$;

CREATE TABLE nine_box_policies (
  tenant_id               uuid PRIMARY KEY,
  scale_min               numeric(6, 2) NOT NULL,
  scale_max               numeric(6, 2) NOT NULL,
  perf_medium_from        numeric(6, 2) NOT NULL,
  perf_high_from          numeric(6, 2) NOT NULL,
  pot_medium_from         numeric(6, 2) NOT NULL,
  pot_high_from           numeric(6, 2) NOT NULL,
  max_steps_per_axis      integer NOT NULL CHECK (max_steps_per_axis BETWEEN 1 AND 2),
  top_box_warning_share   numeric(5, 4) NOT NULL,
  min_justification_length integer NOT NULL CHECK (min_justification_length >= 0),
  updated_at              timestamptz NOT NULL
);

CREATE TABLE employees (
  tenant_id   uuid NOT NULL,
  id          uuid NOT NULL,
  name        text NOT NULL,
  job_title   text NOT NULL,
  role_since  date NOT NULL,
  PRIMARY KEY (tenant_id, id)
);

CREATE TABLE cycles (
  id         uuid PRIMARY KEY,
  tenant_id  uuid NOT NULL,
  name       text NOT NULL,
  type       text NOT NULL CHECK (type IN ('self', 'manager', 'peers', '180', '360')),
  status     text NOT NULL CHECK (status IN ('draft', 'open', 'calibration', 'closed')),
  created_at timestamptz NOT NULL
);
CREATE INDEX cycles_tenant_idx ON cycles (tenant_id, created_at DESC);

CREATE TABLE evaluations (
  id                uuid PRIMARY KEY,
  tenant_id         uuid NOT NULL,
  cycle_id          uuid NOT NULL REFERENCES cycles (id),
  employee_id       uuid NOT NULL,
  kind              text NOT NULL CHECK (kind IN ('self', 'manager')),
  evaluator_id      uuid NOT NULL,
  performance_score numeric(6, 2) NOT NULL,
  potential_score   numeric(6, 2),
  competencies      jsonb NOT NULL,
  comment           text,
  submitted_at      timestamptz NOT NULL,
  -- uma avaliação de cada tipo por pessoa e ciclo: reenviar é 409, não sobrescrita silenciosa.
  UNIQUE (cycle_id, employee_id, kind)
);

CREATE TABLE calibration_sessions (
  id                      uuid PRIMARY KEY,
  tenant_id               uuid NOT NULL,
  cycle_id                uuid NOT NULL UNIQUE REFERENCES cycles (id),
  status                  text NOT NULL CHECK (status IN ('open', 'closed')),
  -- política vigente na abertura: mudar a política depois não reescreve uma sessão em andamento.
  scale_min               numeric(6, 2) NOT NULL,
  scale_max               numeric(6, 2) NOT NULL,
  perf_medium_from        numeric(6, 2) NOT NULL,
  perf_high_from          numeric(6, 2) NOT NULL,
  pot_medium_from         numeric(6, 2) NOT NULL,
  pot_high_from           numeric(6, 2) NOT NULL,
  max_steps_per_axis      integer NOT NULL,
  top_box_warning_share   numeric(5, 4) NOT NULL,
  min_justification_length integer NOT NULL,
  opened_at               timestamptz NOT NULL,
  closed_at               timestamptz
);

CREATE TABLE calibration_placements (
  tenant_id         uuid NOT NULL,
  session_id        uuid NOT NULL REFERENCES calibration_sessions (id),
  employee_id       uuid NOT NULL,
  performance_score numeric(6, 2) NOT NULL,
  potential_score   numeric(6, 2) NOT NULL,
  PRIMARY KEY (session_id, employee_id)
);

CREATE TABLE calibration_adjustments (
  id            uuid PRIMARY KEY,
  tenant_id     uuid NOT NULL,
  session_id    uuid NOT NULL REFERENCES calibration_sessions (id),
  employee_id   uuid NOT NULL,
  from_quadrant text NOT NULL,
  to_quadrant   text NOT NULL,
  calibrated_by uuid NOT NULL,
  justification text NOT NULL,
  at            timestamptz NOT NULL
);
CREATE INDEX calibration_adjustments_session_idx ON calibration_adjustments (session_id, at);

CREATE TABLE nine_box_history (
  tenant_id   uuid NOT NULL,
  employee_id uuid NOT NULL,
  cycle_id    uuid NOT NULL REFERENCES cycles (id),
  quadrant    text NOT NULL,
  calibrated  boolean NOT NULL,
  closed_at   timestamptz NOT NULL,
  PRIMARY KEY (employee_id, cycle_id)
);

CREATE TABLE critical_positions (
  id           uuid PRIMARY KEY,
  tenant_id    uuid NOT NULL,
  title        text NOT NULL,
  incumbent_id uuid,
  created_at   timestamptz NOT NULL
);

CREATE TABLE successors (
  tenant_id    uuid NOT NULL,
  position_id  uuid NOT NULL REFERENCES critical_positions (id),
  employee_id  uuid NOT NULL,
  readiness    text NOT NULL CHECK (readiness IN ('READY_NOW', 'ONE_TO_TWO_YEARS', 'THREE_PLUS_YEARS')),
  nominated_by uuid NOT NULL,
  at           timestamptz NOT NULL,
  PRIMARY KEY (position_id, employee_id)
);

CREATE TABLE pdi_actions (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL,
  employee_id uuid NOT NULL,
  title       text NOT NULL,
  competency  text,
  kind        text NOT NULL CHECK (kind IN ('COURSE', 'MENTORING', 'PROJECT', 'READING')),
  due_on      date NOT NULL,
  status      text NOT NULL CHECK (status IN ('PLANNED', 'IN_PROGRESS', 'DONE')),
  created_by  uuid NOT NULL,
  created_at  timestamptz NOT NULL
);
CREATE INDEX pdi_actions_employee_idx ON pdi_actions (tenant_id, employee_id);

-- Transactional outbox: o evento sai na mesma transação da mudança de estado e é publicado
-- no Kafka depois, pelo relay (entrega at-least-once; consumidores deduplicam por event_id).
CREATE TABLE outbox (
  id             uuid PRIMARY KEY,
  tenant_id      uuid NOT NULL,
  aggregate_type text NOT NULL,
  aggregate_id   uuid NOT NULL,
  event_type     text NOT NULL,
  event_version  integer NOT NULL,
  payload        jsonb NOT NULL,
  occurred_at    timestamptz NOT NULL,
  published_at   timestamptz,
  attempts       integer NOT NULL DEFAULT 0,
  last_error     text
);
CREATE INDEX outbox_pending_idx ON outbox (occurred_at, id) WHERE published_at IS NULL;

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY[
    'nine_box_policies', 'employees', 'cycles', 'evaluations', 'calibration_sessions',
    'calibration_placements', 'calibration_adjustments', 'nine_box_history', 'critical_positions',
    'successors', 'pdi_actions', 'outbox'
  ] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format(
      'CREATE POLICY %I ON %I USING (app_is_platform() OR tenant_id = app_current_tenant())
         WITH CHECK (app_is_platform() OR tenant_id = app_current_tenant())',
      t || '_isolation', t);
  END LOOP;
END
$$;

GRANT USAGE ON SCHEMA public TO performance_app;
GRANT SELECT, INSERT, UPDATE ON
  nine_box_policies, employees, cycles, evaluations, calibration_sessions, calibration_placements,
  calibration_adjustments, nine_box_history, critical_positions, successors, pdi_actions, outbox
  TO performance_app;
GRANT EXECUTE ON FUNCTION app_current_tenant(), app_is_platform() TO performance_app;
