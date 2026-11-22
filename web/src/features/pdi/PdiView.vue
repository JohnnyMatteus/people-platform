<script setup lang="ts">
import { computed } from 'vue'
import { errorMessage } from '@/lib/http'
import { useMe, usePdi } from '@/features/performance/queries'

const me = useMe()
const employeeId = computed(() => me.data.value?.employee_id)
const pdi = usePdi(employeeId)
const statusLabel = { PLANNED: 'Planejada', IN_PROGRESS: 'Em andamento', DONE: 'Concluída' } as const
</script>

<template>
  <div class="pa-6 reading">
    <h1 class="screen-title mb-4">Plano de desenvolvimento</h1>
    <v-alert v-if="pdi.isError.value" type="error" variant="tonal">{{ errorMessage(pdi.error.value) }}</v-alert>
    <v-progress-linear v-else-if="pdi.isPending.value" indeterminate />
    <template v-else-if="pdi.data.value">
      <h2 class="mb-2">Lacunas de competência</h2>
      <p v-if="!pdi.data.value.gaps.length" class="mb-4">Nenhuma lacuna registrada.</p>
      <v-chip v-for="g in pdi.data.value.gaps" :key="g.competency" class="mr-2 mb-4">{{ g.competency }} ({{ g.gap }})</v-chip>
      <h2 class="mb-2">Ações</h2>
      <p v-if="!pdi.data.value.actions.length">Nenhuma ação no plano.</p>
      <v-list v-else lines="two">
        <v-list-item
          v-for="a in pdi.data.value.actions"
          :key="a.id"
          :title="a.title"
          :subtitle="`${statusLabel[a.status]} · prazo ${a.due_on}`"
        />
      </v-list>
    </template>
  </div>
</template>
