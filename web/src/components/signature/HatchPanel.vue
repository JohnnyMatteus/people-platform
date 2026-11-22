<script setup lang="ts">
import { mdiShieldLockOutline } from '@mdi/js'

/**
 * Assinatura do produto: o que a plataforma protege é desenhado com hachura de 45 graus e o
 * rótulo "protegido". `fill` cobre o bloco inteiro (dado indisponível para quem vê); `corner`
 * marca só o canto de um campo que existe mas é restrito (visível só para gestão e RH).
 */
withDefaults(defineProps<{ label?: string; variant?: 'fill' | 'corner' }>(), {
  label: 'protegido',
  variant: 'corner',
})
</script>

<template>
  <div class="hatch" :class="`hatch--${variant}`" role="group" :aria-label="`Conteúdo ${label}`">
    <span class="hatch__tag text-caption">
      <v-icon :icon="mdiShieldLockOutline" size="14" aria-hidden="true" />
      {{ label }}
    </span>
    <div class="hatch__body"><slot /></div>
  </div>
</template>

<style scoped>
.hatch {
  position: relative;
  border: 1px solid rgba(var(--v-theme-hatch), 0.7);
  border-radius: 8px;
  padding: 12px 16px;
  background: rgb(var(--v-theme-surface));
}
.hatch--fill {
  background-image: repeating-linear-gradient(45deg, rgba(var(--v-theme-hatch), 0.55) 0 1px, transparent 1px 6px);
}
.hatch--corner::before {
  content: '';
  position: absolute;
  top: 0;
  right: 0;
  width: 56px;
  height: 56px;
  border-top-right-radius: 8px;
  clip-path: polygon(0 0, 100% 0, 100% 100%);
  background-image: repeating-linear-gradient(45deg, rgba(var(--v-theme-hatch), 0.7) 0 1px, transparent 1px 6px);
}
.hatch__tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-bottom: 4px;
  padding: 0 6px;
  border-radius: 4px;
  background: rgb(var(--v-theme-surface));
  color: rgb(var(--v-theme-on-surface));
  position: relative;
}
.hatch__body {
  position: relative;
}
</style>
