# recruitment-service

ATS da People Platform: vagas, candidaturas, pipeline por etapas configurável por tenant, entrevistas e scorecards. PHP 8.4 + Laravel 13. Contrato HTTP em [`contracts/openapi/recruitment-service.yaml`](../../contracts/openapi/recruitment-service.yaml).

## O que faz

- **Pipeline por tenant:** `PUT /v1/pipeline-template` define as etapas padrão; cada vaga pode trazer as próprias. A última etapa é a de contratação.
- **Candidaturas:** `POST /v1/jobs/{id}/applications` cria candidato e candidatura e devolve uma URL pré-assinada para o currículo, que vai direto ao S3 (LocalStack) em `tenants/{tenant_id}/applications/{id}/resume.<ext>`. O banco guarda só a chave; o download também é por URL temporária.
- **Entrevistas e scorecards:** agendamento por candidatura; um scorecard por avaliador (o `sub` do token), com recomendação e notas de 1 a 5 por critério.
- **`CandidatoContratado`:** mover para a etapa final (com `start_date`) grava o evento na outbox na mesma transação. `php artisan outbox:relay` publica em `people.recruitment.events.v1` (chave = `tenant_id`), validado nos testes contra o schema do AsyncAPI.
- **Acesso:** só `tenant-admin` e `manager` (dados pessoais de candidatos).

## Arquitetura

```
app/Domain           Pipeline, status, recomendação, Principal (sem framework)
app/Application      casos de uso e portas (ResumeStorage, EventOutbox, EventPublisher)
app/Infrastructure   JWT/JWKS, S3, outbox + Kafka, escopo de tenant
app/Http             middlewares (jwt, role, tenant), controllers finos, problem+json
```

## Multitenancy (ADR 0002)

O tenant vem só da claim `tenant_id` do JWT (realm `people`: assinatura RS256 por JWKS, issuer e audience `people-api`). Cada requisição roda numa transação com `set_config('app.tenant_id', ..., true)`; todas as tabelas têm RLS com `FORCE`, e a aplicação conecta com `recruitment_service_app`, sem BYPASSRLS. Os models também filtram por tenant (defesa em camadas). O relay da outbox usa o escopo de plataforma.

## Rodar

```bash
make recruitment-service-up        # na raiz: núcleo + migração + serviço + relay (porta 8082)
make recruitment-service-test      # unitários e API, SQLite em memória, sem Docker
```

Migrações usam a conexão `migrate` (dono do esquema): `php artisan migrate --database=migrate`.

## Testes

- `Unit` e `Feature` (SQLite): domínio, JWT, URLs do S3, fluxo completo da API, isolamento no nível da aplicação e conformidade do evento com o AsyncAPI.
- `Integration` (Postgres real, `RECRUITMENT_PG_TESTS=1`, roda no CI): vazamento entre tenants com RLS (consulta sem WHERE, escrita cruzada, sem escopo, escopo que não sobrevive à transação) e outbox.

## Exclusão de titular (LGPD)

`privacy-events:consume` lê `TitularExclusaoSolicitada` e `DataSubjectEraser` apaga, no tenant do evento, candidato, candidaturas, entrevistas, scorecards e o currículo no S3 (o endpoint interno `S3_ENDPOINT` é distinto do de URLs assinadas). Para um colaborador que entrevistou ou avaliou, só a ligação é desfeita. Deduplicação por `event_id`. O relay da outbox registra `attempts` e `last_error` e não deixa um tenant com mensagem recusada travar os outros.

## Fora do escopo desta rodada

Imagem de produção (php-fpm + nginx) e expurgo de currículos de não contratados além da expiração do bucket.
