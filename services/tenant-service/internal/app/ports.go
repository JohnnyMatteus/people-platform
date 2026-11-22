// Package app orquestra os casos de uso do tenant-service. Depende só do domínio e das
// portas declaradas aqui; os adaptadores (Postgres, HTTP, Kafka) implementam as portas.
package app

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/domain"
)

// Repository é a porta de persistência. Toda operação que recebe tenantID roda no escopo
// daquele tenant (no Postgres, com Row-Level Security). As operações de plataforma
// (criar, listar, resolver host) são as únicas que enxergam vários tenants.
type Repository interface {
	// CreateTenant grava o tenant e o evento na outbox na mesma transação.
	CreateTenant(ctx context.Context, t domain.Tenant, evt domain.Event) error
	ListTenants(ctx context.Context) ([]domain.Tenant, error)
	GetTenant(ctx context.Context, tenantID uuid.UUID) (domain.Tenant, error)
	// UpdateTenant grava a alteração e, se evt não for nil, o evento na mesma transação.
	UpdateTenant(ctx context.Context, t domain.Tenant, evt *domain.Event) error

	// RequestErasure grava o pedido de exclusão e o evento na outbox na mesma transação, no escopo do tenant.
	RequestErasure(ctx context.Context, req domain.ErasureRequest, evt domain.Event) error

	GetTheme(ctx context.Context, tenantID uuid.UUID) (domain.Theme, bool, error)
	SaveTheme(ctx context.Context, tenantID uuid.UUID, theme domain.Theme) error

	GetOverrides(ctx context.Context, tenantID uuid.UUID) (map[domain.Feature]bool, error)
	SaveOverride(ctx context.Context, tenantID uuid.UUID, f domain.Feature, enabled bool) error

	AddDomain(ctx context.Context, tenantID uuid.UUID, host string) error
	ListDomains(ctx context.Context, tenantID uuid.UUID) ([]string, error)
	// ResolveHost devolve o tenant dono do domínio próprio, ou domain.ErrNotFound.
	ResolveHost(ctx context.Context, host string) (uuid.UUID, error)
}

// AssetStorage é a porta do armazenamento de objetos (S3 na AWS, LocalStack no ambiente local).
// O navegador envia o arquivo direto ao bucket por URL pré-assinada: o serviço nunca trafega
// o binário, só autoriza o upload.
type AssetStorage interface {
	PresignUpload(ctx context.Context, key, contentType string, ttl time.Duration) (string, error)
	PublicURL(key string) string
}

// Clock e IDGenerator existem para testes determinísticos.
type Clock func() time.Time

// IDGenerator gera identificadores (UUID v7 em produção, ordenáveis por tempo).
type IDGenerator func() uuid.UUID
