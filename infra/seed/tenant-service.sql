-- Seed de desenvolvimento do tenant-service: os dois tenants fictícios com IDs fixos, que são
-- os mesmos da claim tenant_id dos usuários do realm do Keycloak (infra/keycloak/realm-people.json).
-- Idempotente. Roda como dono do esquema (make seed); os eventos TenantCriado também entram na
-- outbox, então o relay os publica no Kafka como num cadastro feito pela API.
BEGIN;

-- As tabelas têm FORCE ROW LEVEL SECURITY, que vale inclusive para o dono: o seed usa o escopo de plataforma.
SET LOCAL app.platform = 'on';

INSERT INTO tenants (id, slug, name, plan, status, created_at, updated_at) VALUES
  ('0193a1c0-0000-7000-8000-000000000001', 'construtora-horizonte', 'Construtora Horizonte Exemplo', 'corporativo', 'active', now(), now()),
  ('0193a1c0-0000-7000-8000-000000000002', 'rede-varejo-sol', 'Rede Varejo Sol Exemplo', 'crescimento', 'active', now(), now())
ON CONFLICT (id) DO NOTHING;

INSERT INTO tenant_themes (tenant_id, primary_color, on_primary_color, background_color, text_color, accent_color,
                           logo_url, email_from_name, email_reply_to) VALUES
  ('0193a1c0-0000-7000-8000-000000000001', '#8a2d00', '#ffffff', '#ffffff', '#1f1f1f', '#005a9c',
   NULL, 'RH Construtora Horizonte Exemplo', 'rh@horizonte.example.com'),
  ('0193a1c0-0000-7000-8000-000000000002', '#004d40', '#ffffff', '#fffdf5', '#1a1a1a', '#8c4a00',
   NULL, 'Gente e Cultura Rede Varejo Sol', 'pessoas@sol.example.com')
ON CONFLICT (tenant_id) DO NOTHING;

-- *.localhost resolve para 127.0.0.1 em navegadores e no curl: dá para testar domínio próprio sem /etc/hosts.
INSERT INTO tenant_domains (host, tenant_id) VALUES
  ('horizonte.people.localhost', '0193a1c0-0000-7000-8000-000000000001')
ON CONFLICT (host) DO NOTHING;

INSERT INTO tenant_feature_overrides (tenant_id, feature, enabled) VALUES
  ('0193a1c0-0000-7000-8000-000000000002', 'analytics', true)
ON CONFLICT (tenant_id, feature) DO NOTHING;

INSERT INTO outbox (id, tenant_id, aggregate_type, aggregate_id, event_type, event_version, payload, occurred_at) VALUES
  ('0193a1c0-0000-7000-8000-0000000000e1', '0193a1c0-0000-7000-8000-000000000001', 'tenant',
   '0193a1c0-0000-7000-8000-000000000001', 'TenantCriado', 1,
   '{"slug":"construtora-horizonte","name":"Construtora Horizonte Exemplo","plan":"corporativo","features":["ai_assist","analytics","climate_survey","custom_domain","engagement","idp","nine_box","okr","performance","recruitment","succession"]}',
   now()),
  ('0193a1c0-0000-7000-8000-0000000000e2', '0193a1c0-0000-7000-8000-000000000002', 'tenant',
   '0193a1c0-0000-7000-8000-000000000002', 'TenantCriado', 1,
   '{"slug":"rede-varejo-sol","name":"Rede Varejo Sol Exemplo","plan":"crescimento","features":["climate_survey","engagement","idp","nine_box","okr","performance","recruitment"]}',
   now())
ON CONFLICT (id) DO NOTHING;

COMMIT;
