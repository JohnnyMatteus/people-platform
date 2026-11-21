// Package postgres implementa as portas de persistência do tenant-service com PostgreSQL,
// Row-Level Security por tenant e transactional outbox.
package postgres

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// migrationLockKey é a chave do advisory lock que serializa migrações concorrentes.
const migrationLockKey = 0x7e4a47

// Migrate aplica, em ordem, as migrações ainda não aplicadas. Deve rodar com o usuário
// dono do esquema, nunca com o usuário da aplicação (que está sujeito a RLS).
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("adquirir conexão: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockKey); err != nil {
		return fmt.Errorf("lock de migração: %w", err)
	}
	defer func() {
		_, _ = conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", migrationLockKey)
	}()

	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("criar schema_migrations: %w", err)
	}

	names, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		if err := applyOne(ctx, conn.Conn(), name); err != nil {
			return err
		}
	}
	return nil
}

func applyOne(ctx context.Context, conn *pgx.Conn, name string) error {
	var applied bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", name).Scan(&applied); err != nil {
		return fmt.Errorf("consultar migração %s: %w", name, err)
	}
	if applied {
		return nil
	}
	sql, err := migrationFiles.ReadFile(name)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("aplicar %s: %w", name, err)
		}
		_, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", name)
		return err
	})
}
