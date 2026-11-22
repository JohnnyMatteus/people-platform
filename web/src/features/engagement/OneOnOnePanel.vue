<script setup lang="ts">
import { ref } from 'vue'
import { useMe } from '@/features/performance/queries'
import { useNames } from '@/features/people/useNames'
import { dateTime } from '@/lib/format'
import { errorMessage } from '@/lib/http'
import { useAddAgendaItem, useCompleteAgendaItem, useMeetingContext, useMeetings, useSchedule } from './queries'

const me = useMe()
const { employees, name } = useNames()
const meetings = useMeetings()
const schedule = useSchedule()
const addItem = useAddAgendaItem()
const complete = useCompleteAgendaItem()
const participant = ref<string | undefined>()
const at = ref('')
const selected = ref<string | undefined>()
const context = useMeetingContext(selected)
const text = ref('')
const error = ref<string | null>(null)

async function run(fn: () => Promise<unknown>) {
  error.value = null
  try {
    await fn()
  } catch (e) {
    error.value = errorMessage(e)
  }
}
const book = () =>
  run(async () => {
    await schedule.mutateAsync({ participant_id: participant.value!, scheduled_for: new Date(at.value).toISOString() })
    at.value = ''
  })
const add = () =>
  run(async () => {
    await addItem.mutateAsync({ meetingId: selected.value!, text: text.value.trim() })
    text.value = ''
  })
const other = (m: { leader_id: string; participant_id: string }) => (m.leader_id === me.data.value?.employee_id ? m.participant_id : m.leader_id)
</script>

<template>
  <div class="reading">
    <v-alert v-if="error" type="error" variant="tonal" class="mb-3">{{ error }}</v-alert>
    <v-card variant="outlined" class="pa-4 mb-6" title="Agendar 1:1">
      <v-select v-model="participant" :items="employees.filter((e) => e.id !== me.data.value?.employee_id)" item-title="name" item-value="id" label="Com" density="compact" />
      <v-text-field v-model="at" type="datetime-local" label="Data e hora" density="compact" />
      <v-btn color="primary" :disabled="!participant || !at" :loading="schedule.isPending.value" @click="book">Agendar</v-btn>
    </v-card>
    <v-progress-linear v-if="meetings.isPending.value" indeterminate />
    <p v-else-if="!meetings.data.value?.length">Nenhuma 1:1 agendada.</p>
    <v-list v-else>
      <v-list-item
        v-for="m in meetings.data.value"
        :key="m.id"
        :title="`1:1 com ${name(other(m))}`"
        :subtitle="`${dateTime(m.scheduled_for)} · ${m.agenda.filter((i) => !i.done).length} item(ns) em aberto`"
        :active="selected === m.id"
        @click="selected = m.id"
      />
    </v-list>

    <template v-if="selected && context.data.value">
      <h2 class="section-title mt-6 mb-2">Pauta</h2>
      <p v-if="!context.data.value.meeting.agenda.length" class="text-caption">Sem itens na pauta.</p>
      <v-list density="compact">
        <v-list-item v-for="i in context.data.value.meeting.agenda" :key="i.id">
          <template #prepend>
            <v-checkbox-btn :model-value="i.done" :disabled="i.done" :aria-label="`Concluir: ${i.text}`" @update:model-value="run(() => complete.mutateAsync({ meetingId: selected!, itemId: i.id }))" />
          </template>
          <v-list-item-title class="voice" :class="{ 'text-decoration-line-through': i.done }">{{ i.text }}</v-list-item-title>
        </v-list-item>
      </v-list>
      <div class="d-flex ga-2 align-center">
        <v-text-field v-model="text" label="Novo item" density="compact" hide-details />
        <v-btn :disabled="!text.trim()" :loading="addItem.isPending.value" @click="add">Adicionar</v-btn>
      </div>
      <h3 class="axis-label mt-6 mb-2">Contexto entre vocês</h3>
      <p v-for="f in context.data.value.recent_feedbacks" :key="f.id" class="voice mb-2">{{ name(f.from_employee_id) }}: {{ f.body }}</p>
      <p v-for="p in context.data.value.recent_praises" :key="p.id" class="voice mb-2">Elogio de {{ name(p.from_employee_id) }}: {{ p.message }}</p>
      <p v-if="!context.data.value.recent_feedbacks.length && !context.data.value.recent_praises.length" class="text-caption">Nada recente.</p>
    </template>
  </div>
</template>
