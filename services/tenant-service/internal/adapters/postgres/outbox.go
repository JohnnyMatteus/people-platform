package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/domain"
)

// TopicTenantEvents é o tópico Kafka dos eventos de tenant (ver contracts/asyncapi).
const TopicTenantEvents = "people.tenant.events.v1"

// TopicPrivacyEvents é o tópico dos pedidos de exclusão de titular (contracts/asyncapi, canal privacyEvents).
const TopicPrivacyEvents = "people.privacy.events.v1"

// topicFor escolhe o tópico pelo tipo do evento: um só relay, uma só outbox, mais de um tópico.
func topicFor(eventType string) string {
	if eventType == domain.EventDataSubjectErasureRequested {
		return TopicPrivacyEvents
	}
	return TopicTenantEvents
}

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

// RunOnce publica um lote e devolve quantos eventos saíram. Os eventos de um tenant são publicados em ordem; se
// um falha, o tenant fica bloqueado nesta rodada (as mensagens seguintes dele esperam, para manter a ordem por
// chave) e é excluído da busca seguinte, então um tenant com mensagem envenenada não esgota o lote nem impede os
// outros de publicar. A falha é gravada (attempts, last_error) só na linha que falhou, e a transação confirma o que
// já foi publicado. O erro devolvido junta as falhas da rodada.
func (r *Relay) RunOnce(ctx context.Context) (int, error) {
	published := 0
	var failures []error
	err := asPlatform(ctx, r.pool, func(tx pgx.Tx) error {
		blocked := []uuid.UUID{}
		for published+len(failures) < r.batchSize {
			pending, err := r.fetch(ctx, tx, r.batchSize-published-len(failures), blocked)
			if err != nil {
				return err
			}
			if len(pending) == 0 {
				return nil
			}
			var order []uuid.UUID
			groups := map[uuid.UUID][]pendingRow{}
			for _, p := range pending {
				if _, ok := groups[p.env.TenantID]; !ok {
					order = append(order, p.env.TenantID)
				}
				groups[p.env.TenantID] = append(groups[p.env.TenantID], p)
			}
			for _, tenant := range order {
				n, failure, err := r.publishTenant(ctx, tx, groups[tenant])
				if err != nil {
					return err
				}
				published += n
				if failure != nil {
					failures = append(failures, failure)
					blocked = append(blocked, tenant)
				}
			}
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("relay da outbox: %w", err)
	}
	if published > 0 {
		r.metrics.Published(published)
	}
	for range failures {
		r.metrics.Failed()
	}
	if len(failures) > 0 {
		return published, fmt.Errorf("publicar eventos: %w", errors.Join(failures...))
	}
	return published, nil
}

func (r *Relay) fetch(ctx context.Context, tx pgx.Tx, limit int, blocked []uuid.UUID) ([]pendingRow, error) {
	rows, err := tx.Query(ctx, `SELECT id, tenant_id, aggregate_id, event_type, event_version, payload,
			traceparent, occurred_at
		FROM outbox WHERE published_at IS NULL AND tenant_id <> ALL($2)
		ORDER BY occurred_at, id LIMIT $1 FOR UPDATE SKIP LOCKED`, limit, blocked)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (pendingRow, error) {
		var p pendingRow
		err := row.Scan(&p.id, &p.env.TenantID, &p.env.AggregateID, &p.env.EventType, &p.env.EventVersion,
			&p.env.Data, &p.traceparent, &p.env.OccurredAt)
		p.env.EventID, p.env.Producer = p.id, "tenant-service"
		return p, err
	})
}

// publishTenant publica as mensagens de um tenant. Primeiro tenta o grupo todo; se o broker recusa, repete uma a uma
// em ordem para descobrir qual linha falha, e para nela. Devolve quantas saíram e a falha, se houve.
func (r *Relay) publishTenant(ctx context.Context, tx pgx.Tx, rows []pendingRow) (int, error, error) {
	msgs := make([]Message, 0, len(rows))
	for _, p := range rows {
		m, err := toMessage(p)
		if err != nil {
			return 0, nil, err
		}
		msgs = append(msgs, m)
	}

	sent := 0
	var failure error
	if err := r.publisher.Publish(ctx, msgs); err == nil {
		sent = len(rows)
	} else {
		failure = err
		for i, m := range msgs {
			if len(msgs) == 1 {
				break
			}
			if err := r.publisher.Publish(ctx, []Message{m}); err != nil {
				failure = err
				break
			}
			sent = i + 1
			failure = nil
		}
	}

	// Se o commit falhar depois do publish, os eventos saem de novo: por isso a entrega é
	// at-least-once e os consumidores deduplicam por event_id.
	if sent > 0 {
		ids := make([]uuid.UUID, sent)
		for i := range ids {
			ids[i] = rows[i].id
		}
		if _, err := tx.Exec(ctx, `UPDATE outbox SET published_at = now(), last_error = NULL WHERE id = ANY($1)`, ids); err != nil {
			return 0, nil, err
		}
	}
	if failure != nil {
		if _, err := tx.Exec(ctx, `UPDATE outbox SET attempts = attempts + 1, last_error = $2 WHERE id = $1`,
			rows[sent].id, truncate(failure.Error(), 500)); err != nil {
			return 0, nil, err
		}
		return sent, failure, nil
	}
	return sent, nil, nil
}

func toMessage(p pendingRow) (Message, error) {
	value, err := json.Marshal(p.env)
	if err != nil {
		return Message{}, fmt.Errorf("serializar envelope %s: %w", p.id, err)
	}
	headers := map[string]string{"event_type": p.env.EventType, "event_version": fmt.Sprint(p.env.EventVersion)}
	if p.traceparent != nil {
		headers["traceparent"] = *p.traceparent
	}
	// Chave = tenant_id: eventos do mesmo tenant caem na mesma partição e mantêm a ordem.
	return Message{Topic: topicFor(p.env.EventType), Key: p.env.TenantID.String(), Value: value, Headers: headers}, nil
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
