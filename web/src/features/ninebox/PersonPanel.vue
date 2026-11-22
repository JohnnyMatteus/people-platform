<script setup lang="ts">
import { mdiArrowRight } from '@mdi/js'
import { computed, ref, watch } from 'vue'
import HatchPanel from '@/components/signature/HatchPanel.vue'
import { useAdjust, useNineBoxHistory } from '@/features/performance/queries'
import { dateTime, decimal } from '@/lib/format'
import { QUADRANTS, reachableQuadrants } from '@/lib/ninebox'
import { errorMessage } from '@/lib/http'
import type { CalibrationSession, Placement, Quadrant } from '@/lib/schemas/performance'

const props = defineProps<{ session: CalibrationSession; placement: Placement }>()

const id = computed(() => props.placement.employee_id)
const history = useNineBoxHistory(id)
const adjust = useAdjust()

const target = ref<Quadrant | null>(null)
const justification = ref('')
const error = ref<string | null>(null)
watch(id, () => {
  target.value = null
  justification.value = ''
  error.value = null
})

const open = computed(() => props.session.status === 'open')
const min = computed(() => props.session.policy.min_justification_length)
const options = computed(() =>
  reachableQuadrants(props.placement.original, props.session.policy.max_steps_per_axis)
    .filter((q) => q !== props.placement.current)
    .map((q) => ({ value: q, title: `${q} · ${QUADRANTS[q].label}` })),
)
const mine = computed(() => props.session.adjustments.filter((a) => a.employee_id === id.value))
const valid = computed(() => !!target.value && justification.value.trim().length >= min.value)

async function submit() {
  if (!valid.value || !target.value) return
  error.value = null
  try {
    await adjust.mutateAsync({
      sessionId: props.session.id,
      cycleId: props.session.cycle_id,
      employee_id: id.value,
      to: target.value,
      justification: justification.value.trim(),
    })
    target.value = null
    justification.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
}

const trendText: Record<string, string> = {
  UP: 'subindo',
  DOWN: 'descendo',
  STABLE: 'estável',
  LATERAL: 'lateral (subiu num eixo e desceu no outro)',
  INSUFFICIENT_DATA: 'poucos ciclos para uma tendência',
}
</script>

<template>
  <aside aria-label="Pessoa selecionada">
    <p class="axis-label text-medium-emphasis mb-1">Pessoa selecionada</p>
    <h2 class="section-title mb-1">{{ placement.employee_name }}</h2>
    <p class="mb-4">
      <v-chip>{{ placement.original }} {{ QUADRANTS[placement.original].label }}</v-chip>
      <template v-if="placement.original !== placement.current">
        <v-icon :icon="mdiArrowRight" size="18" class="mx-1" aria-label="calibrado para" />
        <v-chip>{{ placement.current }} {{ QUADRANTS[placement.current].label }}</v-chip>
      </template>
    </p>

    <dl class="scores tnum mb-4">
      <div><dt class="axis-label">desempenho</dt><dd>{{ decimal(placement.performance_score) }}</dd></div>
      <div><dt class="axis-label">potencial</dt><dd>{{ decimal(placement.potential_score) }}</dd></div>
    </dl>

    <h3 class="axis-label mb-2">Histórico de posições</h3>
    <p v-if="history.isPending.value" class="text-medium-emphasis text-body-2" role="status">Carregando histórico…</p>
    <p v-else-if="!history.data.value || history.data.value.entries.length === 0" class="text-medium-emphasis text-body-2">
      Primeiro ciclo fechado desta pessoa: ainda não há trajetória para comparar.
    </p>
    <template v-else>
      <ol class="trail-line mb-1" aria-label="Posições por ciclo">
        <li v-for="e in history.data.value.entries" :key="e.cycle_id">
          <span class="tnum">{{ e.quadrant }}</span>
          <span v-if="e.calibrated" class="text-caption"> calibrado</span>
        </li>
      </ol>
      <p class="text-caption text-medium-emphasis mb-4">Tendência: {{ trendText[history.data.value.trend] }}.</p>
    </template>

    <HatchPanel label="visível só para gestão e RH" class="mb-4">
      <h3 class="axis-label mb-2">Justificativas de calibração</h3>
      <p v-if="mine.length === 0" class="text-body-2 text-medium-emphasis">Nenhum ajuste nesta sessão.</p>
      <ul v-else class="adjustments">
        <li v-for="a in mine" :key="a.at">
          <p class="text-caption tnum">{{ a.from }} → {{ a.to }} · {{ dateTime(a.at) }}</p>
          <p class="voice">{{ a.justification }}</p>
        </li>
      </ul>
    </HatchPanel>

    <form v-if="open" @submit.prevent="submit">
      <h3 class="axis-label mb-2">Mover pessoa</h3>
      <v-select v-model="target" :items="options" label="Novo quadrante" hint="Até a faixa permitida pela política, contada a partir da posição original" persistent-hint class="mb-4" />
      <v-textarea
        v-model="justification"
        label="Justificativa (vai para a trilha de auditoria)"
        :counter="min"
        :rows="3"
        class="mb-2"
        :hint="`Mínimo de ${min} caracteres`"
      />
      <v-alert v-if="error" type="error" class="mb-2" density="compact">{{ error }}</v-alert>
      <v-btn type="submit" color="primary" :disabled="!valid" :loading="adjust.isPending.value">Mover e registrar</v-btn>
    </form>
    <p v-else class="text-body-2 text-medium-emphasis">Sessão fechada: o resultado é definitivo e já foi publicado.</p>
  </aside>
</template>

<style scoped>
.scores {
  display: flex;
  gap: 32px;
  margin: 0;
}
.scores dd {
  font-size: 32px;
  line-height: 36px;
  font-weight: 600;
  margin: 0;
}
.scores dt {
  color: rgba(var(--v-theme-on-surface), 0.7);
}
.trail-line {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  list-style: none;
  padding: 0;
  margin: 0;
}
.trail-line li {
  border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 4px;
  padding: 2px 8px;
}
.trail-line li + li::before {
  content: '→ ';
}
.adjustments {
  list-style: none;
  padding: 0;
  margin: 0;
  display: grid;
  gap: 12px;
}
</style>
