<script setup lang="ts">
import { mdiClose, mdiFileDocumentOutline } from '@mdi/js'
import { computed, reactive, ref } from 'vue'
import { errorMessage } from '@/lib/http'
import { dateTime } from '@/lib/format'
import type { Job, Recommendation } from '@/lib/schemas/recruitment'
import { fetchResumeUrl, useApplication, useMoveStage, useReject, useScheduleInterview, useScorecard } from './queries'

const props = defineProps<{ applicationId: string; job: Job }>()
defineEmits<{ close: [] }>()

const id = computed(() => props.applicationId)
const app = useApplication(id)
const move = useMoveStage()
const reject = useReject()
const schedule = useScheduleInterview()
const scorecard = useScorecard()
const error = ref<string | null>(null)
const startDate = ref('')
const interview = reactive({ kind: 'entrevista-rh', at: '' })
const card = reactive({ interviewId: '', recommendation: 'yes' as Recommendation, criterion: 'Aderência à vaga', score: 3, notes: '' })

const recommendations: { value: Recommendation; title: string }[] = [
  { value: 'strong_yes', title: 'Contratar com convicção' },
  { value: 'yes', title: 'Contratar' },
  { value: 'no', title: 'Não contratar' },
  { value: 'strong_no', title: 'Descartar' },
]
const lastStage = computed(() => props.job.stages[props.job.stages.length - 1])
const closed = computed(() => app.data.value?.status !== 'active')

async function run(fn: () => Promise<unknown>) {
  error.value = null
  try {
    await fn()
  } catch (e) {
    error.value = errorMessage(e)
  }
}
const goTo = (stage: string) =>
  run(() =>
    move.mutateAsync({
      applicationId: props.applicationId,
      jobId: props.job.id,
      stage,
      start_date: stage === lastStage.value ? startDate.value || undefined : undefined,
    }),
  )
const openResume = () => run(async () => window.open((await fetchResumeUrl(props.applicationId)).url, '_blank', 'noopener'))
const book = () =>
  run(async () => {
    await schedule.mutateAsync({
      applicationId: props.applicationId,
      kind: interview.kind,
      scheduled_at: new Date(interview.at).toISOString(),
    })
    interview.at = ''
  })
const sendCard = () =>
  run(async () => {
    await scorecard.mutateAsync({
      interviewId: card.interviewId,
      applicationId: props.applicationId,
      recommendation: card.recommendation,
      ratings: [{ criterion: card.criterion, score: Number(card.score) }],
      notes: card.notes || undefined,
    })
    card.notes = ''
    card.interviewId = ''
  })
</script>

<template>
  <div class="pa-4">
    <div class="d-flex align-center mb-2">
      <h2 class="section-title">{{ app.data.value?.candidate?.name ?? 'Candidatura' }}</h2>
      <v-spacer />
      <v-btn icon variant="text" aria-label="Fechar" @click="$emit('close')"><v-icon :icon="mdiClose" /></v-btn>
    </div>
    <v-alert v-if="error" type="error" variant="tonal" class="mb-3">{{ error }}</v-alert>
    <v-progress-linear v-if="app.isPending.value" indeterminate />
    <template v-else-if="app.data.value">
      <p class="text-medium-emphasis">{{ app.data.value.candidate?.email }}</p>
      <p class="mb-3">
        Etapa <strong>{{ app.data.value.stage }}</strong>
        <template v-if="app.data.value.start_date"> · início em {{ app.data.value.start_date }}</template>
      </p>
      <v-btn variant="tonal" size="small" :prepend-icon="mdiFileDocumentOutline" class="mb-4" @click="openResume">Abrir currículo</v-btn>

      <template v-if="!closed">
        <h3 class="axis-label mb-2">Mover de etapa</h3>
        <div class="d-flex flex-wrap ga-2 mb-2">
          <v-btn
            v-for="s in job.stages.filter((x) => x !== app?.data.value?.stage)"
            :key="s"
            size="small"
            variant="outlined"
            :loading="move.isPending.value"
            @click="goTo(s)"
          >
            {{ s }}
          </v-btn>
        </div>
        <v-text-field
          v-model="startDate"
          type="date"
          density="compact"
          :label="`Data de início (usada ao mover para ${lastStage})`"
          hide-details
          class="mb-3"
        />
        <v-btn color="error" variant="text" size="small" :loading="reject.isPending.value" @click="run(() => reject.mutateAsync({ applicationId, jobId: job.id }))">
          Recusar candidatura
        </v-btn>
        <v-divider class="my-4" />
      </template>

      <h3 class="axis-label mb-2">Entrevistas</h3>
      <p v-if="!app.data.value.interviews?.length" class="text-caption">Nenhuma entrevista agendada.</p>
      <v-list v-else density="compact">
        <v-list-item v-for="i in app.data.value.interviews" :key="i.id" :title="i.kind" :subtitle="dateTime(i.scheduled_at)">
          <template #append>
            <v-chip v-for="sc in i.scorecards ?? []" :key="sc.id" size="x-small" class="ml-1">
              {{ recommendations.find((r) => r.value === sc.recommendation)?.title }}
            </v-chip>
          </template>
        </v-list-item>
      </v-list>

      <template v-if="!closed">
        <v-text-field v-model="interview.kind" label="Tipo" density="compact" class="mt-3" />
        <v-text-field v-model="interview.at" type="datetime-local" label="Data e hora" density="compact" />
        <v-btn size="small" color="primary" :disabled="!interview.at || !interview.kind" :loading="schedule.isPending.value" @click="book">
          Agendar entrevista
        </v-btn>

        <template v-if="app.data.value.interviews?.length">
          <h3 class="axis-label mt-6 mb-2">Scorecard</h3>
          <v-select
            v-model="card.interviewId"
            :items="app.data.value.interviews.map((i) => ({ value: i.id, title: `${i.kind} · ${dateTime(i.scheduled_at)}` }))"
            label="Entrevista"
            density="compact"
          />
          <v-select v-model="card.recommendation" :items="recommendations" label="Recomendação" density="compact" />
          <v-text-field v-model="card.criterion" label="Critério" density="compact" />
          <v-slider v-model="card.score" :min="1" :max="5" :step="1" show-ticks="always" thumb-label label="Nota" />
          <v-textarea v-model="card.notes" label="Notas" rows="2" density="compact" class="voice" />
          <v-btn size="small" color="primary" :disabled="!card.interviewId" :loading="scorecard.isPending.value" @click="sendCard">Enviar scorecard</v-btn>
        </template>
      </template>
    </template>
  </div>
</template>
