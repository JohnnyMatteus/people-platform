package postgres

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/domain"
)

func TestErasure_RequestAndEventAreAtomicPublishedToThePrivacyTopicAndIsolated(t *testing.T) {
	ctx := context.Background()
	drainOutbox(t)
	repo := NewRepository(appPool)
	a := createTenant(t, repo, uniqueSlug("horizonte"), domain.PlanGrowth)
	b := createTenant(t, repo, uniqueSlug("sol"), domain.PlanGrowth)
	drainOutbox(t)

	req := domain.ErasureRequest{ID: uuid.New(), TenantID: a.ID, SubjectID: uuid.New(), RequestedBy: "adm", RequestedAt: time.Now().UTC()}
	if err := repo.RequestErasure(ctx, req, domain.NewErasureRequestedEvent(uuid.New(), req)); err != nil {
		t.Fatal(err)
	}

	var n int
	if err := ownerPool.QueryRow(ctx, `SELECT count(*) FROM erasure_requests WHERE id = $1`, req.ID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("pedido gravado: %d %v", n, err)
	}

	pub := &fakePublisher{}
	relay := NewRelay(appPool, pub, 10, &countingMetrics{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if got, err := relay.RunOnce(ctx); err != nil || got != 1 {
		t.Fatalf("RunOnce = %d, %v", got, err)
	}
	msg := pub.msgs[0]
	if msg.Topic != TopicPrivacyEvents || msg.Key != a.ID.String() || msg.Headers["event_type"] != domain.EventDataSubjectErasureRequested {
		t.Fatalf("mensagem: %+v", msg)
	}
	if err := validateAgainst(t, compileSchema(t, "titular-exclusao-solicitada.v1.json"), msg.Value); err != nil {
		t.Fatalf("o evento viola o contrato: %v\n%s", err, msg.Value)
	}

	// RLS: no escopo de B o pedido do A não existe, e sem escopo nenhum pedido é visível.
	if err := inTenantCount(t, b.ID, `SELECT count(*) FROM erasure_requests`, 0); err != nil {
		t.Fatal(err)
	}
	if err := appPool.QueryRow(ctx, `SELECT count(*) FROM erasure_requests`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("sem escopo deveria ver 0 pedidos: %d %v", n, err)
	}
	if err := inTenantCount(t, a.ID, `SELECT count(*) FROM erasure_requests`, 1); err != nil {
		t.Fatal(err)
	}
}

func inTenantCount(t *testing.T, tenant uuid.UUID, query string, want int) error {
	t.Helper()
	ctx := context.Background()
	return inTenant(ctx, appPool, tenant, func(tx pgx.Tx) error {
		var n int
		if err := tx.QueryRow(ctx, query).Scan(&n); err != nil {
			return err
		}
		if n != want {
			t.Errorf("%s no escopo de %s = %d, esperado %d", query, tenant, n, want)
		}
		return nil
	})
}
