-- Esquema do tenant-service. Banco exclusivo deste serviço (database per service).

-- Papel sem login usado pela aplicação. O usuário de login de cada ambiente é membro dele.
-- Ele não é dono das tabelas nem tem BYPASSRLS, então as políticas de RLS se aplicam.
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'tenant_app') THEN
    CREATE ROLE tenant_app NOLOGIN;
  END IF;
END
$$;

CREATE TABLE tenants (
  id          uuid PRIMARY KEY,
  slug        text NOT NULL UNIQUE,
  name        text NOT NULL,
  plan        text NOT NULL,
  status      text NOT NULL CHECK (status IN ('active', 'suspended', 'deactivated')),
  created_at  timestamptz NOT NULL,
  updated_at  timestamptz NOT NULL
);

CREATE TABLE tenant_themes (
  tenant_id         uuid PRIMARY KEY REFERENCES tenants (id),
  primary_color     char(7) NOT NULL,
  on_primary_color  char(7) NOT NULL,
  background_color  char(7) NOT NULL,
  text_color        char(7) NOT NULL,
  accent_color      char(7) NOT NULL,
  logo_url          text,
  email_from_name   text NOT NULL,
  email_reply_to    text,
  updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tenant_feature_overrides (
  tenant_id   uuid NOT NULL REFERENCES tenants (id),
  feature     text NOT NULL,
  enabled     boolean NOT NULL,
  updated_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, feature)
);

-- host é a chave primária: um domínio pertence a um único tenant na plataforma inteira.
CREATE TABLE tenant_domains (
  host        text PRIMARY KEY,
  tenant_id   uuid NOT NULL REFERENCES tenants (id),
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX tenant_domains_tenant_idx ON tenant_domains (tenant_id);

-- Transactional outbox: o evento é gravado na mesma transação da mudança de estado e
-- publicado no Kafka depois, pelo relay (entrega at-least-once).
CREATE TABLE outbox (
  id             uuid PRIMARY KEY,
  tenant_id      uuid NOT NULL,
  aggregate_type text NOT NULL,
  aggregate_id   uuid NOT NULL,
  event_type     text NOT NULL,
  event_version  integer NOT NULL,
  payload        jsonb NOT NULL,
  traceparent    text,
  occurred_at    timestamptz NOT NULL,
  published_at   timestamptz,
  attempts       integer NOT NULL DEFAULT 0,
  last_error     text
);
CREATE INDEX outbox_pending_idx ON outbox (occurred_at, id) WHERE published_at IS NULL;

-- Escopo da sessão, definido pela aplicação com SET LOCAL dentro de cada transação.
-- Sem escopo definido, nenhuma linha é visível (falha fechada).
CREATE FUNCTION app_current_tenant() RETURNS uuid
  LANGUAGE sql STABLE
  AS $$ SELECT NULLIF(current_setting('app.tenant_id', true), '')::uuid $$;

CREATE FUNCTION app_is_platform() RETURNS boolean
  LANGUAGE sql STABLE
  AS $$ SELECT coalesce(current_setting('app.platform', true), '') = 'on' $$;

ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants FORCE ROW LEVEL SECURITY;
CREATE POLICY tenants_isolation ON tenants
  USING (app_is_platform() OR id = app_current_tenant())
  WITH CHECK (app_is_platform() OR id = app_current_tenant());

ALTER TABLE tenant_themes ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_themes FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_themes_isolation ON tenant_themes
  USING (app_is_platform() OR tenant_id = app_current_tenant())
  WITH CHECK (app_is_platform() OR tenant_id = app_current_tenant());

ALTER TABLE tenant_feature_overrides ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_feature_overrides FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_feature_overrides_isolation ON tenant_feature_overrides
  USING (app_is_platform() OR tenant_id = app_current_tenant())
  WITH CHECK (app_is_platform() OR tenant_id = app_current_tenant());

ALTER TABLE tenant_domains ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_domains FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_domains_isolation ON tenant_domains
  USING (app_is_platform() OR tenant_id = app_current_tenant())
  WITH CHECK (app_is_platform() OR tenant_id = app_current_tenant());

ALTER TABLE outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE outbox FORCE ROW LEVEL SECURITY;
CREATE POLICY outbox_isolation ON outbox
  USING (app_is_platform() OR tenant_id = app_current_tenant())
  WITH CHECK (app_is_platform() OR tenant_id = app_current_tenant());

GRANT USAGE ON SCHEMA public TO tenant_app;
GRANT SELECT, INSERT, UPDATE ON tenants, tenant_themes, tenant_feature_overrides, tenant_domains, outbox TO tenant_app;
GRANT EXECUTE ON FUNCTION app_current_tenant(), app_is_platform() TO tenant_app;
