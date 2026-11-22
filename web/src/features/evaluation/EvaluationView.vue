<script setup lang="ts">
import { errorMessage } from '@/lib/http'
import { useCycles } from '@/features/performance/queries'

const cycles = useCycles()
const status = { draft: 'Rascunho', open: 'Aberto', calibration: 'Em calibração', closed: 'Encerrado' } as const
</script>

<template>
  <div class="pa-6 reading">
    <h1 class="screen-title mb-4">Avaliações</h1>
    <v-alert v-if="cycles.isError.value" type="error" variant="tonal">{{ errorMessage(cycles.error.value) }}</v-alert>
    <v-progress-linear v-else-if="cycles.isPending.value" indeterminate />
    <p v-else-if="!cycles.data.value?.length">Nenhum ciclo de avaliação.</p>
    <v-list v-else lines="two">
      <v-list-item v-for="c in cycles.data.value" :key="c.id" :title="c.name" :subtitle="`${status[c.status]} · tipo ${c.type}`" />
    </v-list>
  </div>
</template>
