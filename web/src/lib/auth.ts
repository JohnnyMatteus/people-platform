import { UserManager, WebStorageStateStore, type User } from 'oidc-client-ts'
import { computed, ref } from 'vue'
import { config } from './config'

/** Login OIDC (code + PKCE) no Keycloak, realm `people`. O token fica só em sessionStorage. */
const manager = new UserManager({
  authority: config.oidc.authority,
  client_id: config.oidc.clientId,
  redirect_uri: `${window.location.origin}/auth/callback`,
  post_logout_redirect_uri: `${window.location.origin}/entrar`,
  response_type: 'code',
  scope: 'openid profile email',
  automaticSilentRenew: true,
  userStore: new WebStorageStateStore({ store: window.sessionStorage }),
})

const user = ref<User | null>(null)
const ready = ref(false)

manager.events.addUserLoaded((u) => (user.value = u))
manager.events.addUserUnloaded(() => (user.value = null))
manager.events.addAccessTokenExpired(() => (user.value = null))

export async function restoreSession() {
  user.value = await manager.getUser()
  ready.value = true
}

export const session = {
  user,
  ready,
  isAuthenticated: computed(() => !!user.value && !user.value.expired),
  accessToken: () => (user.value && !user.value.expired ? user.value.access_token : null),
  login: (returnTo?: string) => manager.signinRedirect({ state: { returnTo } }),
  async completeLogin(): Promise<string> {
    const u = await manager.signinCallback()
    user.value = u ?? null
    return ((u?.state as { returnTo?: string } | undefined)?.returnTo) ?? '/'
  },
  logout: () => manager.signoutRedirect(),
}
