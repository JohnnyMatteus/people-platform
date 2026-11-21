package domain

import (
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Status é o ciclo de vida de um tenant.
type Status string

// Estados possíveis de um tenant.
const (
	StatusActive      Status = "active"
	StatusSuspended   Status = "suspended"
	StatusDeactivated Status = "deactivated"
)

func (s Status) valid() bool {
	return s == StatusActive || s == StatusSuspended || s == StatusDeactivated
}

var (
	slugPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{1,38}[a-z0-9])$`)
	// hostPattern aceita nomes DNS em minúsculas com pelo menos um ponto (sem porta e sem esquema).
	hostPattern = regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)
)

// Tenant é uma empresa cliente da plataforma.
type Tenant struct {
	ID        uuid.UUID
	Slug      string
	Name      string
	Plan      PlanCode
	Status    Status
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewTenantInput são os dados para criar um tenant.
type NewTenantInput struct {
	Slug string
	Name string
	Plan PlanCode
}

// NewTenant valida a entrada e cria um tenant ativo.
func NewTenant(id uuid.UUID, in NewTenantInput, now time.Time) (Tenant, error) {
	verr := &ValidationError{}
	slug := strings.TrimSpace(in.Slug)
	if !slugPattern.MatchString(slug) {
		verr.add("slug", "slug deve ter de 3 a 40 caracteres: letras minúsculas, dígitos e hífen")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" || len([]rune(name)) > 120 {
		verr.add("name", "nome é obrigatório e tem até 120 caracteres")
	}
	if _, err := LookupPlan(in.Plan); err != nil {
		verr.add("plan", "plano desconhecido: %s", in.Plan)
	}
	if err := verr.orNil(); err != nil {
		return Tenant{}, err
	}
	return Tenant{
		ID: id, Slug: slug, Name: name, Plan: in.Plan, Status: StatusActive,
		CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
	}, nil
}

// TenantPatch descreve uma alteração parcial; campos nil não mudam.
type TenantPatch struct {
	Name   *string
	Plan   *PlanCode
	Status *Status
}

// Apply devolve o tenant alterado e se o plano mudou (o que gera evento).
func (t Tenant) Apply(p TenantPatch, now time.Time) (Tenant, bool, error) {
	verr := &ValidationError{}
	next := t
	if p.Name != nil {
		name := strings.TrimSpace(*p.Name)
		if name == "" || len([]rune(name)) > 120 {
			verr.add("name", "nome é obrigatório e tem até 120 caracteres")
		}
		next.Name = name
	}
	if p.Plan != nil {
		if _, err := LookupPlan(*p.Plan); err != nil {
			verr.add("plan", "plano desconhecido: %s", *p.Plan)
		}
		next.Plan = *p.Plan
	}
	if p.Status != nil {
		if !p.Status.valid() {
			verr.add("status", "status inválido: %s", *p.Status)
		} else if t.Status == StatusDeactivated && *p.Status != StatusDeactivated {
			verr.add("status", "tenant desativado não pode ser reativado")
		}
		next.Status = *p.Status
	}
	if err := verr.orNil(); err != nil {
		return Tenant{}, false, err
	}
	next.UpdatedAt = now.UTC()
	return next, next.Plan != t.Plan, nil
}

// NormalizeHost valida e normaliza um domínio próprio do tenant (ex.: "rh.horizonte.example.com").
func NormalizeHost(raw string) (string, error) {
	host := strings.ToLower(strings.TrimSpace(raw))
	host = strings.TrimSuffix(host, ".")
	if len(host) > 253 || !hostPattern.MatchString(host) {
		return "", &ValidationError{Fields: []FieldError{{Field: "host", Message: "domínio inválido: use um nome DNS sem esquema nem porta"}}}
	}
	return host, nil
}

// HostFromRequest remove a porta de um cabeçalho Host para a resolução de tenant.
func HostFromRequest(hostHeader string) string {
	host := strings.ToLower(strings.TrimSpace(hostHeader))
	if i := strings.LastIndexByte(host, ':'); i >= 0 && !strings.Contains(host[i:], "]") {
		host = host[:i]
	}
	return strings.TrimSuffix(host, ".")
}
