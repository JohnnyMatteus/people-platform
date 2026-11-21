# 0009. API gateway declarativo com KrakenD

- Status: aceito
- Data: 2026-11-21

## Contexto

O front-end e integrações externas precisam de um ponto de entrada único que roteie para os serviços, valide o token na borda, aplique rate limit e repasse o host original (necessário para a resolução de tenant por domínio próprio, ADR 0003).

## Decisão

Usar o **KrakenD** como gateway, com configuração declarativa versionada em `infra/gateway/krakend.json`:

- valida o JWT do Keycloak (JWKS, emissor, audiência) nas rotas autenticadas;
- deixa públicas só as rotas de branding e o catálogo de planos;
- repassa `Authorization` e `X-Forwarded-Host` para os serviços;
- aplica rate limit por cliente nas rotas públicas;
- não guarda estado nem precisa de banco.

## Consequências

- Positivas: o gateway é *stateless* e leve, a configuração é revisável em PR, a validação de JWT é nativa e sobe rápido no compose.
- Negativas: a configuração em JSON fica verbosa conforme as rotas crescem (mitigável com templates do KrakenD). Plugins customizados exigem Go. E o gateway não substitui a autorização nos serviços, que continuam validando o token.

## Alternativas consideradas

- **Kong:** muito completo, mas a versão com banco é mais pesada para o ambiente local, e o modo DB-less tem configuração equivalente.
- **Envoy:** poderoso, porém de configuração de baixo nível para este estágio.
- **Sem gateway (front-end chamando cada serviço):** expõe a topologia interna e espalha CORS, rate limit e TLS.
