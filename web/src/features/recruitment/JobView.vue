<script setup lang="ts">
import { mdiAccountPlusOutline, mdiArrowLeft } from '@mdi/js'
import { computed, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { errorMessage } from '@/lib/http'
import ApplicationPanel from './ApplicationPanel.vue'
import { uploadResume, useApplications, useApply, useJob } from './queries'

const route = useRoute()
const jobId = computed(() => String(route.params.jobId))
const job = useJob(jobId)
const applications = useApplications(jobId)
const apply = useApply()
const selected = ref<string | undefined>()
const dialog = ref(false)
const form = reactive({ name: '', email: '' })
const file = ref<File | null>(null)
const error = ref<string | null>(null)
const notice = ref<string | null>(null)

const columns = computed(() => {
  const items = applications.data.value?.items ?? []
  return (job.data.value?.stages ?? []).map((stage) => ({ stage, items: items.filter((a) => a.stage === stage && a.status === 'active') }))
})
const closed = computed(() => (applications.data.value?.items ?? []).filter((a) => a.status !== 'active'))
const statusLabel = { hired: 'Contratado', rejected: 'Recusado', active: 'Em andamento' } as const

async function submit() {
  error.value = null
  notice.value = null
  try {
    const created = await apply.mutateAsync({
      jobId: jobId.value,
      name: form.name.trim(),
      email: form.email.trim(),
      resume_content_type: file.value?.type || undefined,
    })
    if (file.value) {
      try {
        await uploadResume(created.resume_upload, file.value)
      } catch (e) {
        notice.value = `Candidatura registrada, mas o currículo não subiu: ${errorMessage(e)}`
      }
    }
    dialog.value = false
    Object.assign(form, { name: '', email: '' })
    file.value = null
  } catch (e) {
    error.value = errorMessage(e)
  }
}
</script>

<template>
  <div class="pa-6">
    <v-btn variant="text" :prepend-icon="mdiArrowLeft" :to="{ name: 'recrutamento' }" class="mb-2">Vagas</v-btn>
    <v-alert v-if="job.isError.value" type="error" variant="tonal">{{ errorMessage(job.error.value) }}</v-alert>
    <v-progress-linear v-else-if="job.isPending.value" indeterminate />
    <template v-else-if="job.data.value">
      <div class="d-flex align-center mb-1">
        <h1 class="screen-title">{{ job.data.value.title }}</h1>
        <v-spacer />
        <v-btn color="primary" :prepend-icon="mdiAccountPlusOutline" :disabled="job.data.value.status !== 'open'" @click="dialog = true">
          Registrar candidatura
        </v-btn>
      </div>
      <p class="text-medium-emphasis mb-4">{{ job.data.value.unit || 'Sem unidade' }} · {{ job.data.value.status === 'open' ? 'Aberta' : 'Fechada' }}</p>
      <v-alert v-if="notice" type="warning" variant="tonal" class="mb-4" closable @click:close="notice = null">{{ notice }}</v-alert>
      <v-alert v-if="applications.isError.value" type="error" variant="tonal">{{ errorMessage(applications.error.value) }}</v-alert>
      <v-progress-linear v-else-if="applications.isPending.value" indeterminate />
      <div v-else class="pipeline" role="list" aria-label="Pipeline da vaga">
        <section v-for="col in columns" :key="col.stage" class="pipeline__col" role="listitem" :aria-label="`Etapa ${col.stage}`">
          <h2 class="axis-label mb-2">{{ col.stage }} <span class="tnum">({{ col.items.length }})</span></h2>
          <v-card
            v-for="a in col.items"
            :key="a.id"
            variant="outlined"
            class="mb-2 pa-3"
            :color="selected === a.id ? 'primary' : undefined"
            @click="selected = a.id"
          >
            <button type="button" class="pipeline__card" @click.stop="selected = a.id">
              <span class="font-weight-medium">{{ a.candidate?.name }}</span>
              <span class="text-caption text-medium-emphasis">{{ a.candidate?.email }}</span>
            </button>
          </v-card>
          <p v-if="!col.items.length" class="text-caption text-medium-emphasis">Ninguém nesta etapa.</p>
        </section>
      </div>
      <template v-if="closed.length">
        <h2 class="section-title mt-6 mb-2">Encerradas</h2>
        <v-list density="compact">
          <v-list-item v-for="a in closed" :key="a.id" :title="a.candidate?.name" :subtitle="`${statusLabel[a.status]} · ${a.stage}`" @click="selected = a.id" />
        </v-list>
      </template>
    </template>

    <v-navigation-drawer :model-value="!!selected" location="right" temporary width="440" @update:model-value="(v: boolean) => !v && (selected = undefined)">
      <ApplicationPanel v-if="selected && job.data.value" :application-id="selected" :job="job.data.value" @close="selected = undefined" />
    </v-navigation-drawer>

    <v-dialog v-model="dialog" max-width="480">
      <v-card title="Registrar candidatura">
        <v-card-text>
          <v-alert v-if="error" type="error" variant="tonal" class="mb-3">{{ error }}</v-alert>
          <v-text-field v-model="form.name" label="Nome" autofocus />
          <v-text-field v-model="form.email" label="E-mail" type="email" />
          <v-file-input v-model="file" label="Currículo (opcional)" accept="application/pdf" />
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="dialog = false">Cancelar</v-btn>
          <v-btn color="primary" :disabled="!form.name.trim() || !form.email.includes('@')" :loading="apply.isPending.value" @click="submit">
            Registrar
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<style scoped>
.pipeline {
  display: grid;
  grid-auto-flow: column;
  grid-auto-columns: minmax(220px, 1fr);
  gap: 16px;
  overflow-x: auto;
  padding-bottom: 8px;
}
.pipeline__card {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  width: 100%;
  text-align: left;
}
</style>
