package provisioning

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type fake struct {
	orgs []Organization
	err  error
}

func (f *fake) EnsureOrganization(_ context.Context, o Organization) error {
	if f.err != nil {
		return f.err
	}
	f.orgs = append(f.orgs, o)
	return nil
}

func event(eventType string, tenant uuid.UUID, slug, name string) []byte {
	return []byte(`{"event_type":"` + eventType + `","event_version":1,"tenant_id":"` + tenant.String() +
		`","data":{"slug":"` + slug + `","name":"` + name + `","plan":"essencial","features":[]}}`)
}

func TestHandleEvent_ProvisionsTenantCriadoWithTenantIDFromTheEnvelope(t *testing.T) {
	f := &fake{}
	tenant := uuid.New()
	applied, err := NewHandler(f).HandleEvent(context.Background(), event("TenantCriado", tenant, "construtora-horizonte", "Construtora Horizonte Exemplo"))
	if err != nil || !applied {
		t.Fatalf("applied=%v err=%v", applied, err)
	}
	if len(f.orgs) != 1 || f.orgs[0] != (Organization{TenantID: tenant, Slug: "construtora-horizonte", Name: "Construtora Horizonte Exemplo"}) {
		t.Fatalf("organização: %+v", f.orgs)
	}
}

func TestHandleEvent_IgnoresOtherEventTypes(t *testing.T) {
	f := &fake{}
	applied, err := NewHandler(f).HandleEvent(context.Background(), event("TenantPlanoAlterado", uuid.New(), "x", "X"))
	if applied || err != nil || len(f.orgs) != 0 {
		t.Fatalf("outro tipo não provisiona: %v %v %v", applied, err, f.orgs)
	}
}

func TestHandleEvent_MalformedIsPermanentAndProviderErrorsPassThrough(t *testing.T) {
	h := NewHandler(&fake{})
	for _, raw := range [][]byte{[]byte("{nao-json"), event("TenantCriado", uuid.Nil, "x", "X"), event("TenantCriado", uuid.New(), "", "X")} {
		if _, err := h.HandleEvent(context.Background(), raw); !errors.Is(err, ErrPermanent) {
			t.Fatalf("mensagem inválida deveria ser permanente: %v", err)
		}
	}
	boom := errors.New("keycloak fora do ar")
	if _, err := NewHandler(&fake{err: boom}).HandleEvent(context.Background(), event("TenantCriado", uuid.New(), "x", "X")); !errors.Is(err, boom) || errors.Is(err, ErrPermanent) {
		t.Fatalf("falha do provedor é transitória e passa intacta: %v", err)
	}
}
