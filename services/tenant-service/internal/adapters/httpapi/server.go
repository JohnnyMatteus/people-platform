// Package httpapi expõe os casos de uso do tenant-service via HTTP/JSON, conforme
// contracts/openapi/tenant-service.yaml.
package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/app"
	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/domain"
	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/platform/telemetry"
)

// maxBodyBytes limita o corpo das requisições: nenhum payload legítimo passa de alguns KB.
const maxBodyBytes = 64 << 10

// ReadinessCheck verifica uma dependência (banco, Kafka) para o /readyz.
type ReadinessCheck func(context.Context) error

// Server reúne as dependências dos handlers.
type Server struct {
	svc      *app.Service
	verifier TokenVerifier
	metrics  *telemetry.Metrics
	logger   *slog.Logger
	ready    []ReadinessCheck
}

// NewServer cria o servidor HTTP.
func NewServer(svc *app.Service, v TokenVerifier, m *telemetry.Metrics, logger *slog.Logger, ready ...ReadinessCheck) *Server {
	return &Server{svc: svc, verifier: v, metrics: m, logger: logger, ready: ready}
}

// Handler monta as rotas com o roteador padrão do Go (padrões com método e curingas).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", s.readyz)
	mux.Handle("GET /metrics", promhttp.HandlerFor(s.metrics.Registry, promhttp.HandlerOpts{}))

	// Públicas: o front-end chama antes do login para pintar a tela com a marca do tenant.
	mux.HandleFunc("GET /v1/plans", s.listPlans)
	mux.HandleFunc("GET /v1/branding", s.brandingByHost)

	mux.HandleFunc("GET /v1/me/tenant", s.authenticated(s.brandingByClaim))
	mux.HandleFunc("POST /v1/tenants", s.authenticated(s.createTenant))
	mux.HandleFunc("GET /v1/tenants", s.authenticated(s.listTenants))
	mux.HandleFunc("GET /v1/tenants/{id}", s.authenticated(s.getTenant))
	mux.HandleFunc("PATCH /v1/tenants/{id}", s.authenticated(s.patchTenant))
	mux.HandleFunc("DELETE /v1/tenants/{id}", s.authenticated(s.deleteTenant))
	mux.HandleFunc("GET /v1/tenants/{id}/features", s.authenticated(s.getFeatures))
	mux.HandleFunc("PUT /v1/tenants/{id}/features/{feature}", s.authenticated(s.putFeature))
	mux.HandleFunc("GET /v1/tenants/{id}/theme", s.authenticated(s.getTheme))
	mux.HandleFunc("PUT /v1/tenants/{id}/theme", s.authenticated(s.putTheme))
	mux.HandleFunc("POST /v1/tenants/{id}/theme/logo-upload", s.authenticated(s.createLogoUpload))
	mux.HandleFunc("GET /v1/tenants/{id}/domains", s.authenticated(s.listDomains))
	mux.HandleFunc("POST /v1/tenants/{id}/domains", s.authenticated(s.addDomain))

	return otelhttp.NewHandler(observe(s.metrics, s.logger, mux), "tenant-service",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string { return r.Method + " " + r.URL.Path }))
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	for _, check := range s.ready {
		if err := check(r.Context()); err != nil {
			s.logger.WarnContext(r.Context(), "não pronto", slog.String("error", err.Error()))
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func decode(r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("%w: JSON inválido: %v", domain.ErrInvalidInput, err)
	}
	return nil
}

func pathID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		// ID malformado é tratado como inexistente: não há o que distinguir para o cliente.
		return uuid.Nil, domain.ErrNotFound
	}
	return id, nil
}

func (s *Server) listPlans(w http.ResponseWriter, _ *http.Request) {
	out := []planResponse{}
	for _, p := range domain.Plans() {
		out = append(out, planResponse{Code: string(p.Code), Name: p.Name, Included: p.Included, Addable: p.Addable})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) brandingByHost(w http.ResponseWriter, r *http.Request) {
	// O gateway repassa o host original em X-Forwarded-Host; sem gateway, vale o Host.
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	b, err := s.svc.ResolveByHost(r.Context(), host)
	if err != nil {
		writeError(w, r, s.logger, err)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	writeJSON(w, http.StatusOK, toBrandingResponse(b))
}

func (s *Server) brandingByClaim(w http.ResponseWriter, r *http.Request) {
	b, err := s.svc.ResolveByClaim(r.Context(), principalFrom(r.Context()))
	if err != nil {
		writeError(w, r, s.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, toBrandingResponse(b))
}

func (s *Server) createTenant(w http.ResponseWriter, r *http.Request) {
	var req createTenantRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, s.logger, err)
		return
	}
	t, err := s.svc.CreateTenant(r.Context(), principalFrom(r.Context()),
		domain.NewTenantInput{Slug: req.Slug, Name: req.Name, Plan: domain.PlanCode(req.Plan)})
	if err != nil {
		writeError(w, r, s.logger, err)
		return
	}
	w.Header().Set("Location", "/v1/tenants/"+t.ID.String())
	writeJSON(w, http.StatusCreated, toTenantResponse(t))
}

func (s *Server) listTenants(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListTenants(r.Context(), principalFrom(r.Context()))
	if err != nil {
		writeError(w, r, s.logger, err)
		return
	}
	out := make([]tenantResponse, 0, len(list))
	for _, t := range list {
		out = append(out, toTenantResponse(t))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getTenant(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err == nil {
		var t domain.Tenant
		if t, err = s.svc.GetTenant(r.Context(), principalFrom(r.Context()), id); err == nil {
			writeJSON(w, http.StatusOK, toTenantResponse(t))
			return
		}
	}
	writeError(w, r, s.logger, err)
}

func (s *Server) patchTenant(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, s.logger, err)
		return
	}
	var req patchTenantRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, s.logger, err)
		return
	}
	t, err := s.svc.UpdateTenant(r.Context(), principalFrom(r.Context()), id, req.toPatch())
	if err != nil {
		writeError(w, r, s.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, toTenantResponse(t))
}

func (s *Server) deleteTenant(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err == nil {
		err = s.svc.DeactivateTenant(r.Context(), principalFrom(r.Context()), id)
	}
	if err != nil {
		writeError(w, r, s.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getFeatures(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err == nil {
		var v app.FeatureView
		if v, err = s.svc.Features(r.Context(), principalFrom(r.Context()), id); err == nil {
			writeJSON(w, http.StatusOK, toFeaturesResponse(v))
			return
		}
	}
	writeError(w, r, s.logger, err)
}

func (s *Server) putFeature(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, s.logger, err)
		return
	}
	var req featureOverrideRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, s.logger, err)
		return
	}
	if req.Enabled == nil {
		writeError(w, r, s.logger, &domain.ValidationError{Fields: []domain.FieldError{{Field: "enabled", Message: "campo obrigatório"}}})
		return
	}
	v, err := s.svc.SetFeatureOverride(r.Context(), principalFrom(r.Context()), id, domain.Feature(r.PathValue("feature")), *req.Enabled)
	if err != nil {
		writeError(w, r, s.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, toFeaturesResponse(v))
}

func (s *Server) getTheme(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err == nil {
		var t domain.Theme
		if t, err = s.svc.Theme(r.Context(), principalFrom(r.Context()), id); err == nil {
			writeJSON(w, http.StatusOK, toThemeResponse(t))
			return
		}
	}
	writeError(w, r, s.logger, err)
}

func (s *Server) putTheme(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, s.logger, err)
		return
	}
	var req themeRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, s.logger, err)
		return
	}
	t, err := s.svc.SetTheme(r.Context(), principalFrom(r.Context()), id, req.toInput())
	if err != nil {
		writeError(w, r, s.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, toThemeResponse(t))
}

func (s *Server) createLogoUpload(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, s.logger, err)
		return
	}
	var req logoUploadRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, s.logger, err)
		return
	}
	up, err := s.svc.CreateLogoUpload(r.Context(), principalFrom(r.Context()), id, req.ContentType)
	if err != nil {
		writeError(w, r, s.logger, err)
		return
	}
	writeJSON(w, http.StatusCreated, logoUploadResponse{UploadURL: up.UploadURL, PublicURL: up.PublicURL, ExpiresAt: up.ExpiresAt})
}

func (s *Server) listDomains(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err == nil {
		var hosts []string
		if hosts, err = s.svc.ListDomains(r.Context(), principalFrom(r.Context()), id); err == nil {
			writeJSON(w, http.StatusOK, map[string][]string{"domains": hosts})
			return
		}
	}
	writeError(w, r, s.logger, err)
}

func (s *Server) addDomain(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, s.logger, err)
		return
	}
	var req domainRequest
	if err := decode(r, &req); err != nil {
		writeError(w, r, s.logger, err)
		return
	}
	host, err := s.svc.AddDomain(r.Context(), principalFrom(r.Context()), id, req.Host)
	if err != nil {
		writeError(w, r, s.logger, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"host": host})
}
