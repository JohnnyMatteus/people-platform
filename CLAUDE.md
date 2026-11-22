# CLAUDE.md

Guia para sessões de agente neste repositório.

## Visão

People Platform: plataforma de gestão de pessoas multitenant e white label, construída em microsserviços poliglotas. É um projeto de portfólio com domínio 100% fictício. Leia `README.md` (status por módulo) e `docs/design-doc.md` antes de mudar arquitetura.

## Regras inegociáveis

1. **Domínio fictício.** Nenhum nome real de empresa, pessoa ou produto de RH. E-mails só `@example.com`. Os tenants de exemplo são "Construtora Horizonte Exemplo" e "Rede Varejo Sol Exemplo".
2. **Nenhum segredo real.** Senhas de dev são `dev-only-change-me`. `.env` nunca é versionado; `.env.example` sim.
3. **Nunca leia** `data/`, `.env` ou `config.local.yaml`.
4. **Docs em português do Brasil; identificadores de código em inglês.** Nomes de evento ficam em português, em ASCII e PascalCase (`TenantCriado`), e os campos de payload ficam em inglês.
5. **Multitenancy:**
   - toda tabela nova tem `tenant_id NOT NULL` + RLS (`ENABLE` e `FORCE`), e todo evento tem `tenant_id`;
   - o tenant vem da claim do token, nunca da requisição;
   - toda persistência nova ganha um teste de vazamento entre tenants (veja `TestRLS_CrossTenantIsolation`).
6. **Contratos primeiro.** Mudou API ou evento? Atualize `contracts/` no mesmo PR. Mudança incompatível de evento exige `event_version` e tópico novos.
7. **Testes nunca acessam a internet.** Integração com Postgres usa testcontainers. LLM usa cliente falso.
8. **Commits:** Conventional Commits em português, mensagem num arquivo e `git commit -F <arquivo>` (nunca `-m`), terminando com a linha `Co-Authored-By` indicada pelo orquestrador. Não faça push.

## Comandos

```bash
make help                      # lista tudo
make lint                      # Spectral + gofmt/vet + ktlint
make test                      # unitários (Go -short + Kotlin via Docker)
make test-integration          # Go com Postgres real (testcontainers)
make up | down | ps | logs     # ambiente local (perfil core)
make tenant-service-up && make seed && make smoke
make tenant-service-test       # um alvo por serviço: <serviço>-lint|test|build
make performance-service-test  # roda na imagem gradle:8.14.3-jdk21 (sem Java local)
```

## Estrutura

- `services/tenant-service`: Go, hexagonal (`internal/domain`, `internal/app`, `internal/adapters/*`, `internal/platform/*`). Migrações SQL embutidas em `internal/adapters/postgres/migrations`.
- `services/engagement-service`: TypeScript/NestJS, hexagonal (`src/domain` puro, `src/app` casos de uso e portas, `src/adapters/{http,postgres,kafka}`, `src/platform`). Migrações em `migrations/`; `make engagement-service-test-unit` roda sem Docker e `make engagement-service-test` inclui a integração (testcontainers).
- `services/performance-service`: Kotlin. Só o núcleo de domínio puro em `com.peopleplatform.performance.domain` (nine box, calibração, histórico, sucessão). Spring Boot entra como adaptador; o domínio não importa framework.
- `contracts/openapi`: um arquivo por serviço (completos: tenant e performance; os demais são esboços).
- `contracts/asyncapi`: `people-platform.yaml` e `schemas/*.vN.json` (envelope comum).
- `docs/adr`: ADRs numerados. Decisão nova vira ADR novo; não reescreva ADR aceito.
- `docs/roteiros`: defesa dos núcleos para entrevista.
- `infra/` e `init/aws/`: compose, Keycloak, KrakenD, observabilidade, LocalStack.

## Fluxo de trabalho

1. Leia o ADR e o roteiro do núcleo que vai mexer.
2. Contrato primeiro (`contracts/`), depois domínio com testes (TDD onde couber), depois adaptadores.
3. Rode `make lint` e o `make <serviço>-test` antes de commitar; o CI roda os mesmos comandos.
4. Atualize o status por módulo no README e o roadmap do design doc quando algo mudar de estado.
5. Novo serviço: crie `services/<nome>/` com Makefile (`lint`, `test`, `build`), um job no `.github/workflows/ci.yml` com filtro de caminho, um perfil no `docker-compose.yml` e um banco em `infra/postgres/init/01-databases.sh` (que já cria os bancos dos serviços planejados).
