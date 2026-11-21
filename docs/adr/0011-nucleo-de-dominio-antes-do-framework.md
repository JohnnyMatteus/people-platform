# 0011. performance-service: núcleo de domínio em Kotlin puro antes do Spring Boot

- Status: aceito
- Data: 2026-11-21

## Contexto

O performance-service concentra as regras mais delicadas da plataforma: a classificação na nine box com limiares por tenant, a calibração com limites e trilha de auditoria, o histórico entre ciclos e o mapa de sucessão com prontidão. Erros nessas regras afetam carreira e remuneração das pessoas. A rodada de fundação tem tempo limitado e precisa de um núcleo pequeno e bem testado, não de um serviço inteiro e raso.

## Decisão

- Implementar nesta rodada **só o núcleo de domínio**, em Kotlin puro (pacote `com.peopleplatform.performance.domain`), sem Spring, JPA ou Kafka:
  - `ninebox`: política por tenant (escala, limiares, passos máximos de calibração, diretriz de topo), classificador, sessão de calibração, histórico e tendência;
  - `succession`: posição crítica, mapa de sucessão, força do banco e sugestão de prontidão.
- Os valores das notas usam `BigDecimal` para que limiares como 3.75 não sofram erro de ponto flutuante.
- Os testes usam JUnit 5 com testes parametrizados e rodam na imagem Docker oficial `gradle:8.14.3-jdk21`, tanto localmente quanto no CI, porque Java e Gradle não estão instalados na máquina de desenvolvimento.
- A camada Spring Boot entra na próxima rodada como **adaptadores em volta do núcleo**, sem que o domínio dependa dela:
  - API HTTP conforme `contracts/openapi/performance-service.yaml`, que já está especificado;
  - persistência com RLS;
  - consumo de `TenantCriado` e publicação de `NineBoxCalibrado` via outbox.

## Consequências

- Positivas: as regras são testáveis em milissegundos e sem infraestrutura. O domínio fica independente de framework, e o contrato HTTP já existe para guiar a próxima rodada.
- Negativas: o serviço ainda não é executável como API. O README e o status por módulo deixam isso claro para não haver expectativa errada.

## Alternativas consideradas

- **Spring Boot completo nesta rodada:** espalharia o tempo entre configuração e infraestrutura, com menos profundidade nas regras.
- **Kotest como framework de teste:** é expressivo, mas o JUnit 5 com parametrizados cobre o necessário com menos dependências. A troca fica aberta se os testes de propriedade ganharem peso.
