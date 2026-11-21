# 0002. Multitenancy: banco compartilhado por serviço com `tenant_id` e Row-Level Security

- Status: aceito
- Data: 2026-11-21

## Contexto

Cada cliente da plataforma (tenant) é uma empresa com dados de RH sensíveis: avaliações, salários no futuro e respostas de clima. Um vazamento entre tenants é o pior incidente possível para o produto. Ao mesmo tempo, a plataforma precisa atender muitos tenants pequenos (redes de varejo, construtoras) com custo baixo por cliente.

Os modelos clássicos são três: banco por tenant (silo), schema por tenant (bridge) e tabelas compartilhadas com `tenant_id` (pool).

## Decisão

Usar o modelo **pool** em cada serviço, com duas camadas de proteção:

1. **Aplicação:** toda tabela de dados de tenant tem `tenant_id NOT NULL`. Todo caso de uso recebe o tenant do token (claim `tenant_id`), nunca do corpo da requisição, e toda consulta filtra por ele. Todo evento carrega `tenant_id` no envelope.
2. **Banco (Postgres):** Row-Level Security habilitada com `FORCE ROW LEVEL SECURITY` em todas as tabelas com `tenant_id`. A aplicação conecta com um usuário membro de um papel sem `BYPASSRLS` e que não é dono das tabelas. O escopo é definido por transação com `set_config('app.tenant_id', ..., true)` (equivalente a `SET LOCAL`). Assim ele morre com a transação e nunca vaza para a próxima requisição que reutiliza a conexão do pool.
3. **Falha fechada:** sem escopo definido, nenhuma linha é visível. Operações de plataforma (listar tenants, resolver domínio, relay da outbox) usam um escopo explícito `app.platform = on`, restrito ao código de plataforma.

Cada serviço implementa testes de vazamento entre tenants contra um Postgres real (testcontainers). O mínimo é este:

- consulta **sem WHERE** no escopo do tenant A só retorna linhas de A;
- `INSERT`/`UPDATE` em linhas de B a partir do escopo de A é bloqueado (`WITH CHECK`);
- sem escopo, zero linhas;
- o escopo não sobrevive ao fim da transação na mesma conexão.

No tenant-service esses testes estão em `services/tenant-service/internal/adapters/postgres/repository_test.go`.

| Serviço | Isolamento |
| --- | --- |
| tenant-service | RLS em `tenants`, temas, overrides, domínios e outbox (implementado) |
| performance-service | `tenant_id` + RLS (próxima rodada); o domínio já recusa operações entre tenants |
| recruitment-service | `tenant_id` + RLS; currículos no S3 com prefixo `tenants/{id}/` |
| engagement-service | `tenant_id` + RLS; respostas de clima num esquema separado e sem identidade (ADR 0007) |
| analytics-ai-service | só agregados por tenant; consultas sempre parametrizadas por `tenant_id` |

## Consequências

- Positivas: custo baixo por tenant, uma migração por serviço (e não uma por tenant) e onboarding instantâneo. O RLS protege mesmo quando alguém esquece um `WHERE`.
- Negativas: existe o risco de "vizinho barulhento", em que um tenant grande degrada os outros. Backup e restore por tenant exigem exportação lógica filtrada. O RLS custa um pouco de desempenho (predicado extra em toda consulta, mitigado por índices que começam por `tenant_id`). E o modelo depende de disciplina: o escopo de plataforma é um poder grande e fica restrito a poucas funções revisadas.
- Caminho de evolução: um tenant corporativo que exija isolamento físico pode ganhar um banco dedicado (silo) sem mudar o código, porque o roteamento por tenant é só a escolha do DSN. Isso está registrado como plano no design doc.

## Alternativas consideradas

- **Banco por tenant (silo):** isolamento máximo, mas o custo e as migrações crescem linearmente com o número de tenants. Fica reservado como opção comercial para clientes grandes.
- **Schema por tenant:** isolamento razoável, mas o catálogo do Postgres incha com milhares de schemas, as migrações viram um laço por tenant e o pool de conexões precisa trocar `search_path`.
- **Só `tenant_id` na aplicação, sem RLS:** é mais simples, mas um único `WHERE` esquecido vira vazamento. O RLS custa pouco e protege contra exatamente esse bug.
