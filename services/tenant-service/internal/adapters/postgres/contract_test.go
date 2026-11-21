package postgres

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/domain"
)

// schemaDir aponta para os contratos versionados na raiz do monorepo. O produtor valida o
// que publica contra o mesmo JSON Schema que os consumidores leem (contract test do produtor).
var schemaDir = filepath.Join("..", "..", "..", "..", "..", "contracts", "asyncapi", "schemas")

func compileSchema(t *testing.T, file string) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	abs, err := filepath.Abs(filepath.Join(schemaDir, file))
	if err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile(abs)
	if err != nil {
		t.Fatalf("compilar schema %s: %v", file, err)
	}
	return s
}

func validateAgainst(t *testing.T, s *jsonschema.Schema, raw []byte) error {
	t.Helper()
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return s.Validate(doc)
}

func TestContract_PublishedEventsMatchAsyncAPISchemas(t *testing.T) {
	ctx := context.Background()
	drainOutbox(t)
	repo := NewRepository(appPool)
	created := createTenant(t, repo, uniqueSlug("contrato"), domain.PlanEssential)

	plan := domain.PlanEnterprise
	next, _, err := created.Apply(domain.TenantPatch{Plan: &plan}, created.UpdatedAt.Add(1))
	if err != nil {
		t.Fatal(err)
	}
	features := domain.EffectiveFeatures(domain.Catalog[plan], nil)
	changed := domain.NewTenantPlanChangedEvent(uuid.New(), created, next, features)
	if err := repo.UpdateTenant(ctx, next, &changed); err != nil {
		t.Fatal(err)
	}

	pub := &fakePublisher{}
	relay := NewRelay(appPool, pub, 10, &countingMetrics{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := relay.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}

	schemas := map[string]*jsonschema.Schema{
		domain.EventTenantCreated:     compileSchema(t, "tenant-criado.v1.json"),
		domain.EventTenantPlanChanged: compileSchema(t, "tenant-plano-alterado.v1.json"),
	}
	if len(pub.msgs) != 2 {
		t.Fatalf("esperava 2 eventos, obteve %d", len(pub.msgs))
	}
	for _, m := range pub.msgs {
		s := schemas[m.Headers["event_type"]]
		if s == nil {
			t.Fatalf("evento sem schema no contrato: %s", m.Headers["event_type"])
		}
		if err := validateAgainst(t, s, m.Value); err != nil {
			t.Errorf("%s viola o contrato: %v\n%s", m.Headers["event_type"], err, m.Value)
		}
	}

	// O schema precisa recusar um evento sem tenant_id: prova que o teste não passa por vacuidade.
	broken := []byte(`{"event_id":"8f1d7a52-3f0e-4a8e-9a57-6b0e1f2d3c4b","event_type":"TenantCriado","event_version":1,
		"aggregate_id":"8f1d7a52-3f0e-4a8e-9a57-6b0e1f2d3c4b","occurred_at":"2026-11-21T10:00:00Z","producer":"tenant-service",
		"data":{"slug":"abc","name":"A","plan":"essencial","features":[]}}`)
	if err := validateAgainst(t, schemas[domain.EventTenantCreated], broken); err == nil {
		t.Fatal("schema deveria recusar evento sem tenant_id")
	}
}
