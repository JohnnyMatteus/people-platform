// Comando tenant-service: "serve" sobe a API e o relay da outbox; "migrate" aplica o esquema;
// "healthcheck" consulta o /healthz local (a imagem distroless não tem curl nem shell).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/adapters/httpapi"
	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/adapters/kafka"
	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/adapters/postgres"
	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/adapters/s3"
	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/app"
	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/platform/auth"
	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/platform/config"
	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/platform/telemetry"
)

const serviceName = "tenant-service"

// version é preenchida no build com -ldflags "-X main.version=...".
var version = "dev"

func main() {
	logger := telemetry.NewLogger(os.Stdout, serviceName)
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch cmd {
	case "serve":
		err = serve(ctx, logger)
	case "migrate":
		err = migrate(ctx)
	case "healthcheck":
		err = healthcheck(ctx)
	default:
		err = fmt.Errorf("comando desconhecido %q (use serve, migrate ou healthcheck)", cmd)
	}
	if err != nil {
		logger.Error("encerrando com erro", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func migrate(ctx context.Context) error {
	dsn := os.Getenv("DATABASE_MIGRATION_URL")
	if dsn == "" {
		return errors.New("DATABASE_MIGRATION_URL é obrigatória para migrate")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	return postgres.Migrate(ctx, pool)
}

func healthcheck(ctx context.Context) error {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/healthz", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthz respondeu %d", resp.StatusCode)
	}
	return nil
}

func serve(ctx context.Context, logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	shutdownTracing, err := telemetry.SetupTracing(ctx, serviceName, version)
	if err != nil {
		return fmt.Errorf("configurar tracing: %w", err)
	}
	defer func() { _ = shutdownTracing(context.WithoutCancel(ctx)) }()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("conectar ao Postgres: %w", err)
	}
	defer pool.Close()

	publisher, err := kafka.NewPublisher(cfg.KafkaBrokers)
	if err != nil {
		return err
	}
	defer publisher.Close()

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		return fmt.Errorf("configurar AWS: %w", err)
	}
	assets := s3.New(awsCfg, cfg.S3Bucket, cfg.S3PresignEndpoint, cfg.AssetsPublicBaseURL)

	metrics := telemetry.NewMetrics()
	svc := app.NewService(postgres.NewRepository(pool), assets, cfg.LogoUploadTTL, time.Now, newUUIDv7)
	verifier := auth.NewVerifier(cfg.OIDCIssuer, cfg.OIDCAudience, cfg.OIDCJWKSURL, &http.Client{Timeout: 5 * time.Second})
	server := httpapi.NewServer(svc, verifier, metrics, logger, pool.Ping, publisher.Ping)

	relay := postgres.NewRelay(pool, publisher, cfg.OutboxBatchSize, metrics, logger)
	go relay.Run(ctx, cfg.OutboxInterval)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		logger.Info("tenant-service ouvindo", slog.String("addr", cfg.HTTPAddr), slog.String("version", version))
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		logger.Info("sinal recebido, encerrando com graça")
	}
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}

// newUUIDv7 gera IDs ordenáveis por tempo, o que mantém o índice da PK compacto. NewV7 só
// falha se o gerador aleatório do sistema falhar, situação em que o processo não deve seguir.
func newUUIDv7() uuid.UUID { return uuid.Must(uuid.NewV7()) }
