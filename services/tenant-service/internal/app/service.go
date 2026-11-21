package app

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/domain"
)

// Service implementa os casos de uso do tenant-service.
type Service struct {
	repo      Repository
	assets    AssetStorage
	uploadTTL time.Duration
	now       Clock
	newID     IDGenerator
}

// NewService cria o serviço com as dependências explícitas.
func NewService(repo Repository, assets AssetStorage, uploadTTL time.Duration, now Clock, newID IDGenerator) *Service {
	return &Service{repo: repo, assets: assets, uploadTTL: uploadTTL, now: now, newID: newID}
}

// CreateTenant cria um tenant e publica TenantCriado via outbox. Só o operador da plataforma.
func (s *Service) CreateTenant(ctx context.Context, p Principal, in domain.NewTenantInput) (domain.Tenant, error) {
	if !p.IsPlatformAdmin() {
		return domain.Tenant{}, domain.ErrForbidden
	}
	t, err := domain.NewTenant(s.newID(), in, s.now())
	if err != nil {
		return domain.Tenant{}, err
	}
	if err := s.repo.CreateTenant(ctx, t, domain.NewTenantCreatedEvent(s.newID(), t)); err != nil {
		return domain.Tenant{}, fmt.Errorf("criar tenant: %w", err)
	}
	return t, nil
}

// ListTenants lista todos os tenants. Só o operador da plataforma.
func (s *Service) ListTenants(ctx context.Context, p Principal) ([]domain.Tenant, error) {
	if !p.IsPlatformAdmin() {
		return nil, domain.ErrForbidden
	}
	return s.repo.ListTenants(ctx)
}

// GetTenant devolve o tenant se o principal puder lê-lo. Para quem não pode, responde
// ErrNotFound em vez de ErrForbidden, para não revelar quais IDs existem.
func (s *Service) GetTenant(ctx context.Context, p Principal, id uuid.UUID) (domain.Tenant, error) {
	if !p.CanRead(id) {
		return domain.Tenant{}, domain.ErrNotFound
	}
	return s.repo.GetTenant(ctx, id)
}

// UpdateTenant altera nome, plano ou status. Mudança de plano publica TenantPlanoAlterado.
func (s *Service) UpdateTenant(ctx context.Context, p Principal, id uuid.UUID, patch domain.TenantPatch) (domain.Tenant, error) {
	if !p.IsPlatformAdmin() {
		return domain.Tenant{}, domain.ErrForbidden
	}
	current, err := s.repo.GetTenant(ctx, id)
	if err != nil {
		return domain.Tenant{}, err
	}
	next, planChanged, err := current.Apply(patch, s.now())
	if err != nil {
		return domain.Tenant{}, err
	}
	var evt *domain.Event
	if planChanged {
		overrides, err := s.repo.GetOverrides(ctx, id)
		if err != nil {
			return domain.Tenant{}, err
		}
		features := domain.EffectiveFeatures(domain.Catalog[next.Plan], overrides)
		e := domain.NewTenantPlanChangedEvent(s.newID(), current, next, features)
		evt = &e
	}
	if err := s.repo.UpdateTenant(ctx, next, evt); err != nil {
		return domain.Tenant{}, fmt.Errorf("atualizar tenant: %w", err)
	}
	return next, nil
}

// DeactivateTenant é o "delete" do CRUD: desativa sem apagar, porque dados de RH têm prazo
// legal de retenção. A exclusão física segue o fluxo de LGPD descrito no design doc.
func (s *Service) DeactivateTenant(ctx context.Context, p Principal, id uuid.UUID) error {
	status := domain.StatusDeactivated
	_, err := s.UpdateTenant(ctx, p, id, domain.TenantPatch{Status: &status})
	return err
}

// FeatureView é a visão das features efetivas de um tenant.
type FeatureView struct {
	Plan      domain.PlanCode
	Effective []domain.Feature
	Overrides map[domain.Feature]bool
}

// Features devolve as features efetivas (plano + overrides).
func (s *Service) Features(ctx context.Context, p Principal, id uuid.UUID) (FeatureView, error) {
	t, err := s.GetTenant(ctx, p, id)
	if err != nil {
		return FeatureView{}, err
	}
	overrides, err := s.repo.GetOverrides(ctx, id)
	if err != nil {
		return FeatureView{}, err
	}
	return FeatureView{Plan: t.Plan, Effective: domain.EffectiveFeatures(domain.Catalog[t.Plan], overrides), Overrides: overrides}, nil
}

// SetFeatureOverride liga ou desliga uma feature, dentro do que o plano permite.
// Ligar adicionais é decisão comercial: só o operador da plataforma. O admin do tenant
// pode apenas desligar features do próprio pacote.
func (s *Service) SetFeatureOverride(ctx context.Context, p Principal, id uuid.UUID, f domain.Feature, enabled bool) (FeatureView, error) {
	if !p.CanManage(id) {
		return FeatureView{}, domain.ErrForbidden
	}
	if enabled && !p.IsPlatformAdmin() {
		return FeatureView{}, domain.ErrForbidden
	}
	t, err := s.repo.GetTenant(ctx, id)
	if err != nil {
		return FeatureView{}, err
	}
	if err := domain.Catalog[t.Plan].ValidateOverride(f, enabled); err != nil {
		return FeatureView{}, err
	}
	if err := s.repo.SaveOverride(ctx, id, f, enabled); err != nil {
		return FeatureView{}, err
	}
	return s.Features(ctx, p, id)
}

// Theme devolve o tema do tenant, ou o tema padrão se ele ainda não definiu um.
func (s *Service) Theme(ctx context.Context, p Principal, id uuid.UUID) (domain.Theme, error) {
	if !p.CanRead(id) {
		return domain.Theme{}, domain.ErrNotFound
	}
	return s.themeOrDefault(ctx, id)
}

// SetTheme valida (inclusive contraste WCAG AA) e grava o tema white label.
func (s *Service) SetTheme(ctx context.Context, p Principal, id uuid.UUID, in domain.ThemeInput) (domain.Theme, error) {
	if !p.CanManage(id) {
		return domain.Theme{}, domain.ErrForbidden
	}
	theme, err := domain.NewTheme(in)
	if err != nil {
		return domain.Theme{}, err
	}
	if _, err := s.repo.GetTenant(ctx, id); err != nil {
		return domain.Theme{}, err
	}
	if err := s.repo.SaveTheme(ctx, id, theme); err != nil {
		return domain.Theme{}, err
	}
	return theme, nil
}

// AddDomain associa um domínio próprio ao tenant, se o plano tiver custom_domain.
func (s *Service) AddDomain(ctx context.Context, p Principal, id uuid.UUID, rawHost string) (string, error) {
	if !p.CanManage(id) {
		return "", domain.ErrForbidden
	}
	host, err := domain.NormalizeHost(rawHost)
	if err != nil {
		return "", err
	}
	view, err := s.Features(ctx, p, id)
	if err != nil {
		return "", err
	}
	if !slices.Contains(view.Effective, domain.FeatureCustomDomain) {
		return "", domain.ErrFeatureLocked
	}
	if err := s.repo.AddDomain(ctx, id, host); err != nil {
		return "", err
	}
	return host, nil
}

// ListDomains lista os domínios próprios do tenant.
func (s *Service) ListDomains(ctx context.Context, p Principal, id uuid.UUID) ([]string, error) {
	if !p.CanRead(id) {
		return nil, domain.ErrNotFound
	}
	return s.repo.ListDomains(ctx, id)
}

// Branding é a visão pública de um tenant, servida antes do login para pintar a tela.
// Não contém dados sensíveis: só identidade visual e o slug para o realm/organização.
type Branding struct {
	TenantID uuid.UUID
	Slug     string
	Name     string
	Theme    domain.Theme
}

// ResolveByHost descobre o tenant pelo domínio próprio (cabeçalho Host). Tenants que não
// estão ativos não são resolvidos, para não servir marca de cliente suspenso.
func (s *Service) ResolveByHost(ctx context.Context, hostHeader string) (Branding, error) {
	host := domain.HostFromRequest(hostHeader)
	if host == "" {
		return Branding{}, domain.ErrNotFound
	}
	id, err := s.repo.ResolveHost(ctx, host)
	if err != nil {
		return Branding{}, err
	}
	return s.branding(ctx, id)
}

// ResolveByClaim descobre o tenant pela claim tenant_id do token autenticado.
func (s *Service) ResolveByClaim(ctx context.Context, p Principal) (Branding, error) {
	if p.TenantID == uuid.Nil {
		return Branding{}, domain.ErrNotFound
	}
	return s.branding(ctx, p.TenantID)
}

func (s *Service) branding(ctx context.Context, id uuid.UUID) (Branding, error) {
	t, err := s.repo.GetTenant(ctx, id)
	if err != nil {
		return Branding{}, err
	}
	if t.Status != domain.StatusActive {
		return Branding{}, domain.ErrNotFound
	}
	theme, err := s.themeOrDefault(ctx, id)
	if err != nil {
		return Branding{}, err
	}
	return Branding{TenantID: t.ID, Slug: t.Slug, Name: t.Name, Theme: theme}, nil
}

func (s *Service) themeOrDefault(ctx context.Context, id uuid.UUID) (domain.Theme, error) {
	theme, ok, err := s.repo.GetTheme(ctx, id)
	if err != nil {
		return domain.Theme{}, err
	}
	if !ok {
		return domain.DefaultTheme(), nil
	}
	return theme, nil
}

// logoContentTypes são os formatos aceitos para logo, com a extensão do objeto no bucket.
var logoContentTypes = map[string]string{
	"image/png":     "png",
	"image/svg+xml": "svg",
	"image/webp":    "webp",
}

// LogoUpload é a autorização de upload de um logo.
type LogoUpload struct {
	UploadURL string
	PublicURL string
	ExpiresAt time.Time
}

// CreateLogoUpload gera uma URL pré-assinada para o admin enviar o logo ao bucket. A chave é
// prefixada pelo tenant, então uma URL de um tenant nunca escreve no espaço de outro. Depois
// do upload, o cliente grava PublicURL no tema com SetTheme.
func (s *Service) CreateLogoUpload(ctx context.Context, p Principal, id uuid.UUID, contentType string) (LogoUpload, error) {
	if !p.CanManage(id) {
		return LogoUpload{}, domain.ErrForbidden
	}
	ext, ok := logoContentTypes[contentType]
	if !ok {
		return LogoUpload{}, &domain.ValidationError{Fields: []domain.FieldError{{Field: "content_type", Message: "use image/png, image/svg+xml ou image/webp"}}}
	}
	if _, err := s.repo.GetTenant(ctx, id); err != nil {
		return LogoUpload{}, err
	}
	key := fmt.Sprintf("tenants/%s/logo/%s.%s", id, s.newID(), ext)
	url, err := s.assets.PresignUpload(ctx, key, contentType, s.uploadTTL)
	if err != nil {
		return LogoUpload{}, fmt.Errorf("pré-assinar upload: %w", err)
	}
	return LogoUpload{UploadURL: url, PublicURL: s.assets.PublicURL(key), ExpiresAt: s.now().Add(s.uploadTTL)}, nil
}
