package app_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/adapters/memory"
	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/app"
	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/domain"
)

var (
	operator = app.Principal{Subject: "op-1", Roles: []string{app.RolePlatformAdmin}}
	now      = time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
)

func newTestService(t *testing.T) (*app.Service, *memory.Repository) {
	t.Helper()
	repo := memory.NewRepository()
	return app.NewService(repo, fakeAssets{}, 10*time.Minute, func() time.Time { return now }, uuid.New), repo
}

type fakeAssets struct{}

func (fakeAssets) PresignUpload(_ context.Context, key, _ string, _ time.Duration) (string, error) {
	return "https://bucket.example.com/" + key + "?X-Amz-Signature=fake", nil
}

func (fakeAssets) PublicURL(key string) string { return "https://assets.example.com/" + key }

func mustCreate(t *testing.T, s *app.Service, slug string, plan domain.PlanCode) domain.Tenant {
	t.Helper()
	tn, err := s.CreateTenant(context.Background(), operator, domain.NewTenantInput{Slug: slug, Name: "Tenant " + slug, Plan: plan})
	if err != nil {
		t.Fatalf("criar %s: %v", slug, err)
	}
	return tn
}

func adminOf(id uuid.UUID) app.Principal {
	return app.Principal{Subject: "adm", TenantID: id, Roles: []string{app.RoleTenantAdmin}}
}

func TestCreateTenant_WritesOutboxEvent(t *testing.T) {
	s, repo := newTestService(t)
	tn := mustCreate(t, s, "construtora-horizonte", domain.PlanGrowth)

	if len(repo.Events()) != 1 {
		t.Fatalf("esperava 1 evento na outbox, obteve %d", len(repo.Events()))
	}
	evt := repo.Events()[0]
	if evt.Type != domain.EventTenantCreated || evt.TenantID != tn.ID || evt.Version != 1 {
		t.Fatalf("evento inesperado: %+v", evt)
	}
	data := evt.Data.(domain.TenantCreatedData)
	if !slices.Contains(data.Features, domain.FeatureNineBox) {
		t.Fatalf("evento deve carregar as features do plano: %v", data.Features)
	}
}

func TestCreateTenant_Errors(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	mustCreate(t, s, "rede-sol", domain.PlanEssential)

	if _, err := s.CreateTenant(ctx, operator, domain.NewTenantInput{Slug: "rede-sol", Name: "Outra", Plan: domain.PlanEssential}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("slug duplicado deve dar conflito: %v", err)
	}
	if _, err := s.CreateTenant(ctx, adminOf(uuid.New()), domain.NewTenantInput{Slug: "x-y-z", Name: "X", Plan: domain.PlanEssential}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("admin de tenant não cria tenants: %v", err)
	}
	if _, err := s.CreateTenant(ctx, operator, domain.NewTenantInput{Slug: "!", Name: "", Plan: "nada"}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("entrada inválida: %v", err)
	}
}

func TestGetTenant_HidesOtherTenants(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	a := mustCreate(t, s, "construtora-horizonte", domain.PlanGrowth)
	b := mustCreate(t, s, "rede-sol", domain.PlanGrowth)

	if _, err := s.GetTenant(ctx, adminOf(a.ID), a.ID); err != nil {
		t.Fatalf("admin lê o próprio tenant: %v", err)
	}
	if _, err := s.GetTenant(ctx, adminOf(a.ID), b.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("admin de A não pode ver B e deve receber 404: %v", err)
	}
}

func TestUpdateTenant_PlanChangeEmitsEvent(t *testing.T) {
	s, repo := newTestService(t)
	ctx := context.Background()
	tn := mustCreate(t, s, "rede-sol", domain.PlanEssential)

	name := "Rede Varejo Sol Exemplo"
	if _, err := s.UpdateTenant(ctx, operator, tn.ID, domain.TenantPatch{Name: &name}); err != nil {
		t.Fatal(err)
	}
	if len(repo.Events()) != 1 {
		t.Fatalf("renomear não gera evento; outbox=%d", len(repo.Events()))
	}

	plan := domain.PlanEnterprise
	if _, err := s.UpdateTenant(ctx, operator, tn.ID, domain.TenantPatch{Plan: &plan}); err != nil {
		t.Fatal(err)
	}
	last := repo.Events()[len(repo.Events())-1]
	if last.Type != domain.EventTenantPlanChanged {
		t.Fatalf("mudança de plano deve emitir TenantPlanoAlterado, obteve %s", last.Type)
	}
	if d := last.Data.(domain.TenantPlanChangedData); d.PreviousPlan != domain.PlanEssential || d.Plan != domain.PlanEnterprise {
		t.Fatalf("payload inesperado: %+v", d)
	}
}

func TestDeactivateTenant(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	tn := mustCreate(t, s, "rede-sol", domain.PlanEssential)
	if err := s.DeactivateTenant(ctx, operator, tn.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetTenant(ctx, operator, tn.ID)
	if got.Status != domain.StatusDeactivated {
		t.Fatalf("status = %s", got.Status)
	}
	if err := s.DeactivateTenant(ctx, operator, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("tenant inexistente: %v", err)
	}
}

func TestFeatureOverrides(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	tn := mustCreate(t, s, "construtora-horizonte", domain.PlanGrowth)

	view, err := s.SetFeatureOverride(ctx, operator, tn.ID, domain.FeatureAnalytics, true)
	if err != nil || !slices.Contains(view.Effective, domain.FeatureAnalytics) {
		t.Fatalf("operador liga adicional: %v %v", view, err)
	}
	if _, err := s.SetFeatureOverride(ctx, operator, tn.ID, domain.FeatureLearning, true); !errors.Is(err, domain.ErrFeatureLocked) {
		t.Fatalf("learning não é adicionável no crescimento: %v", err)
	}
	if _, err := s.SetFeatureOverride(ctx, adminOf(tn.ID), tn.ID, domain.FeatureAIAssist, true); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("admin do tenant não liga adicionais: %v", err)
	}
	view, err = s.SetFeatureOverride(ctx, adminOf(tn.ID), tn.ID, domain.FeatureRecruitment, false)
	if err != nil || slices.Contains(view.Effective, domain.FeatureRecruitment) {
		t.Fatalf("admin desliga feature do pacote: %v %v", view, err)
	}
	if _, err := s.SetFeatureOverride(ctx, adminOf(uuid.New()), tn.ID, domain.FeatureRecruitment, false); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("admin de outro tenant: %v", err)
	}
}

func TestTheme_DefaultThenCustom(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	tn := mustCreate(t, s, "construtora-horizonte", domain.PlanGrowth)

	theme, err := s.Theme(ctx, adminOf(tn.ID), tn.ID)
	if err != nil || theme != domain.DefaultTheme() {
		t.Fatalf("sem tema próprio deve vir o padrão: %v", err)
	}
	in := domain.ThemeInput{Primary: "#8a2d00", OnPrimary: "#ffffff", Background: "#ffffff", Text: "#202020", Accent: "#005a9c", EmailFromName: "RH Horizonte"}
	if _, err := s.SetTheme(ctx, adminOf(tn.ID), tn.ID, in); err != nil {
		t.Fatalf("tema válido: %v", err)
	}
	in.OnPrimary = "#a04020"
	if _, err := s.SetTheme(ctx, adminOf(tn.ID), tn.ID, in); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("contraste baixo deve ser rejeitado: %v", err)
	}
	if _, err := s.SetTheme(ctx, adminOf(uuid.New()), tn.ID, in); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("admin de outro tenant: %v", err)
	}
}

func TestDomainsAndResolution(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	enterprise := mustCreate(t, s, "construtora-horizonte", domain.PlanEnterprise)
	essential := mustCreate(t, s, "rede-sol", domain.PlanEssential)

	host, err := s.AddDomain(ctx, adminOf(enterprise.ID), enterprise.ID, "RH.Horizonte.Example.com")
	if err != nil || host != "rh.horizonte.example.com" {
		t.Fatalf("adicionar domínio: %q %v", host, err)
	}
	if _, err := s.AddDomain(ctx, operator, essential.ID, "pessoas.sol.example.com"); !errors.Is(err, domain.ErrFeatureLocked) {
		t.Fatalf("plano essencial não tem domínio próprio: %v", err)
	}
	if _, err := s.AddDomain(ctx, operator, enterprise.ID, "rh.horizonte.example.com"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("domínio duplicado: %v", err)
	}

	b, err := s.ResolveByHost(ctx, "rh.horizonte.example.com:443")
	if err != nil || b.TenantID != enterprise.ID || b.Slug != "construtora-horizonte" {
		t.Fatalf("resolução por host: %+v %v", b, err)
	}
	if _, err := s.ResolveByHost(ctx, "desconhecido.example.com"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("host desconhecido: %v", err)
	}

	b, err = s.ResolveByClaim(ctx, adminOf(essential.ID))
	if err != nil || b.TenantID != essential.ID {
		t.Fatalf("resolução por claim: %+v %v", b, err)
	}
	if _, err := s.ResolveByClaim(ctx, operator); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("operador não tem tenant na claim: %v", err)
	}

	suspended := domain.StatusSuspended
	if _, err := s.UpdateTenant(ctx, operator, enterprise.ID, domain.TenantPatch{Status: &suspended}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ResolveByHost(ctx, "rh.horizonte.example.com"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("tenant suspenso não deve ser resolvido: %v", err)
	}
}

func TestCreateLogoUpload(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	tn := mustCreate(t, s, "construtora-horizonte", domain.PlanGrowth)

	up, err := s.CreateLogoUpload(ctx, adminOf(tn.ID), tn.ID, "image/svg+xml")
	if err != nil {
		t.Fatal(err)
	}
	prefix := "https://assets.example.com/tenants/" + tn.ID.String() + "/logo/"
	if !strings.HasPrefix(up.PublicURL, prefix) || !strings.HasSuffix(up.PublicURL, ".svg") {
		t.Fatalf("chave do logo deve ficar sob o prefixo do tenant: %s", up.PublicURL)
	}
	if !up.ExpiresAt.Equal(now.Add(10 * time.Minute)) {
		t.Fatalf("expiração inesperada: %s", up.ExpiresAt)
	}
	if _, err := s.CreateLogoUpload(ctx, adminOf(tn.ID), tn.ID, "application/x-msdownload"); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("tipo não permitido: %v", err)
	}
	if _, err := s.CreateLogoUpload(ctx, adminOf(uuid.New()), tn.ID, "image/png"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("admin de outro tenant: %v", err)
	}
}
