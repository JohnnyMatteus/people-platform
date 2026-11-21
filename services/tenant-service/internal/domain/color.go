package domain

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// MinContrastAA é a razão mínima de contraste exigida pelo WCAG 2.x nível AA para texto normal.
const MinContrastAA = 4.5

// Color é uma cor sRGB opaca.
type Color struct {
	R, G, B uint8
}

// ParseColor aceita "#RRGGBB" ou "#RGB" (sem distinção de maiúsculas).
func ParseColor(s string) (Color, error) {
	hex := strings.TrimPrefix(strings.TrimSpace(s), "#")
	if !strings.HasPrefix(strings.TrimSpace(s), "#") {
		return Color{}, fmt.Errorf("cor %q deve começar com #", s)
	}
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) != 6 {
		return Color{}, fmt.Errorf("cor %q deve ter o formato #RRGGBB ou #RGB", s)
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return Color{}, fmt.Errorf("cor %q contém dígitos hexadecimais inválidos", s)
	}
	return Color{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v)}, nil
}

// Hex devolve a cor normalizada em "#rrggbb".
func (c Color) Hex() string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// RelativeLuminance segue a definição do WCAG 2.x (sRGB linearizado, coeficientes Rec. 709).
func (c Color) RelativeLuminance() float64 {
	return 0.2126*linearize(c.R) + 0.7152*linearize(c.G) + 0.0722*linearize(c.B)
}

func linearize(channel uint8) float64 {
	v := float64(channel) / 255
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

// ContrastRatio devolve a razão de contraste entre duas cores, de 1 a 21, independente da ordem.
func ContrastRatio(a, b Color) float64 {
	la, lb := a.RelativeLuminance(), b.RelativeLuminance()
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}
