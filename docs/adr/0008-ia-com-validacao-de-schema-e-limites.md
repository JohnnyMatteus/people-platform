# 0008. IA com saída validada por schema e limites de uso

- Status: aceito
- Data: 2026-11-21

## Contexto

A IA aparece em vários módulos: apoio à redação de feedback, plano de ação a partir do clima, sugestões de PDI a partir dos gaps e contexto automático para 1:1. A saída de um LLM é não determinística e pode vir malformada, inventar dados ou trazer conteúdo inadequado. Os dados de entrada são sensíveis (avaliações, feedbacks), e o custo por chamada é real.

## Decisão

- **Serviço único de IA:** os recursos de IA ficam no `analytics-ai-service` (Python/FastAPI). Os outros serviços pedem por API e nunca chamam o provedor de LLM direto.
- **Cliente compatível com a API da OpenAI** (`/v1/chat/completions`), com `base_url`, modelo e chave configuráveis. Isso permite trocar de provedor ou usar um modelo local sem mudar código.
- **Toda saída é validada por schema** (modelos Pydantic, exportados como JSON Schema no contrato). Saída inválida tem uma nova tentativa com a mensagem de erro de validação. Se falhar de novo, o recurso devolve erro explícito (`422 ai_output_invalid`), e nada é gravado.
- **A IA sugere e a pessoa decide.** Nenhuma saída é aplicada sem confirmação humana: o feedback é um rascunho, as ações de PDI são sugestões e o plano de ação do clima é uma proposta.
- **Minimização de dados no prompt:**
  - só o necessário para a tarefa vai no prompt, sem CPF, salário ou dados de saúde;
  - os identificadores são pseudonimizados no prompt e reidentificados na resposta;
  - o resumo do clima só usa dados agregados, com k atingido (ADR 0007).
- **Limites:** cota por tenant e por recurso (feature `ai_assist` do plano), timeout por chamada, teto de tokens por requisição e circuit breaker para o provedor.
- **Rastreabilidade:** cada geração registra tenant, recurso, modelo, versão do prompt, tokens e latência, sem o conteúdo, em métricas e traces.
- **Testes nunca chamam LLM real:** um cliente falso devolve respostas pré-definidas, inclusive inválidas, para testar o caminho de erro.

## Consequências

- Positivas: falhas da IA não corrompem dados, o custo é previsível por tenant, o provedor é intercambiável e os testes são determinísticos.
- Negativas: a validação e a nova tentativa somam latência. Schemas rígidos limitam a criatividade da saída. E a pseudonimização no prompt dá trabalho de implementação.

## Alternativas consideradas

- **Cada serviço com seu cliente LLM:** duplicaria regras de privacidade, cota e validação em cinco linguagens.
- **Texto livre sem schema:** é mais simples, mas impossível de integrar com segurança em fluxos estruturados, como as ações de PDI.
- **SDK proprietário de um provedor:** prende a plataforma ao provedor. O formato compatível com a API da OpenAI é o denominador comum do mercado.
