package keycloak

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/provisioning"
)

// fakeKeycloak é o dublê local da Admin API: guarda as Organizations em memória e registra o que recebeu. Não prova
// nada sobre o Keycloak real, só que o adaptador fala o protocolo documentado (token, busca por atributo, criação).
type fakeKeycloak struct {
	mu          sync.Mutex
	orgs        []organization
	tokens      int
	searches    []url.Values
	created     []organization
	createCode  int // 0 = 201
	searchCode  int // 0 = 200
	rejectToken bool
	expireFirst bool
	racing      bool // outra instância cria a Organization no instante do nosso POST
	srv         *httptest.Server
}

func newFake(t *testing.T) *fakeKeycloak {
	t.Helper()
	f := &fakeKeycloak{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /realms/people/protocol/openid-connect/token", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		_ = r.ParseForm()
		if f.rejectToken || r.Form.Get("grant_type") != "client_credentials" || r.Form.Get("client_secret") != "segredo-de-teste" {
			http.Error(w, `{"error":"unauthorized_client"}`, http.StatusUnauthorized)
			return
		}
		f.tokens++
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "token-" + string(rune('0'+f.tokens)), "expires_in": 300})
	})
	mux.HandleFunc("GET /admin/realms/people/organizations", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.expireFirst && r.Header.Get("Authorization") == "Bearer token-1" {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}
		f.searches = append(f.searches, r.URL.Query())
		if f.searchCode != 0 {
			http.Error(w, "boom", f.searchCode)
			return
		}
		out := []organization{}
		for _, o := range f.orgs {
			if "tenant_id:"+o.Attributes["tenant_id"][0] == r.URL.Query().Get("q") {
				out = append(out, o)
			}
		}
		_ = json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("POST /admin/realms/people/organizations", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		var o organization
		_ = json.NewDecoder(r.Body).Decode(&o)
		f.created = append(f.created, o)
		if f.racing {
			f.orgs = append(f.orgs, o)
			http.Error(w, `{"errorMessage":"já existe"}`, http.StatusConflict)
			return
		}
		if f.createCode != 0 {
			http.Error(w, `{"errorMessage":"recusado"}`, f.createCode)
			return
		}
		f.orgs = append(f.orgs, o)
		w.WriteHeader(http.StatusCreated)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeKeycloak) client() *Client {
	return New(f.srv.URL, "people", "people-provisioner", "segredo-de-teste", f.srv.Client())
}

func org() provisioning.Organization {
	return provisioning.Organization{TenantID: uuid.New(), Slug: "construtora-horizonte", Name: "Construtora Horizonte Exemplo"}
}

func TestEnsureOrganization_CreatesWithTenantIDAttributeAndIsIdempotent(t *testing.T) {
	f := newFake(t)
	c, o := f.client(), org()

	if err := c.EnsureOrganization(t.Context(), o); err != nil {
		t.Fatal(err)
	}
	if len(f.created) != 1 {
		t.Fatalf("criadas = %d", len(f.created))
	}
	got := f.created[0]
	if got.Name != o.Name || got.Alias != o.Slug || !got.Enabled || got.Attributes["tenant_id"][0] != o.TenantID.String() {
		t.Fatalf("Organization enviada: %+v", got)
	}
	if f.searches[0].Get("q") != "tenant_id:"+o.TenantID.String() {
		t.Fatalf("a busca deve ser pelo atributo tenant_id: %v", f.searches[0])
	}

	if err := c.EnsureOrganization(t.Context(), o); err != nil || len(f.created) != 1 {
		t.Fatalf("a segunda chamada não pode criar outra (criadas=%d, err=%v)", len(f.created), err)
	}
	if f.tokens != 1 {
		t.Fatalf("o token deveria ser reaproveitado, foram %d emissões", f.tokens)
	}
}

func TestEnsureOrganization_RaceWithAnotherInstanceCountsAsDone(t *testing.T) {
	f := newFake(t)
	f.racing = true
	if err := f.client().EnsureOrganization(t.Context(), org()); err != nil {
		t.Fatalf("a Organization criada pela outra instância deve contar como feita: %v", err)
	}
	if len(f.searches) != 2 {
		t.Fatalf("depois do POST recusado a busca deve ser repetida (buscas=%d)", len(f.searches))
	}
}

func TestEnsureOrganization_RefusedWithoutExistingOrganizationIsPermanent(t *testing.T) {
	f := newFake(t)
	f.createCode = http.StatusBadRequest
	err := f.client().EnsureOrganization(t.Context(), org())
	if !errors.Is(err, provisioning.ErrPermanent) {
		t.Fatalf("recusa sem Organization existente deveria ser permanente: %v", err)
	}
}

func TestEnsureOrganization_ServerErrorsAreTransient(t *testing.T) {
	f := newFake(t)
	f.searchCode = http.StatusInternalServerError
	if err := f.client().EnsureOrganization(t.Context(), org()); err == nil || errors.Is(err, provisioning.ErrPermanent) {
		t.Fatalf("500 na busca é transitório: %v", err)
	}
	f = newFake(t)
	f.createCode = http.StatusInternalServerError
	if err := f.client().EnsureOrganization(t.Context(), org()); err == nil || errors.Is(err, provisioning.ErrPermanent) {
		t.Fatalf("500 na criação é transitório: %v", err)
	}
}

func TestEnsureOrganization_ExpiredTokenIsRenewedOnce(t *testing.T) {
	f := newFake(t)
	f.expireFirst = true
	if err := f.client().EnsureOrganization(t.Context(), org()); err != nil {
		t.Fatal(err)
	}
	if f.tokens != 2 || len(f.created) != 1 {
		t.Fatalf("tokens=%d criadas=%d", f.tokens, len(f.created))
	}
}

func TestEnsureOrganization_TokenRefusedIsTransientError(t *testing.T) {
	f := newFake(t)
	f.rejectToken = true
	err := f.client().EnsureOrganization(t.Context(), org())
	if err == nil || errors.Is(err, provisioning.ErrPermanent) || len(f.created) != 0 {
		t.Fatalf("token recusado: err=%v criadas=%d", err, len(f.created))
	}
}
