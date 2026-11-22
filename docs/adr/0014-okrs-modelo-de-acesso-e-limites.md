# 0014. OKRs: modelo de acesso e limites

- Status: aceito, com uma decisão pendente do autor (hierarquia de gestão)
- Data: 2026-11-22

## Contexto

O performance-service ganhou objetivos desdobrados (empresa, equipe, pessoa), resultados-chave e check-ins. O serviço não tem hoje nenhuma noção de "quem gere quem": o token traz o papel `manager`, e o cadastro de colaboradores (`employees`) guarda só nome, cargo e data no cargo.

## Decisão

- **Desdobramento só para baixo:** empresa > equipe > pessoa; um objetivo se desdobra de outro de nível mais alto e o da empresa não tem pai. A cadeia não fecha em ciclo.
- **Quem cria:** empresa, só `tenant-admin`; equipe, `manager` ou `tenant-admin`; pessoa, a própria ou um gestor. Dono de outra pessoa precisa ser colaborador cadastrado no mesmo tenant (a RLS esconde os de outros, e a resposta é 404, sem revelar se existe em outro tenant).
- **Quem enxerga:** empresa e equipe são transparentes para o tenant inteiro; individual, só o dono e quem gere. Para os demais responde 404.
- **Quem atualiza:** check-in e encerramento, o dono ou um gestor; empresa, só `tenant-admin`. Objetivo encerrado devolve 409.
- **Números:** valores de resultado-chave e de check-in são decimais como string e têm de caber em `numeric(18, 4)` (até 14 dígitos inteiros e 4 decimais); acima disso, 422 em vez de erro do banco.
- **Progresso:** fração entre 0 e 1 por resultado-chave (vale para metas decrescentes) e média simples no objetivo, calculados na leitura, nunca gravados.

## Limite conhecido e decisão pendente do autor

**`manager` vale para o tenant inteiro.** Não há linha de subordinação: qualquer gestor vê e altera o objetivo individual de qualquer pessoa do tenant, e qualquer gestor cria objetivo de equipe e de pessoa para qualquer colaborador. Isso é coerente com o resto do serviço (`Caller.canSee` também libera qualquer gestor), mas é mais permissivo do que um RH real aceitaria.

Não implementamos hierarquia porque ela não existe em lugar nenhum do modelo e inventá-la aqui criaria uma fonte de verdade paralela. Opções para o autor:

1. **Hierarquia no cadastro:** `manager_id` em `employees`, alimentado pelo diretório de pessoas quando existir; gestor passa a ver a árvore abaixo dele. É a opção correta no longo prazo e a que recomendamos.
2. **Equipe como agrupamento explícito:** um objetivo de equipe lista os membros, e o gestor só enxerga objetivos de quem está na equipe dele.
3. **Manter o tenant inteiro** e documentar como regra do produto (aceitável para empresas pequenas).

Até a decisão, a regra acima fica registrada como limite, não como garantia.

## Consequências

- Positivas: regras de acesso pequenas e testadas, sem estado de hierarquia a sincronizar.
- Negativas: a superfície de acesso do gestor é ampla; trocar para hierarquia muda `canSee`, `canWrite` e a criação de objetivos, e exigirá migração dos dados de gestão.

## Alternativas consideradas

- **Todo objetivo visível a todos:** simples, mas expõe metas individuais, que podem tocar desempenho.
- **Só o dono e o admin:** seguro, mas inviabiliza o gestor acompanhar a equipe, que é o uso principal.
