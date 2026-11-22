import { createRouter, createWebHistory } from 'vue-router'
import { session } from '@/lib/auth'

declare module 'vue-router' {
  interface RouteMeta {
    public?: boolean
    /** Papéis que enxergam a rota; vazio = qualquer pessoa autenticada. */
    roles?: string[]
    title?: string
  }
}

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/entrar', name: 'entrar', component: () => import('@/features/auth/LoginView.vue'), meta: { public: true, title: 'Entrar' } },
    { path: '/auth/callback', name: 'callback', component: () => import('@/features/auth/CallbackView.vue'), meta: { public: true } },
    { path: '/', redirect: '/avaliacoes' },
    { path: '/nine-box', name: 'nine-box', component: () => import('@/features/ninebox/NineBoxView.vue'), meta: { roles: ['manager', 'tenant-admin'], title: 'Nine box' } },
    { path: '/avaliacoes', name: 'avaliacoes', component: () => import('@/features/evaluation/EvaluationView.vue'), meta: { title: 'Avaliações' } },
    { path: '/pdi', name: 'pdi', component: () => import('@/features/pdi/PdiView.vue'), meta: { title: 'PDI' } },
    { path: '/sucessao', name: 'sucessao', component: () => import('@/features/succession/SuccessionView.vue'), meta: { roles: ['manager', 'tenant-admin'], title: 'Sucessão' } },
    { path: '/recrutamento', name: 'recrutamento', component: () => import('@/features/recruitment/RecruitmentView.vue'), meta: { roles: ['manager', 'tenant-admin'], title: 'Recrutamento' } },
    { path: '/engajamento', name: 'engajamento', component: () => import('@/features/engagement/EngagementView.vue'), meta: { title: 'Engajamento' } },
    { path: '/analytics', name: 'analytics', component: () => import('@/features/analytics/AnalyticsView.vue'), meta: { roles: ['manager', 'tenant-admin'], title: 'Analytics' } },
    { path: '/:rest(.*)*', name: 'nao-encontrado', component: () => import('@/features/auth/NotFoundView.vue'), meta: { public: true, title: 'Não encontrado' } },
  ],
})

router.beforeEach((to) => {
  if (to.meta.public) return true
  if (!session.isAuthenticated.value) return { name: 'entrar', query: { voltar: to.fullPath } }
  return true
})

router.afterEach((to) => {
  document.title = to.meta.title ? `${to.meta.title} · People Platform` : 'People Platform'
})
