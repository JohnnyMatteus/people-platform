# Roteiro: isolamento multitenant

Este é o núcleo mais crítico da plataforma. Um vazamento de dados de RH entre empresas clientes encerra a confiança no produto. Este roteiro explica como o isolamento foi construído no tenant-service, por que foi feito assim e como defendê-lo numa entrevista.

Código de referência:

- `services/tenant-service/internal/adapters/postgres/migrations/0001_tenants.sql`: políticas de RLS e papel da aplicação;
- `services/tenant-service/internal/adapters/postgres/scope.go`: escopo por transação;
- `services/tenant-service/internal/adapters/postgres/repository_test.go`: `TestRLS_CrossTenantIsolation`;
- `services/tenant-service/internal/app/principal.go` e `service.go`: autorização por tenant;
- `docs/adr/0002-multitenancy-tenant-id-e-rls.md`.

## 1. O problema em uma frase

Muitas empresas compartilham as mesmas tabelas, e precisamos garantir que uma consulta feita em nome da empresa A **nunca** devolva, altere ou crie linhas da empresa B, mesmo quando o código da aplicação tiver um bug.

## 2. A solução em camadas

| Camada | Mecanismo | Que bug ela pega |
| --- | --- | --- |
| Identidade | `tenant_id` vem da claim do JWT validado (assinatura, emissor, audiência, expiração) | Cliente mandando `tenant_id` de outro no corpo da requisição |
| Caso de uso | `Principal.CanRead(id)` / `CanManage(id)`; recurso de outro tenant responde **404** | Handler que esquece de checar permissão; enumeração de IDs |
| Banco | RLS com `FORCE`, usuário sem `BYPASSRLS`, escopo `SET LOCAL` por transação, falha fechada | Consulta sem `WHERE tenant_id`, `UPDATE` com filtro errado, `INSERT` com tenant trocado |
| Eventos | `tenant_id` obrigatório no envelope (validado pelo JSON Schema) e como chave Kafka | Consumidor gravando efeito no tenant errado |
| Teste | Testes de vazamento contra Postgres real (testcontainers) | Regressão em qualquer camada acima |

### Como o escopo funciona

```go
pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
    tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID.String())
    return fn(tx)
})
```

O terceiro argumento `true` torna a configuração **local à transação**, como um `SET LOCAL`. Quando a transação termina (commit ou rollback), o valor some. Isso é essencial com pool de conexões: sem isso, a próxima requisição que pegasse a mesma conexão herdaria o tenant anterior.

A política é:

```sql
CREATE POLICY tenant_themes_isolation ON tenant_themes
  USING (app_is_platform() OR tenant_id = app_current_tenant())
  WITH CHECK (app_is_platform() OR tenant_id = app_current_tenant());
```

- `USING` filtra o que pode ser **lido, alterado ou apagado**.
- `WITH CHECK` valida o que pode ser **gravado**, ou seja, a linha nova depois do `INSERT` ou `UPDATE`.
- `app_current_tenant()` devolve `NULL` sem escopo, e `tenant_id = NULL` nunca é verdadeiro. Por isso a falha é fechada.
- `FORCE ROW LEVEL SECURITY` faz a política valer até para o dono da tabela. Sem `FORCE`, o dono ignora o RLS.

### O que os testes provam

`TestRLS_CrossTenantIsolation` cria dois tenants e verifica:

1. `SELECT DISTINCT tenant_id FROM <tabela>` **sem WHERE**, no escopo de A, só devolve A, em todas as tabelas;
2. `INSERT` com `tenant_id` de B no escopo de A falha (`WITH CHECK`);
3. `UPDATE ... WHERE id = B` no escopo de A afeta 0 linhas;
4. sem escopo, `count(*)` é 0;
5. depois que uma transação define o escopo e termina, a mesma conexão volta a ver 0 linhas;
6. o repositório no escopo de A não lê os domínios de B.

E os testes de caso de uso e de HTTP mostram que o admin de A recebe 404 ao pedir o tenant B.

## 3. Alternativas e trade-offs

| Opção | Isolamento | Custo por tenant | Operação | Quando eu usaria |
| --- | --- | --- | --- | --- |
| **Pool + RLS (escolhida)** | Lógico, reforçado pelo banco | Muito baixo | Uma migração por serviço | Muitos tenants pequenos e médios |
| Schema por tenant | Lógico, por `search_path` | Médio | Migração em laço; catálogo grande | Poucas centenas de tenants com customização de esquema |
| Banco por tenant (silo) | Físico | Alto | Automação de provisionamento e de migração por banco | Cliente corporativo com exigência contratual; residência de dados |
| Só `WHERE tenant_id` na aplicação | Depende de disciplina | Muito baixo | Simples | Nunca para dados sensíveis |

Os trade-offs aceitos com o modelo pool:

- **Vizinho barulhento:** um tenant grande pode degradar os outros. A mitigação vem com índices que começam por `tenant_id`, rate limit por tenant no gateway e, no limite, mover o tenant para um silo. O roteamento de DSN por tenant é a única mudança necessária.
- **Escopo de plataforma é poder grande:** `app.platform = on` enxerga tudo. Ele fica restrito a três operações (listar tenants, resolver host e relay da outbox), e a revisão de código foca nelas.
- **Custo de desempenho:** o RLS adiciona um predicado em toda consulta. Com índice por `tenant_id`, o custo é desprezível perto do risco evitado.

## 4. Perguntas que um entrevistador faria

**Por que não basta colocar `WHERE tenant_id = ?` em todas as consultas?**
Porque isso depende de nenhum desenvolvedor jamais esquecer, para sempre, em todo serviço. O RLS transforma esse esquecimento de "vazamento" em "lista vazia". O teste com consulta sem `WHERE` prova exatamente isso.

**Como o RLS se comporta com pool de conexões? Não há risco de uma requisição herdar o tenant da anterior?**
Haveria, se o escopo fosse definido com `SET` de sessão. Uso `set_config(..., true)`, que vale só na transação, e toda operação roda dentro de uma transação explícita. Um teste específico pega uma conexão, define o escopo numa transação, termina, e verifica que a mesma conexão volta a ver zero linhas.

**E se alguém conectar como superusuário ou como dono da tabela?**
Superusuário ignora o RLS por definição, então a aplicação nunca usa superusuário. O dono só é usado nas migrações. Mesmo assim, `FORCE ROW LEVEL SECURITY` faz o dono respeitar as políticas: o seed de desenvolvimento, que roda como dono, precisa declarar o escopo de plataforma.

**Por que responder 404 em vez de 403 quando o recurso é de outro tenant?**
Um 403 confirma que o ID existe, e isso é um vazamento de informação: permite enumerar quantos tenants existem e quais IDs são válidos. Para quem não pode ver, o recurso não existe.

**Como você criaria um tenant se o RLS exige escopo de tenant?**
A criação roda no escopo do **próprio tenant novo**: o `INSERT` em `tenants` com `id = X` passa no `WITH CHECK`, porque `app.tenant_id = X`. Não precisei do escopo de plataforma para isso. A unicidade do slug é garantida pela constraint, que vale para a tabela toda, independente do RLS.

**Constraint `UNIQUE` não vaza informação entre tenants?**
Vaza um bit: "esse slug ou domínio já existe". Para slug e domínio isso é inerente, porque são identificadores globais por natureza. A resposta é um 409 genérico, que não diz a qual tenant o valor pertence.

**Como garantir que um serviço novo, em outra linguagem, siga o mesmo padrão?**
Por três vias: o ADR define o padrão; o checklist do PR exige `tenant_id` e teste de isolamento em tabela nova; e os contratos de evento exigem `tenant_id`, validado por JSON Schema tanto no CI quanto no teste do produtor.

**Como você migraria um tenant grande para um banco dedicado?**
A aplicação já recebe o tenant em cada operação. Bastaria um resolvedor `tenant_id → pool`, com os tenants silo num mapa e os demais no pool padrão. A cópia dos dados usa `tenant_id` como filtro em todas as tabelas, o que só é viável porque toda tabela tem a coluna.

**E o Kafka? Um consumidor pode misturar tenants?**
O consumidor abre a transação no escopo do `tenant_id` do envelope antes de gravar. Se o evento vier com `tenant_id` errado, o `WITH CHECK` impede gravar em outro tenant. O teste de contrato garante que o produtor sempre envia `tenant_id`.
