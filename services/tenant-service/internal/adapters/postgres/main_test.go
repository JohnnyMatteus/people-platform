package postgres

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Os testes deste pacote sobem um Postgres real com testcontainers, porque RLS só pode ser
// verificado no banco. Rode com -short para pular quando não houver Docker.
var (
	ownerPool *pgxpool.Pool // dono do esquema: ignora RLS, usado só para migrar e inspecionar
	appPool   *pgxpool.Pool // usuário da aplicação: sujeito a RLS, como em produção
)

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	flag.Parse()
	if testing.Short() {
		fmt.Println("pulando testes de integração com Postgres (-short)")
		return 0
	}
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("tenant_service"),
		tcpostgres.WithUsername("owner"),
		tcpostgres.WithPassword("test-only"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).WithStartupTimeout(3*time.Minute)),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "subir Postgres: %v\n", err)
		return 1
	}
	defer func() { _ = testcontainers.TerminateContainer(container) }()

	ownerDSN, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ownerPool, err = pgxpool.New(ctx, ownerDSN)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer ownerPool.Close()

	if err := Migrate(ctx, ownerPool); err != nil {
		fmt.Fprintf(os.Stderr, "migrar: %v\n", err)
		return 1
	}
	// Rodar de novo prova que a migração é idempotente.
	if err := Migrate(ctx, ownerPool); err != nil {
		fmt.Fprintf(os.Stderr, "migrar de novo: %v\n", err)
		return 1
	}
	if _, err := ownerPool.Exec(ctx, `CREATE USER app_user PASSWORD 'test-only' IN ROLE tenant_app`); err != nil {
		fmt.Fprintf(os.Stderr, "criar usuário da aplicação: %v\n", err)
		return 1
	}

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "5432/tcp")
	appPool, err = pgxpool.New(ctx, fmt.Sprintf("postgres://app_user:test-only@%s:%s/tenant_service?sslmode=disable", host, port.Port()))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer appPool.Close()

	return m.Run()
}
