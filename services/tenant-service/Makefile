# Alvos do tenant-service. A raiz do monorepo delega para cá (make tenant-service-test etc.).
.PHONY: lint test test-unit build docker

lint:
	@unformatted="$$(gofmt -l .)"; if [ -n "$$unformatted" ]; then echo "arquivos sem gofmt:"; echo "$$unformatted"; exit 1; fi
	go vet ./...

# test roda tudo, inclusive integração com Postgres via testcontainers (precisa de Docker).
test:
	go test -race -count=1 ./...

# test-unit pula a integração (-short), para um ciclo rápido sem Docker.
test-unit:
	go test -short -count=1 ./...

build:
	CGO_ENABLED=0 go build -o bin/tenant-service ./cmd/tenant-service

docker:
	docker build -t people-platform/tenant-service:dev .
