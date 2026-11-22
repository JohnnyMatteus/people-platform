// Package provisioning prepara a identidade de um tenant novo: consome TenantCriado e garante a Organization
// correspondente no Keycloak (ADR 0004). O Keycloak fica atrás de uma porta; o adaptador real está em adapters/keycloak.
package provisioning

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/domain"
)

// Organization é o que a identidade precisa saber de um tenant.
type Organization struct {
	TenantID uuid.UUID
	Slug     string
	Name     string
}

// IdentityProvisioner garante que o tenant tem uma Organization no provedor de identidade. EnsureOrganization é
// idempotente: chamar de novo para o mesmo tenant não cria outra nem falha.
type IdentityProvisioner interface {
	EnsureOrganization(ctx context.Context, org Organization) error
}

// ErrPermanent marca uma falha que repetir não resolve (dado recusado pelo provedor). O consumidor descarta a mensagem
// com log; qualquer outro erro é transitório e a mensagem é reentregue.
var ErrPermanent = errors.New("falha permanente no provisionamento")

type envelope struct {
	EventType    string                   `json:"event_type"`
	EventVersion int                      `json:"event_version"`
	TenantID     uuid.UUID                `json:"tenant_id"`
	Data         domain.TenantCreatedData `json:"data"`
}

// Handler traduz mensagens do tópico de tenants em chamadas à porta.
type Handler struct{ provisioner IdentityProvisioner }

func NewHandler(p IdentityProvisioner) *Handler { return &Handler{provisioner: p} }

// HandleEvent devolve true quando provisionou. Outros tipos de evento do mesmo tópico são ignorados (false, nil).
// Mensagem malformada devolve ErrPermanent. A idempotência vem da própria porta, então a reentrega at-least-once
// do Kafka é segura sem tabela de eventos processados.
func (h *Handler) HandleEvent(ctx context.Context, raw []byte) (bool, error) {
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return false, fmt.Errorf("%w: envelope: %v", ErrPermanent, err)
	}
	if env.EventType != domain.EventTenantCreated {
		return false, nil
	}
	if env.TenantID == uuid.Nil || env.Data.Slug == "" || env.Data.Name == "" {
		return false, fmt.Errorf("%w: TenantCriado sem tenant_id, slug ou name", ErrPermanent)
	}
	err := h.provisioner.EnsureOrganization(ctx, Organization{TenantID: env.TenantID, Slug: env.Data.Slug, Name: env.Data.Name})
	return err == nil, err
}
