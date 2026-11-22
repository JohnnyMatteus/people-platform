# analytics-ai-service (Python 3.12 + FastAPI)

People analytics agregado e recursos de IA assistida. Contrato em `contracts/openapi/analytics-ai-service.yaml`; decisões em `docs/adr/0008`.

## O que faz

- **Read models por eventos Kafka** (`CandidatoContratado`, `AvaliacaoConcluida`, `NineBoxCalibrado`, `PesquisaClimaEncerrada`), idempotentes por `event_id` e particionados por tenant.
- **People analytics** (`GET /v1/analytics/turnover`, `GET /v1/analytics/nine-box`): turnover, tempo de casa, admissões e desligamentos por área, unidade, faixa de tempo de casa, faixa etária e gênero. Só agregados; grupos abaixo de `MIN_GROUP_SIZE` (k, padrão 5) são suprimidos e, havendo supressão, o total é omitido para não revelar o grupo por subtração.
- **IA** (sempre sugestão, `requires_confirmation: true`): rascunho de feedback, plano de ação do clima, sugestões de PDI e contexto de 1:1.
  - Cliente compatível com OpenAI (`LLM_BASE_URL`, `LLM_MODEL`, `LLM_API_KEY`).
  - Saída validada por Pydantic, com uma nova tentativa que inclui o erro de validação; falhando de novo, `422 ai_output_invalid`.
  - Orçamento: cota diária de chamadas por tenant e recurso, teto diário de tokens por tenant e `max_tokens` por chamada (`429 ai_budget_exceeded`).
  - Todo conteúdo externo vai delimitado em `<external_content>` (com neutralização de fechamento de tag) e o system prompt o trata como dado. Nomes são pseudonimizados (`PESSOA_n`) e restaurados na resposta; o PDI não leva identidade alguma.

## Arquitetura

```
domain/      vínculo, bandas, cálculo de indicadores e supressão (puro)
app/         ingestão de eventos, casos de uso, portas; app/ai: guard, runner, budget, schemas
adapters/    memory (read models), http (FastAPI + JWT), kafka_consumer, openai_llm
```

## Comandos

```bash
make venv && make lint && make test
```

Na raiz: `make analytics-ai-service-up` (porta 8084 no host, `ANALYTICS_AI_SERVICE_PORT`).

Os testes usam fixtures validadas contra `contracts/asyncapi/schemas` e um LLM falso; não acessam a rede.

## Variáveis de ambiente

`OIDC_ISSUER`, `OIDC_JWKS_URL`, `OIDC_AUDIENCE`, `LLM_API_KEY` (obrigatórias); `KAFKA_BROKERS` (vazio desliga o consumidor), `KAFKA_TOPICS`, `MIN_GROUP_SIZE`, `LLM_BASE_URL`, `LLM_MODEL`, `LLM_TIMEOUT_S`, `LLM_MAX_TOKENS`, `AI_DAILY_CALLS_PER_RESOURCE`, `AI_DAILY_TOKENS`, `HTTP_PORT`.

## Limitações conhecidas

- Read models em Postgres com RLS (`adapters/postgres.py`, migração em `adapters/migrations`, job `analytics-ai-service-migrate` no compose). O consumo usa unidade de trabalho: o registro do evento e o efeito saem na mesma transação. A cota de IA (`InMemoryAiBudget`) segue em memória, então reinicia com o processo e não é compartilhada entre réplicas. `TitularExclusaoSolicitada` apaga vínculo, avaliação e nine box do titular; o vínculo provisório criado por `CandidatoContratado` usa `application_id` como `employee_id`.
- Não há evento de desligamento nem de demografia/área no contrato: hoje o vínculo nasce de `CandidatoContratado` (unidade) e o desligamento entra por `AnalyticsService.record_termination`, à espera do evento.
- Os endpoints `GET /v1/analytics/nine-box` e `POST /v1/ai/one-on-one-briefs` constam no OpenAPI e no gateway, mas as respostas de IA têm só o essencial descrito.
- Circuit breaker do provedor e a feature `ai_assist` do plano (ADR 0008) ainda não foram ligados.
