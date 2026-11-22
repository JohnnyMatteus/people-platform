/** Utilitários de cor para derivar o tema do tenant sem quebrar contraste (camada 2 da identidade). */

export type Rgb = readonly [number, number, number]

export function parseHex(hex: string): Rgb {
  const m = /^#?([0-9a-f]{6})$/i.exec(hex.trim())
  if (!m?.[1]) throw new Error(`cor inválida: ${hex}`)
  const n = parseInt(m[1], 16)
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255]
}

export function toHex([r, g, b]: Rgb): string {
  const part = (v: number) => Math.round(Math.min(255, Math.max(0, v))).toString(16).padStart(2, '0')
  return `#${part(r)}${part(g)}${part(b)}`.toUpperCase()
}

const toLinear = (c: number) => {
  const v = c / 255
  return v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4
}
const fromLinear = (v: number) => 255 * (v <= 0.0031308 ? 12.92 * v : 1.055 * v ** (1 / 2.4) - 0.055)

export function luminance(hex: string): number {
  const [r, g, b] = parseHex(hex).map(toLinear) as [number, number, number]
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

/** Razão de contraste WCAG entre duas cores (1 a 21). */
export function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x) as [number, number]
  return (hi + 0.05) / (lo + 0.05)
}

/** Mistura em RGB linear: `ratio` é a fatia de `a` (0 a 1). */
export function mix(a: string, b: string, ratio: number): string {
  const [ra, rb] = [parseHex(a).map(toLinear), parseHex(b).map(toLinear)]
  const out = ra.map((v, i) => fromLinear(v * ratio + (rb[i] as number) * (1 - ratio))) as unknown as Rgb
  return toHex(out)
}

// OKLCH: a luminosidade perceptual é o que o dark mode do tenant precisa ajustar.
function toOklch(hex: string): [number, number, number] {
  const [r, g, b] = parseHex(hex).map(toLinear) as [number, number, number]
  const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b)
  const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b)
  const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b)
  const L = 0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s
  const A = 1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s
  const B = 0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s
  return [L, Math.hypot(A, B), Math.atan2(B, A)]
}

function fromOklch(L: number, C: number, h: number): string {
  const A = C * Math.cos(h)
  const B = C * Math.sin(h)
  const l = (L + 0.3963377774 * A + 0.2158037573 * B) ** 3
  const m = (L - 0.1055613458 * A - 0.0638541728 * B) ** 3
  const s = (L - 0.0894841775 * A - 1.291485548 * B) ** 3
  const lin: Rgb = [
    4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
    -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
    -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s,
  ]
  return toHex(lin.map((v) => fromLinear(Math.min(1, Math.max(0, v)))) as unknown as Rgb)
}

/** Clareia a cor até a luminosidade OKLCH mínima, mantendo matiz e croma. */
export function liftLightness(hex: string, minL: number): string {
  const [L, C, h] = toOklch(hex)
  return L >= minL ? hex : fromOklch(minL, C, h)
}

export const AA = 4.5

/** Escolhe, entre candidatos, o primeiro que passa AA sobre o fundo; senão o de maior contraste. */
export function readableOn(background: string, candidates: string[]): string {
  const passing = candidates.find((c) => contrast(c, background) >= AA)
  if (passing) return passing
  return [...candidates].sort((a, b) => contrast(b, background) - contrast(a, background))[0] as string
}
