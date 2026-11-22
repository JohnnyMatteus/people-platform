<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMe } from '@/features/performance/queries'
import { useNames } from '@/features/people/useNames'
import { dateTime } from '@/lib/format'
import { errorMessage } from '@/lib/http'
import { useConfirmRead, usePosts, usePublishPost } from './queries'

const posts = usePosts()
const publish = usePublishPost()
const confirm = useConfirmRead()
const me = useMe()
const { name } = useNames()
const title = ref('')
const body = ref('')
const error = ref<string | null>(null)
const canPublish = computed(() => me.data.value?.roles.some((r) => r === 'manager' || r === 'tenant-admin') ?? false)

async function send() {
  error.value = null
  try {
    await publish.mutateAsync({ title: title.value.trim(), body: body.value.trim() })
    title.value = ''
    body.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
}
</script>

<template>
  <div class="reading">
    <v-card v-if="canPublish" variant="outlined" class="pa-4 mb-6" title="Novo comunicado">
      <v-alert v-if="error" type="error" variant="tonal" class="mb-3">{{ error }}</v-alert>
      <v-text-field v-model="title" label="Título" density="compact" />
      <v-textarea v-model="body" label="Texto" rows="3" class="voice" />
      <v-btn color="primary" :disabled="!title.trim() || !body.trim()" :loading="publish.isPending.value" @click="send">Publicar</v-btn>
    </v-card>
    <v-alert v-if="posts.isError.value" type="error" variant="tonal">{{ errorMessage(posts.error.value) }}</v-alert>
    <v-progress-linear v-else-if="posts.isPending.value" indeterminate />
    <p v-else-if="!posts.data.value?.length">Nenhum comunicado.</p>
    <article v-for="p in posts.data.value" :key="p.id" class="mb-6">
      <h2 class="section-title">{{ p.title }}</h2>
      <p class="text-caption text-medium-emphasis mb-2">{{ name(p.author_id) }} · {{ dateTime(p.created_at) }}</p>
      <p class="voice" style="white-space: pre-wrap">{{ p.body }}</p>
      <div class="d-flex align-center mt-2">
        <v-btn v-if="!p.read_by_me" size="small" variant="tonal" :loading="confirm.isPending.value" @click="confirm.mutate(p.id)">Confirmar leitura</v-btn>
        <span v-else class="text-caption">Você confirmou a leitura.</span>
        <span class="text-caption ml-4 tnum">{{ p.read_count }} leitura(s)</span>
      </div>
    </article>
  </div>
</template>
