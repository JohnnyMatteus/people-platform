package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/domain"
	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/platform/auth"
)

// problem segue o RFC 9457 (application/problem+json).
type problem struct {
	Type   string              `json:"type"`
	Title  string              `json:"title"`
	Status int                 `json:"status"`
	Detail string              `json:"detail,omitempty"`
	Errors []domain.FieldError `json:"errors,omitempty"`
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

// writeError traduz erros de domínio para HTTP. Erros inesperados viram 500 sem detalhe
// para o cliente (o detalhe vai só para o log, com trace_id).
func writeError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	var verr *domain.ValidationError
	switch {
	case errors.As(err, &verr):
		writeProblem(w, problem{Type: "urn:people:validation", Title: "Entrada inválida", Status: http.StatusUnprocessableEntity, Errors: verr.Fields})
	case errors.Is(err, domain.ErrInvalidInput):
		writeProblem(w, problem{Type: "urn:people:validation", Title: "Entrada inválida", Status: http.StatusBadRequest, Detail: err.Error()})
	case errors.Is(err, auth.ErrUnauthenticated):
		w.Header().Set("WWW-Authenticate", `Bearer realm="people"`)
		writeProblem(w, problem{Type: "urn:people:unauthenticated", Title: "Não autenticado", Status: http.StatusUnauthorized})
	case errors.Is(err, domain.ErrForbidden):
		writeProblem(w, problem{Type: "urn:people:forbidden", Title: "Acesso negado", Status: http.StatusForbidden})
	case errors.Is(err, domain.ErrNotFound):
		writeProblem(w, problem{Type: "urn:people:not-found", Title: "Não encontrado", Status: http.StatusNotFound})
	case errors.Is(err, domain.ErrConflict):
		writeProblem(w, problem{Type: "urn:people:conflict", Title: "Conflito", Status: http.StatusConflict, Detail: "o recurso já existe"})
	case errors.Is(err, domain.ErrFeatureLocked):
		writeProblem(w, problem{Type: "urn:people:feature-locked", Title: "Feature indisponível no plano", Status: http.StatusConflict})
	default:
		logger.ErrorContext(r.Context(), "erro inesperado", slog.String("error", err.Error()), slog.String("path", r.URL.Path))
		writeProblem(w, problem{Type: "urn:people:internal", Title: "Erro interno", Status: http.StatusInternalServerError})
	}
}
