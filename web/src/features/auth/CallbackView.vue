<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { session } from '@/lib/auth'

const router = useRouter()
const failed = ref<string | null>(null)

onMounted(async () => {
  try {
    await router.replace(await session.completeLogin())
  } catch (e) {
    failed.value = e instanceof Error ? e.message : 'Falha ao concluir o login.'
  }
})
</script>

<template>
  <div class="pa-8">
    <v-alert v-if="failed" type="error" title="Não foi possível entrar">{{ failed }}</v-alert>
    <p v-else class="text-medium-emphasis" role="status">Concluindo o login…</p>
  </div>
</template>
