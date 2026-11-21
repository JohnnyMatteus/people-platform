package domain

import (
	"math"
	"net/mail"
	"net/url"
	"strings"
)

// ThemeInput é o tema white label como chega do cliente, com cores em texto.
type ThemeInput struct {
	Primary       string
	OnPrimary     string
	Background    string
	Text          string
	Accent        string
	LogoURL       string
	EmailFromName string
	EmailReplyTo  string
}

// Theme é o tema white label validado de um tenant. Só existe com contraste WCAG AA garantido.
type Theme struct {
	Primary       Color
	OnPrimary     Color
	Background    Color
	Text          Color
	Accent        Color
	LogoURL       string
	EmailFromName string
	EmailReplyTo  string
}

// ContrastCheck descreve a verificação de um par de cores que precisa ser legível.
type ContrastCheck struct {
	Pair  string  `json:"pair"`
	Ratio float64 `json:"ratio"`
	Pass  bool    `json:"pass"`
}

// contrastPairs são os pares de tokens que o front-end usa como texto sobre fundo.
var contrastPairs = []struct {
	name    string
	fg, bg  func(Theme) Color
	fgField string
}{
	{"on_primary/primary", func(t Theme) Color { return t.OnPrimary }, func(t Theme) Color { return t.Primary }, "on_primary"},
	{"text/background", func(t Theme) Color { return t.Text }, func(t Theme) Color { return t.Background }, "text"},
	{"accent/background", func(t Theme) Color { return t.Accent }, func(t Theme) Color { return t.Background }, "accent"},
}

// DefaultTheme é o tema neutro da plataforma, usado enquanto o tenant não define o próprio.
func DefaultTheme() Theme {
	return Theme{
		Primary:       Color{0x1f, 0x4e, 0x79},
		OnPrimary:     Color{0xff, 0xff, 0xff},
		Background:    Color{0xff, 0xff, 0xff},
		Text:          Color{0x1a, 0x1a, 0x1a},
		Accent:        Color{0x0b, 0x6e, 0x4f},
		EmailFromName: "People Platform",
	}
}

// NewTheme valida cores, contraste WCAG AA, logo e remetente de e-mail.
func NewTheme(in ThemeInput) (Theme, error) {
	verr := &ValidationError{}
	parse := func(field, value string) Color {
		c, err := ParseColor(value)
		if err != nil {
			verr.add(field, "%s", err.Error())
		}
		return c
	}
	t := Theme{
		Primary:       parse("primary", in.Primary),
		OnPrimary:     parse("on_primary", in.OnPrimary),
		Background:    parse("background", in.Background),
		Text:          parse("text", in.Text),
		Accent:        parse("accent", in.Accent),
		LogoURL:       strings.TrimSpace(in.LogoURL),
		EmailFromName: strings.TrimSpace(in.EmailFromName),
		EmailReplyTo:  strings.TrimSpace(in.EmailReplyTo),
	}
	if len(verr.Fields) > 0 {
		return Theme{}, verr
	}
	for _, check := range t.ContrastChecks() {
		if !check.Pass {
			verr.add(fieldForPair(check.Pair), "contraste %s de %.2f:1 abaixo do mínimo WCAG AA de %.1f:1",
				check.Pair, check.Ratio, MinContrastAA)
		}
	}
	if t.LogoURL != "" {
		if !validLogoURL(t.LogoURL) {
			verr.add("logo_url", "logo deve ser uma URL https absoluta")
		}
	}
	if t.EmailFromName == "" || len(t.EmailFromName) > 80 {
		verr.add("email_from_name", "nome do remetente é obrigatório e tem até 80 caracteres")
	}
	if t.EmailReplyTo != "" {
		if addr, err := mail.ParseAddress(t.EmailReplyTo); err != nil || addr.Address != t.EmailReplyTo {
			verr.add("email_reply_to", "endereço de resposta inválido")
		}
	}
	if err := verr.orNil(); err != nil {
		return Theme{}, err
	}
	return t, nil
}

// ContrastChecks calcula a razão de contraste de cada par obrigatório, arredondada em 2 casas.
func (t Theme) ContrastChecks() []ContrastCheck {
	checks := make([]ContrastCheck, 0, len(contrastPairs))
	for _, p := range contrastPairs {
		ratio := ContrastRatio(p.fg(t), p.bg(t))
		checks = append(checks, ContrastCheck{
			Pair:  p.name,
			Ratio: math.Round(ratio*100) / 100,
			// A comparação usa o valor exato: arredondar antes poderia aprovar 4.495:1.
			Pass: ratio >= MinContrastAA,
		})
	}
	return checks
}

// Tokens devolve o tema como design tokens consumidos pelo front-end white label.
func (t Theme) Tokens() map[string]string {
	return map[string]string{
		"color.primary":    t.Primary.Hex(),
		"color.on-primary": t.OnPrimary.Hex(),
		"color.background": t.Background.Hex(),
		"color.text":       t.Text.Hex(),
		"color.accent":     t.Accent.Hex(),
	}
}

func fieldForPair(pair string) string {
	for _, p := range contrastPairs {
		if p.name == pair {
			return p.fgField
		}
	}
	return pair
}

// validLogoURL exige https. A única exceção é loopback (localhost), onde o LocalStack serve
// os assets em http no ambiente de desenvolvimento.
func validLogoURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	switch u.Scheme {
	case "https":
		return true
	case "http":
		h := u.Hostname()
		return h == "localhost" || h == "127.0.0.1"
	default:
		return false
	}
}
