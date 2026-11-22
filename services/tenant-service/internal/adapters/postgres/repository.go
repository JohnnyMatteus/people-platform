package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/propagation"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/app"
	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/domain"
)

// Repository implementa app.Repository.
type Repository struct {
	pool *pgxpool.Pool
}

var _ app.Repository = (*Repository)(nil)

// NewRepository cria o repositório. O pool deve usar o usuário da aplicação (sem BYPASSRLS).
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const tenantColumns = "id, slug, name, plan, status, created_at, updated_at"

func scanTenant(row pgx.Row) (domain.Tenant, error) {
	var t domain.Tenant
	var plan, status string
	if err := row.Scan(&t.ID, &t.Slug, &t.Name, &plan, &status, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return domain.Tenant{}, translate(err)
	}
	t.Plan, t.Status = domain.PlanCode(plan), domain.Status(status)
	return t, nil
}

// CreateTenant roda no escopo do próprio tenant novo: a política WITH CHECK aceita a linha
// porque id = app.tenant_id. Não é preciso escopo de plataforma para criar.
func (r *Repository) CreateTenant(ctx context.Context, t domain.Tenant, evt domain.Event) error {
	return translate(inTenant(ctx, r.pool, t.ID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO tenants (`+tenantColumns+`) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			t.ID, t.Slug, t.Name, string(t.Plan), string(t.Status), t.CreatedAt, t.UpdatedAt)
		if err != nil {
			return err
		}
		return insertOutbox(ctx, tx, evt)
	}))
}

// ListTenants é uma operação de plataforma.
func (r *Repository) ListTenants(ctx context.Context) ([]domain.Tenant, error) {
	var out []domain.Tenant
	err := asPlatform(ctx, r.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+tenantColumns+` FROM tenants ORDER BY slug`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			t, err := scanTenant(rows)
			if err != nil {
				return err
			}
			out = append(out, t)
		}
		return rows.Err()
	})
	return out, translate(err)
}

// GetTenant busca no escopo do tenant.
func (r *Repository) GetTenant(ctx context.Context, tenantID uuid.UUID) (domain.Tenant, error) {
	var t domain.Tenant
	err := inTenant(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		var err error
		t, err = scanTenant(tx.QueryRow(ctx, `SELECT `+tenantColumns+` FROM tenants WHERE id = $1`, tenantID))
		return err
	})
	return t, translate(err)
}

// UpdateTenant grava a alteração e o evento opcional na mesma transação.
func (r *Repository) UpdateTenant(ctx context.Context, t domain.Tenant, evt *domain.Event) error {
	return translate(inTenant(ctx, r.pool, t.ID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE tenants SET name=$2, plan=$3, status=$4, updated_at=$5 WHERE id=$1`,
			t.ID, t.Name, string(t.Plan), string(t.Status), t.UpdatedAt)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		if evt == nil {
			return nil
		}
		return insertOutbox(ctx, tx, *evt)
	}))
}

// RequestErasure grava o pedido e o evento na mesma transação, no escopo do tenant: se a outbox falha, o pedido não existe.
func (r *Repository) RequestErasure(ctx context.Context, req domain.ErasureRequest, evt domain.Event) error {
	return translate(inTenant(ctx, r.pool, req.TenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO erasure_requests (id, tenant_id, subject_id, requested_by, requested_at)
				VALUES ($1,$2,$3,$4,$5)`, req.ID, req.TenantID, req.SubjectID, req.RequestedBy, req.RequestedAt); err != nil {
			return err
		}
		return insertOutbox(ctx, tx, evt)
	}))
}

// GetTheme devolve o tema gravado; ok=false quando o tenant ainda não definiu um.
func (r *Repository) GetTheme(ctx context.Context, tenantID uuid.UUID) (domain.Theme, bool, error) {
	var theme domain.Theme
	found := false
	err := inTenant(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		var in domain.ThemeInput
		var logo, replyTo *string
		err := tx.QueryRow(ctx, `SELECT primary_color, on_primary_color, background_color, text_color,
			accent_color, logo_url, email_from_name, email_reply_to FROM tenant_themes WHERE tenant_id = $1`, tenantID).
			Scan(&in.Primary, &in.OnPrimary, &in.Background, &in.Text, &in.Accent, &logo, &in.EmailFromName, &replyTo)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		in.LogoURL, in.EmailReplyTo = deref(logo), deref(replyTo)
		// O tema foi validado ao gravar; revalidar na leitura protege contra edição manual no banco.
		theme, err = domain.NewTheme(in)
		if err != nil {
			return fmt.Errorf("tema gravado inválido para o tenant %s: %w", tenantID, err)
		}
		found = true
		return nil
	})
	return theme, found, translate(err)
}

// SaveTheme faz upsert do tema.
func (r *Repository) SaveTheme(ctx context.Context, tenantID uuid.UUID, t domain.Theme) error {
	return translate(inTenant(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO tenant_themes (tenant_id, primary_color, on_primary_color, background_color,
				text_color, accent_color, logo_url, email_from_name, email_reply_to, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9, now())
			ON CONFLICT (tenant_id) DO UPDATE SET primary_color=EXCLUDED.primary_color,
				on_primary_color=EXCLUDED.on_primary_color, background_color=EXCLUDED.background_color,
				text_color=EXCLUDED.text_color, accent_color=EXCLUDED.accent_color, logo_url=EXCLUDED.logo_url,
				email_from_name=EXCLUDED.email_from_name, email_reply_to=EXCLUDED.email_reply_to, updated_at=now()`,
			tenantID, t.Primary.Hex(), t.OnPrimary.Hex(), t.Background.Hex(), t.Text.Hex(), t.Accent.Hex(),
			nullable(t.LogoURL), t.EmailFromName, nullable(t.EmailReplyTo))
		return err
	}))
}

// GetOverrides devolve os overrides de feature do tenant.
func (r *Repository) GetOverrides(ctx context.Context, tenantID uuid.UUID) (map[domain.Feature]bool, error) {
	out := map[domain.Feature]bool{}
	err := inTenant(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT feature, enabled FROM tenant_feature_overrides WHERE tenant_id = $1`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var f string
			var enabled bool
			if err := rows.Scan(&f, &enabled); err != nil {
				return err
			}
			out[domain.Feature(f)] = enabled
		}
		return rows.Err()
	})
	return out, translate(err)
}

// SaveOverride faz upsert de um override.
func (r *Repository) SaveOverride(ctx context.Context, tenantID uuid.UUID, f domain.Feature, enabled bool) error {
	return translate(inTenant(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO tenant_feature_overrides (tenant_id, feature, enabled) VALUES ($1,$2,$3)
			ON CONFLICT (tenant_id, feature) DO UPDATE SET enabled = EXCLUDED.enabled, updated_at = now()`,
			tenantID, string(f), enabled)
		return err
	}))
}

// AddDomain associa um domínio ao tenant. Domínio de outro tenant gera ErrConflict pela PK,
// sem revelar a quem ele pertence.
func (r *Repository) AddDomain(ctx context.Context, tenantID uuid.UUID, host string) error {
	return translate(inTenant(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO tenant_domains (host, tenant_id) VALUES ($1, $2)`, host, tenantID)
		return err
	}))
}

// ListDomains lista os domínios do tenant.
func (r *Repository) ListDomains(ctx context.Context, tenantID uuid.UUID) ([]string, error) {
	out := []string{}
	err := inTenant(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT host FROM tenant_domains WHERE tenant_id = $1 ORDER BY host`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var h string
			if err := rows.Scan(&h); err != nil {
				return err
			}
			out = append(out, h)
		}
		return rows.Err()
	})
	return out, translate(err)
}

// ResolveHost é uma operação de plataforma: o host chega antes de sabermos o tenant.
func (r *Repository) ResolveHost(ctx context.Context, host string) (uuid.UUID, error) {
	var id uuid.UUID
	err := asPlatform(ctx, r.pool, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT tenant_id FROM tenant_domains WHERE host = $1`, host).Scan(&id)
	})
	return id, translate(err)
}

func insertOutbox(ctx context.Context, tx pgx.Tx, evt domain.Event) error {
	payload, err := json.Marshal(evt.Data)
	if err != nil {
		return fmt.Errorf("serializar evento %s: %w", evt.Type, err)
	}
	carrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(ctx, carrier)
	_, err = tx.Exec(ctx, `INSERT INTO outbox (id, tenant_id, aggregate_type, aggregate_id, event_type, event_version,
			payload, traceparent, occurred_at) VALUES ($1,$2,'tenant',$3,$4,$5,$6,$7,$8)`,
		evt.ID, evt.TenantID, evt.AggregateID, evt.Type, evt.Version, payload, nullable(carrier.Get("traceparent")), evt.OccurredAt)
	return err
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
