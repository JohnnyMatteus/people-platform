<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useMe } from '@/features/performance/queries'
import { useNames } from '@/features/people/useNames'
import { errorMessage } from '@/lib/http'
import type { ItemKind } from '@/lib/schemas/learning'
import {
  useAddItem, useBindOnboarding, useComplete, useCreateTrack, useDefaultTrack, useEnroll, useMyEnrollments, usePendingOnboarding,
  useSetDefaultTrack, useSuggestions, useTracks,
} from './queries'

const me = useMe()
const isAdmin = computed(() => me.data.value?.roles.includes('tenant-admin') ?? false)
const tracks = useTracks()
const mine = useMyEnrollments()
const competency = ref('')
const suggestions = useSuggestions(competency)
const enroll = useEnroll()
const complete = useComplete()
const createTrack = useCreateTrack()
const addItem = useAddItem()
const setDefault = useSetDefaultTrack()
const bind = useBindOnboarding()
const defaultTrack = useDefaultTrack(isAdmin)
const pending = usePendingOnboarding(isAdmin)
const { employees } = useNames()
const error = ref<string | null>(null)
const draft = reactive({ title: '', description: '' })
const newItem = reactive({ trackId: '', title: '', kind: 'video' as ItemKind, minutes: 10, competency: '' })
const bindTo = reactive<Record<string, string>>({})
const enrolledIds = computed(() => new Set(mine.data.value?.map((e) => e.track_id)))
const kinds: { value: ItemKind; title: string }[] = [
  { value: 'video', title: 'Vídeo' },
  { value: 'article', title: 'Leitura' },
  { value: 'course', title: 'Curso' },
  { value: 'practice', title: 'Prática' },
]
const sourceLabel = { self: 'Escolhida por você', assigned: 'Atribuída pela empresa', onboarding: 'Integração' } as const

async function run(fn: () => Promise<unknown>) {
  error.value = null
  try {
    await fn()
  } catch (e) {
    error.value = errorMessage(e)
  }
}
</script>

<template>
  <div class="pa-6 reading">
    <h1 class="screen-title mb-4">Aprendizagem</h1>
    <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>

    <section aria-labelledby="mine-title" class="mb-8">
      <h2 id="mine-title" class="section-title mb-2">Minhas trilhas</h2>
      <v-progress-linear v-if="mine.isPending.value" indeterminate />
      <p v-else-if="!mine.data.value?.length">Você ainda não está em nenhuma trilha.</p>
      <v-card v-for="e in mine.data.value" :key="e.id" variant="outlined" class="pa-4 mb-3">
        <div class="d-flex align-center">
          <h3 class="font-weight-medium">{{ e.track?.title }}</h3>
          <v-spacer />
          <v-chip size="x-small">{{ sourceLabel[e.source] }}</v-chip>
        </div>
        <v-progress-linear :model-value="((e.done_items ?? 0) / Math.max(1, e.total_items ?? 1)) * 100" height="6" class="my-2" />
        <p class="text-caption tnum">{{ e.done_items }} de {{ e.total_items }} conteúdos</p>
        <div v-if="e.next_item" class="d-flex align-center mt-2">
          <span>Próximo: {{ e.next_item.title }} <span class="text-caption tnum">({{ e.next_item.minutes }} min)</span></span>
          <v-spacer />
          <v-btn size="small" color="primary" :loading="complete.isPending.value" @click="run(() => complete.mutateAsync({ enrollmentId: e.id, itemId: e.next_item!.id }))">
            Concluir
          </v-btn>
        </div>
        <p v-else class="mt-2">Trilha concluída.</p>
      </v-card>
    </section>

    <section aria-labelledby="sugg-title" class="mb-8">
      <h2 id="sugg-title" class="section-title mb-2">O que fazer agora</h2>
      <v-list v-if="suggestions.data.value?.next.length" density="compact">
        <v-list-item v-for="s in suggestions.data.value.next" :key="s.item.id" :title="s.item.title" :subtitle="`${s.track_title} · ${s.reason}`" />
      </v-list>
      <p v-else class="text-caption">Nada pendente.</p>
      <v-text-field v-model="competency" label="Desenvolver uma competência (ex.: comunicação)" density="compact" class="mt-3" clearable />
      <v-list v-if="competency && suggestions.data.value?.for_competency.length" density="compact">
        <v-list-item
          v-for="s in suggestions.data.value.for_competency"
          :key="s.item.id"
          :title="s.track_title"
          :subtitle="`${s.item.title} · ${s.reason}`"
        />
      </v-list>
    </section>

    <section aria-labelledby="all-title" class="mb-8">
      <h2 id="all-title" class="section-title mb-2">Trilhas da empresa</h2>
      <p v-if="!tracks.data.value?.length" class="text-caption">Nenhuma trilha cadastrada.</p>
      <v-list>
        <v-list-item v-for="t in tracks.data.value" :key="t.id" :title="t.title" :subtitle="`${t.items.length} conteúdos · ${t.minutes} min`">
          <template #append>
            <v-btn v-if="!enrolledIds.has(t.id) && t.items.length" size="small" variant="tonal" @click="run(() => enroll.mutateAsync({ trackId: t.id }))">Começar</v-btn>
          </template>
        </v-list-item>
      </v-list>
    </section>

    <template v-if="isAdmin">
      <v-divider class="mb-6" />
      <h2 class="section-title mb-4">Administração</h2>
      <v-card variant="outlined" class="pa-4 mb-4" title="Nova trilha">
        <v-text-field v-model="draft.title" label="Título" density="compact" />
        <v-text-field v-model="draft.description" label="Descrição (opcional)" density="compact" />
        <v-btn color="primary" :disabled="!draft.title.trim()" :loading="createTrack.isPending.value" @click="run(async () => { await createTrack.mutateAsync({ title: draft.title.trim(), description: draft.description }); draft.title = ''; draft.description = '' })">
          Criar trilha
        </v-btn>
      </v-card>
      <v-card variant="outlined" class="pa-4 mb-4" title="Novo conteúdo">
        <v-select v-model="newItem.trackId" :items="tracks.data.value ?? []" item-title="title" item-value="id" label="Trilha" density="compact" />
        <v-text-field v-model="newItem.title" label="Título" density="compact" />
        <v-select v-model="newItem.kind" :items="kinds" label="Tipo" density="compact" />
        <v-text-field v-model.number="newItem.minutes" type="number" label="Minutos" density="compact" />
        <v-text-field v-model="newItem.competency" label="Competência (opcional)" density="compact" />
        <v-btn color="primary" :disabled="!newItem.trackId || !newItem.title.trim()" :loading="addItem.isPending.value" @click="run(async () => { await addItem.mutateAsync({ trackId: newItem.trackId, title: newItem.title.trim(), kind: newItem.kind, minutes: newItem.minutes, competency: newItem.competency.trim() || undefined }); newItem.title = '' })">
          Adicionar
        </v-btn>
      </v-card>
      <v-card variant="outlined" class="pa-4" title="Integração de contratados">
        <v-select
          :model-value="defaultTrack.data.value?.track_id ?? undefined"
          :items="tracks.data.value ?? []"
          item-title="title"
          item-value="id"
          label="Trilha de integração do tenant"
          density="compact"
          @update:model-value="(id: string) => run(() => setDefault.mutateAsync(id))"
        />
        <p v-if="!pending.data.value?.length" class="text-caption">Nenhuma contratação aguardando vínculo.</p>
        <div v-for="p in pending.data.value" :key="p.application_id" class="d-flex align-center ga-2 mb-2">
          <span class="flex-grow-1">{{ p.position_title }} · {{ p.unit }} · início {{ p.start_date }}</span>
          <v-select v-model="bindTo[p.application_id]" :items="employees" item-title="name" item-value="id" label="Colaborador" density="compact" hide-details style="max-width: 240px" />
          <v-btn size="small" :disabled="!bindTo[p.application_id]" @click="run(() => bind.mutateAsync({ applicationId: p.application_id, employee_id: bindTo[p.application_id]! }))">Vincular</v-btn>
        </div>
      </v-card>
    </template>
  </div>
</template>
