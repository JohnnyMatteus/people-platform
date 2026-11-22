// Package httpapi expõe os casos de uso do learning-service via HTTP/JSON, conforme contracts/openapi/learning-service.yaml.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/app"
	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/domain"
	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/platform/auth"
	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/platform/telemetry"
)

const maxBodyBytes = 64 << 10

// TokenVerifier valida o bearer token e devolve o principal.
type TokenVerifier interface {
	Verify(ctx context.Context, raw string) (app.Principal, error)
}

// ReadinessCheck verifica uma dependência (banco, Kafka) para o /readyz.
type ReadinessCheck func(context.Context) error

type Server struct {
	svc      *app.Service
	verifier TokenVerifier
	metrics  *telemetry.Metrics
	logger   *slog.Logger
	ready    []ReadinessCheck
}

func NewServer(svc *app.Service, v TokenVerifier, m *telemetry.Metrics, logger *slog.Logger, ready ...ReadinessCheck) *Server {
	return &Server{svc: svc, verifier: v, metrics: m, logger: logger, ready: ready}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", s.readyz)
	mux.Handle("GET /metrics", promhttp.HandlerFor(s.metrics.Registry, promhttp.HandlerOpts{}))

	mux.HandleFunc("GET /v1/tracks", s.authenticated(s.listTracks))
	mux.HandleFunc("POST /v1/tracks", s.authenticated(s.createTrack))
	mux.HandleFunc("GET /v1/tracks/{trackId}", s.authenticated(s.getTrack))
	mux.HandleFunc("POST /v1/tracks/{trackId}/items", s.authenticated(s.addItem))
	mux.HandleFunc("POST /v1/tracks/{trackId}/enrollments", s.authenticated(s.enroll))
	mux.HandleFunc("GET /v1/me/enrollments", s.authenticated(s.myEnrollments))
	mux.HandleFunc("POST /v1/me/enrollments/{enrollmentId}/items/{itemId}/complete", s.authenticated(s.complete))
	mux.HandleFunc("GET /v1/me/suggestions", s.authenticated(s.suggestions))
	mux.HandleFunc("GET /v1/onboarding/default-track", s.authenticated(s.getDefaultTrack))
	mux.HandleFunc("PUT /v1/onboarding/default-track", s.authenticated(s.putDefaultTrack))
	mux.HandleFunc("GET /v1/onboarding/pending", s.authenticated(s.pendingOnboarding))
	mux.HandleFunc("POST /v1/onboarding/{applicationId}/bind", s.authenticated(s.bindOnboarding))

	return otelhttp.NewHandler(s.observe(mux), "learning-service",
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

type principalKey struct{}

// authenticated exige um bearer token válido. O gateway também valida; validar de novo é defesa em profundidade.
func (s *Server) authenticated(next func(http.ResponseWriter, *http.Request, app.Principal)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || raw == "" {
			s.writeError(w, r, auth.ErrUnauthenticated)
			return
		}
		p, err := s.verifier.Verify(r.Context(), raw)
		if err != nil {
			s.writeError(w, r, err)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)), p)
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// observe registra métricas RED por padrão de rota (não pela URL crua, para não explodir a cardinalidade) e um log de acesso.
func (s *Server) observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		s.metrics.HTTPRequests.WithLabelValues(route, r.Method, strconv.Itoa(rec.status)).Inc()
		s.metrics.HTTPDuration.WithLabelValues(route, r.Method).Observe(time.Since(start).Seconds())
		if route != "GET /metrics" && route != "GET /healthz" && route != "GET /readyz" {
			s.logger.InfoContext(r.Context(), "http", slog.String("route", route), slog.Int("status", rec.status),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()))
		}
	})
}

type problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeProblem(w http.ResponseWriter, p problem) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}

// writeError traduz erros de domínio para HTTP (RFC 9457). Erro inesperado vira 500 sem detalhe para o cliente.
func (s *Server) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrUnauthenticated):
		w.Header().Set("WWW-Authenticate", `Bearer realm="people"`)
		writeProblem(w, problem{Type: "urn:people:unauthenticated", Title: "Não autenticado", Status: http.StatusUnauthorized})
	case errors.Is(err, domain.ErrInvalidInput):
		writeProblem(w, problem{Type: "urn:people:validation", Title: "Entrada inválida", Status: http.StatusUnprocessableEntity, Detail: err.Error()})
	case errors.Is(err, domain.ErrForbidden):
		writeProblem(w, problem{Type: "urn:people:forbidden", Title: "Acesso negado", Status: http.StatusForbidden})
	case errors.Is(err, domain.ErrNotFound):
		writeProblem(w, problem{Type: "urn:people:not-found", Title: "Não encontrado", Status: http.StatusNotFound})
	case errors.Is(err, domain.ErrConflict):
		writeProblem(w, problem{Type: "urn:people:conflict", Title: "Conflito", Status: http.StatusConflict, Detail: err.Error()})
	default:
		s.logger.ErrorContext(r.Context(), "erro inesperado", slog.String("error", err.Error()), slog.String("path", r.URL.Path))
		writeProblem(w, problem{Type: "urn:people:internal", Title: "Erro interno", Status: http.StatusInternalServerError})
	}
}

func decode(r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("%w: JSON inválido: %v", domain.ErrInvalidInput, err)
	}
	return nil
}

// pathUUID lê um UUID do caminho; malformado responde como inexistente, sem distinguir para o cliente.
func pathUUID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		return uuid.Nil, domain.ErrNotFound
	}
	return id, nil
}
