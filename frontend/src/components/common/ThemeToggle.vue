<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
const { t } = useI18n()
const dark = ref(document.documentElement.classList.contains('dark'))
function toggle() {
  dark.value = !document.documentElement.classList.contains('dark')
  document.documentElement.classList.toggle('dark', dark.value)
  localStorage.setItem('theme', dark.value ? 'dark' : 'light')
}
</script>
<template>
  <button type="button" class="btn-ghost btn-icon" :aria-label="t(dark ? 'home.switchToLight' : 'home.switchToDark')" @click="toggle">
    <Icon :name="dark ? 'sun' : 'moon'" size="md" />
  </button>
</template>
