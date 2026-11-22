<script setup lang="ts">
import { mdiCheckCircleOutline, mdiLockOutline } from '@mdi/js'
import { computed, ref, watch } from 'vue'
import NineBoxGrid from '@/components/signature/NineBoxGrid.vue'
import { QUADRANTS, GRID_ORDER } from '@/lib/ninebox'
import { errorMessage } from '@/lib/http'
import { ApiError } from '@/lib/http'
import {
  useCloseSession,
  useCycleSession,
  useCycles,
  useMe,
  useOpenSession,
} from '@/features/performance/queries'
import DistributionBars from './DistributionBars.vue'
import PersonPanel from './PersonPanel.vue'

const cycles = useCycles()
const cycleId = ref<string | undefined>()
const me = useMe()
watch(
  () => cycles.data.value,
  (list) => {
    if (list && !cycleId.value) cycleId.value = (list.find((c) => c.status === 'calibration') ?? list[0])?.id
  },
  { immediate: true },
)
const cycle = computed(() => cycles.data.value?.find((c) => c.id === cycleId.value))
const session = useCycleSession(cycleId)
const selected = ref<string | null>(null)
const openSession = useOpenSession()
const closeSession = useCloseSession()
const confirmClose = ref(false)
const actionError = ref<string | null>(null)

const noSession = computed(() => session.error.value instanceof ApiError && session.error.value.status === 404)
const current = computed(() => session.data.value)
const chosen = computed(() => current.value?.placements.find((p) => p.employee_id === selected.value))
watch(cycleId, () => (selected.value = null))

const isManager = computed(() => me.data.value?.roles.some((r) => r === 'manager' || r === 'tenant-admin') ?? false)

async function open() {
  actionError.value = null
  try {
    await openSession.mutateAsync(cycleId.value!)
  } catch (e) {
    actionError.value = errorMessage(e)
  }
}
async function close() {
  actionError.value = null
  try {
    await closeSession.mutateAsync({ sessionId: current.value!.id, cycleId: cycleId.value! })
    confirmClose.value = false
  } catch (e) {
    actionError.value = errorMessage(e)
    confirmClose.value = false
  }
}
</script>

<template>
  <div class="pa-6">
    <header class="d-flex flex-wrap align-center ga-4 mb-6">
      <div>
        <p class="axis-label text-medium-emphasis">Calibração</p>
        <h1 class="screen-title">Nine box</h1>
      </div>
      <v-spacer />
      <v-select
        v-if="cycles.data.value?.length"
        v-model="cycleId"
        :items="cycles.data.value"
        item-title="name"
        item-value="id"
        label="Ciclo"
        hide-details
        style="max-width: 280px"
      />
      <v-chip v-if="cycle" :prepend-icon="cycle.status === 'closed' ? mdiLockOutline : undefined">{{ cycle.status === 'closed' ? 'fechado' : cycle.status === 'calibration' ? 'em calibração' : cycle.status === 'open' ? 'avaliações abertas' : 'rascunho' }}</v-chip>
    </header>

    <v-alert v-if="actionError" type="error" class="mb-4" closable @click:close="actionError = null">{{ actionError }}</v-alert>

    <p v-if="cycles.isPending.value" class="text-medium-emphasis" role="status">Carregando ciclos…</p>
    <v-alert v-else-if="cycles.isError.value" type="error" title="Não foi possível carregar os ciclos">{{ errorMessage(cycles.error.value) }}</v-alert>
    <section v-else-if="!cycles.data.value?.length" class="reading">
      <h2 class="section-title mb-2">Nenhum ciclo de avaliação ainda</h2>
      <p>Crie um ciclo em Avaliações. Quando gestores e colaboradores enviarem as notas, a sessão de calibração abre aqui.</p>
    </section>

    <template v-else>
      <p v-if="session.isPending.value" class="text-medium-emphasis" role="status">Carregando sessão…</p>

      <!-- Estado vazio: a grade com as nove células nomeadas, como a direção pede. -->
      <section v-else-if="noSession" class="empty">
        <div class="empty__grid" aria-hidden="true">
          <div v-for="q in GRID_ORDER" :key="q" class="empty__cell text-caption">{{ q }} · {{ QUADRANTS[q].label }}</div>
        </div>
        <div class="reading">
          <h2 class="section-title mb-2">As notas deste ciclo ainda não foram fechadas</h2>
          <p class="mb-4">
            A sessão de calibração nasce das avaliações consolidadas (autoavaliação e gestor).
            {{ cycle?.status === 'open' ? 'O ciclo está aberto: quando houver ao menos uma pessoa consolidada, abra a sessão.' : 'O ciclo ainda não está aberto para avaliações.' }}
          </p>
          <v-btn v-if="isManager && cycle?.status === 'open'" color="primary" :loading="openSession.isPending.value" @click="open">Abrir sessão de calibração</v-btn>
        </div>
      </section>

      <v-alert v-else-if="session.isError.value" type="error">{{ errorMessage(session.error.value) }}</v-alert>

      <div v-else-if="current" class="layout">
        <div>
          <v-alert
            v-for="w in current.warnings"
            :key="w.code"
            type="warning"
            class="mb-4"
            title="Topo da matriz acima do recomendado"
          >
            {{ Math.round(Number(w.share) * 100) }}% das pessoas estão em Q9; o limite recomendado do tenant é {{ Math.round(Number(w.guideline) * 100) }}%. É um alerta: a decisão é do comitê.
          </v-alert>
          <NineBoxGrid :placements="current.placements" :policy="current.policy" :selected-id="selected" @select="selected = $event" />
          <v-divider class="my-6" />
          <DistributionBars :session="current" />
          <div class="mt-6 d-flex align-center ga-4">
            <v-btn v-if="current.status === 'open'" color="primary" :prepend-icon="mdiCheckCircleOutline" @click="confirmClose = true">Fechar sessão e publicar</v-btn>
            <p class="text-body-2 text-medium-emphasis">
              {{ current.status === 'open' ? 'Fechar publica a posição final de cada pessoa e encerra o ciclo.' : 'Sessão fechada e publicada.' }}
            </p>
          </div>
        </div>
        <div class="panel">
          <PersonPanel v-if="chosen" :session="current" :placement="chosen" />
          <div v-else>
            <p class="axis-label text-medium-emphasis mb-1">Painel</p>
            <p class="text-body-2">Selecione um ponto para ver o histórico, as justificativas e mover a pessoa de quadrante. Linhas tracejadas mostram quem já foi calibrado e de onde veio.</p>
          </div>
        </div>
      </div>
    </template>

    <v-dialog v-model="confirmClose" max-width="460">
      <v-card class="pa-4">
        <h2 class="section-title mb-2">Fechar a sessão?</h2>
        <p class="mb-4">Depois de fechada a sessão não aceita mais ajustes e cada posição final é publicada para o PDI e o analytics.</p>
        <div class="d-flex justify-end ga-2">
          <v-btn variant="text" @click="confirmClose = false">Cancelar</v-btn>
          <v-btn color="primary" :loading="closeSession.isPending.value" @click="close">Fechar sessão</v-btn>
        </div>
      </v-card>
    </v-dialog>
  </div>
</template>

<style scoped>
.layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 360px;
  gap: 32px;
  align-items: start;
}
.panel {
  position: sticky;
  top: 72px;
  border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 8px;
  padding: 20px;
  background: rgb(var(--v-theme-surface));
}
.empty {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 32px;
  align-items: center;
}
.empty__grid {
  display: grid;
  grid-template: repeat(3, 1fr) / repeat(3, 1fr);
  aspect-ratio: 1.25 / 1;
}
.empty__cell {
  border: 1px dashed rgba(var(--v-border-color), 0.4);
  border-radius: 4px;
  padding: 6px 8px;
  color: rgba(var(--v-theme-on-surface), 0.7);
}
@media (max-width: 1100px) {
  .layout,
  .empty {
    grid-template-columns: 1fr;
  }
  .panel {
    position: static;
  }
}
</style>
