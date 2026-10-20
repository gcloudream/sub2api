<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="hasHomeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Compact Home Page -->
  <div
    v-else-if="compactHomeEnabled"
    data-testid="compact-home"
    class="flex min-h-screen flex-col bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white"
  >
    <header class="border-b border-gray-200 px-4 py-4 sm:px-6 dark:border-dark-800">
      <nav class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 sm:gap-4">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          <img
            :src="siteLogo || '/logo.svg'"
            alt="Logo"
            class="h-9 w-9 shrink-0 rounded-lg object-contain"
          />
          <span class="min-w-0 truncate text-base font-semibold">{{ siteName }}</span>
        </div>
        <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
          <LocaleSwitcher />
          <router-link
            to="/docs"
            class="inline-flex min-h-10 shrink-0 items-center gap-1.5 rounded-lg px-2.5 text-sm font-medium text-gray-700 hover:bg-gray-100 dark:text-dark-200 dark:hover:bg-dark-800"
          >
            <Icon name="book" size="md" />
            <span>{{ t('nav.usageDocs') }}</span>
          </router-link>
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex min-h-10 shrink-0 items-center gap-1.5 rounded-lg px-2.5 text-sm text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
            <span>{{ t('home.docs') }}</span>
          </a>
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="flex h-10 shrink-0 items-center gap-1.5 rounded-lg px-2.5 text-sm font-medium text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="md" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>
          <button
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex min-h-10 shrink-0 items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="flex min-w-0 flex-1 items-center justify-center px-4 py-16 sm:px-6">
      <div class="min-w-0 max-w-2xl text-center">
        <img
          :src="siteLogo || '/logo.svg'"
          alt="Logo"
          class="mx-auto mb-6 h-20 w-20 rounded-2xl object-contain"
        />
        <h1 class="[overflow-wrap:anywhere] text-3xl font-bold md:text-4xl">{{ siteName }}</h1>
        <p class="mt-4 whitespace-pre-wrap [overflow-wrap:anywhere] text-base text-gray-600 dark:text-dark-300">{{ siteSubtitle }}</p>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="mt-8 inline-flex min-h-10 items-center justify-center rounded-lg bg-primary-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-primary-700"
        >
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}
        </router-link>
      </div>
    </main>

    <footer class="min-w-0 border-t border-gray-200 px-4 py-5 text-center text-sm text-gray-500 [overflow-wrap:anywhere] sm:px-6 dark:border-dark-800 dark:text-dark-400">
      <p>&copy; {{ currentYear }} {{ siteName }}</p>
      <router-link to="/docs" class="mt-2 inline-flex min-h-10 items-center hover:text-gray-900 dark:hover:text-white">{{ t('nav.usageDocs') }}</router-link>
    </footer>
  </div>

  <!-- Fantasy built-in home; custom and compact modes above remain supported. -->
  <div v-else class="lux-home lux-ambient">
    <div class="lux-wrap">
      <header class="lux-nav flex-wrap py-4 sm:flex-nowrap sm:py-0">
        <router-link to="/home" class="lux-brand">
          <img :src="siteLogo || '/logo.svg'" :alt="siteName" />
          <span><span class="lux-brand-name">{{ siteName }}</span><small>AI API GATEWAY</small></span>
        </router-link>
        <nav class="lux-nav-actions" :aria-label="t('home.dashboard')">
          <router-link to="/docs" class="btn btn-ghost shrink-0 whitespace-nowrap">{{ t('nav.usageDocs') }}</router-link>
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="lux-nav-extra btn btn-ghost">{{ t('home.docs') }}</a>
          <router-link v-if="showModelPlazaEntry" to="/model-plaza" class="lux-nav-extra btn btn-ghost">{{ t('nav.modelPlaza') }}</router-link>
          <LocaleSwitcher />
          <button type="button" class="btn-ghost btn-icon" :aria-label="t(isDark ? 'home.switchToLight' : 'home.switchToDark')" @click="toggleTheme"><Icon :name="isDark ? 'sun' : 'moon'" size="md" /></button>
          <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="lux-nav-extra btn btn-secondary">{{ t(isAuthenticated ? 'home.dashboard' : 'home.login') }}</router-link>
        </nav>
      </header>
      <main>
        <section class="lux-hero">
          <div class="lux-hero-copy">
            <p class="lux-eyebrow">{{ t('fantasy.eyebrow') }}</p>
            <h1>{{ siteName }}</h1>
            <p class="lux-display">AI API Gateway</p>
            <h2 class="lux-hero-intro">{{ t('fantasy.intro') }}</h2>
            <p class="lux-hero-desc">{{ t('fantasy.description') }}</p>
            <div class="lux-hero-actions">
              <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="btn btn-primary">{{ t('home.goToDashboard') }}<Icon name="arrowRight" size="sm" /></router-link>
              <router-link v-if="!isAuthenticated && registrationEnabled" to="/register" class="btn btn-secondary">{{ t('fantasy.register') }}</router-link>
            </div>
            <div class="lux-tags"><span>{{ t('home.tags.subscriptionToApi') }}</span><span>{{ t('home.tags.stickySession') }}</span><span>{{ t('home.tags.realtimeBilling') }}</span></div>
          </div>
          <BrandPortal />
        </section>
        <section class="lux-services">
          <div class="lux-section-heading"><h2>{{ t('fantasy.services') }}</h2><span class="lux-eyebrow">THE GATEWAY / 01—03</span></div>
          <div class="lux-service-grid">
            <article v-for="(feature,index) in features" :key="feature.key" class="lux-service">
              <div class="lux-service-head"><span>0{{ index + 1 }}</span><Icon :name="feature.icon" size="md" /></div>
              <h3>{{ t('home.features.' + feature.key) }}</h3><p>{{ t('home.features.' + feature.key + 'Desc') }}</p>
            </article>
          </div>
        </section>
        <section class="lux-models"><div><p class="lux-eyebrow mb-3">CONNECTED MODELS</p><h2 class="text-sm">{{ t('home.providers.title') }}</h2></div><div class="lux-model-list"><span>Claude</span><span>GPT</span></div></section>
      </main>
      <footer class="lux-footer"><p>© {{ currentYear }} {{ siteName }} · AI API Gateway</p><div class="flex flex-wrap items-center gap-5"><router-link to="/docs">{{ t('nav.usageDocs') }}</router-link><a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">{{ t('home.docs') }}</a><a :href="githubUrl" target="_blank" rel="noopener noreferrer">Powered by Sub2API · LGPL-3.0</a></div></footer>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import BrandPortal from '@/components/common/BrandPortal.vue'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings - directly from appStore (already initialized from injected config)
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway Platform')
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))

// Check if homeContent is a URL (for iframe display)
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

// Theme
const isDark = ref(document.documentElement.classList.contains('dark'))

// GitHub URL
const githubUrl = 'https://github.com/Wei-Shaw/sub2api'

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const modelPlazaRequiresAuth = computed(
  () => appStore.cachedPublicSettings?.model_plaza_require_auth === true,
)
const showModelPlazaEntry = computed(
  () => modelPlazaEnabled.value && (isAuthenticated.value || !modelPlazaRequiresAuth.value),
)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const registrationEnabled = computed(() => appStore.cachedPublicSettings?.registration_enabled === true)
const features = [
  { key: 'unifiedGateway', icon: 'swap' as const },
  { key: 'multiAccount', icon: 'shield' as const },
  { key: 'balanceQuota', icon: 'chart' as const }
]

// Current year for footer
const currentYear = computed(() => new Date().getFullYear())

// Toggle theme
function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

// Initialize theme
function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    !savedTheme
  ) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

onMounted(() => {
  initTheme()

  // Check auth state
  authStore.checkAuth()

  // Ensure public settings are loaded (will use cache if already loaded from injected config)
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>
