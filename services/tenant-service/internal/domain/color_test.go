package domain

import (
	"math"
	"testing"
)

func TestParseColor(t *testing.T) {
	tests := []struct {
		in      string
		want    Color
		wantErr bool
	}{
		{in: "#FFFFFF", want: Color{255, 255, 255}},
		{in: "#1f4e79", want: Color{0x1f, 0x4e, 0x79}},
		{in: "#abc", want: Color{0xaa, 0xbb, 0xcc}},
		{in: "  #000000 ", want: Color{0, 0, 0}},
		{in: "FFFFFF", wantErr: true},
		{in: "#FFFF", wantErr: true},
		{in: "#GGGGGG", wantErr: true},
		{in: "", wantErr: true},
		{in: "#", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseColor(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("esperava erro para %q, obteve %v", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContrastRatio_KnownValues(t *testing.T) {
	black, white := Color{0, 0, 0}, Color{255, 255, 255}
	tests := []struct {
		name string
		a, b Color
		want float64
	}{
		{"preto e branco é o máximo", black, white, 21},
		{"mesma cor é o mínimo", white, white, 1},
		{"#767676 sobre branco passa no limite", Color{0x76, 0x76, 0x76}, white, 4.54},
		{"#777777 sobre branco falha por pouco", Color{0x77, 0x77, 0x77}, white, 4.48},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContrastRatio(tt.a, tt.b)
			if math.Abs(got-tt.want) > 0.01 {
				t.Fatalf("contraste = %.4f, want %.2f", got, tt.want)
			}
			if rev := ContrastRatio(tt.b, tt.a); math.Abs(rev-got) > 1e-12 {
				t.Fatalf("contraste deve ser simétrico: %.4f vs %.4f", got, rev)
			}
		})
	}
}

func TestColorHex_Normalizes(t *testing.T) {
	c, err := ParseColor("#ABC")
	if err != nil {
		t.Fatal(err)
	}
	if c.Hex() != "#aabbcc" {
		t.Fatalf("Hex() = %s", c.Hex())
	}
}
