-- Esquema do engagement-service. Banco exclusivo deste serviço (database per service).

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'engagement_app') THEN
    CREATE ROLE engagement_app NOLOGIN;
  END IF;
END
$$;

CREATE FUNCTION app_current_tenant() RETURNS uuid
  LANGUAGE sql STABLE
  AS $$ SELECT NULLIF(current_setting('app.tenant_id', true), '')::uuid $$;

CREATE FUNCTION app_is_platform() RETURNS boolean
  LANGUAGE sql STABLE
  AS $$ SELECT coalesce(current_setting('app.platform', true), '') = 'on' $$;

CREATE TABLE feedbacks (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL,
  from_id     uuid NOT NULL,
  to_id       uuid NOT NULL,
  body        text NOT NULL,
  created_at  timestamptz NOT NULL
);
CREATE INDEX feedbacks_pair_idx ON feedbacks (tenant_id, from_id, to_id, created_at DESC);
CREATE INDEX feedbacks_to_idx ON feedbacks (tenant_id, to_id, created_at DESC);

CREATE TABLE praises (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL,
  from_id     uuid NOT NULL,
  to_ids      uuid[] NOT NULL,
  message     text NOT NULL,
  praise_values text[] NOT NULL DEFAULT '{}',
  created_at  timestamptz NOT NULL
);
CREATE INDEX praises_feed_idx ON praises (tenant_id, created_at DESC);

CREATE TABLE one_on_ones (
  id             uuid PRIMARY KEY,
  tenant_id      uuid NOT NULL,
  leader_id      uuid NOT NULL,
  participant_id uuid NOT NULL,
  scheduled_for  timestamptz NOT NULL,
  agenda         jsonb NOT NULL DEFAULT '[]',
  created_at     timestamptz NOT NULL
);
CREATE INDEX one_on_ones_people_idx ON one_on_ones (tenant_id, leader_id, participant_id);

CREATE TABLE surveys (
  id             uuid PRIMARY KEY,
  tenant_id      uuid NOT NULL,
  title          text NOT NULL,
  status         text NOT NULL CHECK (status IN ('open', 'closed')),
  questions      jsonb NOT NULL,
  segments       text[] NOT NULL,
  min_group_size integer NOT NULL CHECK (min_group_size >= 3),
  created_at     timestamptz NOT NULL
);

-- ADR 0007: participação e resposta são tabelas independentes. Não há coluna, chave estrangeira
-- nem índice que as ligue, e nenhuma das duas guarda horário preciso (só a data).
CREATE TABLE survey_participation (
  tenant_id       uuid NOT NULL,
  survey_id       uuid NOT NULL,
  employee_id     uuid NOT NULL,
  participated_on date NOT NULL,
  PRIMARY KEY (tenant_id, survey_id, employee_id)
);

CREATE TABLE survey_response (
  id           uuid PRIMARY KEY,
  tenant_id    uuid NOT NULL,
  survey_id    uuid NOT NULL,
  segment_key  text NOT NULL,
  answers      jsonb NOT NULL,
  comment      text,
  submitted_on date NOT NULL
);
CREATE INDEX survey_response_survey_idx ON survey_response (tenant_id, survey_id, segment_key);

CREATE TABLE points_ledger (
  id              uuid PRIMARY KEY,
  tenant_id       uuid NOT NULL,
  employee_id     uuid NOT NULL,
  points          integer NOT NULL,
  reason          text NOT NULL,
  source_event_id uuid NOT NULL,
  created_at      timestamptz NOT NULL,
  -- Entrega at-least-once do Kafka: o mesmo evento nunca pontua duas vezes a mesma pessoa pelo mesmo motivo.
  UNIQUE (tenant_id, source_event_id, employee_id, reason)
);
CREATE INDEX points_ledger_employee_idx ON points_ledger (tenant_id, employee_id, created_at DESC);

CREATE TABLE badges (
  tenant_id   uuid NOT NULL,
  employee_id uuid NOT NULL,
  code        text NOT NULL,
  awarded_at  timestamptz NOT NULL,
  PRIMARY KEY (tenant_id, employee_id, code)
);

CREATE TABLE posts (
  id         uuid PRIMARY KEY,
  tenant_id  uuid NOT NULL,
  author_id  uuid NOT NULL,
  title      text NOT NULL,
  body       text NOT NULL,
  created_at timestamptz NOT NULL
);
CREATE INDEX posts_feed_idx ON posts (tenant_id, created_at DESC);

CREATE TABLE read_receipts (
  tenant_id   uuid NOT NULL,
  post_id     uuid NOT NULL REFERENCES posts (id),
  employee_id uuid NOT NULL,
  read_at     timestamptz NOT NULL,
  PRIMARY KEY (tenant_id, post_id, employee_id)
);

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
  FOREACH t IN ARRAY ARRAY['feedbacks', 'praises', 'one_on_ones', 'surveys', 'survey_participation',
    'survey_response', 'points_ledger', 'badges', 'posts', 'read_receipts', 'outbox']
  LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY %I ON %I USING (app_is_platform() OR tenant_id = app_current_tenant())
      WITH CHECK (app_is_platform() OR tenant_id = app_current_tenant())', t || '_isolation', t);
  END LOOP;
END
$$;

GRANT USAGE ON SCHEMA public TO engagement_app;
-- Registros imutáveis (inclusive a resposta de clima, que não pode ser corrigida) ficam sem UPDATE.
GRANT SELECT, INSERT ON feedbacks, praises, survey_participation, survey_response, points_ledger, badges,
  posts, read_receipts TO engagement_app;
GRANT SELECT, INSERT, UPDATE ON one_on_ones, surveys, outbox TO engagement_app;
GRANT EXECUTE ON FUNCTION app_current_tenant(), app_is_platform() TO engagement_app;
