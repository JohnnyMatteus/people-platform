package domain

import (
	"time"

	"github.com/google/uuid"
)

// Tipos de evento publicados pelo tenant-service (ver contracts/asyncapi).
const (
	EventTenantCreated     = "TenantCriado"
	EventTenantPlanChanged = "TenantPlanoAlterado"
	// EventDataSubjectErasureRequested vai ao tópico de privacidade, não ao de tenants.
	EventDataSubjectErasureRequested = "TitularExclusaoSolicitada"
	// EventSchemaVersion é a versão do envelope e do payload; mudanças incompatíveis criam v2.
	EventSchemaVersion = 1
)

// Event é um evento de domínio pronto para ir à outbox. O payload segue o schema versionado.
type Event struct {
	ID          uuid.UUID
	Type        string
	Version     int
	TenantID    uuid.UUID
	AggregateID uuid.UUID
	OccurredAt  time.Time
	Data        any
}

// TenantCreatedData é o payload de TenantCriado.
type TenantCreatedData struct {
	Slug     string    `json:"slug"`
	Name     string    `json:"name"`
	Plan     PlanCode  `json:"plan"`
	Features []Feature `json:"features"`
}

// TenantPlanChangedData é o payload de TenantPlanoAlterado.
type TenantPlanChangedData struct {
	PreviousPlan PlanCode  `json:"previous_plan"`
	Plan         PlanCode  `json:"plan"`
	Features     []Feature `json:"features"`
}

// NewTenantCreatedEvent monta o evento TenantCriado.
func NewTenantCreatedEvent(id uuid.UUID, t Tenant) Event {
	plan := Catalog[t.Plan]
	return Event{
		ID: id, Type: EventTenantCreated, Version: EventSchemaVersion,
		TenantID: t.ID, AggregateID: t.ID, OccurredAt: t.CreatedAt,
		Data: TenantCreatedData{Slug: t.Slug, Name: t.Name, Plan: t.Plan, Features: EffectiveFeatures(plan, nil)},
	}
}

// NewTenantPlanChangedEvent monta o evento TenantPlanoAlterado com as features efetivas.
func NewTenantPlanChangedEvent(id uuid.UUID, before, after Tenant, features []Feature) Event {
	return Event{
		ID: id, Type: EventTenantPlanChanged, Version: EventSchemaVersion,
		TenantID: after.ID, AggregateID: after.ID, OccurredAt: after.UpdatedAt,
		Data: TenantPlanChangedData{PreviousPlan: before.Plan, Plan: after.Plan, Features: features},
	}
}

// ErasureRequestedData é o payload de TitularExclusaoSolicitada: só identificadores, nunca dado pessoal.
type ErasureRequestedData struct {
	RequestID uuid.UUID `json:"request_id"`
	SubjectID uuid.UUID `json:"subject_id"`
}

// ErasureRequest é o registro do pedido de exclusão, guardado no tenant-service fora do barramento.
type ErasureRequest struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	SubjectID   uuid.UUID
	RequestedBy string // sub do token de quem pediu
	RequestedAt time.Time
}

// NewErasureRequestedEvent monta o evento que cada serviço consome para apagar os dados do titular.
func NewErasureRequestedEvent(eventID uuid.UUID, r ErasureRequest) Event {
	return Event{
		ID: eventID, Type: EventDataSubjectErasureRequested, Version: EventSchemaVersion,
		TenantID: r.TenantID, AggregateID: r.SubjectID, OccurredAt: r.RequestedAt,
		Data: ErasureRequestedData{RequestID: r.ID, SubjectID: r.SubjectID},
	}
}
