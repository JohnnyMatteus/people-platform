# 0006. Database per service e transactional outbox

- Status: aceito
- Data: 2026-11-21

## Contexto

Os serviços precisam evoluir os esquemas de forma independente, e nenhum serviço pode ler as tabelas de outro. Ao mesmo tempo, toda mudança de estado relevante precisa virar um evento. O problema clássico do *dual write* aparece aqui: gravar no banco e publicar no Kafka são duas operações. Se a segunda falha, o sistema fica inconsistente. Por exemplo, o tenant existe, mas o Keycloak nunca soube dele.

## Decisão

- **Um banco Postgres por serviço**, com usuário próprio. Um serviço não tem credencial para o banco de outro. Os dados de outro contexto chegam por evento ou API e, quando necessário, são mantidos como projeção local.
- **Transactional outbox:** o evento é gravado na tabela `outbox` **na mesma transação** da mudança de estado. Um relay lê as linhas pendentes com `FOR UPDATE SKIP LOCKED`, publica no Kafka (com `acks=all` e produtor idempotente) e marca `published_at`. Se a publicação falha, a linha continua pendente, com `attempts` e `last_error` registrados, e é retentada na próxima rodada.
- Vários relays podem rodar ao mesmo tempo, porque o `SKIP LOCKED` garante que cada linha é processada por um só. Isso é testado com relays concorrentes em `outbox_test.go`.
- A semântica é **at-least-once**: se o processo cai entre o publish e o commit, o evento sai de novo, e os consumidores deduplicam por `event_id`.
- O `traceparent` do request que gerou o evento é gravado na outbox e publicado como header, então o trace atravessa o barramento.
- **Ambiente local:** um único servidor Postgres com **um banco e um usuário por serviço** (ADR 0010). Em produção, cada serviço tem instância ou cluster próprio.

## Consequências

- Positivas: não há perda de evento por *dual write*, o esquema de cada serviço é autônomo e a falha do Kafka não derruba escritas, só atrasa a propagação.
- Negativas: consistência eventual entre contextos. A tabela outbox precisa de limpeza periódica (job de retenção, planejado). Há uma latência extra de até um intervalo de polling (1 s por padrão; lotes cheios disparam a rodada seguinte na hora). E as projeções locais duplicam dados.
- Métricas expostas: `outbox_events_published_total` e `outbox_publish_failures_total`. Um alerta sobre falhas contínuas está planejado no Grafana.

## Alternativas consideradas

- **CDC com Debezium lendo o WAL:** elimina o polling e é a evolução natural quando o volume crescer. Mas acrescenta Kafka Connect e Debezium ao ambiente. A tabela outbox foi desenhada para ser compatível com o *outbox event router* do Debezium, então a troca é só de infraestrutura.
- **Publicar direto no Kafka depois do commit:** é o *dual write*. Perde eventos em falha.
- **Banco compartilhado entre serviços:** acoplamento de esquema, e uma migração de um serviço quebra outro.
