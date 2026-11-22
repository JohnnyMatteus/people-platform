import { useQuery } from '@tanstack/vue-query'
import { computed, watch } from 'vue'
import { useTheme } from 'vuetify'
import { session } from '@/lib/auth'
import { get } from '@/lib/http'
import { branding } from '@/lib/schemas/tenant'
import { applyTenantTheme, resetTenantTheme, tokensFromApi } from '@/lib/theme'

/**
 * White label: antes do login o tenant vem do domínio acessado (`/v1/branding`); depois, da claim
 * do token (`/v1/me/tenant`). Só os cinco tokens de marca entram; o resto do tema é derivado e
 * conferido contra AA em `lib/theme.ts`.
 */
export function useBranding() {
  const theme = useTheme()
  const authenticated = session.isAuthenticated
  const query = useQuery({
    queryKey: computed(() => ['branding', authenticated.value]),
    queryFn: () => get(branding, authenticated.value ? '/tenant/v1/me/tenant' : '/tenant/v1/branding'),
    retry: false,
    staleTime: 10 * 60_000,
  })

  watch(
    () => query.data.value,
    (data) => {
      const tokens = data ? tokensFromApi(data.tokens) : null
      if (tokens) applyTenantTheme(theme, tokens)
      else resetTenantTheme(theme)
    },
    { immediate: true },
  )

  return {
    name: computed(() => query.data.value?.name ?? 'People Platform'),
    logoUrl: computed(() => query.data.value?.logo_url),
    isTenant: computed(() => !!query.data.value),
  }
}
