<script setup lang="ts">
import { ref } from 'vue'
import FeedbackPanel from './FeedbackPanel.vue'
import OneOnOnePanel from './OneOnOnePanel.vue'
import PointsPanel from './PointsPanel.vue'
import PostsPanel from './PostsPanel.vue'
import PraisePanel from './PraisePanel.vue'
import SurveyPanel from './SurveyPanel.vue'

const tab = ref('mural')
</script>

<template>
  <div class="pa-6">
    <h1 class="screen-title mb-2">Engajamento</h1>
    <v-tabs v-model="tab" color="primary" class="mb-4">
      <v-tab value="mural">Mural</v-tab>
      <v-tab value="elogios">Elogios</v-tab>
      <v-tab value="feedback">Feedback</v-tab>
      <v-tab value="um-a-um">1:1</v-tab>
      <v-tab value="clima">Clima</v-tab>
      <v-tab value="pontos">Pontos</v-tab>
    </v-tabs>
    <v-tabs-window v-model="tab">
      <v-tabs-window-item value="mural"><PostsPanel /></v-tabs-window-item>
      <v-tabs-window-item value="elogios"><PraisePanel /></v-tabs-window-item>
      <v-tabs-window-item value="feedback"><FeedbackPanel /></v-tabs-window-item>
      <v-tabs-window-item value="um-a-um"><OneOnOnePanel /></v-tabs-window-item>
      <v-tabs-window-item value="clima"><SurveyPanel /></v-tabs-window-item>
      <v-tabs-window-item value="pontos"><PointsPanel /></v-tabs-window-item>
    </v-tabs-window>
  </div>
</template>
