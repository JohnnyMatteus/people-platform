package httpapi

import (
	"time"

	"github.com/google/uuid"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/app"
	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/domain"
)

// Os DTOs espelham contracts/openapi/tenant-service.yaml (snake_case no JSON).

type createTenantRequest struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
	Plan string `json:"plan"`
}

type patchTenantRequest struct {
	Name   *string `json:"name"`
	Plan   *string `json:"plan"`
	Status *string `json:"status"`
}

func (r patchTenantRequest) toPatch() domain.TenantPatch {
	var p domain.TenantPatch
	p.Name = r.Name
	if r.Plan != nil {
		plan := domain.PlanCode(*r.Plan)
		p.Plan = &plan
	}
	if r.Status != nil {
		status := domain.Status(*r.Status)
		p.Status = &status
	}
	return p
}

type tenantResponse struct {
	ID        uuid.UUID `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	Plan      string    `json:"plan"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toTenantResponse(t domain.Tenant) tenantResponse {
	return tenantResponse{ID: t.ID, Slug: t.Slug, Name: t.Name, Plan: string(t.Plan), Status: string(t.Status), CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}

type planResponse struct {
	Code     string           `json:"code"`
	Name     string           `json:"name"`
	Included []domain.Feature `json:"included"`
	Addable  []domain.Feature `json:"addable"`
}

type featuresResponse struct {
	Plan      string                  `json:"plan"`
	Effective []domain.Feature        `json:"effective"`
	Overrides map[domain.Feature]bool `json:"overrides"`
}

func toFeaturesResponse(v app.FeatureView) featuresResponse {
	return featuresResponse{Plan: string(v.Plan), Effective: v.Effective, Overrides: v.Overrides}
}

type featureOverrideRequest struct {
	Enabled *bool `json:"enabled"`
}

type themeRequest struct {
	Primary       string `json:"primary"`
	OnPrimary     string `json:"on_primary"`
	Background    string `json:"background"`
	Text          string `json:"text"`
	Accent        string `json:"accent"`
	LogoURL       string `json:"logo_url"`
	EmailFromName string `json:"email_from_name"`
	EmailReplyTo  string `json:"email_reply_to"`
}

func (r themeRequest) toInput() domain.ThemeInput {
	return domain.ThemeInput{Primary: r.Primary, OnPrimary: r.OnPrimary, Background: r.Background, Text: r.Text,
		Accent: r.Accent, LogoURL: r.LogoURL, EmailFromName: r.EmailFromName, EmailReplyTo: r.EmailReplyTo}
}

type themeResponse struct {
	Tokens        map[string]string      `json:"tokens"`
	LogoURL       string                 `json:"logo_url,omitempty"`
	EmailFromName string                 `json:"email_from_name"`
	EmailReplyTo  string                 `json:"email_reply_to,omitempty"`
	Contrast      []domain.ContrastCheck `json:"contrast"`
}

func toThemeResponse(t domain.Theme) themeResponse {
	return themeResponse{Tokens: t.Tokens(), LogoURL: t.LogoURL, EmailFromName: t.EmailFromName, EmailReplyTo: t.EmailReplyTo, Contrast: t.ContrastChecks()}
}

type domainRequest struct {
	Host string `json:"host"`
}

type logoUploadRequest struct {
	ContentType string `json:"content_type"`
}

type logoUploadResponse struct {
	UploadURL string    `json:"upload_url"`
	PublicURL string    `json:"public_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// brandingResponse é público: só identidade visual, nunca dados do tenant além do nome.
type brandingResponse struct {
	TenantID uuid.UUID         `json:"tenant_id"`
	Slug     string            `json:"slug"`
	Name     string            `json:"name"`
	Tokens   map[string]string `json:"tokens"`
	LogoURL  string            `json:"logo_url,omitempty"`
}

func toBrandingResponse(b app.Branding) brandingResponse {
	return brandingResponse{TenantID: b.TenantID, Slug: b.Slug, Name: b.Name, Tokens: b.Theme.Tokens(), LogoURL: b.Theme.LogoURL}
}
