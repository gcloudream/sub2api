import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { mount, RouterLinkStub } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AppSidebar from '../AppSidebar.vue'

const { authStore, appStore, adminSettingsStore } = vi.hoisted(() => ({
  authStore: { isAdmin: false, isSimpleMode: false },
  appStore: {
    siteName: 'Test site',
    siteLogo: '',
    sidebarCollapsed: false,
    mobileOpen: false,
    publicSettingsLoaded: true,
    sidebarScrollTop: 0,
    backendModeEnabled: false,
    cachedPublicSettings: {
      payment_enabled: false,
      subscription_enabled: false,
      custom_menu_items: [],
    },
    setMobileOpen: vi.fn(),
    toggleSidebar: vi.fn(),
  },
  adminSettingsStore: {
    customMenuItems: [],
    opsMonitoringEnabled: false,
    paymentEnabled: false,
    fetch: vi.fn(),
  },
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => authStore,
  useAppStore: () => appStore,
  useAdminSettingsStore: () => adminSettingsStore,
  useOnboardingStore: () => ({ isCurrentStep: vi.fn(() => false), nextStep: vi.fn() }),
}))

vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))

vi.mock('@/composables/useBatchImageAccess', () => ({
  useBatchImageAccess: () => ({ canUseBatchImage: { value: false }, refreshBatchImageAccess: vi.fn() }),
}))

vi.mock('vue-router', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-router')>(),
  useRoute: () => ({ path: '/keys' }),
  useRouter: () => ({ push: vi.fn() }),
}))

vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')

describe('AppSidebar usage guide navigation', () => {
  beforeEach(() => {
    authStore.isAdmin = false
    authStore.isSimpleMode = false
    appStore.sidebarCollapsed = false
    appStore.mobileOpen = false
    appStore.backendModeEnabled = false
    appStore.setMobileOpen.mockClear()
  })

  it.each([
    ['regular user', false, false, false],
    ['administrator', true, false, false],
    ['simple-mode user', false, true, false],
    ['simple-mode administrator', true, true, false],
    ['backend-mode user', false, false, true],
  ])('shows one guide entry for %s even with payment and subscriptions disabled', (_label, admin, simple, backend) => {
    authStore.isAdmin = admin
    authStore.isSimpleMode = simple
    appStore.backendModeEnabled = backend
    const wrapper = mount(AppSidebar, {
      global: { stubs: { RouterLink: RouterLinkStub, Icon: true } },
    })
    const guides = wrapper.findAllComponents(RouterLinkStub)
      .filter((link) => link.props('to') === '/docs')

    expect(guides).toHaveLength(1)
    expect(guides[0].text()).toBe('nav.usageDocs')
    expect(guides[0].get('icon-stub').attributes('name')).toBe('book')
    wrapper.unmount()
  })

  it('keeps the collapsed guide accessible by its title and closes the mobile drawer after navigation', async () => {
    vi.useFakeTimers()
    try {
      appStore.sidebarCollapsed = true
      appStore.mobileOpen = true
      const wrapper = mount(AppSidebar, {
        global: { stubs: { RouterLink: RouterLinkStub, Icon: true } },
      })
      const guide = wrapper.findAllComponents(RouterLinkStub)
        .find((link) => link.props('to') === '/docs')!

      expect(guide.attributes('title')).toBe('nav.usageDocs')
      await guide.trigger('click')
      await vi.advanceTimersByTimeAsync(150)
      expect(appStore.setMobileOpen).toHaveBeenCalledWith(false)
      wrapper.unmount()
    } finally {
      vi.useRealTimers()
    }
  })
})

describe('AppSidebar custom SVG styles', () => {
  it('does not override uploaded SVG fill or stroke colors', () => {
    expect(componentSource).toContain('.sidebar-svg-icon {')
    expect(componentSource).toContain('color: currentColor;')
    expect(componentSource).toContain('display: block;')
    expect(componentSource).not.toContain('stroke: currentColor;')
    expect(componentSource).not.toContain('fill: none;')
  })
})

describe('AppSidebar scroll position persistence', () => {
  it('binds a template ref to the sidebar nav element', () => {
    expect(componentSource).toContain('ref="sidebarNavRef"')
    expect(componentSource).toContain('sidebar-nav')
  })

  it('declares sidebarNavRef in script setup', () => {
    expect(componentSource).toContain("const sidebarNavRef = ref<HTMLElement | null>(null)")
  })

  it('saves scroll position on beforeUnmount', () => {
    expect(componentSource).toContain('onBeforeUnmount')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('sidebarNavRef.value.scrollTop')
  })

  it('restores scroll position on mount', () => {
    expect(componentSource).toContain('onMounted')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('nextTick')
  })
})

describe('AppSidebar collapsible groups', () => {
  it('lets the user collapse a group even while a child route is active', () => {
    // The expand state must come from the user's override first, falling back
    // to the active-route heuristic only when the user has not clicked yet.
    expect(componentSource).toContain('const groupExpandOverrides = ref<Map<string, boolean>>(new Map())')
    expect(componentSource).not.toContain('expandedGroups.value.has(item.path) || isGroupActive(item)')
  })
})

describe('AppSidebar header styles', () => {
  it('does not clip the version badge dropdown', () => {
    const sidebarHeaderBlockMatch = styleSource.match(/\.sidebar-header\s*\{[\s\S]*?\n {2}\}/)
    const sidebarBrandBlockMatch = componentSource.match(/\.sidebar-brand\s*\{[\s\S]*?\n\}/)

    expect(sidebarHeaderBlockMatch).not.toBeNull()
    expect(sidebarBrandBlockMatch).not.toBeNull()
    expect(sidebarHeaderBlockMatch?.[0]).not.toContain('@apply overflow-hidden;')
    expect(sidebarBrandBlockMatch?.[0]).not.toContain('overflow: hidden;')
  })
})

describe('AppSidebar subscription feature flag', () => {
  it('gates the My Subscriptions entry behind the subscription public-settings flag', () => {
    expect(componentSource).toContain('const flagSubscription = makeSidebarFlag(FeatureFlags.subscription)')
    expect(componentSource).toMatch(/path: '\/subscriptions'[^\n]*featureFlag: flagSubscription/)
  })

  it('also hides the admin Subscription Management entry on recharge-only sites', () => {
    expect(componentSource).toMatch(/path: '\/admin\/subscriptions'[^\n]*featureFlag: flagSubscription/)
  })

  it('derives the purchase entry label from the site billing mode', () => {
    expect(componentSource).toContain("import { resolveSiteBillingMode } from '@/utils/siteBillingMode'")
    expect(componentSource).toMatch(/case 'recharge_only':\s*return t\('nav\.recharge'\)/)
    expect(componentSource).toMatch(/case 'subscription_only':\s*return t\('nav\.subscribe'\)/)
    expect(componentSource).toMatch(/path: '\/purchase'[^\n]*label: purchaseNavLabel\.value/)
  })
})
