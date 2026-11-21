# 0003. White label por design tokens com contraste WCAG AA garantido

- Status: aceito
- Data: 2026-11-21

## Contexto

Cada tenant quer a plataforma com a própria identidade: cores, logo, domínio próprio (ex.: `rh.horizonte.example.com`) e e-mails com remetente da empresa. Deixar o tenant escolher qualquer cor cria um risco real de acessibilidade, como texto branco sobre amarelo, ilegível para muita gente. A plataforma também precisa saber de quem é a marca **antes do login**, só pelo domínio acessado.

## Decisão

- O tema é um conjunto pequeno e fechado de **design tokens semânticos** (`color.primary`, `color.on-primary`, `color.background`, `color.text`, `color.accent`), mais logo e remetente de e-mail. O front-end (Vue 3) consome só esses tokens como CSS custom properties. Nenhum componente usa cor fixa.
- O tenant-service **recusa** um tema cujo par texto/fundo tenha razão de contraste abaixo de **4.5:1** (WCAG 2.x nível AA para texto normal). Os pares verificados são `on_primary/primary`, `text/background` e `accent/background`. O cálculo segue a fórmula de luminância relativa do WCAG. A comparação usa o valor exato, sem arredondar, para não aprovar 4.495:1. O erro devolvido (HTTP 422) diz qual par falhou e qual foi a razão obtida, para a tela de configuração orientar o admin.
- Enquanto o tenant não define um tema, vale o tema padrão da plataforma, que também é testado contra AA.
- Resolução de tenant:
  - **por host:** `GET /v1/branding` usa `X-Forwarded-Host` (repassado pelo gateway) ou `Host` e devolve só a identidade visual. A rota é pública e tem cache curto;
  - **por claim:** `GET /v1/me/tenant` usa a claim `tenant_id` do token.
  - Tenants suspensos ou desativados não são resolvidos.
- Domínio próprio é uma feature de plano (`custom_domain`). O host é chave primária global, então um domínio pertence a um único tenant.
- O logo é enviado direto ao bucket por **URL pré-assinada** com chave prefixada pelo tenant (`tenants/{id}/logo/...`). O serviço nunca trafega o binário.
- Os e-mails transacionais usam o remetente do tema (`email_from_name`, `email_reply_to`) e saem via SES. Localmente, o SES é emulado pelo LocalStack e entregue ao Mailpit (ADR 0010).

## Consequências

- Positivas: a acessibilidade é garantida por construção e não depende de revisão manual. O front-end fica simples, só com tokens. E a marca aparece já na tela de login.
- Negativas: um tenant com identidade visual de baixo contraste precisa ajustar tons, e isso pode gerar atrito comercial. A resposta de validação mostra a razão obtida e o mínimo para orientar o ajuste. O conjunto fechado de tokens limita customizações profundas, como fontes e espaçamentos, e isso é intencional.

## Alternativas consideradas

- **CSS livre por tenant:** máxima flexibilidade, mas quebra acessibilidade e consistência, e ainda abre risco de injeção de CSS.
- **Avisar sem bloquear:** um alerta ignorável não protege quem usa a plataforma. Contraste é requisito, não preferência.
- **Calcular a cor de texto automaticamente (preto ou branco):** resolve parte do problema, mas tira do tenant o controle da marca. Fica como sugestão futura na tela de configuração.
