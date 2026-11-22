<script setup lang="ts">
import { useNames } from '@/features/people/useNames'
import { dateTime } from '@/lib/format'
import { errorMessage } from '@/lib/http'
import { useLeaderboard, usePoints } from './queries'

const points = usePoints()
const board = useLeaderboard()
const { name } = useNames()
</script>

<template>
  <div class="reading">
    <v-alert v-if="points.isError.value" type="error" variant="tonal">{{ errorMessage(points.error.value) }}</v-alert>
    <v-progress-linear v-else-if="points.isPending.value" indeterminate />
    <template v-else-if="points.data.value">
      <p class="axis-label">Seu saldo</p>
      <p class="screen-title tnum mb-4">{{ points.data.value.balance }}</p>
      <v-chip v-for="b in points.data.value.badges" :key="b.code" class="mr-2 mb-4">{{ b.code }}</v-chip>
      <h2 class="section-title mb-2">Extrato</h2>
      <p v-if="!points.data.value.ledger.length">Sem movimentações.</p>
      <v-list density="compact">
        <v-list-item v-for="(l, i) in points.data.value.ledger" :key="i" :title="l.reason" :subtitle="dateTime(l.created_at)">
          <template #append><span class="tnum">+{{ l.points }}</span></template>
        </v-list-item>
      </v-list>
    </template>
    <h2 class="section-title mt-6 mb-2">Ranking</h2>
    <v-progress-linear v-if="board.isPending.value" indeterminate />
    <ol v-else class="pl-6">
      <li v-for="e in board.data.value" :key="e.employee_id">{{ name(e.employee_id) }} <span class="tnum">({{ e.points }})</span></li>
    </ol>
  </div>
</template>
