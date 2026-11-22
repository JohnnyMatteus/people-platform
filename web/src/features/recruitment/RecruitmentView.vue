<script setup lang="ts">
import { mdiPlus } from '@mdi/js'
import { reactive, ref } from 'vue'
import { errorMessage } from '@/lib/http'
import { dateOnly } from '@/lib/format'
import { useCreateJob, useJobs } from './queries'

const jobs = useJobs()
const create = useCreateJob()
const dialog = ref(false)
const form = reactive({ title: '', unit: '', stages: '' })
const error = ref<string | null>(null)

const statusLabel: Record<string, string> = { open: 'Aberta', closed: 'Fechada' }

async function submit() {
  error.value = null
  const stages = form.stages
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
  try {
    await create.mutateAsync({ title: form.title.trim(), unit: form.unit.trim() || undefined, stages: stages.length ? stages : undefined })
    dialog.value = false
    Object.assign(form, { title: '', unit: '', stages: '' })
  } catch (e) {
    error.value = errorMessage(e)
  }
}
</script>

<template>
  <div class="pa-6">
    <div class="d-flex align-center mb-4">
      <h1 class="screen-title">Recrutamento</h1>
      <v-spacer />
      <v-btn color="primary" :prepend-icon="mdiPlus" @click="dialog = true">Nova vaga</v-btn>
    </div>

    <v-alert v-if="jobs.isError.value" type="error" variant="tonal">{{ errorMessage(jobs.error.value) }}</v-alert>
    <v-progress-linear v-else-if="jobs.isPending.value" indeterminate />
    <p v-else-if="!jobs.data.value?.items.length">Nenhuma vaga ainda. Abra a primeira para receber candidaturas.</p>
    <v-table v-else>
      <caption class="sr-only">Vagas</caption>
      <thead>
        <tr>
          <th scope="col">Vaga</th>
          <th scope="col">Unidade</th>
          <th scope="col">Etapas</th>
          <th scope="col">Situação</th>
          <th scope="col">Aberta em</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="j in jobs.data.value.items" :key="j.id">
          <td>
            <router-link :to="{ name: 'vaga', params: { jobId: j.id } }">{{ j.title }}</router-link>
          </td>
          <td>{{ j.unit || '—' }}</td>
          <td class="tnum">{{ j.stages.length }}</td>
          <td>{{ statusLabel[j.status] ?? j.status }}</td>
          <td class="tnum">{{ j.created_at ? dateOnly(j.created_at) : '—' }}</td>
        </tr>
      </tbody>
    </v-table>
    <p v-if="jobs.data.value?.next_cursor" class="text-caption mt-2">Há mais vagas do que as exibidas.</p>

    <v-dialog v-model="dialog" max-width="480">
      <v-card title="Nova vaga">
        <v-card-text>
          <v-alert v-if="error" type="error" variant="tonal" class="mb-3">{{ error }}</v-alert>
          <v-text-field v-model="form.title" label="Título" autofocus />
          <v-text-field v-model="form.unit" label="Unidade (opcional)" />
          <v-text-field
            v-model="form.stages"
            label="Etapas, separadas por vírgula (opcional)"
            hint="Minúsculas e hífen, ao menos duas. Sem preencher, vale o pipeline padrão do tenant."
            persistent-hint
          />
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="dialog = false">Cancelar</v-btn>
          <v-btn color="primary" :disabled="form.title.trim().length < 2" :loading="create.isPending.value" @click="submit">Abrir vaga</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>
