# Alvos do engagement-service. A raiz do monorepo delega para cá (make engagement-service-test etc.).
.PHONY: install lint test test-unit build docker

install:
	npm ci

lint:
	npm run lint

# test roda tudo, inclusive integração com Postgres via testcontainers (precisa de Docker).
test:
	npx jest

# test-unit pula a integração, para um ciclo rápido sem Docker.
test-unit:
	npx jest --testPathIgnorePatterns integration

build:
	npm run build

docker:
	docker build -t people-platform/engagement-service:dev .
