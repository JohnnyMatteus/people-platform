# engagement-service

TypeScript + NestJS. Feedback privado com histórico, elogios públicos, 1:1 com pauta e contexto, clima anônimo ([ADR 0007](../../docs/adr/0007-anonimato-do-clima-por-desenho.md)), gamificação por eventos e rede social com confirmação de leitura. Contrato: [`contracts/openapi/engagement-service.yaml`](../../contracts/openapi/engagement-service.yaml).

## Estrutura

- `src/domain`: regras puras (k-anonimato e supressão complementar em `anonymity.ts`, pontos e badges, 1:1, feedback).
- `src/app`: casos de uso e portas (`Store.inTenant` abre a transação com o escopo do tenant).
- `src/adapters/postgres`: repositórios, outbox com relay (`FOR UPDATE SKIP LOCKED`) e migrador. `adapters/kafka`: publicador e consumidor da gamificação. `adapters/http`: controllers, guard JWT e erros RFC 9457.
- `migrations/`: esquema com RLS (`ENABLE` + `FORCE`) em toda tabela.

## Decisões

- O tenant e o colaborador vêm do JWT (`tenant_id`, `sub`); o segmento do clima vem da claim `segment_key`.
- Gamificação consome o próprio `ElogioPublicado`; o razão é único por (evento, pessoa, motivo), então reentrega não duplica pontos.
- Resposta de clima: sem `employee_id`, sem horário (só a data), imutável para o usuário da aplicação.

## Comandos

```bash
make install && make lint && make test-unit   # sem Docker
make test                                     # inclui Postgres real (testcontainers)
```

Na raiz: `make engagement-service-up` (porta 8083 no host, `ENGAGEMENT_SERVICE_PORT`).

Variáveis: `DATABASE_URL`, `KAFKA_BROKERS`, `OIDC_ISSUER`, `OIDC_JWKS_URL`, `OIDC_AUDIENCE`, `HTTP_PORT`, `OUTBOX_INTERVAL_MS`; a migração usa `DATABASE_MIGRATION_URL` (usuário dono do esquema).
