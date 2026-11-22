package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/domain"
)

// EventCandidatoContratado é o tipo do evento do recruitment-service que dispara a integração.
const EventCandidatoContratado = "CandidatoContratado"

type envelope struct {
	EventID      uuid.UUID       `json:"event_id"`
	EventType    string          `json:"event_type"`
	EventVersion int             `json:"event_version"`
	TenantID     uuid.UUID       `json:"tenant_id"`
	Data         json.RawMessage `json:"data"`
}

type hiredData struct {
	ApplicationID uuid.UUID `json:"application_id"`
	PositionTitle string    `json:"position_title"`
	Unit          string    `json:"unit"`
	StartDate     string    `json:"start_date"`
}

// HandleEvent consome uma mensagem do tópico do recrutamento. Devolve true se produziu efeito e false se a mensagem
// era repetida ou de outro tipo. Mensagem malformada devolve erro (o consumidor decide o que fazer, ver adapters/kafka).
// A entrega do Kafka é pelo menos uma vez: a deduplicação por event_id e o efeito saem na mesma transação do tenant
// do próprio evento.
func (s *Service) HandleEvent(ctx context.Context, raw []byte) (bool, error) {
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return false, fmt.Errorf("%w: envelope: %v", domain.ErrInvalidInput, err)
	}
	if env.EventType != EventCandidatoContratado {
		return false, nil
	}
	if env.EventVersion != 1 || env.EventID == uuid.Nil || env.TenantID == uuid.Nil {
		return false, fmt.Errorf("%w: evento sem versão 1, event_id ou tenant_id", domain.ErrInvalidInput)
	}
	var d hiredData
	if err := json.Unmarshal(env.Data, &d); err != nil || d.ApplicationID == uuid.Nil {
		return false, fmt.Errorf("%w: dados de CandidatoContratado", domain.ErrInvalidInput)
	}
	start, err := time.Parse("2006-01-02", d.StartDate)
	if err != nil {
		return false, fmt.Errorf("%w: start_date", domain.ErrInvalidInput)
	}
	applied := false
	err = s.store.InTenant(ctx, env.TenantID, func(tx Tx) error {
		first, err := tx.ProcessedEvents().Claim(ctx, env.EventID, env.EventType, s.now())
		if err != nil || !first {
			return err
		}
		inserted, err := tx.Onboarding().Insert(ctx, domain.OnboardingAssignment{
			ID: s.newID(), TenantID: env.TenantID, ApplicationID: d.ApplicationID, PositionTitle: d.PositionTitle,
			Unit: d.Unit, StartDate: start, CreatedAt: s.now(),
		})
		applied = inserted
		return err
	})
	return applied, err
}
