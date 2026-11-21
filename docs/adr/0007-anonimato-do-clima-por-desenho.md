# 0007. Anonimato das pesquisas de clima por desenho

- Status: aceito
- Data: 2026-11-21

## Contexto

As pesquisas de clima só funcionam se as pessoas confiarem que são anônimas. Uma "anonimidade" que dependa de boa vontade (o gestor promete não olhar) ou de um filtro na tela não se sustenta. Quem tem acesso ao banco descobre o autor, e um corte demográfico fino (área + faixa etária + gênero) pode identificar uma pessoa mesmo sem o nome. Isso também é questão de LGPD: dado de opinião ligado a uma pessoa identificável é dado pessoal.

## Decisão

O anonimato é garantido pela arquitetura, em cinco camadas:

1. **Separação de identidade e resposta.** O engagement-service guarda dois registros independentes:
   - `survey_participation (tenant_id, survey_id, employee_id)`, que só serve para impedir voto duplo e medir adesão;
   - `survey_response (tenant_id, survey_id, segment_key, answers)`, **sem** `employee_id`, sem timestamp preciso (só a data) e com ordem de inserção embaralhada.

   Não existe chave que ligue um registro ao outro.
2. **Segmento grosso definido antes da coleta.** A resposta guarda só o segmento (ex.: área + unidade), calculado no momento do envio a partir de atributos permitidos pelo tenant. Atributos demográficos finos não são gravados na resposta.
3. **Tamanho mínimo de grupo (k-anonimato).** Nenhum resultado é exibido para um segmento com menos de **k respondentes** (padrão k = 5, configurável pelo tenant só para cima, nunca abaixo de 3). Segmentos abaixo de k são agregados no nível superior ou suprimidos.
4. **Proteção contra diferenciação.** Cruzamentos ad hoc não são permitidos. O filtro "área X" menos o filtro "área X sem a unidade Y" poderia isolar uma pessoa, então só os cortes pré-definidos são consultáveis, e todo corte exibido respeita k. Se um corte vizinho tiver diferença menor que k, ele é suprimido (supressão complementar).
5. **Texto livre tratado.** Comentários abertos passam por remoção de nomes e e-mails antes de serem gravados, só aparecem com k atingido e, quando resumidos por IA, o resumo é feito sobre o conjunto do segmento, nunca sobre um comentário isolado.

O plano de ação gerado a partir do clima usa só os resultados agregados.

## Consequências

- Positivas: o anonimato vale até contra quem tem acesso ao banco. A confiança aumenta a adesão e o requisito de LGPD fica atendido por minimização.
- Negativas: times pequenos (menos de k pessoas) não veem resultado próprio, só o da área acima. É impossível "descobrir quem disse", inclusive em casos graves como uma denúncia, e por isso denúncias têm canal próprio, fora da pesquisa. Também não dá para corrigir uma resposta depois de enviada.
- Testes obrigatórios na implementação (próxima rodada): segmento com k−1 respostas não aparece; a diferença entre cortes não isola ninguém; não existe coluna nem índice que ligue participação e resposta.

## Alternativas consideradas

- **Pseudonimização (hash do employee_id):** é reversível por força bruta, porque o universo de colaboradores é pequeno e conhecido.
- **Privacidade diferencial com ruído:** protege bem contra ataques estatísticos, mas distorce resultados de grupos pequenos e é difícil de explicar para o RH. Fica como evolução para o people analytics, não para o clima.
- **Anonimato só na interface:** não resiste a quem acessa o banco.
