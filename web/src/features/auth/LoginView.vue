<script setup lang="ts">
import { mdiLockOutline } from '@mdi/js'
import { useRoute } from 'vue-router'
import { useBranding } from '@/features/branding/useBranding'
import { session } from '@/lib/auth'

const brand = useBranding()
const route = useRoute()
const back = typeof route.query.voltar === 'string' ? route.query.voltar : '/'
</script>

<template>
  <div class="login">
    <v-card class="pa-8 login__sheet" max-width="440">
      <img v-if="brand.logoUrl.value" :src="brand.logoUrl.value" :alt="brand.name.value" height="36" class="mb-4" />
      <p class="axis-label text-medium-emphasis mb-1">{{ brand.isTenant.value ? 'Acesso do colaborador' : 'People Platform' }}</p>
      <h1 class="screen-title text-primary mb-4">{{ brand.name.value }}</h1>
      <p class="voice mb-6">Avaliação, desenvolvimento e sucessão num só caderno. Notas e justificativas só aparecem para quem tem papel de gestão.</p>
      <v-btn color="primary" block size="large" :prepend-icon="mdiLockOutline" @click="session.login(back)">Entrar com a conta da empresa</v-btn>
      <p class="text-caption text-medium-emphasis mt-4">Login pelo Keycloak (realm people), com PKCE.</p>
    </v-card>
  </div>
</template>

<style scoped>
.login {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 16px;
}
.login__sheet {
  width: 100%;
}
</style>
