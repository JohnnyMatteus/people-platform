# 0010. Infraestrutura local: compose por perfis, Postgres com banco por serviço, LocalStack e Mailpit

- Status: aceito
- Data: 2026-11-21

## Contexto

A plataforma depende de Postgres, Kafka, Keycloak, gateway, stack de observabilidade e de serviços AWS: S3 para currículos do ATS e assets white label, e SES para e-mails transacionais com remetente do tenant. Quem clona o repositório precisa subir tudo com um comando, sem conta na nuvem, e conseguir subir só o necessário para o serviço em que está trabalhando.

## Decisão

- **Um `docker-compose.yml` na raiz, organizado por `profiles`:**
  - `core`: Postgres, Kafka (KRaft), Keycloak, gateway, LocalStack e Mailpit;
  - `observability`: OpenTelemetry Collector, Prometheus, Grafana e Jaeger;
  - `tools`: Kafka UI e Adminer;
  - um perfil por serviço (`tenant-service`, e os demais conforme forem implementados), que inclui o job de migração do serviço.
- **Imagens com versão fixada** (nunca `latest`). Todo serviço de infraestrutura tem `healthcheck`, e as dependências usam `depends_on` com `condition: service_healthy` (ou `service_completed_successfully` para jobs de migração e bootstrap). Volumes e rede são nomeados (`people-platform-net`). As variáveis ficam em `.env.example`.
- **Postgres local: um servidor com um banco e um usuário por serviço**, criados pelo script `infra/postgres/init/01-databases.sh`. Cada serviço recebe dois usuários: o dono do esquema, usado só nas migrações, e o da aplicação, sem `BYPASSRLS` (ADR 0002). Isso preserva a fronteira de *database per service* (nenhum usuário enxerga o banco de outro serviço) com o custo de um só contêiner. Em produção, cada serviço tem instância própria.
- **LocalStack** emula S3 e SES. O bootstrap fica em `init/aws/*.sh`, montado em `/etc/localstack/init/ready.d`, e cria:
  - o bucket `people-platform-tenant-assets` (logos e assets white label), com CORS para upload direto do navegador;
  - o bucket `people-platform-recruitment-resumes` (currículos do ATS), privado e com criptografia;
  - as identidades SES verificadas dos remetentes dos tenants fictícios.
- **Mailpit** recebe os e-mails. O LocalStack é configurado com `SMTP_HOST=mailpit:1025`, então todo e-mail enviado via SES aparece na interface do Mailpit (`http://localhost:8025`).
- O código acessa esses serviços por **portas e adaptadores** com endpoint configurável. O mesmo binário funciona na AWS real trocando só as variáveis (ex.: `S3_PRESIGN_ENDPOINT` vazio na AWS e `http://localhost:4566` no local).
- **Rede compartilhada opcional `portfolio-net`** (externa, criada por `make network`): se ela existir, o gateway também é anexado a ela, para outros projetos do ecossistema o enxergarem. Por padrão, o compose usa só a rede própria.
- **Makefile na raiz** com alvos padronizados: `up`, `down`, `logs`, `ps`, `seed`, `test`, `test-integration`, `clean`, `network`, e um alvo por serviço (`tenant-service-test`, `performance-service-test` etc.).

## Consequências

- Positivas: um comando sobe o ambiente. Quem trabalha num só serviço sobe só `core` e o perfil dele. Os e-mails white label ficam visíveis localmente, e o mesmo código roda contra AWS real e LocalStack.
- Negativas: com tudo ligado (`core` + `observability` + serviços), o ambiente consome vários GB de RAM. O LocalStack não reproduz todas as nuances da AWS (políticas IAM, limites do SES), então testes de aceitação em conta real continuam necessários antes de produção.

## Alternativas consideradas

- **MinIO para object storage:** é excelente quando só se precisa de S3. Como também precisamos de SES, o LocalStack cobre os dois com uma imagem só, e manter MinIO e LocalStack juntos seria redundante.
- **SQS para filas:** não foi incluído. Todos os fluxos assíncronos entre contextos passam pelo Kafka (ADR 0005), e não há fluxo que justifique uma segunda tecnologia de mensageria. Se o envio de e-mails em massa precisar de fila de trabalho com *visibility timeout*, o SQS entra num ADR próprio.
- **Um contêiner Postgres por serviço:** é mais fiel à produção, mas multiplica memória e portas no ambiente local sem ganho de isolamento lógico, que os usuários separados já garantem.
- **Kubernetes local (kind) desde já:** fica para a rodada de plataforma (Helm e GitOps). Para desenvolvimento diário, o compose é mais rápido.
