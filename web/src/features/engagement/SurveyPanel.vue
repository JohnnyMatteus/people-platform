<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useMe } from '@/features/performance/queries'
import { errorMessage } from '@/lib/http'
import type { Survey } from '@/lib/schemas/engagement'
import { useAnswerSurvey, useCloseSurvey, useCreateSurvey, useSurveyResults, useSurveys } from './queries'

const me = useMe()
const surveys = useSurveys()
const answer = useAnswerSurvey()
const close = useCloseSurvey()
const create = useCreateSurvey()
const isAdmin = computed(() => me.data.value?.roles.includes('tenant-admin') ?? false)
const canSeeResults = computed(() => me.data.value?.roles.some((r) => r === 'tenant-admin' || r === 'manager') ?? false)
const selectedId = ref<string | undefined>()
const selected = computed<Survey | undefined>(() => surveys.data.value?.find((s) => s.id === selectedId.value))
const results = useSurveyResults(selectedId, canSeeResults)
const scores = reactive<Record<string, number>>({})
const comment = ref('')
const error = ref<string | null>(null)
const draft = reactive({ title: '', questions: '', segments: '' })

async function run(fn: () => Promise<unknown>) {
  error.value = null
  try {
    await fn()
  } catch (e) {
    error.value = errorMessage(e)
  }
}
const send = () =>
  run(async () => {
    await answer.mutateAsync({ surveyId: selectedId.value!, scores: { ...scores }, comment: comment.value.trim() || undefined })
    comment.value = ''
  })
const createSurvey = () =>
  run(async () => {
    await create.mutateAsync({
      title: draft.title.trim(),
      questions: draft.questions.split('\n').map((t) => t.trim()).filter(Boolean).map((text, i) => ({ id: `q${i + 1}`, text })),
      segments: draft.segments.split(',').map((s) => s.trim()).filter(Boolean),
    })
    Object.assign(draft, { title: '', questions: '', segments: '' })
  })
</script>

<template>
  <div class="reading">
    <v-alert type="info" variant="tonal" class="mb-4">
      A resposta é anônima por desenho: o sistema guarda que você participou, mas não o que respondeu, e os resultados só aparecem
      para grupos com pessoas suficientes.
    </v-alert>
    <v-alert v-if="error" type="error" variant="tonal" class="mb-3">{{ error }}</v-alert>
    <v-progress-linear v-if="surveys.isPending.value" indeterminate />
    <p v-else-if="!surveys.data.value?.length">Nenhuma pesquisa de clima.</p>
    <v-list v-else>
      <v-list-item
        v-for="s in surveys.data.value"
        :key="s.id"
        :title="s.title"
        :subtitle="`${s.status === 'open' ? 'Aberta' : 'Encerrada'}${s.already_answered ? ' · você já participou' : ''}`"
        :active="selectedId === s.id"
        @click="selectedId = s.id"
      />
    </v-list>

    <template v-if="selected">
      <h2 class="section-title mt-6 mb-2">{{ selected.title }}</h2>
      <template v-if="selected.status === 'open' && !selected.already_answered">
        <div v-for="q in selected.questions" :key="q.id" class="mb-4">
          <p :id="`q-${q.id}`">{{ q.text }}</p>
          <v-btn-toggle v-model="scores[q.id]" color="primary" variant="outlined" divided density="comfortable" :aria-labelledby="`q-${q.id}`">
            <v-btn v-for="n in 5" :key="n" :value="n" :aria-label="`Nota ${n}`">{{ n }}</v-btn>
          </v-btn-toggle>
          <p class="text-caption text-medium-emphasis mt-1">1 = discordo totalmente, 5 = concordo totalmente</p>
        </div>
        <v-textarea v-model="comment" label="Comentário (opcional, anônimo)" rows="2" class="voice" />
        <v-btn color="primary" :loading="answer.isPending.value" :disabled="selected.questions.some((q) => scores[q.id] === undefined)" @click="send">
          Enviar resposta anônima
        </v-btn>
      </template>
      <p v-else-if="selected.already_answered">Obrigado, a sua participação já foi registrada.</p>

      <v-btn v-if="isAdmin && selected.status === 'open'" class="mt-4" variant="tonal" :loading="close.isPending.value" @click="run(() => close.mutateAsync(selected!.id))">
        Encerrar pesquisa
      </v-btn>

      <template v-if="canSeeResults">
        <h3 class="axis-label mt-6 mb-2">Resultados</h3>
        <v-progress-linear v-if="results.isPending.value" indeterminate />
        <v-alert v-else-if="results.isError.value" type="warning" variant="tonal">{{ errorMessage(results.error.value) }}</v-alert>
        <template v-else-if="results.data.value">
          <p class="mb-2 tnum">{{ results.data.value.participation }} participação(ões) · mínimo de {{ results.data.value.min_group_size }} pessoas por grupo</p>
          <p v-if="!results.data.value.segments.length" class="text-caption">Ainda não há grupo com pessoas suficientes para publicar resultado.</p>
          <v-card v-for="seg in results.data.value.segments" :key="seg.segment_key" variant="outlined" class="pa-3 mb-3">
            <p class="font-weight-medium">{{ seg.segment_key === '_all' ? 'Toda a empresa' : seg.segment_key }} <span class="tnum text-caption">({{ seg.respondents }} respondentes)</span></p>
            <p v-for="q in selected.questions" :key="q.id" class="text-body-2 tnum">{{ q.text }}: {{ seg.scores[q.id]?.toFixed(2) ?? '—' }}</p>
            <p v-for="(c, i) in seg.comments" :key="i" class="voice">“{{ c }}”</p>
          </v-card>
          <p v-if="results.data.value.suppressed > 0" class="text-caption">{{ results.data.value.suppressed }} grupo(s) omitido(s) por terem poucas respostas.</p>
        </template>
      </template>
    </template>

    <v-card v-if="isAdmin" variant="outlined" class="pa-4 mt-8" title="Nova pesquisa">
      <v-text-field v-model="draft.title" label="Título" density="compact" />
      <v-textarea v-model="draft.questions" label="Perguntas, uma por linha (nota de 1 a 5)" rows="3" />
      <v-text-field v-model="draft.segments" label="Segmentos, separados por vírgula (ex.: obras, obras/sp)" density="compact" hint="Definidos antes da coleta; não há filtro depois." persistent-hint />
      <v-btn class="mt-3" color="primary" :disabled="!draft.title.trim() || !draft.questions.trim() || !draft.segments.trim()" :loading="create.isPending.value" @click="createSurvey">Abrir pesquisa</v-btn>
    </v-card>
  </div>
</template>
