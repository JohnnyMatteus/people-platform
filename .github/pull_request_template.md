## O que muda

<!-- Uma ou duas frases: o problema e a solução. -->

## Serviços e contratos afetados

- [ ] tenant-service
- [ ] performance-service
- [ ] contratos (OpenAPI / AsyncAPI)
- [ ] infraestrutura (compose, gateway, Keycloak, observabilidade)
- [ ] documentação (design doc, ADR, roteiros)

## Checklist

- [ ] Testes cobrem o caminho feliz, os casos de borda e as falhas
- [ ] Toda tabela e todo evento novos têm `tenant_id`, com teste de isolamento quando houver persistência
- [ ] Mudança de contrato é compatível ou cria uma versão nova (`event_version` / `v2`)
- [ ] Sem dado pessoal novo em logs, spans ou prompts de IA (LGPD)
- [ ] Decisão relevante registrada em ADR
- [ ] `make lint` e `make test` passam localmente

## Como testar

<!-- Comandos copiáveis. -->
