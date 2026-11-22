<script setup lang="ts">
import { computed } from 'vue'
import { axisOf, GRID_ORDER, initials, plot, QUADRANTS } from '@/lib/ninebox'
import type { Placement, Policy, Quadrant } from '@/lib/schemas/performance'

/**
 * Nine box com a assinatura da identidade: cada pessoa é um ponto posicionado pela nota contínua
 * dentro da célula (não uma lista de nomes) e a calibração deixa um trilho do ponto original ao
 * novo. Os limiares do tenant ficam visíveis nas bordas. Sem semáforo: células em tons neutros.
 */
const props = defineProps<{ placements: Placement[]; policy: Policy; selectedId?: string | null }>()
const emit = defineEmits<{ select: [employeeId: string] }>()

const points = computed(() => plot(props.placements, props.policy))
const byId = computed(() => new Map(props.placements.map((p) => [p.employee_id, p])))
const perf = computed(() => axisOf(props.policy, 'performance'))
const pot = computed(() => axisOf(props.policy, 'potential'))
const counts = computed(() => {
  const c = Object.fromEntries(GRID_ORDER.map((q) => [q, 0])) as Record<Quadrant, number>
  for (const p of props.placements) c[p.current] += 1
  return c
})
const tone = (q: Quadrant) => {
  const rank = QUADRANTS[q].performance + QUADRANTS[q].potential
  return `color-mix(in srgb, rgb(var(--v-theme-surface-variant)) ${10 + rank * 14}%, rgb(var(--v-theme-surface)))`
}
const fmt = (n: number) => n.toLocaleString('pt-BR', { maximumFractionDigits: 2 })
const pct = (v: number) => `${(v * 100).toFixed(2)}%`

const describe = (p: Placement) => {
  const moved = p.original !== p.current ? `, calibrado de ${p.original} para ${p.current}` : ''
  return `${p.employee_name}, desempenho ${p.performance_score}, potencial ${p.potential_score}, quadrante ${p.current} ${QUADRANTS[p.current].label}${moved}`
}
</script>

<template>
  <figure class="nbx">
    <figcaption class="sr-only">Matriz de desempenho por potencial com {{ placements.length }} pessoas</figcaption>
    <span class="nbx__ylabel axis-label" aria-hidden="true">potencial</span>
    <div class="nbx__yticks tnum" aria-hidden="true">
      <span style="bottom: 100%">{{ fmt(pot.max) }}</span>
      <span style="bottom: 66.66%">{{ fmt(pot.highFrom) }}</span>
      <span style="bottom: 33.33%">{{ fmt(pot.mediumFrom) }}</span>
      <span style="bottom: 0%">{{ fmt(pot.min) }}</span>
    </div>

    <div class="nbx__plot" role="group" aria-label="Grade de nove quadrantes">
      <div
        v-for="q in GRID_ORDER"
        :key="q"
        class="nbx__cell"
        :style="{ background: tone(q) }"
      >
        <span class="nbx__cell-name">{{ QUADRANTS[q].label }}</span>
        <span class="nbx__cell-count tnum" :aria-label="`${counts[q]} pessoas em ${q}`">{{ counts[q] }}</span>
      </div>

      <svg class="nbx__trails" viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true">
        <line
          v-for="p in points.filter((x) => x.origin)"
          :key="p.employeeId"
          :x1="p.origin!.x * 100"
          :y1="100 - p.origin!.y * 100"
          :x2="p.x * 100"
          :y2="100 - p.y * 100"
          class="nbx__trail"
        />
      </svg>

      <template v-for="p in points" :key="p.employeeId">
        <span
          v-if="p.origin"
          class="nbx__ghost"
          aria-hidden="true"
          :style="{ left: pct(p.origin.x), bottom: pct(p.origin.y) }"
        />
        <button
          type="button"
          class="nbx__point"
          :class="{
            'nbx__point--selected': p.employeeId === selectedId,
            'nbx__point--moved': !!p.origin,
          }"
          :style="{ left: pct(p.x), bottom: pct(p.y) }"
          :aria-label="describe(byId.get(p.employeeId)!)"
          :aria-pressed="p.employeeId === selectedId"
          @click="emit('select', p.employeeId)"
        >
          {{ initials(byId.get(p.employeeId)!.employee_name) }}
        </button>
      </template>
    </div>

    <div class="nbx__xticks tnum" aria-hidden="true">
      <span style="left: 0%">{{ fmt(perf.min) }}</span>
      <span style="left: 33.33%">{{ fmt(perf.mediumFrom) }}</span>
      <span style="left: 66.66%">{{ fmt(perf.highFrom) }}</span>
      <span style="left: 100%">{{ fmt(perf.max) }}</span>
    </div>
    <span class="nbx__xlabel axis-label" aria-hidden="true">desempenho</span>

    <table class="sr-only">
      <caption>Pessoas por quadrante</caption>
      <thead>
        <tr><th>Pessoa</th><th>Quadrante</th><th>Original</th></tr>
      </thead>
      <tbody>
        <tr v-for="p in placements" :key="p.employee_id">
          <td>{{ p.employee_name }}</td><td>{{ p.current }}</td><td>{{ p.original }}</td>
        </tr>
      </tbody>
    </table>
  </figure>
</template>

<style scoped>
.nbx {
  margin: 0;
  display: grid;
  grid-template-columns: 20px 40px minmax(0, 1fr);
  grid-template-rows: minmax(0, 1fr) 24px 20px;
  gap: 0 6px;
  width: 100%;
}
.nbx__ylabel {
  grid-row: 1;
  grid-column: 1;
  writing-mode: vertical-rl;
  transform: rotate(180deg);
  text-align: center;
  color: rgb(var(--v-theme-on-surface));
}
.nbx__yticks {
  grid-row: 1;
  grid-column: 2;
  position: relative;
  font-size: 12px;
}
.nbx__yticks span {
  position: absolute;
  right: 0;
  transform: translateY(50%);
  color: rgba(var(--v-theme-on-surface), 0.7);
}
.nbx__plot {
  grid-row: 1;
  grid-column: 3;
  position: relative;
  display: grid;
  grid-template: repeat(3, 1fr) / repeat(3, 1fr);
  aspect-ratio: 1.25 / 1;
  min-height: 320px;
}
.nbx__cell {
  position: relative;
  border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 4px;
  margin: -0.5px;
  padding: 6px 8px;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 6px;
}
.nbx__cell-name {
  font-size: 12px;
  line-height: 16px;
  color: rgba(var(--v-theme-on-surface), 0.7);
}
.nbx__cell-count {
  font-size: 12px;
  font-weight: 600;
}
.nbx__trails {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
}
.nbx__trail {
  stroke: rgb(var(--v-theme-hatch));
  stroke-width: 2;
  stroke-dasharray: 4 3;
  vector-effect: non-scaling-stroke;
}
.nbx__ghost {
  position: absolute;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  border: 1.5px dashed rgb(var(--v-theme-hatch));
  background: transparent;
  transform: translate(-50%, 50%);
  pointer-events: none;
}
.nbx__point {
  position: absolute;
  width: 30px;
  height: 30px;
  border-radius: 50%;
  transform: translate(-50%, 50%);
  background: rgb(var(--v-theme-secondary));
  color: rgb(var(--v-theme-on-secondary));
  border: 2px solid rgb(var(--v-theme-surface));
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  display: grid;
  place-items: center;
}
@media (prefers-reduced-motion: no-preference) {
  .nbx__point {
    transition: left 200ms ease, bottom 200ms ease;
  }
}
.nbx__point--moved {
  box-shadow: 0 0 0 2px rgb(var(--v-theme-secondary));
}
.nbx__point--selected {
  outline: 3px solid rgb(var(--v-theme-on-surface));
  outline-offset: 2px;
  z-index: 2;
}
.nbx__xticks {
  grid-row: 2;
  grid-column: 3;
  position: relative;
  font-size: 12px;
  color: rgba(var(--v-theme-on-surface), 0.7);
}
.nbx__xticks span {
  position: absolute;
  top: 4px;
  transform: translateX(-50%);
}
.nbx__xlabel {
  grid-row: 3;
  grid-column: 3;
  text-align: center;
}
</style>
