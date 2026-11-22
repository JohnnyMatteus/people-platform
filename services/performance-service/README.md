# performance-service (Kotlin)

É o contexto de **Desempenho** da People Platform: avaliações, calibração, nine box, sucessão, assessment, OKRs e PDI.

## Status

Núcleo de domínio em Kotlin puro ([ADR 0011](../../docs/adr/0011-nucleo-de-dominio-antes-do-framework.md)) com adaptadores Spring Boot em volta:

| Pacote | Conteúdo |
| --- | --- |
| `domain.*` | Nine box (política, classificador, calibração, histórico), sucessão, avaliação e PDI, sem Spring, JPA ou Kafka |
| `app` | Casos de uso: ciclos, avaliações, calibração, sucessão e PDI |
| `adapters.web` | Controllers conforme [`contracts/openapi/performance-service.yaml`](../../contracts/openapi/performance-service.yaml), JWT (`tenant_id`, `sub`, papéis) e problem+json |
| `adapters.persistence` | JPA, migrações Flyway e RLS por `app.tenant_id` |
| `adapters.messaging` | Outbox e relay para `people.performance.events.v1` (`NineBoxCalibrado`, `AvaliacaoConcluida`) |

OKRs (objetivos desdobrados, resultados-chave e check-ins) estão em `app/Okr.kt` e `domain/okr`, com o modelo de acesso no [ADR 0014](../../docs/adr/0014-okrs-modelo-de-acesso-e-limites.md). Ainda não existem: consumo de eventos (`TenantCriado`, `TitularExclusaoSolicitada`) e assessment.

O raciocínio das regras está no [roteiro de nine box e calibração](../../docs/roteiros/nine-box-e-calibracao.md).

## Comandos

Java e Gradle não precisam estar instalados: tudo roda na imagem `gradle:8.14.3-jdk21`, localmente e no CI.

```bash
make performance-service-lint    # ktlint
make performance-service-test    # JUnit 5 (domínio e contrato; sem Docker)
make performance-service-build   # compila e empacota
make performance-service-up      # sobe no compose (porta 8085, atrás do gateway em /performance)
```

Sem o Makefile da raiz, o equivalente é:

```bash
docker run --rm -v "$PWD":/home/gradle/project -w /home/gradle/project gradle:8.14.3-jdk21 \
  gradle --no-daemon ktlintCheck test build
```

Os testes de integração (Postgres real via Testcontainers: RLS, vazamento entre tenants, outbox, contrato) ficam na tarefa `integrationTest` e precisam de Docker; o CI roda as duas.
