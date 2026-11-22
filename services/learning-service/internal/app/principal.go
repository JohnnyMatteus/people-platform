package app

import (
	"github.com/google/uuid"

	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/domain"
)

const (
	RoleTenantAdmin = "tenant-admin"
	RoleManager     = "manager"
)

// Principal é a identidade autenticada. O tenant vem sempre da claim do token, nunca da requisição.
type Principal struct {
	Subject  string
	TenantID uuid.UUID
	Roles    []string
}

func (p Principal) HasRole(role string) bool {
	for _, r := range p.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// EmployeeID é o colaborador do token (o `sub` do Keycloak, como nos demais serviços).
func (p Principal) EmployeeID() (uuid.UUID, error) {
	id, err := uuid.Parse(p.Subject)
	if err != nil {
		return uuid.Nil, domain.ErrForbidden
	}
	return id, nil
}

// requireTenant recusa token sem tenant (operador da plataforma não usa este serviço).
func (p Principal) requireTenant() error {
	if p.TenantID == uuid.Nil {
		return domain.ErrForbidden
	}
	return nil
}

func (p Principal) requireAdmin() error {
	if err := p.requireTenant(); err != nil {
		return err
	}
	if !p.HasRole(RoleTenantAdmin) {
		return domain.ErrForbidden
	}
	return nil
}
