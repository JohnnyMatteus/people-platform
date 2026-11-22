/** Configuração de runtime. Os padrões apontam para o ambiente local do docker-compose. */
export const config = {
  apiBase: import.meta.env.VITE_API_BASE ?? '/api',
  oidc: {
    authority: import.meta.env.VITE_OIDC_AUTHORITY ?? 'http://localhost:8180/realms/people',
    clientId: import.meta.env.VITE_OIDC_CLIENT_ID ?? 'people-web',
  },
} as const
