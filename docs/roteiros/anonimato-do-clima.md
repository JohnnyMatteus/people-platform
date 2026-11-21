# Roteiro: anonimato das pesquisas de clima

As pesquisas de clima só produzem dado útil se as pessoas confiarem que podem ser francas. Este roteiro explica como a plataforma garante o anonimato **por arquitetura**, e não por promessa. A implementação acontece no engagement-service, na rodada 2. A decisão, o modelo e os testes obrigatórios já estão fixados aqui e no [ADR 0007](../adr/0007-anonimato-do-clima-por-desenho.md), e o contrato do evento que sai do contexto (`PesquisaClimaEncerrada`) já está publicado em `contracts/asyncapi/schemas/pesquisa-clima-encerrada.v1.json`.

## 1. O problema

"Anônimo" costuma significar "não mostramos o nome na tela". Isso não resiste a três ataques comuns:

1. **Acesso ao banco:** quem tem acesso à tabela de respostas e à tabela de colaboradores liga as duas por `employee_id` ou por timestamp.
2. **Corte fino:** "Área de Obras, unidade Norte, mulheres, 50+ anos" pode ter uma pessoa só. O resultado do corte é a resposta dela.
3. **Diferenciação:** o resultado de "Obras" menos o resultado de "Obras sem a unidade Norte" isola a unidade Norte. Se ela tem duas pessoas, e uma delas é quem consulta, a outra fica exposta.

## 2. A solução em cinco camadas

### 2.1 Separação física de identidade e resposta

```sql
-- Quem participou: só para impedir voto duplo e medir adesão.
survey_participation (tenant_id, survey_id, employee_id, participated_on date)

-- O que foi respondido: sem employee_id, sem horário, sem chave para a participação.
survey_response (tenant_id, survey_id, response_id uuid /* aleatório */, segment_key, answers jsonb, submitted_on date)
```

As duas gravações acontecem na mesma transação (para a adesão bater), mas nada liga uma à outra:

- não há chave estrangeira entre as tabelas;
- `response_id` é aleatório (UUID v4, não v7, que é ordenável por tempo e permitiria correlação);
- a data é só o dia, sem hora;
- a ordem física das inserções não revela a ordem das participações quando o lote é gravado embaralhado. Respostas entram num buffer e são gravadas em lotes de tamanho mínimo.

### 2.2 Segmento grosso, decidido antes da coleta

A resposta guarda só `segment_key` (ex.: `area=obras|unidade=norte`), calculado no envio a partir dos atributos que o tenant habilitou para aquela pesquisa. Atributos demográficos finos nunca vão para a resposta. Se não estão gravados, não há corte fino a fazer.

### 2.3 Tamanho mínimo de grupo (k-anonimato)

Nenhum resultado aparece para um segmento com menos de **k** respondentes. O padrão é 5. O tenant pode **aumentar** esse valor, mas nunca abaixo de 3. Segmentos abaixo de k são agregados no nível superior (unidade → área → empresa) ou suprimidos.

### 2.4 Proteção contra diferenciação

- Só os **cortes pré-definidos** da pesquisa podem ser consultados, sem filtro livre.
- **Supressão complementar:** se dois cortes publicados diferem em menos de k pessoas, o menor deles é suprimido. Assim, a subtração não isola um grupo pequeno.
- O evento `PesquisaClimaEncerrada` só carrega segmentos que passaram nas duas regras (`respondents ≥ min_group_size ≥ 3` no schema).

### 2.5 Texto livre

- Nomes, e-mails e telefones são removidos **antes** de gravar.
- O texto só aparece com k atingido no segmento.
- O resumo por IA (plano de ação) é feito sobre o conjunto do segmento, nunca sobre um comentário isolado, e o resultado é validado por schema ([ADR 0008](../adr/0008-ia-com-validacao-de-schema-e-limites.md)).

## 3. Como verificar (testes obrigatórios na implementação)

| Teste | O que prova |
| --- | --- |
| Segmento com k−1 respostas não aparece no resultado nem no evento | k-anonimato |
| Dois cortes que diferem em menos de k pessoas: um é suprimido | Proteção contra diferenciação |
| Introspecção do esquema: `survey_response` não tem `employee_id` nem FK para `survey_participation` | Separação física |
| `response_id` não é ordenável por tempo; `submitted_on` não tem hora | Sem correlação temporal |
| Evento `PesquisaClimaEncerrada` validado contra o JSON Schema (segmentos com `respondents ≥ 3`) | Contrato não vaza grupos pequenos |
| Comentário com nome e e-mail é gravado sem eles | Remoção de identificadores |
| Votar duas vezes falha, e a segunda resposta não é gravada | Participação controla duplicidade sem identificar a resposta |

## 4. Alternativas e trade-offs

| Alternativa | Problema |
| --- | --- |
| Anonimato só na interface | Quem acessa o banco identifica todo mundo |
| Pseudonimização (`hash(employee_id)`) | Reversível por força bruta: o universo de IDs é pequeno e conhecido |
| Criptografar a resposta com chave do RH | Quem tem a chave vê tudo; o problema muda de lugar mas não some |
| Privacidade diferencial (ruído) | Protege bem contra ataques estatísticos, mas distorce grupos pequenos e é difícil de explicar ao RH; fica como evolução para o people analytics |
| k fixo e baixo (ex.: 3) | Times de 3 pessoas ficam frágeis a inferência por exclusão; 5 é o padrão mais comum em pesquisas organizacionais |

Os trade-offs aceitos:

- **Times pequenos não veem resultado próprio,** só o da área acima. É o preço do anonimato, e a interface explica o motivo.
- **Não dá para "descobrir quem disse",** nem em casos graves. Por isso denúncia tem canal próprio, fora da pesquisa de clima.
- **Não é possível corrigir uma resposta** depois de enviada. Não existe ligação para encontrá-la.

## 5. Perguntas que um entrevistador faria

**Se as duas tabelas são gravadas na mesma transação, o horário do commit não permite ligar uma à outra?**
Por isso a resposta não guarda horário (só a data) e não usa ID ordenável por tempo. Além disso, as respostas vão para um buffer e são gravadas em lote embaralhado quando o lote atinge um tamanho mínimo. Quem lê o WAL ou o log de auditoria do banco ainda vê a ordem das transações, e é por isso que o acesso a esse nível de log é restrito e auditado. Esse risco residual está documentado.

**Por que k = 5 e não outro número?**
É um equilíbrio entre proteção e utilidade. Com k = 3, numa equipe de 3 em que duas pessoas conversam entre si, a terceira resposta fica exposta. Com k = 10, metade dos times não veria nada. O tenant pode subir o valor, e o piso de 3 é fixo no código e no schema do evento.

**Como você evita que o gestor descubra quem respondeu pela taxa de adesão?**
A adesão também é um agregado e segue a mesma regra de k. Em segmentos pequenos, só aparece "menos de k" e não o número exato. A lista de quem participou nunca é exposta ao gestor, só ao próprio participante (para ele saber que já respondeu).

**E se o tenant pedir para ver respostas individuais, alegando que o dado é dele?**
O tenant é o controlador do dado, mas a plataforma vendeu um recurso de pesquisa anônima, e os participantes responderam sob essa condição. Entregar a resposta individual violaria a finalidade informada (LGPD, princípio da finalidade). A arquitetura simplesmente não tem como entregar, o que encerra a discussão.

**Como a IA entra sem quebrar o anonimato?**
Ela só recebe agregados de segmentos que passaram em k, mais comentários já sem identificadores, e produz um plano de ação estruturado e validado por schema. O prompt nunca contém um comentário isolado associado a um segmento pequeno.

**Como isso se relaciona com o isolamento multitenant?**
São proteções ortogonais. O RLS impede um tenant de ver as respostas de outro, e o desenho de anonimato impede o próprio tenant de identificar quem respondeu. As duas tabelas de clima também têm `tenant_id` e RLS.
