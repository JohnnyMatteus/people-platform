#!/usr/bin/env bash
# Cria um banco e dois usuários por serviço (database per service, ADR 0006 e 0010):
#   <svc>_owner  dono do esquema, usado só pelas migrações;
#   <svc>_app    usuário da aplicação, membro do papel <svc>_app_role e SEM BYPASSRLS.
# Senhas vêm do ambiente e são só de desenvolvimento (ver .env.example).
set -euo pipefail

create_service_db() {
  local db="$1" owner="$2" owner_pw="$3" app="$4" app_pw="$5" app_role="$6"
  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres <<SQL
CREATE ROLE ${owner} LOGIN PASSWORD '${owner_pw}';
CREATE DATABASE ${db} OWNER ${owner};
REVOKE ALL ON DATABASE ${db} FROM PUBLIC;
DO \$\$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '${app_role}') THEN
    CREATE ROLE ${app_role} NOLOGIN;
  END IF;
END \$\$;
CREATE ROLE ${app} LOGIN PASSWORD '${app_pw}' IN ROLE ${app_role};
GRANT CONNECT ON DATABASE ${db} TO ${owner}, ${app};
SQL
  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "${db}" <<SQL
ALTER SCHEMA public OWNER TO ${owner};
REVOKE ALL ON SCHEMA public FROM PUBLIC;
SQL
}

create_service_db tenant_service tenant_service_owner "$TENANT_DB_OWNER_PASSWORD" \
  tenant_service_app "$TENANT_DB_APP_PASSWORD" tenant_app
create_service_db performance_service performance_service_owner "$PERFORMANCE_DB_OWNER_PASSWORD" \
  performance_service_app "$PERFORMANCE_DB_APP_PASSWORD" performance_app
create_service_db recruitment_service recruitment_service_owner "$RECRUITMENT_DB_OWNER_PASSWORD" \
  recruitment_service_app "$RECRUITMENT_DB_APP_PASSWORD" recruitment_app
create_service_db engagement_service engagement_service_owner "$ENGAGEMENT_DB_OWNER_PASSWORD" \
  engagement_service_app "$ENGAGEMENT_DB_APP_PASSWORD" engagement_app
create_service_db analytics_service analytics_service_owner "$ANALYTICS_DB_OWNER_PASSWORD" \
  analytics_service_app "$ANALYTICS_DB_APP_PASSWORD" analytics_app
create_service_db keycloak keycloak "$KEYCLOAK_DB_PASSWORD" keycloak_app "$KEYCLOAK_DB_PASSWORD" keycloak_app_role
