package kafka

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/provisioning"
)

type scripted struct {
	results []error
	calls   int
}

func (s *scripted) HandleEvent(context.Context, []byte) (bool, error) {
	err := s.results[min(s.calls, len(s.results)-1)]
	s.calls++
	return err == nil, err
}

func consumer(h Handler) *Consumer {
	return &Consumer{handler: h, logger: slog.New(slog.NewTextHandler(io.Discard, nil)), retry: time.Millisecond}
}

func TestProcess_AdvancesOnSuccessAndOnPoisonMessage(t *testing.T) {
	ctx := context.Background()
	if !consumer(&scripted{results: []error{nil}}).process(ctx, &kgo.Record{}) {
		t.Fatal("sucesso deve avançar o offset")
	}
	poison := &scripted{results: []error{provisioning.ErrPermanent}}
	if !consumer(poison).process(ctx, &kgo.Record{}) || poison.calls != 1 {
		t.Fatalf("mensagem inválida é descartada sem repetir (chamadas=%d)", poison.calls)
	}
}

func TestProcess_RetriesTransientFailureWithoutSkippingTheMessage(t *testing.T) {
	h := &scripted{results: []error{errors.New("banco fora"), errors.New("banco fora"), nil}}
	if !consumer(h).process(context.Background(), &kgo.Record{}) || h.calls != 3 {
		t.Fatalf("deveria repetir até dar certo (chamadas=%d)", h.calls)
	}
}

func TestProcess_StopsRetryingWhenCancelledAndDoesNotCommit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := &scripted{results: []error{errors.New("banco fora")}}
	if consumer(h).process(ctx, &kgo.Record{}) {
		t.Fatal("com o contexto cancelado e a falha pendente o offset não pode avançar")
	}
}
