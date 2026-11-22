# 0012. learning-service em Go e vínculo manual da contratação

- Status: proposto (a linguagem estava "a definir" no ADR 0001; o autor confirma ou troca)
- Data: 2026-11-22

## Contexto

O ADR 0001 deixou a linguagem do `learning-service` em aberto. O serviço é pequeno e de regra simples: trilhas ordenadas, matrículas, progresso, sugestão do próximo conteúdo e a integração de quem é contratado. Não tem processamento pesado, nem LLM, nem formulário rico, nem tempo real, que são os motivos que justificam as outras linguagens.

Há também uma lacuna de contrato: `CandidatoContratado` traz `application_id`, cargo, unidade e data de início, mas não o colaborador, porque o diretório de pessoas (`people-directory`) ainda não existe. Sem identidade da pessoa, a matrícula na trilha de integração não tem a quem apontar.

## Decisão

- **Go**, com a mesma arquitetura hexagonal do tenant-service (`internal/domain`, `internal/app`, `internal/adapters/*`, `internal/platform/*`), RLS por `tenant_id` e testes de vazamento entre tenants. Motivos: a toolchain já existe no repositório (nenhuma sexta toolchain), o consumidor Kafka com commit manual e o servidor HTTP são curtos, e o binário estático pequeno combina com um serviço de CRUD com poucas dependências.
- **Consumo de `CandidatoContratado`** com offset confirmado só depois do efeito gravado, deduplicação por `event_id` na mesma transação do tenant do próprio evento e descarte (com log) de mensagem inválida, que não adianta repetir. Falha transitória repete a mesma mensagem, sem pular, para manter a ordem por tenant.
- **Vínculo manual da contratação:** o evento cria uma atribuição de onboarding *pendente*. Alguém com papel `tenant-admin` informa quem é a pessoa contratada (`POST /v1/onboarding/{applicationId}/bind`), e só então ela é matriculada na trilha de integração escolhida pelo tenant. Isso evita inventar um vínculo candidatura → colaborador que o contrato não oferece e não muda o schema de `CandidatoContratado`.
- O serviço **não publica eventos** nesta rodada: ninguém os consome ainda.

## Consequências

- Positivas: sem nova toolchain, o serviço nasce testável (domínio, casos de uso com store em memória que reproduz o escopo por tenant, HTTP, consumidor e Postgres real no CI) e segue as convenções de observabilidade e de erro dos demais.
- Negativas: o RH precisa de um passo manual por contratação até existir o diretório de pessoas. Go não ajuda no que seria o diferencial da recomendação (aprendizado de máquina), que fica fora de escopo; a sugestão de hoje é por regra (integração, atribuição da empresa, escolha da pessoa, e competência do PDI).
- A troca de linguagem custa reescrever o serviço, mas os contratos (OpenAPI, eventos) e o banco independem dela.

## Alternativas consideradas

- **TypeScript/NestJS:** reaproveitaria o front e o engagement-service, mas não há tempo real aqui, e dois serviços de CRUD no mesmo stack não diferenciam o portfólio.
- **Python/FastAPI:** o ecossistema de recomendação seria vantagem se houvesse modelo; com regra simples, é só mais um runtime para operar.
- **Elixir ou Rust:** forçariam uma sexta toolchain sem ganho que o domínio peça.
- **Vincular a contratação ao colaborador por e-mail ou nome:** exigiria dado pessoal no evento (`CandidatoContratado` foi desenhado sem dados de contato) ou heurística frágil.
