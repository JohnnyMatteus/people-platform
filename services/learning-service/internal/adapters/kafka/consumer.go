// Package kafka consome o tópico de recrutamento com franz-go.
package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/domain"
)

// Handler processa o valor de uma mensagem. Erro de domínio (mensagem malformada) não adianta repetir; qualquer outro
// erro (banco fora do ar, por exemplo) é transitório e a mensagem é reentregue.
type Handler interface {
	HandleEvent(ctx context.Context, raw []byte) (bool, error)
}

// Consumer lê um tópico em grupo, com commit manual: o offset só avança depois que o efeito foi gravado, então a
// entrega é pelo menos uma vez e a deduplicação por event_id fica no caso de uso.
type Consumer struct {
	client  *kgo.Client
	handler Handler
	logger  *slog.Logger
	retry   time.Duration
}

func NewConsumer(brokers []string, topic, group string, h Handler, logger *slog.Logger) (*Consumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topic),
		kgo.DisableAutoCommit(),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.AllowAutoTopicCreation(),
	)
	if err != nil {
		return nil, fmt.Errorf("criar consumidor Kafka: %w", err)
	}
	return &Consumer{client: client, handler: h, logger: logger, retry: 2 * time.Second}, nil
}

func (c *Consumer) Close() { c.client.Close() }

// Ping verifica a conexão com os brokers (usado no /readyz).
func (c *Consumer) Ping(ctx context.Context) error { return c.client.Ping(ctx) }

// Run consome até o contexto ser cancelado.
func (c *Consumer) Run(ctx context.Context) {
	for ctx.Err() == nil {
		fetches := c.client.PollFetches(ctx)
		if fetches.IsClientClosed() {
			return
		}
		fetches.EachError(func(topic string, partition int32, err error) {
			if !errors.Is(err, context.Canceled) {
				c.logger.WarnContext(ctx, "erro ao ler o tópico", slog.String("topic", topic), slog.Int("partition", int(partition)), slog.String("error", err.Error()))
			}
		})
		fetches.EachRecord(func(r *kgo.Record) {
			if c.process(ctx, r) {
				if err := c.client.CommitRecords(ctx, r); err != nil && ctx.Err() == nil {
					c.logger.WarnContext(ctx, "falha ao confirmar offset", slog.String("error", err.Error()))
				}
			}
		})
	}
}

// process devolve true quando o offset pode avançar: o efeito foi gravado, a mensagem era repetida ou era inválida.
// Nos erros transitórios repete com espera até dar certo ou o contexto acabar, sem pular a mensagem: ordem por tenant.
func (c *Consumer) process(ctx context.Context, r *kgo.Record) bool {
	for {
		applied, err := c.handler.HandleEvent(ctx, r.Value)
		switch {
		case err == nil:
			if applied {
				c.logger.InfoContext(ctx, "evento aplicado", slog.Int64("offset", r.Offset), slog.Int("partition", int(r.Partition)))
			}
			return true
		case errors.Is(err, domain.ErrInvalidInput):
			c.logger.ErrorContext(ctx, "mensagem inválida descartada", slog.Int64("offset", r.Offset), slog.String("error", err.Error()))
			return true
		}
		c.logger.WarnContext(ctx, "falha transitória ao processar, tentando de novo", slog.String("error", err.Error()))
		select {
		case <-ctx.Done():
			return false
		case <-time.After(c.retry):
		}
	}
}
