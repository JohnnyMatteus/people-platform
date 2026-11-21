# Roteiro: nine box, calibração e sucessão

Este roteiro cobre o núcleo de domínio do performance-service: classificar pessoas na matriz desempenho × potencial com limiares configuráveis por tenant, permitir que um comitê calibre as posições com regras e trilha de auditoria, manter o histórico entre ciclos e usar esse histórico para sugerir a prontidão no mapa de sucessão.

Código de referência (Kotlin puro, sem framework):

- `services/performance-service/src/main/kotlin/com/peopleplatform/performance/domain/ninebox/`: `Policy.kt`, `Quadrant.kt`, `Classifier.kt`, `Calibration.kt`, `History.kt`;
- `services/performance-service/src/main/kotlin/com/peopleplatform/performance/domain/succession/Succession.kt`;
- testes em `src/test/kotlin/...`;
- `docs/adr/0011-nucleo-de-dominio-antes-do-framework.md`.

## 1. O problema

Ao fim de um ciclo, cada pessoa tem uma nota consolidada de **desempenho** (avaliações) e de **potencial** (assessment). O RH quer:

1. posicionar cada pessoa num dos 9 quadrantes, de forma determinística e explicável;
2. deixar cada empresa definir a própria escala e os limiares (1–5, 0–10, com cortes diferentes);
3. permitir que o comitê **corrija** posições (a nota não captura tudo), mas sem virar arbitrário: com justificativa, limite de movimento e registro de quem mudou o quê;
4. acompanhar a trajetória da pessoa entre ciclos;
5. alimentar o mapa de sucessão das posições críticas.

## 2. A solução

### 2.1 Classificação

`AxisThresholds(mediumFrom, highFrom)` divide cada eixo em LOW, MEDIUM e HIGH. Os limites inferiores são **inclusivos**: nota exatamente 3.75 com `highFrom = 3.75` é HIGH. O quadrante é a combinação dos dois níveis, de Q1 (baixo/baixo) a Q9 (alto/alto).

Decisões importantes:

- **`BigDecimal` em vez de `Double`:** `3.75` em ponto flutuante pode virar `3.7499999…` e mudar a faixa de alguém. Numa decisão que afeta carreira, erro de arredondamento não é aceitável.
- **A política valida a si mesma:** `scale.min < mediumFrom < highFrom ≤ scale.max`. Uma política incoerente não chega a existir.
- **Notas fora da escala são recusadas,** em vez de serem "grudadas" na borda.
- **Tenant conferido:** avaliação de outro tenant gera `DomainException`. O domínio também participa do isolamento.

### 2.2 Calibração

`CalibrationSession` recebe as avaliações do ciclo e mantém duas visões: a posição **original** (calculada) e a **atual** (após ajustes). As regras são estas:

| Regra | Por quê |
| --- | --- |
| Justificativa com tamanho mínimo (padrão 20) | Ajuste sem motivo registrado vira viés invisível |
| No máximo N faixas por eixo (padrão 1), **medidas contra a posição original** | Medir contra a atual permitiria somar vários ajustes pequenos num salto grande |
| Ninguém calibra a si mesmo | Conflito de interesse óbvio |
| Ajuste para o mesmo quadrante é recusado | Evita ruído na trilha de auditoria |
| Só avaliações do tenant e do ciclo da sessão | Isolamento e coerência |
| Sessão fechada é imutável | O resultado vira fato publicado (`NineBoxCalibrado`) |
| Alerta se o quadrante de topo passar da diretriz (padrão 20%) | Inflação de notas é comum; o alerta **não bloqueia**, porque distribuição forçada é decisão do RH |

O fechamento produz um `CalibrationOutcome`, com as posições finais e os ajustes, e `events()` gera um `NineBoxCalibrado` por pessoa, marcando quem foi calibrado.

### 2.3 Histórico e tendência

`NineBoxHistory` registra um resultado por ciclo, em ordem cronológica, sem repetir ciclo e sem aceitar outro tenant. `trend()` compara os dois últimos ciclos:

- os dois eixos subindo ou parados → **UP**;
- os dois descendo ou parados → **DOWN**;
- nenhum muda → **STABLE**;
- um sobe e o outro desce → **LATERAL**. Trocar desempenho por potencial não é melhora nem piora, e o sistema não finge que sabe.

### 2.4 Sucessão

- `SuccessionPlan` de uma `CriticalPosition`: o ocupante não é sucessor de si mesmo, cada pessoa aparece uma vez e há teto de sucessores (padrão 5). O mapa precisa ser uma decisão, não uma lista do time inteiro.
- `ranked` ordena por prontidão (pronto agora → 1–2 anos → 3+ anos) e, no empate, pela data de indicação.
- `benchStrength()`:

| Situação | Força |
| --- | --- |
| Sem sucessores | NONE |
| Só sucessores de 3+ anos | WEAK |
| Ao menos um pronto agora ou em 1–2 anos | ADEQUATE |
| Ao menos um pronto agora **e** ao menos dois no total | STRONG |

- `ReadinessAdvisor.suggest()` sugere a prontidão inicial a partir do último quadrante e do tempo na função:

| Último quadrante | Sugestão |
| --- | --- |
| Q9 com 12 meses ou mais na função | Pronto agora |
| Q9 com menos de 12 meses, Q8, Q6 | 1–2 anos |
| Q4, Q5, Q7 | 3+ anos |
| Q1, Q2, Q3 (potencial baixo) | Sem sugestão (não é indicado como sucessor) |

  É só sugestão: o comitê decide, e a sugestão nunca vira decisão sem confirmação.

## 3. Alternativas e trade-offs

| Tema | Alternativa | Por que não |
| --- | --- | --- |
| Limiares | Fixos em terços da escala | Cada empresa tem cultura de nota diferente; uma "nota 3" pode ser excelente numa e mediana noutra |
| Representação numérica | `Double` | Erros de arredondamento nos limiares |
| Calibração | Sobrescrever a nota | Perde a posição original e a trilha de auditoria; impede medir o quanto o comitê mexeu |
| Limite de movimento | Sem limite | O comitê poderia levar alguém de Q1 para Q9 sem reavaliação; o limite força o retorno ao processo formal |
| Distribuição | Curva forçada bloqueante | Polêmica e muitas vezes injusta com times pequenos ou excepcionais; virou alerta configurável |
| Prontidão | Calculada e gravada automaticamente | Decisões de sucessão envolvem contexto que o modelo não tem (mobilidade, interesse da pessoa) |
| Framework | Spring Boot desde já | O núcleo ficaria acoplado a anotações e infraestrutura; em Kotlin puro, os testes rodam em milissegundos |

## 4. Perguntas que um entrevistador faria

**Por que o limite de faixas é medido contra a posição original e não contra a atual?**
Se fosse contra a atual, três ajustes de uma faixa levariam alguém de Q1 a Q9 com a mesma regra de "no máximo uma faixa". Há um teste específico para isso: LOW → MEDIUM é aceito, e o MEDIUM → HIGH seguinte é recusado, porque já seriam duas faixas desde a origem.

**Por que o alerta de topo não bloqueia o fechamento?**
Porque um time pode ser de fato excepcional, e bloquear forçaria o comitê a rebaixar pessoas para caber numa cota. O papel do sistema é tornar a inflação visível, não decidir por eles. Se um tenant quiser curva forçada, isso vira uma política configurável e explícita.

**Como você garante que os limiares do tenant são coerentes?**
O construtor de `NineBoxPolicy` valida `min < médio < alto ≤ max` e os demais parâmetros. Uma política inválida não pode ser instanciada, então nenhuma regra precisa lidar com ela.

**Onde ficam essas regras quando entrar o Spring Boot?**
Exatamente onde estão. O Spring entra como adaptador: o controller converte o DTO em `Assessment`, o repositório JPA reconstrói a sessão e o consumidor Kafka chama `session.add(...)`. O domínio não importa nada do Spring, e por isso os testes não precisam de contexto Spring.

**Como persistir uma `CalibrationSession` que guarda estado em mapas?**
Os ajustes são *append-only* (`calibration_adjustments`). A sessão é reconstruída a partir das avaliações e da sequência de ajustes, uma espécie de *event sourcing* local ao agregado, o que dá auditoria completa de graça. A posição atual pode ser materializada para leitura rápida.

**E concorrência: dois calibradores ajustando a mesma pessoa ao mesmo tempo?**
Na persistência, a sessão tem versão (*optimistic locking*). O segundo `adjust` falha com conflito e precisa recarregar a sessão. Como o limite é contra a origem, não há como dois ajustes simultâneos somarem um salto proibido depois da recarga.

**Por que `LATERAL` em vez de escolher um lado?**
Ganhar potencial e perder desempenho (comum depois de uma promoção recente) não é piora. Forçar UP ou DOWN produziria um indicador enganoso nos painéis de RH.

**Como a sucessão usa o nine box sem criar acoplamento forte?**
O `ReadinessAdvisor` depende só de `HistoryEntry` (quadrante e ciclo), não da sessão de calibração. Em produção, a entrada vem do evento `NineBoxCalibrado`, ou seja, do contrato e não da implementação.

**Que dado pessoal sai nos eventos?**
Só IDs e o quadrante, sem notas, justificativas ou nomes. Justificativas de calibração são sensíveis e ficam no performance-service, visíveis só ao RH e ao comitê.
