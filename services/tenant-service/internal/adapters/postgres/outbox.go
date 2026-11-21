package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TopicTenantEvents é o tópico Kafka dos eventos de tenant (ver contracts/asyncapi).
const TopicTenantEvents = "people.tenant.events.v1"

// Message é uma mensagem pronta para o barramento.
type Message struct {
	Topic   string
	Key     string
	Value   []byte
	Headers map[string]string
}

// Publisher publica um lote de mensagens; só retorna nil se todas foram confirmadas.
type Publisher interface {
	Publish(ctx context.Context, msgs []Message) error
}

// Envelope é o formato comum de todos os eventos da plataforma.
type Envelope struct {
	EventID      uuid.UUID       `json:"event_id"`
	EventType    string          `json:"event_type"`
	EventVersion int             `json:"event_version"`
	TenantID     uuid.UUID       `json:"tenant_id"`
	AggregateID  uuid.UUID       `json:"aggregate_id"`
	OccurredAt   time.Time       `json:"occurred_at"`
	Producer     string          `json:"producer"`
	Data         json.RawMessage `json:"data"`
}

// RelayMetrics recebe as métricas do relay (implementado com Prometheus em main).
type RelayMetrics interface {
	Published(n int)
	Failed()
}

// Relay lê a outbox e publica os eventos pendentes. Vários relays podem rodar ao mesmo tempo:
// FOR UPDATE SKIP LOCKED garante que cada linha é processada por um só.
type Relay struct {
	pool      *pgxpool.Pool
	publisher Publisher
	batchSize int
	metrics   RelayMetrics
	logger    *slog.Logger
}

// NewRelay cria o relay.
func NewRelay(pool *pgxpool.Pool, p Publisher, batchSize int, m RelayMetrics, logger *slog.Logger) *Relay {
	return &Relay{pool: pool, publisher: p, batchSize: batchSize, metrics: m, logger: logger}
}

type pendingRow struct {
	id          uuid.UUID
	env         Envelope
	traceparent *string
}

// RunOnce publica um lote. Devolve quantos eventos publicou. Se a publicação falhar, as
// linhas ficam pendentes com attempts incrementado e o erro é devolvido.
func (r *Relay) RunOnce(ctx context.Context) (int, error) {
	published := 0
	var publishErr error
	err := asPlatform(ctx, r.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id, tenant_id, aggregate_id, event_type, event_version, payload,
				traceparent, occurred_at
			FROM outbox WHERE published_at IS NULL
			ORDER BY occurred_at, id LIMIT $1 FOR UPDATE SKIP LOCKED`, r.batchSize)
		if err != nil {
			return err
		}
		pending, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (pendingRow, error) {
			var p pendingRow
			err := row.Scan(&p.id, &p.env.TenantID, &p.env.AggregateID, &p.env.EventType, &p.env.EventVersion,
				&p.env.Data, &p.traceparent, &p.env.OccurredAt)
			p.env.EventID, p.env.Producer = p.id, "tenant-service"
			return p, err
		})
		if err != nil || len(pending) == 0 {
			return err
		}

		msgs := make([]Message, 0, len(pending))
		ids := make([]uuid.UUID, 0, len(pending))
		for _, p := range pending {
			value, err := json.Marshal(p.env)
			if err != nil {
				return fmt.Errorf("serializar envelope %s: %w", p.id, err)
			}
			headers := map[string]string{"event_type": p.env.EventType, "event_version": fmt.Sprint(p.env.EventVersion)}
			if p.traceparent != nil {
				headers["traceparent"] = *p.traceparent
			}
			// Chave = tenant_id: eventos do mesmo tenant caem na mesma partição e mantêm a ordem.
			msgs = append(msgs, Message{Topic: TopicTenantEvents, Key: p.env.TenantID.String(), Value: value, Headers: headers})
			ids = append(ids, p.id)
		}

		if publishErr = r.publisher.Publish(ctx, msgs); publishErr != nil {
			_, err := tx.Exec(ctx, `UPDATE outbox SET attempts = attempts + 1, last_error = $2 WHERE id = ANY($1)`,
				ids, truncate(publishErr.Error(), 500))
			return err
		}
		// Se o commit falhar depois do publish, os eventos saem de novo: por isso a entrega é
		// at-least-once e os consumidores deduplicam por event_id.
		if _, err := tx.Exec(ctx, `UPDATE outbox SET published_at = now(), last_error = NULL WHERE id = ANY($1)`, ids); err != nil {
			return err
		}
		published = len(ids)
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("relay da outbox: %w", err)
	}
	if publishErr != nil {
		r.metrics.Failed()
		return 0, fmt.Errorf("publicar eventos: %w", publishErr)
	}
	r.metrics.Published(published)
	return published, nil
}

// Run executa RunOnce em intervalo até o contexto ser cancelado. Lotes cheios disparam a
// próxima rodada imediatamente para esvaziar a fila sem esperar o intervalo.
func (r *Relay) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		n, err := r.RunOnce(ctx)
		if err != nil {
			r.logger.ErrorContext(ctx, "falha no relay da outbox", slog.String("error", err.Error()))
		}
		if n == r.batchSize {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
