package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/adapters/memory"
	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/app"
	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/platform/auth"
	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/platform/telemetry"
)

// fakeVerifier mapeia tokens opacos para principals, sem criptografia: a validação real de
// JWT é testada no pacote auth.
type fakeVerifier map[string]app.Principal

func (f fakeVerifier) Verify(_ context.Context, raw string) (app.Principal, error) {
	p, ok := f[raw]
	if !ok {
		return app.Principal{}, auth.ErrUnauthenticated
	}
	return p, nil
}

type fakeAssets struct{}

func (fakeAssets) PresignUpload(_ context.Context, key, _ string, _ time.Duration) (string, error) {
	return "http://localhost:4566/assets/" + key + "?X-Amz-Signature=fake", nil
}
func (fakeAssets) PublicURL(key string) string { return "http://localhost:4566/assets/" + key }

type testEnv struct {
	srv      *httptest.Server
	verifier fakeVerifier
	metrics  *telemetry.Metrics
}

func newEnv(t *testing.T) *testEnv {
	t.Helper()
	svc := app.NewService(memory.NewRepository(), fakeAssets{}, 10*time.Minute, time.Now, uuid.New)
	v := fakeVerifier{"op": {Subject: "op", Roles: []string{app.RolePlatformAdmin}}}
	m := telemetry.NewMetrics()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	srv := httptest.NewServer(NewServer(svc, v, m, logger).Handler())
	t.Cleanup(srv.Close)
	return &testEnv{srv: srv, verifier: v, metrics: m}
}

func (e *testEnv) do(t *testing.T, method, path, token string, body any, headers ...string) (int, map[string]any) {
	t.Helper()
	var buf io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, e.srv.URL+path, buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := e.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

func (e *testEnv) createTenant(t *testing.T, slug, plan string) string {
	t.Helper()
	status, body := e.do(t, http.MethodPost, "/v1/tenants", "op", map[string]string{"slug": slug, "name": "Tenant " + slug, "plan": plan})
	if status != http.StatusCreated {
		t.Fatalf("criar tenant: %d %v", status, body)
	}
	return body["id"].(string)
}

func TestHTTP_TenantLifecycle(t *testing.T) {
	e := newEnv(t)
	id := e.createTenant(t, "construtora-horizonte", "corporativo")

	if status, body := e.do(t, http.MethodGet, "/v1/tenants/"+id, "op", nil); status != 200 || body["slug"] != "construtora-horizonte" {
		t.Fatalf("GET: %d %v", status, body)
	}
	if status, body := e.do(t, http.MethodPatch, "/v1/tenants/"+id, "op", map[string]string{"plan": "crescimento"}); status != 200 || body["plan"] != "crescimento" {
		t.Fatalf("PATCH: %d %v", status, body)
	}
	if status, _ := e.do(t, http.MethodDelete, "/v1/tenants/"+id, "op", nil); status != http.StatusNoContent {
		t.Fatalf("DELETE: %d", status)
	}
	if _, body := e.do(t, http.MethodGet, "/v1/tenants/"+id, "op", nil); body["status"] != "deactivated" {
		t.Fatalf("status após DELETE: %v", body)
	}
}

func TestHTTP_ErrorMapping(t *testing.T) {
	e := newEnv(t)
	id := e.createTenant(t, "rede-sol", "essencial")
	e.verifier["adm-outro"] = app.Principal{Subject: "x", TenantID: uuid.New(), Roles: []string{app.RoleTenantAdmin}}

	cases := []struct {
		name, method, path, token string
		body                      any
		want                      int
	}{
		{"sem token", "GET", "/v1/tenants", "", nil, 401},
		{"token inválido", "GET", "/v1/tenants", "forjado", nil, 401},
		{"admin de tenant não lista todos", "GET", "/v1/tenants", "adm-outro", nil, 403},
		{"outro tenant recebe 404", "GET", "/v1/tenants/" + id, "adm-outro", nil, 404},
		{"id malformado", "GET", "/v1/tenants/nao-e-uuid", "op", nil, 404},
		{"slug duplicado", "POST", "/v1/tenants", "op", map[string]string{"slug": "rede-sol", "name": "X", "plan": "essencial"}, 409},
		{"validação de campos", "POST", "/v1/tenants", "op", map[string]string{"slug": "X", "name": "", "plan": "?"}, 422},
		{"campo desconhecido", "POST", "/v1/tenants", "op", map[string]string{"slug": "abc", "hack": "1"}, 400},
		{"feature bloqueada no plano", "PUT", "/v1/tenants/" + id + "/features/analytics", "op", map[string]bool{"enabled": true}, 409},
		{"enabled ausente", "PUT", "/v1/tenants/" + id + "/features/okr", "op", map[string]string{}, 422},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := e.do(t, tc.method, tc.path, tc.token, tc.body)
			if status != tc.want {
				t.Fatalf("status = %d, want %d (%v)", status, tc.want, body)
			}
			if status >= 400 && body["type"] == nil {
				t.Fatalf("erro deve vir como problem+json: %v", body)
			}
		})
	}
}

func TestHTTP_ThemeContrastValidation(t *testing.T) {
	e := newEnv(t)
	id := e.createTenant(t, "construtora-horizonte", "crescimento")
	e.verifier["adm"] = app.Principal{Subject: "adm", TenantID: uuid.MustParse(id), Roles: []string{app.RoleTenantAdmin}}

	theme := map[string]string{"primary": "#ffd700", "on_primary": "#ffffff", "background": "#ffffff", "text": "#1a1a1a", "accent": "#0b6e4f", "email_from_name": "RH"}
	status, body := e.do(t, http.MethodPut, "/v1/tenants/"+id+"/theme", "adm", theme)
	if status != 422 {
		t.Fatalf("contraste baixo deve dar 422: %d %v", status, body)
	}
	errs := body["errors"].([]any)
	if first := errs[0].(map[string]any); first["field"] != "on_primary" || !strings.Contains(first["message"].(string), "WCAG AA") {
		t.Fatalf("erro deve apontar o par e o critério: %v", errs)
	}

	theme["primary"] = "#1f4e79"
	status, body = e.do(t, http.MethodPut, "/v1/tenants/"+id+"/theme", "adm", theme)
	if status != 200 || body["tokens"].(map[string]any)["color.primary"] != "#1f4e79" {
		t.Fatalf("tema válido: %d %v", status, body)
	}
	if contrast := body["contrast"].([]any); len(contrast) != 3 {
		t.Fatalf("resposta deve trazer as 3 verificações de contraste: %v", contrast)
	}
}

func TestHTTP_BrandingByHostAndClaim(t *testing.T) {
	e := newEnv(t)
	id := e.createTenant(t, "construtora-horizonte", "corporativo")
	if status, body := e.do(t, http.MethodPost, "/v1/tenants/"+id+"/domains", "op", map[string]string{"host": "rh.horizonte.example.com"}); status != 201 {
		t.Fatalf("domínio: %d %v", status, body)
	}

	status, body := e.do(t, http.MethodGet, "/v1/branding", "", nil, "X-Forwarded-Host", "rh.horizonte.example.com")
	if status != 200 || body["tenant_id"] != id || body["tokens"] == nil {
		t.Fatalf("branding por host: %d %v", status, body)
	}
	if status, _ := e.do(t, http.MethodGet, "/v1/branding", "", nil, "X-Forwarded-Host", "outro.example.com"); status != 404 {
		t.Fatalf("host desconhecido: %d", status)
	}

	e.verifier["colab"] = app.Principal{Subject: "c", TenantID: uuid.MustParse(id)}
	if status, body := e.do(t, http.MethodGet, "/v1/me/tenant", "colab", nil); status != 200 || body["slug"] != "construtora-horizonte" {
		t.Fatalf("branding por claim: %d %v", status, body)
	}
}

func TestHTTP_LogoUpload(t *testing.T) {
	e := newEnv(t)
	id := e.createTenant(t, "construtora-horizonte", "crescimento")
	status, body := e.do(t, http.MethodPost, "/v1/tenants/"+id+"/theme/logo-upload", "op", map[string]string{"content_type": "image/png"})
	if status != 201 || !strings.Contains(body["upload_url"].(string), "/tenants/"+id+"/logo/") {
		t.Fatalf("logo upload: %d %v", status, body)
	}
}

func TestHTTP_HealthAndMetrics(t *testing.T) {
	e := newEnv(t)
	if status, _ := e.do(t, http.MethodGet, "/healthz", "", nil); status != 200 {
		t.Fatalf("healthz: %d", status)
	}
	if status, _ := e.do(t, http.MethodGet, "/readyz", "", nil); status != 200 {
		t.Fatalf("readyz: %d", status)
	}
	e.do(t, http.MethodGet, "/v1/plans", "", nil)
	resp, err := e.srv.Client().Get(e.srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(raw), `http_requests_total{method="GET",route="GET /v1/plans",status="200"} 1`) {
		t.Fatalf("métrica por rota ausente:\n%s", raw)
	}
}
