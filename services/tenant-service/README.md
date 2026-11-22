# tenant-service (Go)

É o registro de tenants da People Platform. Cuida do cadastro, dos planos e feature flags, do tema white label (com validação de contraste WCAG AA), dos domínios próprios, da resolução de tenant por host ou por claim e da publicação dos eventos `TenantCriado` e `TenantPlanoAlterado` via transactional outbox.

## Arquitetura

Hexagonal, com as dependências apontando para dentro:

```
cmd/tenant-service          composição (main): config, pools, adaptadores, servidor e relay
internal/domain             regras puras: Tenant, Plan/Feature, Theme e contraste WCAG, eventos
internal/app                casos de uso, autorização (Principal) e portas (Repository, AssetStorage)
internal/adapters/postgres  repositório com RLS, migrações embutidas, outbox e relay
internal/adapters/httpapi   API HTTP (net/http), problem+json, métricas por rota
internal/adapters/kafka     Publisher da outbox (franz-go, acks=all)
internal/adapters/s3        URL pré-assinada para upload de logo (S3 ou LocalStack)
internal/adapters/memory    repositório em memória para testes
internal/platform           config, auth (JWT/JWKS do Keycloak) e telemetria
```

## Comandos

```bash
make lint        # gofmt + go vet
make test-unit   # testes sem Docker (-short)
make test        # tudo, inclusive Postgres real via testcontainers (precisa de Docker)
make build       # binário em bin/
```

Para rodar localmente, `make up` na raiz do monorepo sobe Postgres, Kafka, Keycloak e LocalStack. Depois, `make tenant-service-up` sobe este serviço (perfil `tenant-service` do compose), já com migração e seed.

## Variáveis de ambiente

| Variável | Obrigatória | Descrição |
| --- | --- | --- |
| `DATABASE_URL` | sim | DSN do usuário da aplicação (membro de `tenant_app`, sujeito a RLS) |
| `DATABASE_MIGRATION_URL` | só em `migrate` | DSN do dono do esquema |
| `KAFKA_BROKERS` | sim | lista separada por vírgula |
| `OIDC_ISSUER`, `OIDC_JWKS_URL` | sim | realm do Keycloak |
| `OIDC_AUDIENCE` | não (`people-api`) | audiência exigida no token |
| `S3_ASSETS_BUCKET`, `S3_PRESIGN_ENDPOINT`, `ASSETS_PUBLIC_BASE_URL` | não | assets white label (logo) |
| `AWS_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` | não | credenciais da cadeia padrão da AWS (no local, `test`/`test` do LocalStack) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | não | sem ela, os spans não são exportados |
| `OUTBOX_POLL_INTERVAL`, `OUTBOX_BATCH_SIZE` | não (`1s`, `100`) | relay da outbox |
| `KEYCLOAK_URL` | não | liga o provisionador: com ela, `TenantCriado` vira uma Organization no Keycloak |
| `KEYCLOAK_PROVISIONER_CLIENT_SECRET` | com `KEYCLOAK_URL` | segredo do client de serviço `people-provisioner` (client credentials) |
| `KEYCLOAK_REALM`, `KEYCLOAK_PROVISIONER_CLIENT_ID`, `TENANT_EVENTS_TOPIC`, `KAFKA_PROVISIONER_GROUP` | não | `people`, `people-provisioner`, `people.tenant.events.v1`, `tenant-service-keycloak-provisioner` |

## Provisionador do Keycloak

`internal/provisioning` consome `TenantCriado` e chama a porta `IdentityProvisioner`; o adaptador `internal/adapters/keycloak` usa a Admin REST API (Organizations, Keycloak 26). A Organization leva o `alias` igual ao slug e o atributo `tenant_id`, e a busca por esse atributo (`q=tenant_id:<uuid>`) torna a operação idempotente. Falha transitória reentrega a mensagem; recusa definitiva é descartada com log (`ErrPermanent`).

**Não foi exercitado contra um Keycloak real.** Os endpoints e os campos de `OrganizationRepresentation` foram conferidos no `openapi.json` da Admin API publicada pelo Keycloak; o adaptador é testado contra um dublê HTTP local. Hipótese não confirmada: que o service account precisa dos papéis `manage-realm` e `view-realm` de `realm-management` (é o que o `realm-people.json` concede). Domínios de e-mail da Organization e a associação de usuários a ela ficam fora desta rodada.

O contrato HTTP está em [`contracts/openapi/tenant-service.yaml`](../../contracts/openapi/tenant-service.yaml) e os eventos em [`contracts/asyncapi/people-platform.yaml`](../../contracts/asyncapi/people-platform.yaml).
