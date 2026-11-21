package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/app"
	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/platform/auth"
	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/platform/telemetry"
)

// TokenVerifier valida o bearer token e devolve o principal.
type TokenVerifier interface {
	Verify(ctx context.Context, raw string) (app.Principal, error)
}

type principalKey struct{}

func principalFrom(ctx context.Context) app.Principal {
	p, _ := ctx.Value(principalKey{}).(app.Principal)
	return p
}

// authenticated exige um bearer token válido. O gateway também valida o token; validar de
// novo aqui é defesa em profundidade (ninguém fala com o serviço "por dentro" sem token).
func (s *Server) authenticated(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || raw == "" {
			writeError(w, r, s.logger, auth.ErrUnauthenticated)
			return
		}
		p, err := s.verifier.Verify(r.Context(), raw)
		if err != nil {
			writeError(w, r, s.logger, err)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)))
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

// observe registra métricas RED por rota (o padrão do ServeMux, não a URL crua, para não
// explodir a cardinalidade com IDs) e um log de acesso estruturado.
func observe(m *telemetry.Metrics, logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		elapsed := time.Since(start)
		m.HTTPRequests.WithLabelValues(route, r.Method, strconv.Itoa(rec.status)).Inc()
		m.HTTPDuration.WithLabelValues(route, r.Method).Observe(elapsed.Seconds())
		if route != "GET /metrics" && route != "GET /healthz" && route != "GET /readyz" {
			logger.InfoContext(r.Context(), "http", slog.String("route", route), slog.Int("status", rec.status),
				slog.Int64("duration_ms", elapsed.Milliseconds()))
		}
	})
}
