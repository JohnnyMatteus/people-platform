<script setup lang="ts">
import { errorMessage } from '@/lib/http'
import { useSuccessionPlans } from '@/features/performance/queries'

const plans = useSuccessionPlans()
const bench = { NONE: 'Sem sucessores', WEAK: 'Frágil', ADEQUATE: 'Adequado', STRONG: 'Forte' } as const
const readiness = { READY_NOW: 'Pronto agora', ONE_TO_TWO_YEARS: '1 a 2 anos', THREE_PLUS_YEARS: '3 anos ou mais' } as const
</script>

<template>
  <div class="pa-6 reading">
    <h1 class="screen-title mb-4">Sucessão</h1>
    <v-alert v-if="plans.isError.value" type="error" variant="tonal">{{ errorMessage(plans.error.value) }}</v-alert>
    <v-progress-linear v-else-if="plans.isPending.value" indeterminate />
    <p v-else-if="!plans.data.value?.length">Nenhuma posição crítica cadastrada.</p>
    <v-card v-for="p in plans.data.value" :key="p.position_id" class="pa-4 mb-3">
      <h2>{{ p.title }}</h2>
      <p class="text-medium-emphasis mb-2">
        {{ p.incumbent_name ?? 'Posição vaga' }} · banco de sucessão: {{ bench[p.bench_strength] }}
      </p>
      <v-chip v-for="s in p.successors" :key="s.employee_id" class="mr-2">
        {{ s.employee_name ?? s.employee_id }}: {{ readiness[s.readiness] }}
      </v-chip>
    </v-card>
  </div>
</template>
