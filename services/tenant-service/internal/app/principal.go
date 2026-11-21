package app

import "github.com/google/uuid"

// Papéis reconhecidos, emitidos pelo Keycloak no token.
const (
	RolePlatformAdmin = "platform-admin"
	RoleTenantAdmin   = "tenant-admin"
)

// Principal é a identidade autenticada que chama um caso de uso.
type Principal struct {
	Subject  string
	TenantID uuid.UUID // claim tenant_id; uuid.Nil para operadores da plataforma
	Roles    []string
}

// HasRole informa se o principal tem o papel.
func (p Principal) HasRole(role string) bool {
	for _, r := range p.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// IsPlatformAdmin é o operador da plataforma, que administra todos os tenants.
func (p Principal) IsPlatformAdmin() bool { return p.HasRole(RolePlatformAdmin) }

// CanRead permite ler dados de um tenant: operador da plataforma ou membro do próprio tenant.
func (p Principal) CanRead(tenantID uuid.UUID) bool {
	return p.IsPlatformAdmin() || (p.TenantID != uuid.Nil && p.TenantID == tenantID)
}

// CanManage permite alterar tema, features e domínios: operador ou admin do próprio tenant.
func (p Principal) CanManage(tenantID uuid.UUID) bool {
	return p.IsPlatformAdmin() || (p.HasRole(RoleTenantAdmin) && p.TenantID == tenantID)
}
