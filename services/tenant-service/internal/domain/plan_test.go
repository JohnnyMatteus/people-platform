package domain

import (
	"errors"
	"slices"
	"testing"
)

func TestEffectiveFeatures(t *testing.T) {
	growth := Catalog[PlanGrowth]
	tests := []struct {
		name      string
		overrides map[Feature]bool
		has       []Feature
		hasNot    []Feature
	}{
		{"pacote do plano", nil, []Feature{FeatureRecruitment, FeatureNineBox}, []Feature{FeatureAnalytics}},
		{"adicional ligado", map[Feature]bool{FeatureAnalytics: true}, []Feature{FeatureAnalytics}, nil},
		{"feature incluída desligada", map[Feature]bool{FeatureRecruitment: false}, nil, []Feature{FeatureRecruitment}},
		{"override fora do plano é ignorado", map[Feature]bool{FeatureLearning: true}, nil, []Feature{FeatureLearning}},
		{"feature desconhecida é ignorada", map[Feature]bool{"teleporte": true}, nil, []Feature{"teleporte"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EffectiveFeatures(growth, tt.overrides)
			for _, f := range tt.has {
				if !slices.Contains(got, f) {
					t.Errorf("esperava %s em %v", f, got)
				}
			}
			for _, f := range tt.hasNot {
				if slices.Contains(got, f) {
					t.Errorf("não esperava %s em %v", f, got)
				}
			}
			if !slices.IsSorted(got) {
				t.Errorf("features devem vir ordenadas: %v", got)
			}
		})
	}
}

func TestValidateOverride(t *testing.T) {
	essential := Catalog[PlanEssential]
	if err := essential.ValidateOverride(FeatureAnalytics, true); !errors.Is(err, ErrFeatureLocked) {
		t.Fatalf("analytics não é adicionável no essencial: %v", err)
	}
	if err := essential.ValidateOverride(FeatureAnalytics, false); err != nil {
		t.Fatalf("desligar é sempre permitido: %v", err)
	}
	if err := essential.ValidateOverride(FeatureRecruitment, true); err != nil {
		t.Fatalf("recrutamento é adicionável no essencial: %v", err)
	}
	if err := essential.ValidateOverride("inexistente", true); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("feature desconhecida deve ser entrada inválida: %v", err)
	}
}

func TestCatalog_Consistency(t *testing.T) {
	for code, p := range Catalog {
		if p.Code != code {
			t.Errorf("plano %s com código divergente %s", code, p.Code)
		}
		for _, f := range append(slices.Clone(p.Included), p.Addable...) {
			if !f.IsKnown() {
				t.Errorf("plano %s referencia feature desconhecida %s", code, f)
			}
		}
		for _, f := range p.Addable {
			if slices.Contains(p.Included, f) {
				t.Errorf("plano %s: %s não pode ser incluída e adicional ao mesmo tempo", code, f)
			}
		}
	}
	if _, err := LookupPlan("premium"); !errors.Is(err, ErrUnknownPlan) {
		t.Fatalf("plano inexistente deve falhar: %v", err)
	}
}
