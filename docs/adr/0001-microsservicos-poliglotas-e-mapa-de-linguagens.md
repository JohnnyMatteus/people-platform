# 0001. Microsserviços poliglotas e mapa de linguagens

- Status: aceito
- Data: 2026-11-21

## Contexto

A People Platform reúne módulos com perfis muito diferentes. O ATS é um CRUD rico com fluxos de etapas e muitos formulários. Desempenho tem regras densas: ciclos, calibração, nine box e sucessão. Engajamento é orientado a eventos, com muitos usuários simultâneos (feed, elogios, confirmação de leitura). Analytics e IA dependem do ecossistema de dados e de LLMs. O registro de tenants fica no caminho crítico de toda requisição e precisa de latência baixa e binário pequeno.

O projeto também é um portfólio, e a decisão precisa ser defensável como engenharia, não só como vitrine: cada linguagem tem que ganhar o lugar pelo encaixe com o problema.

## Decisão

Separar a plataforma em serviços por bounded context, cada um na linguagem que melhor serve ao problema:

| Serviço | Linguagem / framework | Por que esta escolha |
| --- | --- | --- |
| `tenant-service` | Go | No caminho de toda requisição (resolução de tenant, tema, feature flags). Precisa de latência previsível, binário estático pequeno, concorrência simples e pouca dependência. |
| `performance-service` | Kotlin + Spring Boot | Domínio com mais regra de negócio: tipos ricos, `sealed`, `value class` e null safety deixam as invariantes explícitas. O ecossistema Spring cobre JPA, segurança e Kafka. |
| `recruitment-service` | PHP + Laravel | O ATS é CRUD com fluxo, formulários, uploads e e-mails, e o Laravel tem produtividade alta nesse tipo de sistema (Eloquent, filas, validação, storage S3). |
| `engagement-service` | TypeScript + NestJS | Feed, elogios e notificações em tempo real (WebSocket), I/O intenso e modelo de módulos do Nest. Compartilha tipos com o front-end em TypeScript. |
| `analytics-ai-service` | Python + FastAPI | Ecossistema de dados (pandas/polars, SQL analítico) e de IA (clientes LLM, validação com Pydantic). |
| `learning-service` | a definir (rodada futura) | Só documentado nesta rodada. |
| `web` | Vue 3 | White label por design tokens carregados do tenant-service. |

As regras comuns valem para todos e são o que torna a poliglotia sustentável:

- contratos primeiro: OpenAPI 3.1 para HTTP e AsyncAPI para eventos, validados no CI;
- envelope de evento único, com `tenant_id` obrigatório;
- observabilidade padronizada: OpenTelemetry (OTLP), `/metrics` Prometheus, logs JSON em stdout com `trace_id`;
- `tenant_id` em todas as tabelas e eventos (ADR 0002), um banco por serviço (ADR 0006);
- cada serviço tem Makefile com `lint`, `test` e `build`, e um job próprio no CI acionado por filtro de caminhos.

## Consequências

- Positivas: cada contexto evolui e escala de forma independente. As falhas ficam isoladas. O time pode trabalhar em paralelo por diretório, e cada linguagem é usada onde rende mais.
- Negativas: o custo operacional cresce, com cinco toolchains, cinco imagens base e cinco conjuntos de dependências para atualizar. A biblioteca comum (auth, telemetria, envelope de evento) precisa ser reimplementada em cada linguagem. E fica mais difícil uma pessoa circular entre todos os serviços.
- Mitigações: contratos e convenções valem mais do que código compartilhado. O CI por caminho mantém o feedback rápido. Os serviços ficam pequenos, e o roteiro de cada um mostra onde estão as regras.

## Alternativas consideradas

- **Monólito modular numa só linguagem.** É mais simples de operar e seria a escolha certa para um time pequeno no início de um produto real. Foi descartado porque o objetivo explícito do projeto inclui demonstrar integração entre contextos por eventos e operação poliglota. O design doc registra que, num produto real, começaríamos com o monólito modular e extrairíamos serviços conforme a dor.
- **Microsserviços numa linguagem só (ex.: tudo em Go ou tudo em Kotlin).** Reduziria o custo de manutenção, mas perderia o encaixe em ATS (Laravel) e IA/dados (Python).
- **Um serviço por módulo de produto (OKR, PDI, nine box separados).** Fragmentaria demais: OKR, PDI e nine box compartilham ciclo, colaborador e gestor, ou seja, o mesmo bounded context de Desempenho.
