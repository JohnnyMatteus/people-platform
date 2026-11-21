package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/domain"
)

type fakePublisher struct {
	mu   sync.Mutex
	msgs []Message
	err  error
}

func (f *fakePublisher) Publish(_ context.Context, msgs []Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.msgs = append(f.msgs, msgs...)
	return nil
}

type countingMetrics struct{ published, failed int }

func (c *countingMetrics) Published(n int) { c.published += n }
func (c *countingMetrics) Failed()         { c.failed++ }

func drainOutbox(t *testing.T) {
	t.Helper()
	if _, err := ownerPool.Exec(context.Background(), "UPDATE outbox SET published_at = now() WHERE published_at IS NULL"); err != nil {
		t.Fatal(err)
	}
}

func TestRelay_PublishesPendingEventsOnce(t *testing.T) {
	ctx := context.Background()
	drainOutbox(t)
	repo := NewRepository(appPool)
	tn := createTenant(t, repo, uniqueSlug("horizonte"), domain.PlanGrowth)

	pub := &fakePublisher{}
	metrics := &countingMetrics{}
	relay := NewRelay(appPool, pub, 10, metrics, slog.New(slog.NewTextHandler(io.Discard, nil)))

	n, err := relay.RunOnce(ctx)
	if err != nil || n != 1 {
		t.Fatalf("RunOnce = %d, %v", n, err)
	}
	msg := pub.msgs[0]
	if msg.Topic != TopicTenantEvents || msg.Key != tn.ID.String() || msg.Headers["event_type"] != domain.EventTenantCreated {
		t.Fatalf("mensagem inesperada: %+v", msg)
	}
	var env Envelope
	if err := json.Unmarshal(msg.Value, &env); err != nil {
		t.Fatal(err)
	}
	var data domain.TenantCreatedData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatal(err)
	}
	if env.TenantID != tn.ID || env.EventVersion != 1 || env.Producer != "tenant-service" || data.Slug != tn.Slug {
		t.Fatalf("envelope inesperado: %+v / %+v", env, data)
	}

	if n, err := relay.RunOnce(ctx); err != nil || n != 0 {
		t.Fatalf("segunda rodada não deve republicar: %d %v", n, err)
	}
	if metrics.published != 1 {
		t.Fatalf("métrica de publicados = %d", metrics.published)
	}
}

func TestRelay_FailureKeepsEventsPendingAndCountsAttempts(t *testing.T) {
	ctx := context.Background()
	drainOutbox(t)
	repo := NewRepository(appPool)
	tn := createTenant(t, repo, uniqueSlug("sol"), domain.PlanEssential)

	pub := &fakePublisher{err: errors.New("broker indisponível")}
	metrics := &countingMetrics{}
	relay := NewRelay(appPool, pub, 10, metrics, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if _, err := relay.RunOnce(ctx); err == nil {
		t.Fatal("esperava erro de publicação")
	}
	var attempts int
	var lastErr *string
	var publishedAt any
	err := ownerPool.QueryRow(ctx, `SELECT attempts, last_error, published_at FROM outbox WHERE tenant_id = $1`, tn.ID).
		Scan(&attempts, &lastErr, &publishedAt)
	if err != nil || attempts != 1 || lastErr == nil || publishedAt != nil {
		t.Fatalf("estado após falha: attempts=%d last_error=%v published_at=%v err=%v", attempts, lastErr, publishedAt, err)
	}
	if metrics.failed != 1 {
		t.Fatalf("métrica de falhas = %d", metrics.failed)
	}

	pub.err = nil
	if n, err := relay.RunOnce(ctx); err != nil || n != 1 {
		t.Fatalf("após o broker voltar, o evento deve sair: %d %v", n, err)
	}
}

func TestRelay_ConcurrentRelaysDoNotDuplicate(t *testing.T) {
	ctx := context.Background()
	drainOutbox(t)
	repo := NewRepository(appPool)
	const total = 20
	for i := 0; i < total; i++ {
		createTenant(t, repo, uniqueSlug("t"), domain.PlanEssential)
	}

	pub := &fakePublisher{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			relay := NewRelay(appPool, pub, 3, &countingMetrics{}, logger)
			for {
				n, err := relay.RunOnce(ctx)
				if err != nil {
					t.Error(err)
					return
				}
				if n == 0 {
					return
				}
			}
		}()
	}
	wg.Wait()

	seen := map[string]bool{}
	for _, m := range pub.msgs {
		var env Envelope
		_ = json.Unmarshal(m.Value, &env)
		if seen[env.EventID.String()] {
			t.Fatalf("evento %s publicado duas vezes", env.EventID)
		}
		seen[env.EventID.String()] = true
	}
	if len(seen) != total {
		t.Fatalf("esperava %d eventos, publicou %d", total, len(seen))
	}
}
