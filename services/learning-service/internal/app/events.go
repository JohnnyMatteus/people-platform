package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/domain"
)

// Tipos de evento que o serviço consome.
const (
	EventCandidatoContratado       = "CandidatoContratado"
	EventTitularExclusaoSolicitada = "TitularExclusaoSolicitada"
)

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

// HandleEvent consome uma mensagem dos tópicos de recrutamento e de privacidade. Devolve true se produziu efeito e false se a mensagem
// era repetida ou de outro tipo. Mensagem malformada devolve erro (o consumidor decide o que fazer, ver adapters/kafka).
// A entrega do Kafka é pelo menos uma vez: a deduplicação por event_id e o efeito saem na mesma transação do tenant
// do próprio evento.
func (s *Service) HandleEvent(ctx context.Context, raw []byte) (bool, error) {
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return false, fmt.Errorf("%w: envelope: %v", domain.ErrInvalidInput, err)
	}
	if env.EventType != EventCandidatoContratado && env.EventType != EventTitularExclusaoSolicitada {
		return false, nil
	}
	if env.EventVersion != 1 || env.EventID == uuid.Nil || env.TenantID == uuid.Nil {
		return false, fmt.Errorf("%w: evento sem versão 1, event_id ou tenant_id", domain.ErrInvalidInput)
	}
	if env.EventType == EventTitularExclusaoSolicitada {
		return s.eraseSubject(ctx, env)
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

type erasureData struct {
	RequestID uuid.UUID `json:"request_id"`
	SubjectID uuid.UUID `json:"subject_id"`
}

// eraseSubject apaga o que o serviço guarda sobre o titular no tenant do evento: matrículas, progresso e as atribuições
// de integração já vinculadas a ele. É idempotente (claim por event_id) e só toca o tenant do envelope.
func (s *Service) eraseSubject(ctx context.Context, env envelope) (bool, error) {
	var d erasureData
	if err := json.Unmarshal(env.Data, &d); err != nil || d.SubjectID == uuid.Nil {
		return false, fmt.Errorf("%w: dados de TitularExclusaoSolicitada", domain.ErrInvalidInput)
	}
	applied := false
	err := s.store.InTenant(ctx, env.TenantID, func(tx Tx) error {
		first, err := tx.ProcessedEvents().Claim(ctx, env.EventID, env.EventType, s.now())
		if err != nil || !first {
			return err
		}
		assignments, err := tx.Onboarding().DeleteForEmployee(ctx, d.SubjectID)
		if err != nil {
			return err
		}
		enrollments, err := tx.Enrollments().DeleteForEmployee(ctx, d.SubjectID)
		if err != nil {
			return err
		}
		applied = assignments+enrollments > 0
		return nil
	})
	return applied, err
}
