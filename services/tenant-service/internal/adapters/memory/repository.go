// Package memory é um adaptador em memória da porta app.Repository. Serve aos testes de
// caso de uso e de HTTP; o isolamento real por tenant é testado contra Postgres.
package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/google/uuid"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/app"
	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/domain"
)

// Repository guarda tudo em mapas protegidos por mutex.
type Repository struct {
	mu        sync.Mutex
	tenants   map[uuid.UUID]domain.Tenant
	themes    map[uuid.UUID]domain.Theme
	overrides map[uuid.UUID]map[domain.Feature]bool
	domains   map[string]uuid.UUID
	outbox    []domain.Event
	erasures  []domain.ErasureRequest
}

var _ app.Repository = (*Repository)(nil)

// NewRepository cria um repositório vazio.
func NewRepository() *Repository {
	return &Repository{
		tenants:   map[uuid.UUID]domain.Tenant{},
		themes:    map[uuid.UUID]domain.Theme{},
		overrides: map[uuid.UUID]map[domain.Feature]bool{},
		domains:   map[string]uuid.UUID{},
	}
}

func (r *Repository) CreateTenant(_ context.Context, t domain.Tenant, evt domain.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.tenants {
		if existing.Slug == t.Slug {
			return domain.ErrConflict
		}
	}
	r.tenants[t.ID] = t
	r.outbox = append(r.outbox, evt)
	return nil
}

func (r *Repository) ListTenants(context.Context) ([]domain.Tenant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.Tenant, 0, len(r.tenants))
	for _, t := range r.tenants {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

func (r *Repository) GetTenant(_ context.Context, id uuid.UUID) (domain.Tenant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tenants[id]
	if !ok {
		return domain.Tenant{}, domain.ErrNotFound
	}
	return t, nil
}

func (r *Repository) UpdateTenant(_ context.Context, t domain.Tenant, evt *domain.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tenants[t.ID]; !ok {
		return domain.ErrNotFound
	}
	r.tenants[t.ID] = t
	if evt != nil {
		r.outbox = append(r.outbox, *evt)
	}
	return nil
}

func (r *Repository) GetTheme(_ context.Context, id uuid.UUID) (domain.Theme, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.themes[id]
	return t, ok, nil
}

func (r *Repository) SaveTheme(_ context.Context, id uuid.UUID, theme domain.Theme) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.themes[id] = theme
	return nil
}

func (r *Repository) GetOverrides(_ context.Context, id uuid.UUID) (map[domain.Feature]bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[domain.Feature]bool{}
	for f, v := range r.overrides[id] {
		out[f] = v
	}
	return out, nil
}

func (r *Repository) SaveOverride(_ context.Context, id uuid.UUID, f domain.Feature, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.overrides[id] == nil {
		r.overrides[id] = map[domain.Feature]bool{}
	}
	r.overrides[id][f] = enabled
	return nil
}

func (r *Repository) AddDomain(_ context.Context, id uuid.UUID, host string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, taken := r.domains[host]; taken {
		return domain.ErrConflict
	}
	r.domains[host] = id
	return nil
}

func (r *Repository) ListDomains(_ context.Context, id uuid.UUID) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for h, owner := range r.domains {
		if owner == id {
			out = append(out, h)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (r *Repository) ResolveHost(_ context.Context, host string) (uuid.UUID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.domains[host]
	if !ok {
		return uuid.Nil, domain.ErrNotFound
	}
	return id, nil
}

func (r *Repository) RequestErasure(_ context.Context, req domain.ErasureRequest, evt domain.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.erasures = append(r.erasures, req)
	r.outbox = append(r.outbox, evt)
	return nil
}

// ErasureRequests devolve uma cópia dos pedidos de exclusão registrados.
func (r *Repository) ErasureRequests() []domain.ErasureRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domain.ErasureRequest(nil), r.erasures...)
}

// Events devolve uma cópia dos eventos gravados na "outbox" em memória.
func (r *Repository) Events() []domain.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domain.Event(nil), r.outbox...)
}
