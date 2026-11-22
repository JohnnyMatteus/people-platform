package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/app"
	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/domain"
)

// Store implementa app.Store: cada unidade de trabalho é uma transação com o escopo de RLS de um único tenant.
type Store struct{ pool *pgxpool.Pool }

var _ app.Store = (*Store)(nil)

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// InTenant define app.tenant_id com set_config(is_local=true), o equivalente a SET LOCAL: o escopo morre com a
// transação e nunca vaza para a próxima requisição que reutilizar a conexão do pool.
func (s *Store) InTenant(ctx context.Context, tenantID uuid.UUID, fn func(app.Tx) error) error {
	if tenantID == uuid.Nil {
		return fmt.Errorf("escopo de tenant vazio: %w", domain.ErrInvalidInput)
	}
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID.String()); err != nil {
			return fmt.Errorf("definir escopo de tenant: %w", err)
		}
		return fn(&txn{tx: tx, tenant: tenantID})
	})
	return translate(err)
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
		case "23505":
			return fmt.Errorf("%w: %s", domain.ErrConflict, pgErr.ConstraintName)
		case "23503", "42501":
			// FK para linha de outro tenant (invisível pela RLS) ou WITH CHECK violado: tratado como inexistente.
			return domain.ErrNotFound
		}
	}
	return err
}

type txn struct {
	tx     pgx.Tx
	tenant uuid.UUID
}

func (t *txn) Tracks() app.TrackRepository                   { return trackRepo{t} }
func (t *txn) Enrollments() app.EnrollmentRepository         { return enrollmentRepo{t} }
func (t *txn) Onboarding() app.OnboardingRepository          { return onboardingRepo{t} }
func (t *txn) Settings() app.SettingsRepository              { return settingsRepo{t} }
func (t *txn) ProcessedEvents() app.ProcessedEventRepository { return processedRepo{t} }
func (t *txn) Privacy() app.PrivacyRepository                { return privacyRepo{t} }
