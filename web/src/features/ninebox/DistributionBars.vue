<script setup lang="ts">
import { computed } from 'vue'
import { GRID_ORDER, QUADRANTS } from '@/lib/ninebox'
import type { CalibrationSession } from '@/lib/schemas/performance'

/** Distribuição real por célula; o topo mostra o limite recomendado do tenant, não um alvo imposto. */
const props = defineProps<{ session: CalibrationSession }>()
const total = computed(() => props.session.placements.length || 1)
const guideline = computed(() => Number(props.session.policy.top_box_warning_share))
const rows = computed(() =>
  GRID_ORDER.map((q) => {
    const n = props.session.distribution[q] ?? 0
    return { q, n, share: n / total.value }
  }),
)
const pct = (v: number) => `${Math.round(v * 100)}%`
</script>

<template>
  <section aria-labelledby="dist-title">
    <h3 id="dist-title" class="axis-label mb-2">Distribuição por célula</h3>
    <ul class="dist">
      <li v-for="r in rows" :key="r.q" class="dist__row">
        <span class="dist__name text-caption">{{ r.q }} {{ QUADRANTS[r.q].label }}</span>
        <span class="dist__track" aria-hidden="true">
          <span class="dist__bar" :style="{ width: pct(r.share) }" />
          <span v-if="r.q === 'Q9'" class="dist__limit" :style="{ left: pct(guideline) }" />
        </span>
        <span class="tnum text-caption dist__n">
          {{ r.n }} ({{ pct(r.share) }})
          <template v-if="r.q === 'Q9'"> · limite recomendado {{ pct(guideline) }}</template>
        </span>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.dist {
  list-style: none;
  padding: 0;
  margin: 0;
  display: grid;
  gap: 6px;
}
.dist__row {
  display: grid;
  grid-template-columns: 190px minmax(60px, 1fr) minmax(120px, auto);
  align-items: center;
  gap: 12px;
}
.dist__track {
  position: relative;
  height: 8px;
  border-radius: 2px;
  background: rgb(var(--v-theme-surface-variant));
}
.dist__bar {
  display: block;
  height: 100%;
  border-radius: 2px;
  background: rgb(var(--v-theme-secondary));
}
.dist__limit {
  position: absolute;
  top: -3px;
  bottom: -3px;
  width: 0;
  border-left: 2px dashed rgb(var(--v-theme-on-surface));
}
@media (max-width: 720px) {
  .dist__row {
    grid-template-columns: 1fr;
    gap: 2px;
  }
}
</style>
