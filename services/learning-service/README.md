# learning-service

Aprendizagem (LXM): trilhas, matrículas, progresso, sugestão do próximo conteúdo e integração de quem é contratado. Go, arquitetura hexagonal como o tenant-service. A linguagem estava "a definir" e a escolha de Go é proposta no [ADR 0012](../../docs/adr/0012-learning-service-em-go.md): o autor confirma ou troca.

## O que faz

- **Trilhas** (`tenant-admin` escreve, todos leem): sequência ordenada de conteúdos (vídeo, leitura, curso, prática), com competência opcional.
- **Matrículas:** a própria pessoa se matricula, ou o `tenant-admin` matricula outra. Concluir um conteúdo é idempotente e só vale para a matrícula da própria pessoa (a de outra responde 404).
- **Sugestões:** o próximo conteúdo de cada matrícula em andamento (integração, depois atribuição da empresa, depois escolha da pessoa) e, com `competency`, conteúdos de trilhas em que a pessoa ainda não está e que desenvolvem aquela competência. É regra, não modelo.
- **Integração de contratados:** `CandidatoContratado` cria uma atribuição *pendente* (o evento traz a candidatura, não o colaborador). O `tenant-admin` informa quem é a pessoa (`POST /v1/onboarding/{applicationId}/bind`) e ela é matriculada na trilha de integração do tenant.
- **LGPD:** `TitularExclusaoSolicitada` apaga matrículas, progresso e atribuições vinculadas ao titular no tenant do evento.

Contrato HTTP: [`contracts/openapi/learning-service.yaml`](../../contracts/openapi/learning-service.yaml), por `http://localhost:8000/learning` no gateway.

## Arquitetura

```
cmd/learning-service      composição: config, pool, consumidor Kafka, HTTP
internal/domain           trilha, matrícula, sugestão, onboarding (sem dependências externas)
internal/app              casos de uso, portas (Store/Tx e repositórios) e tratamento de eventos
internal/adapters/...     postgres (RLS), httpapi, kafka (consumidor), memory (testes)
internal/platform/...     auth (JWT/JWKS), config, telemetria
```

Isolamento: toda tabela tem `tenant_id` e RLS (`ENABLE` e `FORCE`, `WITH CHECK`); o escopo vem da claim `tenant_id` e entra por `set_config(..., true)` dentro da transação. O consumidor confirma o offset só depois do efeito gravado, deduplica por `event_id` na transação do tenant do próprio evento, descarta mensagem inválida com log e repete a transitória sem pular.

## Variáveis de ambiente

| Variável | Obrigatória | Descrição |
| --- | --- | --- |
| `DATABASE_URL` | sim | DSN do usuário da aplicação (membro de `learning_app`, sujeito a RLS) |
| `DATABASE_MIGRATION_URL` | só em `migrate` | DSN do dono do esquema |
| `KAFKA_BROKERS` | sim | lista separada por vírgula |
| `OIDC_ISSUER`, `OIDC_JWKS_URL` | sim | realm do Keycloak |
| `OIDC_AUDIENCE` | não (`people-api`) | audiência exigida no token |
| `RECRUITMENT_EVENTS_TOPIC`, `PRIVACY_EVENTS_TOPIC`, `KAFKA_CONSUMER_GROUP` | não | `people.recruitment.events.v1`, `people.privacy.events.v1`, `learning-service` |

## Rodar e testar

```bash
make learning-service-up         # núcleo + migração + serviço (porta 8086)
make learning-service-test-unit  # sem Docker (-short)
make learning-service-test       # inclui Postgres real (RLS e vazamento entre tenants) via testcontainers
```

## Limites

- O vínculo candidatura → colaborador é manual até existir o diretório de pessoas.
- O serviço não publica eventos: ninguém os consome ainda.
- A sugestão é por regra; recomendação com modelo ficaria em outro serviço ou numa rodada própria.
