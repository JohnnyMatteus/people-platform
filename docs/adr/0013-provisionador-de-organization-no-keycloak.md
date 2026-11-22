# 0013. Provisionador de Organization no Keycloak dentro do tenant-service

- Status: aceito
- Data: 2026-11-22

## Contexto

O ADR 0004 decide um realm único com uma Organization por tenant e diz que criar o tenant "é criar uma Organization, automatizável a partir do evento `TenantCriado`". O design doc lista o provisionador como planejado. Sem ele, quem cria um tenant precisa criar a Organization à mão, e o token do tenant novo não funciona.

## Decisão

- O provisionador é um **consumidor do tópico `people.tenant.events.v1` dentro do tenant-service**, ligado só quando `KEYCLOAK_URL` está definida. O tenant-service já é o dono do evento e da relação com a identidade (mapa de contexto: "provisiona Organization (ACL)"), então não nasce um serviço novo para uma única chamada HTTP.
- O Keycloak fica **atrás de uma porta** (`provisioning.IdentityProvisioner`). O domínio e o consumidor não conhecem HTTP; o adaptador `adapters/keycloak` fala a Admin REST API.
- **Idempotência pela própria operação:** a Organization guarda o `tenant_id` como atributo e a busca `q=tenant_id:<uuid>` decide entre criar e não fazer nada. Isso dispensa uma tabela de eventos processados e tolera a entrega pelo menos uma vez do Kafka e duas instâncias em corrida.
- **Autenticação por client credentials** de um client de serviço (`people-provisioner`), com segredo por variável de ambiente; o token é guardado até perto de expirar e renovado uma vez em caso de 401.
- **Falhas:** transitórias (rede, 5xx, token recusado) reentregam a mesma mensagem, sem pular, para manter a ordem; recusas definitivas do Keycloak e mensagens malformadas viram `ErrPermanent` e são descartadas com log.

## Consequências

- Positivas: criar o tenant passa a criar a Organization sem passo manual, o código é testável sem Keycloak e a troca do provedor mexe só no adaptador.
- Negativas: o adaptador não foi exercitado contra um Keycloak real (só contra um dublê HTTP e o `openapi.json` publicado); a permissão exata do service account é hipótese. Domínios de e-mail e a associação de usuários à Organization não estão cobertos, então o atributo `tenant_id` do usuário continua sendo configurado fora deste fluxo. Uma recusa permanente descartada só deixa rastro no log.
- Um segundo grupo de consumo no tópico de tenants não interfere nos consumidores dos outros serviços.

## Alternativas consideradas

- **Serviço separado só para isso:** isolaria a credencial do Keycloak, mas custa uma imagem, um job de CI e um perfil de compose para uma chamada.
- **Chamar o Keycloak na mesma requisição de criação do tenant:** é o *dual write* que o ADR 0006 evita.
- **Tabela de eventos processados para deduplicar:** desnecessária enquanto a operação é idempotente por natureza.
