# 0005. Kafka como barramento, contratos em AsyncAPI e schemas versionados

- Status: aceito
- Data: 2026-11-21

## Contexto

Os contextos precisam reagir uns aos outros sem chamadas síncronas em cadeia. Por exemplo, `AvaliacaoConcluida` alimenta o nine box e o PDI, `CandidatoContratado` dispara onboarding e analytics, e `TenantCriado` provisiona a Organization no Keycloak e a configuração inicial nos serviços. O analytics precisa de reprocessamento (replay) para recalcular indicadores.

## Decisão

- **Kafka** (modo KRaft, sem ZooKeeper) é o barramento de eventos de domínio.
- **Um tópico por contexto produtor, versionado no nome:** `people.<contexto>.events.v<N>` (ex.: `people.tenant.events.v1`, `people.performance.events.v1`). O tipo do evento vai no envelope e no header `event_type`.
- **Chave da mensagem = `tenant_id`.** Isso garante a ordem por tenant dentro da partição e distribui a carga entre tenants.
- **Envelope comum** a todos os eventos: `event_id` (UUID, chave de deduplicação), `event_type`, `event_version`, `tenant_id`, `aggregate_id`, `occurred_at`, `producer` e `data`. O `traceparent` vai como header para manter o trace distribuído.
- **Contratos em AsyncAPI 3.0** (`contracts/asyncapi/people-platform.yaml`), com os payloads em JSON Schema, validados no CI.
- **Regras de evolução:** adicionar campo opcional é compatível. Remover campo, renomear ou mudar tipo é incompatível e exige `event_version` novo, publicado em paralelo num tópico `vN+1` até os consumidores migrarem.
- **Entrega at-least-once:** consumidores são idempotentes e deduplicam por `event_id` numa tabela `processed_events` na mesma transação do efeito.
- Os nomes de evento ficam em português (linguagem ubíqua do domínio, ex.: `TenantCriado`). Os campos dos payloads ficam em inglês, como os identificadores de código.

## Consequências

- Positivas: desacoplamento temporal entre os contextos. O replay permite reconstruir projeções e o analytics. A ordem por tenant é garantida, e o contrato fica explícito e verificável no CI.
- Negativas: a consistência é eventual (ADR 0006). Kafka é mais pesado de operar do que uma fila simples. Mensagens podem chegar duplicadas, e cada consumidor precisa lidar com isso. E nomes em português no `event_type` exigem cuidado com acentuação, por isso usamos só ASCII nos nomes.
- Próximo passo registrado: adotar um Schema Registry (Apicurio ou Confluent) quando houver mais de um consumidor por evento em produção. Por enquanto, os JSON Schemas versionados no repositório são a fonte da verdade.

## Alternativas consideradas

- **RabbitMQ:** ótimo para filas de trabalho e roteamento, mas sem retenção longa nem replay nativo, que é essencial para o analytics.
- **Chamadas HTTP síncronas entre serviços:** acoplamento temporal e falhas em cascata.
- **Avro ou Protobuf desde o início:** são mais compactos e têm evolução validada pelo registry, mas trazem tooling extra em cinco linguagens. JSON Schema é legível e suficiente para o volume inicial. A migração fica registrada como evolução.
