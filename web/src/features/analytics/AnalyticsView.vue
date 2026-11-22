<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useCycles } from '@/features/performance/queries'
import { errorMessage } from '@/lib/http'
import { GRID_ORDER, QUADRANTS } from '@/lib/ninebox'
import type { GroupBy } from '@/lib/schemas/analytics'
import { useNineBoxDistribution, useTurnover } from './queries'

const groupBy = ref<GroupBy>('area')
const year = ref(new Date().getFullYear())
const part = ref('')
const period = computed(() => `${year.value}${part.value ? `-${part.value}` : ''}`)
const turnover = useTurnover(groupBy, period)

const cycles = useCycles()
const cycleId = ref<string | undefined>()
watch(
  () => cycles.data.value,
  (list) => {
    if (list && !cycleId.value) cycleId.value = (list.find((c) => c.status === 'closed') ?? list[0])?.id
  },
  { immediate: true },
)
const dist = useNineBoxDistribution(cycleId)

const groups: { value: GroupBy; title: string }[] = [
  { value: 'area', title: 'Área' },
  { value: 'unit', title: 'Unidade' },
  { value: 'tenure_band', title: 'Tempo de casa' },
  { value: 'age_band', title: 'Faixa etária' },
  { value: 'gender', title: 'Gênero' },
]
const parts = [
  { value: '', title: 'Ano inteiro' },
  ...[1, 2, 3, 4].map((q) => ({ value: `Q${q}`, title: `${q}º trimestre` })),
]
const pct = (v: number) => `${(v * 100).toFixed(1).replace('.', ',')}%`
const maxRate = computed(() => Math.max(0.0001, ...(turnover.data.value?.groups.map((g) => g.turnover_rate) ?? [0])))
const counts = computed(() => new Map(dist.data.value?.quadrants.map((q) => [q.quadrant, q.count]) ?? []))
</script>

<template>
  <div class="pa-6">
    <h1 class="screen-title mb-1">People analytics</h1>
    <p class="text-medium-emphasis mb-6">Só dados agregados. Grupos pequenos demais são omitidos para que ninguém seja identificado.</p>

    <section aria-labelledby="turnover-title" class="mb-10">
      <h2 id="turnover-title" class="section-title mb-3">Turnover e tempo de casa</h2>
      <div class="d-flex flex-wrap ga-4">
        <v-select v-model="groupBy" :items="groups" label="Agrupar por" density="compact" style="max-width: 220px" />
        <v-text-field v-model.number="year" type="number" label="Ano" density="compact" style="max-width: 120px" />
        <v-select v-model="part" :items="parts" label="Período" density="compact" style="max-width: 200px" />
      </div>
      <v-alert v-if="turnover.isError.value" type="error" variant="tonal">{{ errorMessage(turnover.error.value) }}</v-alert>
      <v-progress-linear v-else-if="turnover.isPending.value && turnover.fetchStatus.value !== 'idle'" indeterminate />
      <template v-else-if="turnover.data.value">
        <p v-if="!turnover.data.value.groups.length" class="mb-2">Nenhum grupo atinge o mínimo de {{ turnover.data.value.min_group_size }} pessoas neste período.</p>
        <v-table v-else>
          <caption class="sr-only">Turnover por grupo</caption>
          <thead>
            <tr>
              <th scope="col">Grupo</th>
              <th scope="col" class="text-right">Início</th>
              <th scope="col" class="text-right">Fim</th>
              <th scope="col" class="text-right">Admissões</th>
              <th scope="col" class="text-right">Desligamentos</th>
              <th scope="col">Turnover</th>
              <th scope="col" class="text-right">Tempo de casa (meses)</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="g in [...turnover.data.value.groups, ...(turnover.data.value.total ? [turnover.data.value.total] : [])]" :key="g.group">
              <th scope="row" class="text-left">{{ g.group }}</th>
              <td class="text-right tnum">{{ g.headcount_start }}</td>
              <td class="text-right tnum">{{ g.headcount_end }}</td>
              <td class="text-right tnum">{{ g.hires }}</td>
              <td class="text-right tnum">{{ g.terminations }}</td>
              <td>
                <span class="bar" aria-hidden="true"><span class="bar__fill" :style="{ width: `${(g.turnover_rate / maxRate) * 100}%` }" /></span>
                <span class="tnum">{{ pct(g.turnover_rate) }}</span>
              </td>
              <td class="text-right tnum">{{ g.avg_tenure_months.toFixed(1).replace('.', ',') }}</td>
            </tr>
          </tbody>
        </v-table>
        <p v-if="turnover.data.value.suppressed_groups > 0" class="text-caption mt-2">
          {{ turnover.data.value.suppressed_groups }} grupo(s) omitido(s) por terem menos de {{ turnover.data.value.min_group_size }} pessoas. A última linha é o total.
        </p>
      </template>
    </section>

    <section aria-labelledby="nb-title">
      <h2 id="nb-title" class="section-title mb-3">Nine box por ciclo</h2>
      <v-select
        v-model="cycleId"
        :items="cycles.data.value ?? []"
        item-title="name"
        item-value="id"
        label="Ciclo"
        density="compact"
        style="max-width: 320px"
      />
      <v-alert v-if="dist.isError.value" type="info" variant="tonal">Este ciclo ainda não tem distribuição publicada.</v-alert>
      <v-progress-linear v-else-if="dist.isPending.value && dist.fetchStatus.value !== 'idle'" indeterminate />
      <template v-else-if="dist.data.value">
        <ul class="nb" aria-label="Pessoas por quadrante">
          <li v-for="q in GRID_ORDER" :key="q" class="nb__cell">
            <span class="text-caption">{{ q }} {{ QUADRANTS[q].label }}</span>
            <span class="tnum nb__n">{{ counts.get(q) ?? 0 }}</span>
          </li>
        </ul>
        <p v-if="dist.data.value.suppressed_quadrants > 0" class="text-caption mt-2">
          {{ dist.data.value.suppressed_quadrants }} quadrante(s) omitido(s) por terem menos de {{ dist.data.value.min_group_size }} pessoas.
        </p>
      </template>
    </section>
  </div>
</template>

<style scoped>
.bar {
  display: inline-block;
  width: 120px;
  height: 8px;
  margin-right: 8px;
  background: rgb(var(--v-theme-surface-variant));
  vertical-align: middle;
}
.bar__fill {
  display: block;
  height: 100%;
  background: rgb(var(--v-theme-chart-1));
}
.nb {
  list-style: none;
  padding: 0;
  margin: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
  gap: 8px;
}
.nb__cell {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  padding: 8px 12px;
  border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
}
.nb__n {
  font-size: 20px;
  font-weight: 600;
}
</style>
