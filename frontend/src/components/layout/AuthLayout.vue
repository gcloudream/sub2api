<template>
  <div class="lux-auth lux-ambient">
    <section class="lux-auth-brand" aria-hidden="true">
      <router-link to="/home" class="lux-brand">
        <img :src="siteLogo || '/logo.svg'" :alt="siteName" />
        <span><span class="lux-brand-name">{{ siteName }}</span><small>AI API GATEWAY</small></span>
      </router-link>
      <div class="lux-auth-art">
        <BrandPortal />
        <h1>{{ t('fantasy.authTitle') }}</h1>
        <p>{{ t('fantasy.authDescription') }}</p>
      </div>
      <p class="lux-auth-aside-footer">{{ siteSubtitle }}</p>
    </section>
    <main class="lux-auth-main">
      <div class="lux-auth-tools"><LocaleSwitcher /><ThemeToggle /></div>
      <div class="lux-auth-mobile-brand lux-brand">
        <img :src="siteLogo || '/logo.svg'" :alt="siteName" />
        <span><span class="lux-brand-name">{{ siteName }}</span><small>AI API GATEWAY</small></span>
      </div>
      <div class="lux-auth-form"><slot /></div>
      <div class="lux-auth-footer"><slot name="footer" /><p class="mt-3 text-xs">&copy; {{ currentYear }} {{ siteName }} · AI API Gateway</p></div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'
import BrandPortal from '@/components/common/BrandPortal.vue'
import ThemeToggle from '@/components/common/ThemeToggle.vue'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import { useI18n } from 'vue-i18n'

const appStore = useAppStore()
const { t } = useI18n()

const siteName = computed(() => appStore.siteName || '幻想家')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway')

const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>
