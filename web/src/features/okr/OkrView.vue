<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useMe } from '@/features/performance/queries'
import { errorMessage } from '@/lib/http'
import type { Level, Objective } from '@/lib/schemas/okr'
import { useCheckIn, useCloseObjective, useCreateObjective, useObjectives } from './queries'

const me = useMe()
const objectives = useObjectives()
const create = useCreateObjective()
const checkIn = useCheckIn()
const close = useCloseObjective()
const error = ref<string | null>(null)
const values = reactive<Record<string, string>>({})
const draft = reactive({ level: 'INDIVIDUAL' as Level, title: '', parent_id: '', kr_title: '', start: '0', target: '' })

const roles = computed(() => new Set(me.data.value?.roles ?? []))
const levelsAllowed = computed<{ value: Level; title: string }[]>(() => [
  ...(roles.value.has('tenant-admin') ? [{ value: 'COMPANY' as Level, title: 'Empresa' }] : []),
  ...(roles.value.has('manager') || roles.value.has('tenant-admin') ? [{ value: 'TEAM' as Level, title: 'Equipe' }] : []),
  { value: 'INDIVIDUAL' as Level, title: 'Pessoa' },
])
const labels: Record<Level, string> = { COMPANY: 'Empresa', TEAM: 'Equipe', INDIVIDUAL: 'Pessoa' }
const groups = computed(() =>
  (['COMPANY', 'TEAM', 'INDIVIDUAL'] as Level[]).map((l) => ({ level: l, items: (objectives.data.value ?? []).filter((o) => o.level === l) })),
)
const parents = computed(() => {
  const order: Level[] = ['COMPANY', 'TEAM', 'INDIVIDUAL']
  return (objectives.data.value ?? []).filter((o) => o.status === 'ACTIVE' && order.indexOf(o.level) < order.indexOf(draft.level))
})
const pct = (p: string) => `${Math.round(Number(p) * 100)}%`
const canWrite = (o: Objective) => o.level === 'COMPANY' ? roles.value.has('tenant-admin') : roles.value.has('manager') || roles.value.has('tenant-admin') || o.owner_id === me.data.value?.employee_id

async function run(fn: () => Promise<unknown>) {
  error.value = null
  try {
    await fn()
  } catch (e) {
    error.value = errorMessage(e)
  }
}
const createObjective = () =>
  run(async () => {
    await create.mutateAsync({
      level: draft.level,
      title: draft.title.trim(),
      parent_id: draft.parent_id || undefined,
      key_results: [{ title: draft.kr_title.trim(), start: draft.start, target: draft.target }],
    })
    Object.assign(draft, { title: '', parent_id: '', kr_title: '', start: '0', target: '' })
  })
</script>

<template>
  <div class="pa-6 reading">
    <h1 class="screen-title mb-1">Objetivos e metas</h1>
    <p class="text-medium-emphasis mb-6">Da empresa à pessoa: cada nível se desdobra do de cima, e o progresso vem dos check-ins.</p>
    <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>
    <v-alert v-if="objectives.isError.value" type="error" variant="tonal">{{ errorMessage(objectives.error.value) }}</v-alert>
    <v-progress-linear v-else-if="objectives.isPending.value" indeterminate />

    <section v-for="g in groups" :key="g.level" class="mb-8">
      <h2 class="section-title mb-2">{{ labels[g.level] }}</h2>
      <p v-if="!g.items.length" class="text-caption">Nenhum objetivo neste nível.</p>
      <v-card v-for="o in g.items" :key="o.id" variant="outlined" class="pa-4 mb-3">
        <div class="d-flex align-center">
          <h3 class="font-weight-medium">{{ o.title }}</h3>
          <v-chip v-if="o.status === 'CLOSED'" size="x-small" class="ml-2">Encerrado</v-chip>
          <v-spacer />
          <span class="tnum">{{ pct(o.progress) }}</span>
        </div>
        <p v-if="o.owner_name" class="text-caption text-medium-emphasis">{{ o.owner_name }}</p>
        <v-progress-linear :model-value="Number(o.progress) * 100" height="6" class="my-2" />
        <div v-for="k in o.key_results" :key="k.id" class="mb-2">
          <div class="d-flex align-center">
            <span class="flex-grow-1">{{ k.title }}</span>
            <span class="tnum text-caption">{{ k.current }} de {{ k.target }} ({{ pct(k.progress) }})</span>
          </div>
          <div v-if="o.status === 'ACTIVE' && canWrite(o)" class="d-flex ga-2 align-center mt-1">
            <v-text-field v-model="values[k.id]" type="number" label="Valor atual" density="compact" hide-details style="max-width: 160px" />
            <v-btn size="small" variant="tonal" :disabled="!values[k.id]" :loading="checkIn.isPending.value" @click="run(async () => { await checkIn.mutateAsync({ keyResultId: k.id, value: values[k.id]! }); values[k.id] = '' })">
              Check-in
            </v-btn>
          </div>
        </div>
        <v-btn v-if="o.status === 'ACTIVE' && canWrite(o)" size="small" variant="text" @click="run(() => close.mutateAsync(o.id))">Encerrar objetivo</v-btn>
      </v-card>
    </section>

    <v-card variant="outlined" class="pa-4" title="Novo objetivo">
      <v-select v-model="draft.level" :items="levelsAllowed" label="Nível" density="compact" />
      <v-select
        v-model="draft.parent_id"
        :items="parents"
        item-title="title"
        item-value="id"
        label="Desdobra de (opcional)"
        density="compact"
        clearable
      />
      <v-text-field v-model="draft.title" label="Objetivo" density="compact" />
      <v-text-field v-model="draft.kr_title" label="Resultado-chave" density="compact" />
      <div class="d-flex ga-2">
        <v-text-field v-model="draft.start" type="number" label="Partida" density="compact" />
        <v-text-field v-model="draft.target" type="number" label="Meta" density="compact" />
      </div>
      <v-btn color="primary" :disabled="draft.title.trim().length < 3 || draft.kr_title.trim().length < 3 || draft.target === ''" :loading="create.isPending.value" @click="createObjective">
        Criar objetivo
      </v-btn>
    </v-card>
  </div>
</template>
