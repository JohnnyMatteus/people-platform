package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/domain"
)

// inTenant executa fn numa transação cujo escopo de RLS é um único tenant. set_config com
// is_local=true equivale a SET LOCAL: o escopo morre com a transação e nunca vaza para a
// próxima requisição que reutilizar a conexão do pool.
func inTenant(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, fn func(pgx.Tx) error) error {
	if tenantID == uuid.Nil {
		return fmt.Errorf("escopo de tenant vazio: %w", domain.ErrInvalidInput)
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID.String()); err != nil {
			return fmt.Errorf("definir escopo de tenant: %w", err)
		}
		return fn(tx)
	})
}

// asPlatform executa fn com visão de todos os tenants. Usado só por operações de plataforma:
// listar tenants, resolver domínio e o relay da outbox.
func asPlatform(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.platform', 'on', true)"); err != nil {
			return fmt.Errorf("definir escopo de plataforma: %w", err)
		}
		return fn(tx)
	})
}

// translate converte erros do driver em erros de domínio.
func translate(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return fmt.Errorf("%w: %s", domain.ErrConflict, pgErr.ConstraintName)
		case "23503": // foreign_key_violation: o tenant não existe ou não é visível no escopo
			return domain.ErrNotFound
		}
	}
	return err
}
