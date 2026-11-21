#!/usr/bin/env bash
# Smoke test ponta a ponta do tenant-service pelo gateway, com tokens reais do Keycloak.
# Pré-requisitos: make tenant-service-up && make seed. Usa só curl e python3 (para ler JSON).
set -euo pipefail

GATEWAY="${GATEWAY:-http://localhost:8000}"
KEYCLOAK="${KEYCLOAK:-http://localhost:8180}"
PASSWORD="dev-only-change-me" # senha dos usuários de teste do realm local
HORIZONTE="0193a1c0-0000-7000-8000-000000000001"
SOL="0193a1c0-0000-7000-8000-000000000002"

token() {
  curl -fsS -X POST "$KEYCLOAK/realms/people/protocol/openid-connect/token" \
    -d grant_type=password -d client_id=people-cli -d "username=$1" -d "password=$PASSWORD" |
    python3 -c 'import json,sys; print(json.load(sys.stdin)["access_token"])'
}

expect() { # descrição, status esperado, args do curl...
  local desc="$1" want="$2"
  shift 2
  local got
  got=$(curl -s -o /dev/null -w '%{http_code}' "$@")
  if [[ "$got" != "$want" ]]; then
    echo "FALHOU: $desc (esperado $want, obtido $got)" >&2
    exit 1
  fi
  echo "ok   $desc ($got)"
}

OPERADOR=$(token operador@example.com)
ADMIN_H=$(token admin@horizonte.example.com)
ADMIN_S=$(token admin@sol.example.com)

expect "catálogo de planos é público" 200 "$GATEWAY/tenant/v1/plans"
expect "branding por domínio próprio" 200 -H "X-Forwarded-Host: horizonte.people.localhost" "$GATEWAY/tenant/v1/branding"
expect "domínio desconhecido não resolve" 404 -H "X-Forwarded-Host: nada.people.localhost" "$GATEWAY/tenant/v1/branding"
expect "sem token é barrado no gateway" 401 "$GATEWAY/tenant/v1/tenants"
expect "operador lista tenants" 200 -H "Authorization: Bearer $OPERADOR" "$GATEWAY/tenant/v1/tenants"
expect "admin de tenant não lista todos" 403 -H "Authorization: Bearer $ADMIN_H" "$GATEWAY/tenant/v1/tenants"
expect "admin lê o próprio tenant" 200 -H "Authorization: Bearer $ADMIN_H" "$GATEWAY/tenant/v1/tenants/$HORIZONTE"
expect "admin de Horizonte não vê Sol (404)" 404 -H "Authorization: Bearer $ADMIN_H" "$GATEWAY/tenant/v1/tenants/$SOL"
expect "resolução por claim" 200 -H "Authorization: Bearer $ADMIN_S" "$GATEWAY/tenant/v1/me/tenant"
expect "tema com contraste baixo é recusado" 422 -X PUT -H "Authorization: Bearer $ADMIN_S" \
  -H "Content-Type: application/json" "$GATEWAY/tenant/v1/tenants/$SOL/theme" \
  -d '{"primary":"#ffd700","on_primary":"#ffffff","background":"#ffffff","text":"#1a1a1a","accent":"#0b6e4f","email_from_name":"RH"}'
expect "upload de logo gera URL pré-assinada" 201 -X POST -H "Authorization: Bearer $ADMIN_H" \
  -H "Content-Type: application/json" "$GATEWAY/tenant/v1/tenants/$HORIZONTE/theme/logo-upload" \
  -d '{"content_type":"image/png"}'

echo "smoke test concluído"
