package domain

import (
	"errors"
	"testing"
)

func validThemeInput() ThemeInput {
	return ThemeInput{
		Primary:       "#1f4e79",
		OnPrimary:     "#ffffff",
		Background:    "#ffffff",
		Text:          "#1a1a1a",
		Accent:        "#0b6e4f",
		LogoURL:       "https://cdn.example.com/horizonte/logo.svg",
		EmailFromName: "RH Construtora Horizonte Exemplo",
		EmailReplyTo:  "rh@horizonte.example.com",
	}
}

func fieldsOf(t *testing.T, err error) map[string]string {
	t.Helper()
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("esperava ValidationError, obteve %v", err)
	}
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("ValidationError deve casar com ErrInvalidInput")
	}
	out := map[string]string{}
	for _, f := range verr.Fields {
		out[f.Field] = f.Message
	}
	return out
}

func TestNewTheme_Valid(t *testing.T) {
	theme, err := NewTheme(validThemeInput())
	if err != nil {
		t.Fatalf("tema válido rejeitado: %v", err)
	}
	for _, c := range theme.ContrastChecks() {
		if !c.Pass {
			t.Errorf("par %s deveria passar, razão %.2f", c.Pair, c.Ratio)
		}
	}
	if theme.Tokens()["color.primary"] != "#1f4e79" {
		t.Fatalf("token primário inesperado: %v", theme.Tokens())
	}
}

func TestNewTheme_RejectsLowContrast(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*ThemeInput)
		wantField string
	}{
		{"texto do botão amarelo sobre amarelo", func(in *ThemeInput) { in.Primary = "#ffd700"; in.OnPrimary = "#ffffff" }, "on_primary"},
		{"texto cinza claro sobre branco", func(in *ThemeInput) { in.Text = "#777777" }, "text"},
		{"destaque verde-claro sobre branco", func(in *ThemeInput) { in.Accent = "#7fdc9f" }, "accent"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validThemeInput()
			tt.mutate(&in)
			_, err := NewTheme(in)
			fields := fieldsOf(t, err)
			if _, ok := fields[tt.wantField]; !ok {
				t.Fatalf("esperava erro em %s, obteve %v", tt.wantField, fields)
			}
		})
	}
}

func TestNewTheme_BoundaryContrast(t *testing.T) {
	in := validThemeInput()
	in.Text = "#767676" // 4.54:1, o cinza mais claro que passa sobre branco
	if _, err := NewTheme(in); err != nil {
		t.Fatalf("4.54:1 deveria passar: %v", err)
	}
}

func TestNewTheme_DarkThemeIsAccepted(t *testing.T) {
	in := validThemeInput()
	in.Background, in.Text, in.Accent = "#121212", "#f5f5f5", "#7fdc9f"
	if _, err := NewTheme(in); err != nil {
		t.Fatalf("tema escuro com bom contraste deveria passar: %v", err)
	}
}

func TestNewTheme_InvalidFields(t *testing.T) {
	in := validThemeInput()
	in.Primary = "azul"
	_, err := NewTheme(in)
	if _, ok := fieldsOf(t, err)["primary"]; !ok {
		t.Fatal("esperava erro de formato em primary")
	}

	in = validThemeInput()
	in.LogoURL = "http://cdn.example.com/logo.svg"
	in.EmailFromName = ""
	in.EmailReplyTo = "nao-e-email"
	fields := fieldsOf(t, func() error { _, e := NewTheme(in); return e }())
	for _, f := range []string{"logo_url", "email_from_name", "email_reply_to"} {
		if _, ok := fields[f]; !ok {
			t.Errorf("esperava erro em %s, obteve %v", f, fields)
		}
	}
}

func TestDefaultTheme_MeetsAA(t *testing.T) {
	for _, c := range DefaultTheme().ContrastChecks() {
		if !c.Pass {
			t.Errorf("tema padrão falha em %s (%.2f:1)", c.Pair, c.Ratio)
		}
	}
}

func TestNewTheme_LogoURL(t *testing.T) {
	cases := map[string]bool{
		"https://cdn.example.com/logo.svg":                   true,
		"http://localhost:4566/people-platform-assets/a.png": true,
		"http://cdn.example.com/logo.svg":                    false,
		"ftp://cdn.example.com/logo.svg":                     false,
		"/relativo/logo.svg":                                 false,
	}
	for raw, ok := range cases {
		in := validThemeInput()
		in.LogoURL = raw
		_, err := NewTheme(in)
		if (err == nil) != ok {
			t.Errorf("logo %q: aceito=%v, esperado=%v (%v)", raw, err == nil, ok, err)
		}
	}
}
