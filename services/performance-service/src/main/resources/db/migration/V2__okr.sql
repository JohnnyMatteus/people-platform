-- OKRs: objetivos desdobrados (empresa, equipe, pessoa), resultados-chave e check-ins.
-- Todas as tabelas têm tenant_id e RLS (ENABLE e FORCE), como as do V1.

CREATE TABLE objectives (
  id         uuid PRIMARY KEY,
  tenant_id  uuid NOT NULL,
  level      text NOT NULL CHECK (level IN ('COMPANY', 'TEAM', 'INDIVIDUAL')),
  owner_id   uuid,
  parent_id  uuid REFERENCES objectives (id),
  title      text NOT NULL,
  status     text NOT NULL CHECK (status IN ('ACTIVE', 'CLOSED')),
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL
);
CREATE INDEX objectives_tenant_level_idx ON objectives (tenant_id, level);
CREATE INDEX objectives_owner_idx ON objectives (tenant_id, owner_id);

CREATE TABLE key_results (
  id           uuid PRIMARY KEY,
  tenant_id    uuid NOT NULL,
  objective_id uuid NOT NULL REFERENCES objectives (id),
  title        text NOT NULL,
  start_value  numeric(18, 4) NOT NULL,
  target_value numeric(18, 4) NOT NULL CHECK (target_value <> start_value),
  current_value numeric(18, 4) NOT NULL
);
CREATE INDEX key_results_objective_idx ON key_results (tenant_id, objective_id);

CREATE TABLE check_ins (
  id            uuid PRIMARY KEY,
  tenant_id     uuid NOT NULL,
  key_result_id uuid NOT NULL REFERENCES key_results (id),
  new_value     numeric(18, 4) NOT NULL,
  comment       text,
  created_by    uuid NOT NULL,
  created_at    timestamptz NOT NULL
);
CREATE INDEX check_ins_key_result_idx ON check_ins (tenant_id, key_result_id, created_at);

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['objectives', 'key_results', 'check_ins'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format(
      'CREATE POLICY %I ON %I USING (app_is_platform() OR tenant_id = app_current_tenant())
         WITH CHECK (app_is_platform() OR tenant_id = app_current_tenant())',
      t || '_isolation', t);
  END LOOP;
END
$$;

GRANT SELECT, INSERT, UPDATE ON objectives, key_results, check_ins TO performance_app;
