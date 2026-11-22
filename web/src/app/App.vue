<script setup lang="ts">
import { mdiAccountOutline, mdiBriefcaseSearchOutline, mdiChartBoxOutline, mdiClipboardTextOutline, mdiForumOutline, mdiGridLarge, mdiLogout, mdiSourceBranch, mdiStairs, mdiWeatherNight, mdiWhiteBalanceSunny } from '@mdi/js'
import { useLocalStorage } from '@vueuse/core'
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useDisplay, useTheme } from 'vuetify'
import { useBranding } from '@/features/branding/useBranding'
import { useMe } from '@/features/performance/queries'
import { session } from '@/lib/auth'

const route = useRoute()
const theme = useTheme()
const display = useDisplay()
const brand = useBranding()
const me = useMe()
const mode = useLocalStorage<'grafiteLight' | 'grafiteDark'>('people-theme', 'grafiteLight')
watch(mode, (m) => (theme.global.name.value = m), { immediate: true })

const roles = computed(() => new Set(me.data.value?.roles ?? []))
const drawer = ref(false)

const nav = [
  { to: '/avaliacoes', label: 'Avaliações', icon: mdiClipboardTextOutline },
  { to: '/nine-box', label: 'Nine box', icon: mdiGridLarge, roles: ['manager', 'tenant-admin'] },
  { to: '/pdi', label: 'PDI', icon: mdiSourceBranch },
  { to: '/sucessao', label: 'Sucessão', icon: mdiStairs, roles: ['manager', 'tenant-admin'] },
  { to: '/recrutamento', label: 'Recrutamento', icon: mdiBriefcaseSearchOutline, roles: ['manager', 'tenant-admin'] },
  { to: '/engajamento', label: 'Engajamento', icon: mdiForumOutline },
  { to: '/analytics', label: 'Analytics', icon: mdiChartBoxOutline, roles: ['manager', 'tenant-admin'] },
]
const visible = computed(() => nav.filter((n) => !n.roles || n.roles.some((r) => roles.value.has(r))))
const shell = computed(() => !route.meta.public && session.isAuthenticated.value)
</script>

<template>
  <v-app>
    <template v-if="shell">
      <v-app-bar>
        <v-app-bar-nav-icon v-if="!display.lgAndUp.value" aria-label="Abrir navegação" @click="drawer = !drawer" />
        <img v-if="brand.logoUrl.value" :src="brand.logoUrl.value" :alt="brand.name.value" height="28" class="ml-2" />
        <v-app-bar-title class="text-body-1 font-weight-medium">{{ brand.name.value }}</v-app-bar-title>
        <v-spacer />
        <v-btn
          icon
          variant="text"
          :aria-label="mode === 'grafiteLight' ? 'Ativar tema escuro' : 'Ativar tema claro'"
          @click="mode = mode === 'grafiteLight' ? 'grafiteDark' : 'grafiteLight'"
        >
          <v-icon :icon="mode === 'grafiteLight' ? mdiWeatherNight : mdiWhiteBalanceSunny" />
        </v-btn>
        <v-menu>
          <template #activator="{ props }">
            <v-btn v-bind="props" variant="text" :prepend-icon="mdiAccountOutline">{{ me.data.value?.name ?? 'Conta' }}</v-btn>
          </template>
          <v-list>
            <v-list-item :title="[...roles].join(', ') || 'sem papéis'" subtitle="papéis" />
            <v-list-item :prepend-icon="mdiLogout" title="Sair" @click="session.logout()" />
          </v-list>
        </v-menu>
      </v-app-bar>
      <v-navigation-drawer v-model="drawer" :permanent="display.lgAndUp.value">
        <v-list nav aria-label="Módulos">
          <v-list-item v-for="item in visible" :key="item.to" :to="item.to" :prepend-icon="item.icon" :title="item.label" color="primary" />
        </v-list>
        <template #append>
          <p class="pa-4 text-caption text-medium-emphasis">
            Notas e justificativas aparecem só para quem tem papel de gestão. Dados protegidos levam hachura.
          </p>
        </template>
      </v-navigation-drawer>
    </template>
    <v-main>
      <router-view />
    </v-main>
  </v-app>
</template>
