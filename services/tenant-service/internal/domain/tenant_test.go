package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

var fixedNow = time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

func TestNewTenant(t *testing.T) {
	id := uuid.New()
	tn, err := NewTenant(id, NewTenantInput{Slug: "construtora-horizonte", Name: " Construtora Horizonte Exemplo ", Plan: PlanGrowth}, fixedNow)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if tn.Status != StatusActive || tn.Name != "Construtora Horizonte Exemplo" || tn.ID != id {
		t.Fatalf("tenant inesperado: %+v", tn)
	}
}

func TestNewTenant_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		in    NewTenantInput
		field string
	}{
		{"slug curto", NewTenantInput{Slug: "ab", Name: "X", Plan: PlanGrowth}, "slug"},
		{"slug com maiúscula", NewTenantInput{Slug: "Rede-Sol", Name: "X", Plan: PlanGrowth}, "slug"},
		{"slug terminando em hífen", NewTenantInput{Slug: "rede-sol-", Name: "X", Plan: PlanGrowth}, "slug"},
		{"nome vazio", NewTenantInput{Slug: "rede-sol", Name: "  ", Plan: PlanGrowth}, "name"},
		{"plano inexistente", NewTenantInput{Slug: "rede-sol", Name: "Rede", Plan: "premium"}, "plan"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTenant(uuid.New(), tt.in, fixedNow)
			if _, ok := fieldsOf(t, err)[tt.field]; !ok {
				t.Fatalf("esperava erro em %s: %v", tt.field, err)
			}
		})
	}
}

func TestTenantApply(t *testing.T) {
	tn, _ := NewTenant(uuid.New(), NewTenantInput{Slug: "rede-sol", Name: "Rede Varejo Sol Exemplo", Plan: PlanEssential}, fixedNow)
	plan := PlanEnterprise
	later := fixedNow.Add(time.Hour)

	next, planChanged, err := tn.Apply(TenantPatch{Plan: &plan}, later)
	if err != nil || !planChanged || next.Plan != PlanEnterprise || !next.UpdatedAt.Equal(later) {
		t.Fatalf("mudança de plano falhou: %+v %v %v", next, planChanged, err)
	}

	same := PlanEssential
	if _, changed, _ := tn.Apply(TenantPatch{Plan: &same}, later); changed {
		t.Fatal("mesmo plano não deve sinalizar mudança")
	}

	deactivated := StatusDeactivated
	gone, _, err := tn.Apply(TenantPatch{Status: &deactivated}, later)
	if err != nil {
		t.Fatal(err)
	}
	active := StatusActive
	if _, _, err := gone.Apply(TenantPatch{Status: &active}, later); err == nil {
		t.Fatal("tenant desativado não pode ser reativado")
	}

	bogus := Status("archived")
	if _, _, err := tn.Apply(TenantPatch{Status: &bogus}, later); err == nil {
		t.Fatal("status inválido deveria falhar")
	}
}

func TestNormalizeHost(t *testing.T) {
	ok := map[string]string{
		"RH.Horizonte.Example.com": "rh.horizonte.example.com",
		"pessoas.sol.example.com.": "pessoas.sol.example.com",
		" app.example.com ":        "app.example.com",
	}
	for in, want := range ok {
		got, err := NormalizeHost(in)
		if err != nil || got != want {
			t.Errorf("NormalizeHost(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"https://rh.example.com", "rh.example.com:8080", "localhost", "-rh.example.com", "rh_x.example.com", ""} {
		if _, err := NormalizeHost(bad); err == nil {
			t.Errorf("NormalizeHost(%q) deveria falhar", bad)
		}
	}
}

func TestHostFromRequest(t *testing.T) {
	cases := map[string]string{
		"rh.horizonte.example.com:8443": "rh.horizonte.example.com",
		"RH.Horizonte.example.com":      "rh.horizonte.example.com",
		"[::1]:8080":                    "[::1]",
		"pessoas.sol.example.com.":      "pessoas.sol.example.com",
	}
	for in, want := range cases {
		if got := HostFromRequest(in); got != want {
			t.Errorf("HostFromRequest(%q) = %q, want %q", in, got, want)
		}
	}
}
