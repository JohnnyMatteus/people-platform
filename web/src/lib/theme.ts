import { aliases, mdi } from 'vuetify/iconsets/mdi-svg'
import { createVuetify, type ThemeDefinition } from 'vuetify'
import { AA, contrast, liftLightness, mix, readableOn } from './color'

/** Camada 3 (fixa da plataforma): o tenant nunca sobrescreve. */
const platform = {
  success: '#1E6B3A',
  warning: '#8C5300',
  error: '#B42318',
  info: '#2B5C8A',
  secondary: '#4A4740',
  hatch: '#8A877F',
  'chart-1': '#3A6EA5',
  'chart-2': '#C7793B',
  'chart-3': '#5E8C61',
  'chart-4': '#8D6A9F',
  'chart-5': '#B5545C',
  'chart-6': '#6B7F8E',
}

export const grafiteLight: ThemeDefinition = {
  dark: false,
  colors: {
    primary: '#1F4E79',
    'on-primary': '#FFFFFF',
    background: '#F5F4F1',
    'on-background': '#1C1B19',
    accent: '#0B6E4F',
    surface: '#FFFFFF',
    'on-surface': '#1C1B19',
    'surface-variant': '#E9E7E2',
    'on-surface-variant': '#1C1B19',
    'surface-light': '#F0EEEA',
    'primary-container': '#E1E8EF',
    'on-primary-container': '#1F4E79',
    ...platform,
    'on-secondary': '#FFFFFF',
  },
  variables: {
    'border-color': '#1C1B19',
    'border-opacity': 0.14,
    'medium-emphasis-opacity': 0.7,
    'disabled-opacity': 0.4,
    'hover-opacity': 0.05,
    'focus-opacity': 0.12,
  },
}

export const grafiteDark: ThemeDefinition = {
  dark: true,
  colors: {
    primary: '#9EC3E8',
    'on-primary': '#0B2238',
    background: '#151515',
    'on-background': '#EDEBE6',
    accent: '#7FD4B0',
    surface: '#1E1E1D',
    'on-surface': '#EDEBE6',
    'surface-variant': '#2F2E2B',
    'on-surface-variant': '#EDEBE6',
    'surface-light': '#262624',
    'primary-container': '#22303D',
    'on-primary-container': '#CFE2F5',
    secondary: '#CFCAC0',
    'on-secondary': '#22201C',
    success: '#86D39E',
    warning: '#F0B866',
    error: '#FFB3A8',
    info: '#A8C8F0',
    hatch: '#6E6B64',
    'chart-1': '#7FA8D6',
    'chart-2': '#E3A26A',
    'chart-3': '#93BE95',
    'chart-4': '#B99BCB',
    'chart-5': '#E08890',
    'chart-6': '#9FB0BD',
  },
  variables: {
    'border-color': '#EDEBE6',
    'border-opacity': 0.14,
    'medium-emphasis-opacity': 0.7,
    'disabled-opacity': 0.4,
    'hover-opacity': 0.07,
    'focus-opacity': 0.14,
  },
}

export const vuetify = createVuetify({
  icons: { defaultSet: 'mdi', aliases, sets: { mdi } },
  theme: { defaultTheme: 'grafiteLight', themes: { grafiteLight, grafiteDark } },
  defaults: {
    VBtn: { rounded: 'md', variant: 'flat', class: 'text-none', height: 40 },
    VCard: { rounded: 'lg', variant: 'outlined', elevation: 0 },
    VChip: { rounded: 'md', variant: 'outlined', size: 'small' },
    VTextField: { variant: 'outlined', density: 'comfortable', rounded: 'md' },
    VTextarea: { variant: 'outlined', autoGrow: true, rounded: 'md' },
    VSelect: { variant: 'outlined', density: 'comfortable', rounded: 'md' },
    VNavigationDrawer: { width: 248, elevation: 0, border: 'e' },
    VAppBar: { flat: true, border: 'b', height: 56 },
    VList: { density: 'compact', nav: true },
    VTabs: { color: 'primary', density: 'compact' },
    VAlert: { variant: 'tonal', rounded: 'md', border: 'start' },
    VTooltip: { location: 'top' },
  },
})

export interface BrandTokens {
  primary: string
  onPrimary: string
  background: string
  text: string
  accent: string
}

export interface DerivedTheme {
  colors: Record<string, string>
  /** Derivados que falharam no contraste e voltaram ao valor Grafite (para a tela de configuração). */
  fallbacks: string[]
}

const HEX = /^#[0-9a-fA-F]{6}$/

export function tokensFromApi(tokens: Record<string, string>): BrandTokens | null {
  const t = {
    primary: tokens['color.primary'],
    onPrimary: tokens['color.on-primary'],
    background: tokens['color.background'],
    text: tokens['color.text'],
    accent: tokens['color.accent'],
  }
  return Object.values(t).every((v) => v !== undefined && HEX.test(v)) ? (t as BrandTokens) : null
}

/**
 * Deriva as camadas 1 e 2 do tema claro a partir dos cinco tokens do tenant. Cada derivado é
 * conferido contra AA e, se falhar, mantém o valor Grafite: a marca do tenant não consegue
 * produzir uma tela ilegível.
 */
export function deriveLight(t: BrandTokens): DerivedTheme {
  const base = grafiteLight.colors as Record<string, string>
  const fallbacks: string[] = []
  const pick = (name: string, candidate: string, against: string, min = AA) => {
    if (contrast(candidate, against) >= min) return candidate
    fallbacks.push(name)
    return base[name] as string
  }
  const surface = pick('surface', mix(t.background, '#FFFFFF', 0.4), t.text)
  const container = mix(t.primary, t.background, 0.14)
  return {
    fallbacks,
    colors: {
      primary: t.primary,
      'on-primary': t.onPrimary,
      background: t.background,
      'on-background': t.text,
      'on-surface': t.text,
      accent: t.accent,
      surface,
      'surface-variant': mix(t.text, t.background, 0.08),
      'surface-light': mix(surface, t.background, 0.5),
      'primary-container': container,
      'on-primary-container': readableOn(container, [t.primary, t.text]),
    },
  }
}

/** Dark do tenant: primária clareada a L OKLCH >= 0,78 e conferida contra a superfície escura. */
export function deriveDark(t: BrandTokens): DerivedTheme {
  const fallbacks: string[] = []
  const surface = (grafiteDark.colors as Record<string, string>).surface as string
  let primary = liftLightness(t.primary, 0.78)
  if (contrast(primary, surface) < AA) {
    primary = (grafiteDark.colors as Record<string, string>).primary as string
    fallbacks.push('primary')
  }
  const container = mix(primary, '#151515', 0.16)
  return {
    fallbacks,
    colors: {
      primary,
      'on-primary': readableOn(primary, ['#0B2238', '#151515', '#FFFFFF']),
      'primary-container': container,
      'on-primary-container': readableOn(container, ['#CFE2F5', primary, '#EDEBE6']),
    },
  }
}

type ThemeApi = { themes: { value: Record<string, ThemeDefinition> } }

export function applyTenantTheme(theme: ThemeApi, tokens: BrandTokens): { light: DerivedTheme; dark: DerivedTheme } {
  const light = deriveLight(tokens)
  const dark = deriveDark(tokens)
  Object.assign(theme.themes.value.grafiteLight?.colors ?? {}, light.colors)
  Object.assign(theme.themes.value.grafiteDark?.colors ?? {}, dark.colors)
  return { light, dark }
}

export function resetTenantTheme(theme: ThemeApi) {
  Object.assign(theme.themes.value.grafiteLight?.colors ?? {}, grafiteLight.colors)
  Object.assign(theme.themes.value.grafiteDark?.colors ?? {}, grafiteDark.colors)
}
