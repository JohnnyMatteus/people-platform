// Package kafka implementa o Publisher da outbox com franz-go.
package kafka

import (
	"context"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/adapters/postgres"
)

// Publisher publica mensagens com acks de todas as réplicas e produtor idempotente.
type Publisher struct {
	client *kgo.Client
}

var _ postgres.Publisher = (*Publisher)(nil)

// NewPublisher conecta aos brokers. O produtor idempotente (padrão do franz-go com
// RequiredAcks=AllISR) evita duplicatas por retry dentro da mesma sessão do produtor.
func NewPublisher(brokers []string) (*Publisher, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.AllowAutoTopicCreation(),
	)
	if err != nil {
		return nil, fmt.Errorf("criar cliente Kafka: %w", err)
	}
	return &Publisher{client: client}, nil
}

// Publish envia o lote e espera a confirmação de todas as mensagens.
func (p *Publisher) Publish(ctx context.Context, msgs []postgres.Message) error {
	records := make([]*kgo.Record, 0, len(msgs))
	for _, m := range msgs {
		r := &kgo.Record{Topic: m.Topic, Key: []byte(m.Key), Value: m.Value}
		for k, v := range m.Headers {
			r.Headers = append(r.Headers, kgo.RecordHeader{Key: k, Value: []byte(v)})
		}
		records = append(records, r)
	}
	return p.client.ProduceSync(ctx, records...).FirstErr()
}

// Ping verifica a conexão com o cluster (usado no readiness).
func (p *Publisher) Ping(ctx context.Context) error { return p.client.Ping(ctx) }

// Close encerra o cliente, esperando mensagens em voo.
func (p *Publisher) Close() { p.client.Close() }
