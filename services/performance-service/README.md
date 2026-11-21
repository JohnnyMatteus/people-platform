# performance-service (Kotlin)

É o contexto de **Desempenho** da People Platform: avaliações, calibração, nine box, sucessão, assessment, OKRs e PDI.

## Status desta rodada

Por enquanto existe só o **núcleo de domínio**, em Kotlin puro, sem Spring, JPA ou Kafka ([ADR 0011](../../docs/adr/0011-nucleo-de-dominio-antes-do-framework.md)):

| Pacote | Conteúdo |
| --- | --- |
| `domain.shared` | IDs (`TenantId`, `EmployeeId`, `CycleId`), `DomainException`, guarda de tenant |
| `domain.ninebox` | `NineBoxPolicy` (escala e limiares por tenant), `NineBoxClassifier`, `Quadrant`, `CalibrationSession`, `NineBoxHistory`, evento `NineBoxCalibrated` |
| `domain.succession` | `CriticalPosition`, `SuccessionPlan` (força do banco), `ReadinessAdvisor` (sugestão de prontidão) |

A **API HTTP (Spring Boot)** ainda não existe. O contrato que ela vai seguir já está em [`contracts/openapi/performance-service.yaml`](../../contracts/openapi/performance-service.yaml). Na próxima rodada, entram como adaptadores em volta deste núcleo:

- controllers;
- persistência com RLS;
- consumo de `TenantCriado` e `AvaliacaoConcluida`;
- publicação de `NineBoxCalibrado` via outbox.

O raciocínio das regras está no [roteiro de nine box e calibração](../../docs/roteiros/nine-box-e-calibracao.md).

## Comandos

Java e Gradle não precisam estar instalados: tudo roda na imagem `gradle:8.14.3-jdk21`, localmente e no CI.

```bash
make performance-service-lint    # ktlint
make performance-service-test    # JUnit 5 (parametrizados)
make performance-service-build   # compila e empacota
```

Sem o Makefile da raiz, o equivalente é:

```bash
docker run --rm -v "$PWD":/home/gradle/project -w /home/gradle/project gradle:8.14.3-jdk21 \
  gradle --no-daemon ktlintCheck test build
```
