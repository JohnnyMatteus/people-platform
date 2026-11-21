package domain

import (
	"time"

	"github.com/google/uuid"
)

// Tipos de evento publicados pelo tenant-service (ver contracts/asyncapi).
const (
	EventTenantCreated     = "TenantCriado"
	EventTenantPlanChanged = "TenantPlanoAlterado"
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
