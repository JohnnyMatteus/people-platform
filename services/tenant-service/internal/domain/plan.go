package domain

import (
	"slices"
	"sort"
)

// Feature é uma capacidade do produto que pode ser ligada ou desligada por tenant.
type Feature string

// Catálogo de features. Os nomes são estáveis porque viajam em eventos e tokens.
const (
	FeatureRecruitment   Feature = "recruitment"
	FeaturePerformance   Feature = "performance"
	FeatureNineBox       Feature = "nine_box"
	FeatureSuccession    Feature = "succession"
	FeatureOKR           Feature = "okr"
	FeatureIDP           Feature = "idp"
	FeatureEngagement    Feature = "engagement"
	FeatureClimateSurvey Feature = "climate_survey"
	FeatureAnalytics     Feature = "analytics"
	FeatureAIAssist      Feature = "ai_assist"
	FeatureLearning      Feature = "learning"
	FeatureCustomDomain  Feature = "custom_domain"
)

var knownFeatures = map[Feature]struct{}{
	FeatureRecruitment: {}, FeaturePerformance: {}, FeatureNineBox: {}, FeatureSuccession: {},
	FeatureOKR: {}, FeatureIDP: {}, FeatureEngagement: {}, FeatureClimateSurvey: {},
	FeatureAnalytics: {}, FeatureAIAssist: {}, FeatureLearning: {}, FeatureCustomDomain: {},
}

// IsKnown informa se a feature pertence ao catálogo.
func (f Feature) IsKnown() bool {
	_, ok := knownFeatures[f]
	return ok
}

// PlanCode identifica um plano comercial.
type PlanCode string

// Planos comerciais fictícios da plataforma.
const (
	PlanEssential  PlanCode = "essencial"
	PlanGrowth     PlanCode = "crescimento"
	PlanEnterprise PlanCode = "corporativo"
)

// Plan define quais features o tenant recebe e quais ele pode ligar como adicional.
type Plan struct {
	Code     PlanCode
	Name     string
	Included []Feature
	// Addable são features fora do pacote que podem ser ligadas por override (venda adicional).
	Addable []Feature
}

// Catalog é o catálogo de planos. É código, não tabela, porque muda junto com o produto
// e precisa ser revisado em PR; o banco guarda só a escolha de cada tenant.
var Catalog = map[PlanCode]Plan{
	PlanEssential: {
		Code:     PlanEssential,
		Name:     "Essencial",
		Included: []Feature{FeaturePerformance, FeatureOKR, FeatureEngagement},
		Addable:  []Feature{FeatureRecruitment, FeatureClimateSurvey},
	},
	PlanGrowth: {
		Code: PlanGrowth,
		Name: "Crescimento",
		Included: []Feature{
			FeatureRecruitment, FeaturePerformance, FeatureNineBox, FeatureOKR, FeatureIDP,
			FeatureEngagement, FeatureClimateSurvey,
		},
		Addable: []Feature{FeatureAnalytics, FeatureAIAssist, FeatureSuccession, FeatureCustomDomain},
	},
	PlanEnterprise: {
		Code: PlanEnterprise,
		Name: "Corporativo",
		Included: []Feature{
			FeatureRecruitment, FeaturePerformance, FeatureNineBox, FeatureSuccession, FeatureOKR,
			FeatureIDP, FeatureEngagement, FeatureClimateSurvey, FeatureAnalytics, FeatureAIAssist,
			FeatureCustomDomain,
		},
		Addable: []Feature{FeatureLearning},
	},
}

// LookupPlan devolve o plano ou ErrUnknownPlan.
func LookupPlan(code PlanCode) (Plan, error) {
	p, ok := Catalog[code]
	if !ok {
		return Plan{}, ErrUnknownPlan
	}
	return p, nil
}

// Plans devolve o catálogo ordenado por código, para respostas determinísticas.
func Plans() []Plan {
	out := make([]Plan, 0, len(Catalog))
	for _, p := range Catalog {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

func (p Plan) includes(f Feature) bool    { return slices.Contains(p.Included, f) }
func (p Plan) allowsAddOn(f Feature) bool { return slices.Contains(p.Addable, f) }

// ValidateOverride verifica se o override é permitido no plano: desligar é sempre permitido;
// ligar só vale para features incluídas ou adicionáveis.
func (p Plan) ValidateOverride(f Feature, enabled bool) error {
	if !f.IsKnown() {
		return &ValidationError{Fields: []FieldError{{Field: "feature", Message: "feature desconhecida: " + string(f)}}}
	}
	if enabled && !p.includes(f) && !p.allowsAddOn(f) {
		return ErrFeatureLocked
	}
	return nil
}

// EffectiveFeatures combina o pacote do plano com os overrides do tenant. Overrides que o
// plano não permite (por exemplo, após um downgrade) são ignorados, em vez de vazar acesso.
func EffectiveFeatures(p Plan, overrides map[Feature]bool) []Feature {
	set := map[Feature]bool{}
	for _, f := range p.Included {
		set[f] = true
	}
	for f, enabled := range overrides {
		if p.ValidateOverride(f, enabled) != nil {
			continue
		}
		set[f] = enabled
	}
	out := make([]Feature, 0, len(set))
	for f, on := range set {
		if on {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
