# 0004. Keycloak multitenant: realm único com Organizations

- Status: aceito
- Data: 2026-11-21

## Contexto

A identidade é centralizada no Keycloak. Cada tenant tem os próprios usuários, papéis (colaborador, gestor, admin do tenant) e, em planos maiores, SSO com o provedor corporativo do cliente (SAML/OIDC). Os serviços precisam saber, a partir do token, **quem** é a pessoa e **de qual tenant** ela é.

Há duas formas de modelar tenants no Keycloak: um realm por tenant ou um realm único com Organizations (recurso nativo desde o Keycloak 26).

## Decisão

Usar um **realm único (`people`) com uma Organization por tenant**:

- cada tenant da plataforma corresponde a uma Organization, com os domínios de e-mail do cliente associados e, quando houver, um identity provider corporativo vinculado a ela;
- o token traz a claim `tenant_id` (UUID do tenant no tenant-service), emitida por um mapper de atributo do usuário, e os papéis em `realm_access.roles` (`platform-admin`, `tenant-admin`, `manager`, `employee`);
- os serviços validam assinatura (JWKS), emissor, audiência (`people-api`) e expiração. O tenant **sempre** vem da claim e nunca de parâmetro da requisição;
- o gateway valida o token na borda e os serviços validam de novo (defesa em profundidade);
- no ambiente local, o realm é importado de `infra/keycloak/realm-people.json` com dois tenants fictícios (Construtora Horizonte Exemplo e Rede Varejo Sol Exemplo) e usuários de teste.

## Consequências

- Positivas: uma única configuração de clientes, mappers e políticas, e um JWKS e um emissor só para todos os serviços. Criar um tenant é criar uma Organization, que é barato e automatizável a partir do evento `TenantCriado`. Usuários que pertencem a mais de um tenant (consultores, por exemplo) são possíveis no futuro.
- Negativas: as configurações por tenant que vivem no realm (política de senha, duração de sessão, temas de login) são compartilhadas, porque o Keycloak as define por realm. Um erro de configuração do realm afeta todos os tenants. E o recurso de Organizations é relativamente novo.
- Mitigações: o realm é versionado como código e revisado em PR. Clientes com exigências muito específicas de política podem ganhar realm dedicado no futuro, já que os serviços só dependem do emissor, do JWKS e da claim `tenant_id`.

## Alternativas consideradas

- **Realm por tenant:** isolamento forte e políticas independentes. Porém exige um emissor e um JWKS por tenant (os serviços precisam descobrir qual realm validar antes de validar), aumenta a carga do Keycloak com milhares de realms e multiplica a automação de clientes e mappers.
- **Claim de grupo (`/tenants/horizonte`) em vez de atributo:** funciona, mas acopla o identificador do tenant ao nome do grupo. O UUID como atributo é estável mesmo se o tenant mudar de nome.
