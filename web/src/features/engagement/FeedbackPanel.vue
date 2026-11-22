<script setup lang="ts">
import { ref } from 'vue'
import { useMe } from '@/features/performance/queries'
import { useNames } from '@/features/people/useNames'
import { dateTime } from '@/lib/format'
import { errorMessage } from '@/lib/http'
import { useFeedbacks, useGiveFeedback } from './queries'

const me = useMe()
const { employees, name } = useNames()
const withId = ref<string | undefined>()
const feedbacks = useFeedbacks(withId)
const give = useGiveFeedback()
const to = ref<string | undefined>()
const body = ref('')
const error = ref<string | null>(null)

async function send() {
  error.value = null
  try {
    await give.mutateAsync({ to_employee_id: to.value!, body: body.value.trim() })
    body.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
}
</script>

<template>
  <div class="reading">
    <p class="mb-4">Feedback é privado: só você e a outra pessoa enxergam.</p>
    <v-card variant="outlined" class="pa-4 mb-6" title="Dar feedback">
      <v-alert v-if="error" type="error" variant="tonal" class="mb-3">{{ error }}</v-alert>
      <v-select v-model="to" :items="employees.filter((e) => e.id !== me.data.value?.employee_id)" item-title="name" item-value="id" label="Para" density="compact" />
      <v-textarea v-model="body" label="Feedback" rows="3" class="voice" />
      <v-btn color="primary" :disabled="!to || !body.trim()" :loading="give.isPending.value" @click="send">Enviar</v-btn>
    </v-card>
    <v-select v-model="withId" :items="employees" item-title="name" item-value="id" label="Filtrar conversa com" clearable density="compact" />
    <v-alert v-if="feedbacks.isError.value" type="error" variant="tonal">{{ errorMessage(feedbacks.error.value) }}</v-alert>
    <v-progress-linear v-else-if="feedbacks.isPending.value" indeterminate />
    <p v-else-if="!feedbacks.data.value?.length">Nenhum feedback.</p>
    <article v-for="f in feedbacks.data.value" :key="f.id" class="mb-4">
      <p class="text-caption text-medium-emphasis">{{ name(f.from_employee_id) }} para {{ name(f.to_employee_id) }} · {{ dateTime(f.created_at) }}</p>
      <p class="voice">{{ f.body }}</p>
    </article>
  </div>
</template>
