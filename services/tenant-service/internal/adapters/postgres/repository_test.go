package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/domain"
)

func newTenant(t *testing.T, slug string, plan domain.PlanCode) domain.Tenant {
	t.Helper()
	tn, err := domain.NewTenant(uuid.New(), domain.NewTenantInput{Slug: slug, Name: "Tenant " + slug, Plan: plan},
		time.Now().UTC().Truncate(time.Microsecond))
	if err != nil {
		t.Fatal(err)
	}
	return tn
}

func createTenant(t *testing.T, repo *Repository, slug string, plan domain.PlanCode) domain.Tenant {
	t.Helper()
	tn := newTenant(t, slug, plan)
	if err := repo.CreateTenant(context.Background(), tn, domain.NewTenantCreatedEvent(uuid.New(), tn)); err != nil {
		t.Fatalf("criar tenant %s: %v", slug, err)
	}
	return tn
}

func uniqueSlug(prefix string) string { return prefix + "-" + uuid.NewString()[:8] }

func TestRepository_TenantCRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository(appPool)
	tn := createTenant(t, repo, uniqueSlug("horizonte"), domain.PlanGrowth)

	got, err := repo.GetTenant(ctx, tn.ID)
	if err != nil || got.Slug != tn.Slug || got.Plan != domain.PlanGrowth || !got.CreatedAt.Equal(tn.CreatedAt) {
		t.Fatalf("GetTenant = %+v, %v", got, err)
	}

	dup := newTenant(t, tn.Slug, domain.PlanGrowth)
	if err := repo.CreateTenant(ctx, dup, domain.NewTenantCreatedEvent(uuid.New(), dup)); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("slug duplicado deve ser conflito: %v", err)
	}

	plan := domain.PlanEnterprise
	next, _, _ := got.Apply(domain.TenantPatch{Plan: &plan}, time.Now())
	if err := repo.UpdateTenant(ctx, next, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.GetTenant(ctx, tn.ID); got.Plan != domain.PlanEnterprise {
		t.Fatalf("plano não atualizado: %s", got.Plan)
	}

	if _, err := repo.GetTenant(ctx, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("tenant inexistente: %v", err)
	}
	ghost := newTenant(t, uniqueSlug("fantasma"), domain.PlanGrowth)
	if err := repo.UpdateTenant(ctx, ghost, nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("atualizar inexistente: %v", err)
	}

	all, err := repo.ListTenants(ctx)
	if err != nil || len(all) == 0 {
		t.Fatalf("ListTenants: %v %v", all, err)
	}
}

func TestRepository_CreateWritesOutboxAtomically(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository(appPool)
	tn := createTenant(t, repo, uniqueSlug("sol"), domain.PlanEssential)

	var eventType string
	var version int
	err := ownerPool.QueryRow(ctx, `SELECT event_type, event_version FROM outbox WHERE tenant_id = $1`, tn.ID).Scan(&eventType, &version)
	if err != nil || eventType != domain.EventTenantCreated || version != 1 {
		t.Fatalf("outbox: %s v%d, %v", eventType, version, err)
	}

	// Falha no meio da transação (slug duplicado) não pode deixar evento órfão na outbox.
	dup := newTenant(t, tn.Slug, domain.PlanEssential)
	_ = repo.CreateTenant(ctx, dup, domain.NewTenantCreatedEvent(uuid.New(), dup))
	var orphans int
	if err := ownerPool.QueryRow(ctx, `SELECT count(*) FROM outbox WHERE tenant_id = $1`, dup.ID).Scan(&orphans); err != nil || orphans != 0 {
		t.Fatalf("evento órfão na outbox: %d %v", orphans, err)
	}
}

func TestRepository_ThemeOverridesAndDomains(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository(appPool)
	tn := createTenant(t, repo, uniqueSlug("horizonte"), domain.PlanEnterprise)

	if _, ok, err := repo.GetTheme(ctx, tn.ID); ok || err != nil {
		t.Fatalf("tenant novo não tem tema: %v %v", ok, err)
	}
	theme, err := domain.NewTheme(domain.ThemeInput{Primary: "#1f4e79", OnPrimary: "#ffffff", Background: "#ffffff",
		Text: "#1a1a1a", Accent: "#0b6e4f", EmailFromName: "RH Horizonte", LogoURL: "https://cdn.example.com/l.svg"})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveTheme(ctx, tn.ID, theme); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveTheme(ctx, tn.ID, theme); err != nil {
		t.Fatalf("upsert do tema: %v", err)
	}
	got, ok, err := repo.GetTheme(ctx, tn.ID)
	if err != nil || !ok || got != theme {
		t.Fatalf("tema lido difere: %+v %v %v", got, ok, err)
	}

	if err := repo.SaveOverride(ctx, tn.ID, domain.FeatureLearning, true); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveOverride(ctx, tn.ID, domain.FeatureLearning, false); err != nil {
		t.Fatal(err)
	}
	if o, _ := repo.GetOverrides(ctx, tn.ID); o[domain.FeatureLearning] {
		t.Fatalf("override deveria estar desligado: %v", o)
	}

	host := uniqueSlug("rh") + ".horizonte.example.com"
	if err := repo.AddDomain(ctx, tn.ID, host); err != nil {
		t.Fatal(err)
	}
	if id, err := repo.ResolveHost(ctx, host); err != nil || id != tn.ID {
		t.Fatalf("ResolveHost = %s %v", id, err)
	}
	if _, err := repo.ResolveHost(ctx, "ninguem.example.com"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("host desconhecido: %v", err)
	}
	if hosts, _ := repo.ListDomains(ctx, tn.ID); len(hosts) != 1 || hosts[0] != host {
		t.Fatalf("ListDomains = %v", hosts)
	}
	if err := repo.SaveTheme(ctx, uuid.New(), theme); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("tema para tenant inexistente: %v", err)
	}
}

// TestRLS_CrossTenantIsolation é o teste de vazamento entre tenants: mesmo uma consulta sem
// WHERE (o bug clássico) só enxerga as linhas do tenant do escopo.
func TestRLS_CrossTenantIsolation(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository(appPool)
	a := createTenant(t, repo, uniqueSlug("horizonte"), domain.PlanEnterprise)
	b := createTenant(t, repo, uniqueSlug("sol"), domain.PlanEnterprise)
	hostB := uniqueSlug("pessoas") + ".sol.example.com"
	for _, tn := range []domain.Tenant{a, b} {
		if err := repo.SaveOverride(ctx, tn.ID, domain.FeatureLearning, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.AddDomain(ctx, b.ID, hostB); err != nil {
		t.Fatal(err)
	}

	tables := map[string]string{
		"tenants":                  "SELECT DISTINCT id FROM tenants",
		"tenant_feature_overrides": "SELECT DISTINCT tenant_id FROM tenant_feature_overrides",
		"tenant_domains":           "SELECT DISTINCT tenant_id FROM tenant_domains",
		"outbox":                   "SELECT DISTINCT tenant_id FROM outbox",
	}
	for table, query := range tables {
		t.Run("consulta sem WHERE em "+table+" só vê o próprio tenant", func(t *testing.T) {
			err := inTenant(ctx, appPool, a.ID, func(tx pgx.Tx) error {
				ids, err := collectIDs(ctx, tx, query)
				if err != nil {
					return err
				}
				for _, id := range ids {
					if id != a.ID {
						t.Errorf("%s vazou linha do tenant %s no escopo de %s", table, id, a.ID)
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}

	t.Run("sem escopo nenhuma linha é visível", func(t *testing.T) {
		var n int
		if err := appPool.QueryRow(ctx, "SELECT count(*) FROM tenants").Scan(&n); err != nil || n != 0 {
			t.Fatalf("sem escopo deveria ver 0 tenants, viu %d (%v)", n, err)
		}
	})

	t.Run("não é possível gravar linha de outro tenant", func(t *testing.T) {
		err := inTenant(ctx, appPool, a.ID, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `INSERT INTO tenant_feature_overrides (tenant_id, feature, enabled) VALUES ($1, 'okr', true)`, b.ID)
			return err
		})
		if err == nil {
			t.Fatal("WITH CHECK deveria bloquear a escrita no tenant B a partir do escopo de A")
		}
	})

	t.Run("não é possível alterar linha de outro tenant", func(t *testing.T) {
		err := inTenant(ctx, appPool, a.ID, func(tx pgx.Tx) error {
			tag, err := tx.Exec(ctx, `UPDATE tenants SET name = 'invadido' WHERE id = $1`, b.ID)
			if err == nil && tag.RowsAffected() != 0 {
				t.Errorf("UPDATE afetou %d linha(s) do tenant B", tag.RowsAffected())
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("escopo não vaza para a próxima transação da mesma conexão", func(t *testing.T) {
		conn, err := appPool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Release()
		err = pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", a.ID.String())
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		var n int
		if err := conn.QueryRow(ctx, "SELECT count(*) FROM tenants").Scan(&n); err != nil || n != 0 {
			t.Fatalf("escopo sobreviveu à transação: viu %d tenants (%v)", n, err)
		}
	})

	t.Run("repositório no escopo de A não lê dados de B", func(t *testing.T) {
		domainsOfA, err := repo.ListDomains(ctx, a.ID)
		if err != nil || len(domainsOfA) != 0 {
			t.Fatalf("A não tem domínios, obteve %v (%v)", domainsOfA, err)
		}
	})
}

func collectIDs(ctx context.Context, tx pgx.Tx, query string) ([]uuid.UUID, error) {
	rows, err := tx.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}
