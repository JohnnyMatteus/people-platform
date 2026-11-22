package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/domain"
	"github.com/google/uuid"
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

// selectivePublisher recusa as mensagens cujo event_id está em reject e registra as demais na ordem em que saíram.
type selectivePublisher struct {
	mu     sync.Mutex
	reject map[string]bool
	sent   []Envelope
}

func (s *selectivePublisher) Publish(_ context.Context, msgs []Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var accepted []Envelope
	for _, m := range msgs {
		var env Envelope
		if err := json.Unmarshal(m.Value, &env); err != nil {
			return err
		}
		if s.reject[env.EventID.String()] {
			return errors.New("mensagem recusada pelo broker")
		}
		accepted = append(accepted, env)
	}
	s.sent = append(s.sent, accepted...)
	return nil
}

// seedOutbox grava n eventos pendentes do tenant, em ordem crescente de occurred_at, e devolve os ids.
func seedOutbox(t *testing.T, tenant uuid.UUID, start time.Time, n int) []uuid.UUID {
	t.Helper()
	ids := make([]uuid.UUID, n)
	for i := range ids {
		ids[i] = uuid.New()
		_, err := ownerPool.Exec(context.Background(), `INSERT INTO outbox
			(id, tenant_id, aggregate_type, aggregate_id, event_type, event_version, payload, occurred_at)
			VALUES ($1, $2, 'tenant', $2, 'TenantCriado', 1, '{}', $3)`, ids[i], tenant, start.Add(time.Duration(i)*time.Second))
		if err != nil {
			t.Fatal(err)
		}
	}
	return ids
}

func outboxState(t *testing.T, id uuid.UUID) (attempts int, published bool, lastErr *string) {
	t.Helper()
	var at *time.Time
	err := ownerPool.QueryRow(context.Background(), `SELECT attempts, published_at, last_error FROM outbox WHERE id = $1`, id).
		Scan(&attempts, &at, &lastErr)
	if err != nil {
		t.Fatal(err)
	}
	return attempts, at != nil, lastErr
}

func TestRelay_PoisonedTenantDoesNotStarveOthersAndKeepsOrder(t *testing.T) {
	ctx := context.Background()
	drainOutbox(t)
	start := time.Now().Add(-time.Hour)
	tenantA, tenantB := uuid.New(), uuid.New()
	// A é a mais antiga e tem mais pendentes que o lote (5): sozinha, esgotaria todas as buscas.
	a := seedOutbox(t, tenantA, start, 7)
	b := seedOutbox(t, tenantB, start.Add(time.Minute), 4)

	pub := &selectivePublisher{reject: map[string]bool{a[1].String(): true, b[2].String(): true}}
	metrics := &countingMetrics{}
	relay := NewRelay(appPool, pub, 5, metrics, slog.New(slog.NewTextHandler(io.Discard, nil)))

	n, err := relay.RunOnce(ctx)
	if err == nil {
		t.Fatal("esperava o erro das mensagens recusadas")
	}

	// A publica só a primeira, e B só as duas primeiras: cada tenant para na sua mensagem recusada.
	got := map[uuid.UUID][]uuid.UUID{}
	for _, env := range pub.sent {
		got[env.TenantID] = append(got[env.TenantID], env.EventID)
	}
	if n != 3 || len(got[tenantA]) != 1 || got[tenantA][0] != a[0] || len(got[tenantB]) != 2 || got[tenantB][0] != b[0] || got[tenantB][1] != b[1] {
		t.Fatalf("publicados=%d por tenant=%v", n, got)
	}
	if metrics.published != 3 || metrics.failed != 2 {
		t.Fatalf("métricas: %+v", metrics)
	}

	// attempts sobe só na linha que falhou; as que esperavam atrás dela não foram tentadas.
	wantAttempts := map[uuid.UUID]int{a[1]: 1, b[2]: 1}
	for _, id := range append(append([]uuid.UUID{}, a...), b...) {
		attempts, _, _ := outboxState(t, id)
		if attempts != wantAttempts[id] {
			t.Fatalf("attempts de %s = %d, esperado %d", id, attempts, wantAttempts[id])
		}
	}
	if _, published, lastErr := outboxState(t, a[1]); published || lastErr == nil {
		t.Fatalf("a linha recusada deve seguir pendente com last_error (published=%v, last_error=%v)", published, lastErr)
	}
	for _, id := range []uuid.UUID{a[2], b[3]} {
		if _, published, _ := outboxState(t, id); published {
			t.Fatalf("%s saiu à frente da mensagem recusada do mesmo tenant", id)
		}
	}

	// Com o broker aceitando de novo, a fila de cada tenant sai em ordem.
	pub.reject = nil
	pub.sent = nil
	for {
		n, err := relay.RunOnce(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			break
		}
	}
	var order []uuid.UUID
	for _, env := range pub.sent {
		if env.TenantID == tenantA {
			order = append(order, env.EventID)
		}
	}
	if len(order) != 6 || order[0] != a[1] || order[5] != a[6] {
		t.Fatalf("ordem de A depois da recuperação: %v", order)
	}
}
