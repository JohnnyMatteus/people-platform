-- Exclusão de titular (LGPD). O papel da aplicação continua sem DELETE: a imutabilidade dos read models é proposital. A
-- exclusão passa por uma função estreita, SECURITY DEFINER, que só age no tenant do escopo da sessão.

-- Lápide sem dado pessoal: faz os projetores ignorarem dado que chegue depois do pedido (não há ordem entre tópicos).
-- A aplicação só lê; quem escreve é a função.
CREATE TABLE erased_subjects (
  tenant_id  uuid NOT NULL,
  subject_id uuid NOT NULL,
  request_id uuid NOT NULL,
  erased_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, subject_id)
);
ALTER TABLE erased_subjects ENABLE ROW LEVEL SECURITY;
ALTER TABLE erased_subjects FORCE ROW LEVEL SECURITY;
CREATE POLICY erased_subjects_isolation ON erased_subjects
  USING (tenant_id = app_current_tenant()) WITH CHECK (tenant_id = app_current_tenant());
GRANT SELECT ON erased_subjects TO analytics_app;

-- Roda com os privilégios do dono. FORCE RLS vale também para o dono e a política compara com app_current_tenant(), que
-- lê o escopo da sessão; o filtro de tenant explícito é a segunda trava e vale mesmo se o dono tiver BYPASSRLS.
-- search_path fixo impede que um objeto homônimo em outro esquema assuma o lugar.
CREATE FUNCTION erase_subject(p_subject uuid, p_request uuid) RETURNS integer
  LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public
  AS $$
DECLARE
  v_tenant uuid := app_current_tenant();
  v_total  integer := 0;
  v_rows   integer;
BEGIN
  IF v_tenant IS NULL THEN
    RAISE EXCEPTION 'erase_subject exige escopo de tenant (app.tenant_id)';
  END IF;
  IF p_subject IS NULL OR p_request IS NULL THEN
    RAISE EXCEPTION 'erase_subject exige subject e request';
  END IF;

  DELETE FROM employment_records WHERE tenant_id = v_tenant AND employee_id = p_subject;
  GET DIAGNOSTICS v_rows = ROW_COUNT;
  v_total := v_total + v_rows;

  DELETE FROM assessment_facts WHERE tenant_id = v_tenant AND employee_id = p_subject;
  GET DIAGNOSTICS v_rows = ROW_COUNT;
  v_total := v_total + v_rows;

  DELETE FROM nine_box_facts WHERE tenant_id = v_tenant AND employee_id = p_subject;
  GET DIAGNOSTICS v_rows = ROW_COUNT;
  v_total := v_total + v_rows;

  INSERT INTO erased_subjects (tenant_id, subject_id, request_id) VALUES (v_tenant, p_subject, p_request)
    ON CONFLICT (tenant_id, subject_id) DO NOTHING;
  RETURN v_total;
END
$$;
REVOKE ALL ON FUNCTION erase_subject(uuid, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION erase_subject(uuid, uuid) TO analytics_app;
