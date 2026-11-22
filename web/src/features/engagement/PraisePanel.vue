<script setup lang="ts">
import { ref } from 'vue'
import { useNames } from '@/features/people/useNames'
import { dateTime } from '@/lib/format'
import { errorMessage } from '@/lib/http'
import { usePraise, usePraises } from './queries'

const praises = usePraises()
const give = usePraise()
const { employees, name } = useNames()
const to = ref<string[]>([])
const message = ref('')
const values = ref('')
const error = ref<string | null>(null)

async function send() {
  error.value = null
  try {
    await give.mutateAsync({
      to_employee_ids: to.value,
      message: message.value.trim(),
      values: values.value.split(',').map((v) => v.trim()).filter(Boolean).slice(0, 5),
    })
    to.value = []
    message.value = ''
    values.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
}
</script>

<template>
  <div class="reading">
    <v-card variant="outlined" class="pa-4 mb-6" title="Reconhecer alguém">
      <v-alert v-if="error" type="error" variant="tonal" class="mb-3">{{ error }}</v-alert>
      <v-select v-model="to" :items="employees" item-title="name" item-value="id" label="Para" multiple chips density="compact" />
      <v-textarea v-model="message" label="Mensagem" rows="2" class="voice" />
      <v-text-field v-model="values" label="Valores (opcional, separados por vírgula)" density="compact" />
      <v-btn color="primary" :disabled="!to.length || !message.trim()" :loading="give.isPending.value" @click="send">Publicar elogio</v-btn>
    </v-card>
    <v-alert v-if="praises.isError.value" type="error" variant="tonal">{{ errorMessage(praises.error.value) }}</v-alert>
    <v-progress-linear v-else-if="praises.isPending.value" indeterminate />
    <p v-else-if="!praises.data.value?.length">Nenhum elogio ainda.</p>
    <article v-for="p in praises.data.value" :key="p.id" class="mb-4">
      <p class="text-caption text-medium-emphasis">
        {{ name(p.from_employee_id) }} para {{ p.to_employee_ids.map(name).join(', ') }} · {{ dateTime(p.created_at) }}
      </p>
      <p class="voice">{{ p.message }}</p>
      <v-chip v-for="v in p.values" :key="v" size="x-small" class="mr-1">{{ v }}</v-chip>
    </article>
  </div>
</template>
