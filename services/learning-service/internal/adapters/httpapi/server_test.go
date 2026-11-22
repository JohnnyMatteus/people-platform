package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/adapters/memory"
	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/app"
	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/platform/auth"
	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/platform/telemetry"
)

var (
	tenantA = uuid.MustParse("0197a000-0000-7000-8000-00000000000a")
	tenantB = uuid.MustParse("0197a000-0000-7000-8000-00000000000b")
	adminA  = uuid.New()
	anaA    = uuid.New()
	brunoA  = uuid.New()
	adminB  = uuid.New()
)

// fakeVerifier troca o token por um principal, sem JWKS: o que está em teste é o HTTP, não a assinatura.
type fakeVerifier map[string]app.Principal

func (f fakeVerifier) Verify(_ context.Context, raw string) (app.Principal, error) {
	if p, ok := f[raw]; ok {
		return p, nil
	}
	return app.Principal{}, auth.ErrUnauthenticated
}

func newServer() http.Handler {
	svc := app.NewService(memory.NewStore(), time.Now, uuid.New)
	verifier := fakeVerifier{
		"admin-a": {Subject: adminA.String(), TenantID: tenantA, Roles: []string{"tenant-admin"}},
		"ana-a":   {Subject: anaA.String(), TenantID: tenantA, Roles: []string{"employee"}},
		"bruno-a": {Subject: brunoA.String(), TenantID: tenantA, Roles: []string{"employee"}},
		"admin-b": {Subject: adminB.String(), TenantID: tenantB, Roles: []string{"tenant-admin"}},
	}
	return NewServer(svc, verifier, telemetry.NewMetrics(), slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()
}

func call(t *testing.T, h http.Handler, method, path, token string, body any) (int, map[string]any, []any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var obj map[string]any
	var arr []any
	raw := rec.Body.Bytes()
	if json.Unmarshal(raw, &obj) != nil {
		_ = json.Unmarshal(raw, &arr)
	}
	return rec.Code, obj, arr
}

func TestHTTP_FlowTrackEnrollCompleteSuggest(t *testing.T) {
	h := newServer()
	code, track, _ := call(t, h, "POST", "/v1/tracks", "admin-a", map[string]any{"title": "Integração"})
	if code != http.StatusCreated {
		t.Fatalf("criar trilha = %d", code)
	}
	trackID := track["id"].(string)
	for _, title := range []string{"Boas-vindas", "Segurança"} {
		if code, _, _ := call(t, h, "POST", "/v1/tracks/"+trackID+"/items", "admin-a", map[string]any{"title": title, "kind": "video", "minutes": 10}); code != http.StatusCreated {
			t.Fatalf("adicionar conteúdo = %d", code)
		}
	}
	code, enrollment, _ := call(t, h, "POST", "/v1/tracks/"+trackID+"/enrollments", "ana-a", nil)
	if code != http.StatusCreated {
		t.Fatalf("matricular = %d", code)
	}
	_, _, list := call(t, h, "GET", "/v1/me/enrollments", "ana-a", nil)
	first := list[0].(map[string]any)["next_item"].(map[string]any)["id"].(string)

	code, done, _ := call(t, h, "POST", "/v1/me/enrollments/"+enrollment["id"].(string)+"/items/"+first+"/complete", "ana-a", nil)
	if code != http.StatusOK || done["done_items"].(float64) != 1 || done["total_items"].(float64) != 2 {
		t.Fatalf("concluir = %d %+v", code, done)
	}
	_, sugg, _ := call(t, h, "GET", "/v1/me/suggestions", "ana-a", nil)
	if len(sugg["next"].([]any)) != 1 {
		t.Fatalf("sugestões: %+v", sugg)
	}
}

func TestHTTP_Authorization(t *testing.T) {
	h := newServer()
	if code, _, _ := call(t, h, "GET", "/v1/tracks", "", nil); code != http.StatusUnauthorized {
		t.Fatalf("sem token = %d", code)
	}
	if code, _, _ := call(t, h, "GET", "/v1/tracks", "token-falso", nil); code != http.StatusUnauthorized {
		t.Fatalf("token inválido = %d", code)
	}
	if code, _, _ := call(t, h, "POST", "/v1/tracks", "ana-a", map[string]any{"title": "x"}); code != http.StatusForbidden {
		t.Fatalf("colaborador criando trilha = %d", code)
	}
	if code, _, _ := call(t, h, "GET", "/v1/onboarding/pending", "ana-a", nil); code != http.StatusForbidden {
		t.Fatalf("colaborador listando pendências = %d", code)
	}
}

func TestHTTP_TenantComesFromTheTokenAndOtherPeopleAreNotFound(t *testing.T) {
	h := newServer()
	_, track, _ := call(t, h, "POST", "/v1/tracks", "admin-a", map[string]any{"title": "Do A"})
	trackID := track["id"].(string)
	call(t, h, "POST", "/v1/tracks/"+trackID+"/items", "admin-a", map[string]any{"title": "x", "kind": "video", "minutes": 5})
	_, enrollment, _ := call(t, h, "POST", "/v1/tracks/"+trackID+"/enrollments", "ana-a", nil)
	itemID := func() string {
		_, _, list := call(t, h, "GET", "/v1/me/enrollments", "ana-a", nil)
		return list[0].(map[string]any)["next_item"].(map[string]any)["id"].(string)
	}()

	// O corpo e a query não mudam o tenant: o admin do B não enxerga nem altera a trilha do A.
	if code, _, list := call(t, h, "GET", "/v1/tracks?tenant_id="+tenantA.String(), "admin-b", nil); code != http.StatusOK || len(list) != 0 {
		t.Fatalf("B listou %d trilha(s) do A (status %d)", len(list), code)
	}
	if code, _, _ := call(t, h, "GET", "/v1/tracks/"+trackID, "admin-b", nil); code != http.StatusNotFound {
		t.Fatalf("B lendo trilha do A = %d", code)
	}
	// Outra pessoa do mesmo tenant também recebe 404 na matrícula alheia.
	if code, _, _ := call(t, h, "POST", "/v1/me/enrollments/"+enrollment["id"].(string)+"/items/"+itemID+"/complete", "bruno-a", nil); code != http.StatusNotFound {
		t.Fatalf("Bruno concluindo item da Ana = %d", code)
	}
}

func TestHTTP_ValidationAndMalformedIDs(t *testing.T) {
	h := newServer()
	if code, _, _ := call(t, h, "POST", "/v1/tracks", "admin-a", map[string]any{"title": ""}); code != http.StatusUnprocessableEntity {
		t.Fatalf("título vazio = %d", code)
	}
	if code, _, _ := call(t, h, "POST", "/v1/tracks", "admin-a", map[string]any{"title": "x", "extra": 1}); code != http.StatusUnprocessableEntity {
		t.Fatalf("campo desconhecido = %d", code)
	}
	if code, _, _ := call(t, h, "GET", "/v1/tracks/nao-e-uuid", "admin-a", nil); code != http.StatusNotFound {
		t.Fatalf("id malformado = %d", code)
	}
}
