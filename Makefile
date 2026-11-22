# Makefile da raiz do monorepo. Alvos padronizados (ADR 0010) e um conjunto por serviço.
# `make help` lista tudo.

SHELL := /bin/bash
.DEFAULT_GOAL := help

COMPOSE        := docker compose
# PORTFOLIO=1 anexa o gateway à rede externa compartilhada portfolio-net (make network).
ifeq ($(PORTFOLIO),1)
COMPOSE        += -f docker-compose.yml -f docker-compose.portfolio.yml
endif
PROFILES_CORE  := --profile core
PROFILES_ALL   := --profile core --profile observability --profile tools --profile tenant-service --profile recruitment-service --profile engagement-service --profile analytics-ai-service

GRADLE_IMAGE   := gradle:8.14.3-jdk21
GRADLE         := docker run --rm -v "$(CURDIR)/services/performance-service":/home/gradle/project \
                  -v people-platform-gradle-cache:/home/gradle/.gradle -w /home/gradle/project $(GRADLE_IMAGE) \
                  gradle --no-daemon --console=plain
SPECTRAL       := npx -y @stoplight/spectral-cli@6.15.0

.PHONY: help up up-all down logs ps seed smoke test test-integration lint clean network \
        contracts-lint \
        tenant-service-lint tenant-service-test tenant-service-test-unit tenant-service-build tenant-service-up \
        performance-service-lint performance-service-test performance-service-build \
        engagement-service-lint engagement-service-test engagement-service-test-unit engagement-service-build engagement-service-up

help: ## Lista os alvos disponíveis
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "} {printf "  \033[36m%-28s\033[0m %s\n", $$1, $$2}'

# ------------------------------------------------------------------ ambiente
up: ## Sobe o núcleo (Postgres, Kafka, Keycloak, gateway, LocalStack, Mailpit)
	$(COMPOSE) $(PROFILES_CORE) up -d --wait

up-all: ## Sobe núcleo, observabilidade, ferramentas e os serviços implementados
	$(COMPOSE) $(PROFILES_ALL) up -d --build --wait

down: ## Para tudo (mantém os volumes)
	$(COMPOSE) $(PROFILES_ALL) down

logs: ## Acompanha os logs (ex.: make logs S=tenant-service)
	$(COMPOSE) $(PROFILES_ALL) logs -f $(S)

ps: ## Lista os contêineres e o estado de saúde
	$(COMPOSE) $(PROFILES_ALL) ps

seed: ## Carrega os tenants fictícios (exige make tenant-service-up antes, para o esquema existir)
	$(COMPOSE) $(PROFILES_CORE) exec -T postgres \
	  psql -v ON_ERROR_STOP=1 -U tenant_service_owner -d tenant_service -f /seed/tenant-service.sql

smoke: ## Exercita o tenant-service pelo gateway com tokens reais do Keycloak
	./scripts/smoke.sh

clean: ## Remove contêineres, volumes e artefatos de build
	$(COMPOSE) $(PROFILES_ALL) down -v --remove-orphans
	rm -rf services/tenant-service/bin services/performance-service/build services/performance-service/.gradle

network: ## Cria a rede externa compartilhada portfolio-net (opcional)
	docker network inspect portfolio-net >/dev/null 2>&1 || docker network create portfolio-net

# ------------------------------------------------------------------ qualidade
lint: contracts-lint tenant-service-lint performance-service-lint engagement-service-lint ## Lint de contratos e de todos os serviços

test: tenant-service-test-unit performance-service-test engagement-service-test-unit ## Testes unitários de todos os serviços (sem Docker no Go)

test-integration: tenant-service-test engagement-service-test ## Testes com infraestrutura real (Postgres via testcontainers)

contracts-lint: ## Valida OpenAPI e AsyncAPI com Spectral
	$(SPECTRAL) lint "contracts/openapi/*.yaml" "contracts/asyncapi/*.yaml" --fail-severity=warn

# ------------------------------------------------------------ tenant-service
tenant-service-lint: ## gofmt + go vet
	$(MAKE) -C services/tenant-service lint

tenant-service-test: ## Todos os testes, inclusive integração com Postgres (precisa de Docker)
	$(MAKE) -C services/tenant-service test

tenant-service-test-unit: ## Testes sem Docker (-short)
	$(MAKE) -C services/tenant-service test-unit

tenant-service-build: ## Compila o binário
	$(MAKE) -C services/tenant-service build

tenant-service-up: ## Sobe o núcleo e o tenant-service (migração inclusa)
	$(COMPOSE) $(PROFILES_CORE) --profile tenant-service up -d --build --wait

# ------------------------------------------------------- performance-service
performance-service-lint: ## ktlint (via Docker, imagem gradle:jdk21)
	$(GRADLE) ktlintCheck

performance-service-test: ## Testes JUnit do núcleo de domínio (via Docker)
	$(GRADLE) test

performance-service-build: ## Compila e empacota o núcleo (via Docker)
	$(GRADLE) build

# -------------------------------------------------------- recruitment-service
.PHONY: recruitment-service-lint recruitment-service-test recruitment-service-up
recruitment-service-lint: ## Pint (exige PHP 8.3+ e composer install no serviço)
	$(MAKE) -C services/recruitment-service lint

recruitment-service-test: ## Testes unitários e de API do recruitment-service (SQLite; a suíte Integration roda no CI)
	$(MAKE) -C services/recruitment-service test-unit

recruitment-service-up: ## Sobe o núcleo e o recruitment-service (migração e relay inclusos)
	$(COMPOSE) $(PROFILES_CORE) --profile recruitment-service up -d --build --wait

# ------------------------------------------------------ engagement-service
engagement-service-lint: ## eslint + tsc
	$(MAKE) -C services/engagement-service lint

engagement-service-test: ## Todos os testes, inclusive integração com Postgres (precisa de Docker)
	$(MAKE) -C services/engagement-service test

engagement-service-test-unit: ## Testes sem Docker
	$(MAKE) -C services/engagement-service test-unit

engagement-service-build: ## Compila o TypeScript
	$(MAKE) -C services/engagement-service build

engagement-service-up: ## Sobe o núcleo e o engagement-service (migração inclusa)
	$(COMPOSE) $(PROFILES_CORE) --profile engagement-service up -d --build --wait

# --------------------------------------------------- analytics-ai-service
.PHONY: analytics-ai-service-lint analytics-ai-service-test analytics-ai-service-up
analytics-ai-service-lint: ## ruff + mypy (precisa de make venv no serviço)
	$(MAKE) -C services/analytics-ai-service lint

analytics-ai-service-test: ## Testes unitários (LLM falso, sem rede)
	$(MAKE) -C services/analytics-ai-service test

analytics-ai-service-up: ## Sobe o núcleo e o analytics-ai-service
	$(COMPOSE) $(PROFILES_CORE) --profile analytics-ai-service up -d --build --wait
