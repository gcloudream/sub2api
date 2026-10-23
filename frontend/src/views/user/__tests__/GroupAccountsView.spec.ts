import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import GroupAccountsView from '../GroupAccountsView.vue'

const { list, getUsage, showError } = vi.hoisted(() => ({
  list: vi.fn(),
  getUsage: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/groupAccounts', () => ({
  groupAccountsAPI: { list, getUsage },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}:${JSON.stringify(params)}` : key,
    }),
  }
})

const mountView = () =>
  mount(GroupAccountsView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        Icon: true,
        PlatformIcon: true,
        UsageProgressBar: {
          props: ['label', 'utilization'],
          template: '<div data-testid="usage-bar">{{ label }}={{ utilization }}</div>',
        },
      },
    },
  })

describe('GroupAccountsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    list.mockResolvedValue([
      {
        id: 8,
        name: 'Claude真神',
        platform: 'anthropic',
        accounts: [
          { id: 1, name: 'claude-a', platform: 'anthropic', type: 'oauth', status: 'active', passive_usage: true },
          {
            id: 2,
            name: 'claude-b',
            platform: 'anthropic',
            type: 'oauth',
            status: 'rate_limited',
            status_until: '2026-10-23T13:00:00Z',
            passive_usage: true,
          },
        ],
      },
      {
        id: 9,
        name: 'GPT 专属',
        platform: 'openai',
        accounts: [{ id: 3, name: 'gpt-a', platform: 'openai', type: 'oauth', status: 'error', passive_usage: false }],
      },
    ])
    getUsage.mockResolvedValue({
      five_hour: { utilization: 40, resets_at: '2026-10-23T15:00:00Z', remaining_seconds: 100 },
      seven_day: { utilization: 12, resets_at: null, remaining_seconds: 0 },
    })
  })

  it('renders groups with account names, statuses and usage windows', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('Claude真神')
    expect(wrapper.text()).toContain('GPT 专属')
    const rows = wrapper.findAll('[data-testid="group-account-row"]')
    expect(rows).toHaveLength(3)
    expect(rows[0].text()).toContain('claude-a')
    expect(rows[0].get('[data-testid="group-account-status"]').text()).toBe('groupAccounts.status.active')
    expect(rows[1].get('[data-testid="group-account-status"]').text()).toBe('groupAccounts.status.rate_limited')
    expect(rows[1].text()).toContain('groupAccounts.until')
    expect(rows[2].get('[data-testid="group-account-status"]').text()).toBe('groupAccounts.status.error')
    expect(rows[0].findAll('[data-testid="usage-bar"]').map((b) => b.text())).toEqual(['5h=40', '7d=12'])
  })

  it('uses the passive snapshot on first load and queries actively on refresh', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(getUsage).toHaveBeenCalledWith(1, 'passive')
    expect(getUsage).toHaveBeenCalledWith(2, 'passive')
    expect(getUsage).toHaveBeenCalledWith(3, 'active')
    expect(getUsage).toHaveBeenCalledTimes(3)

    getUsage.mockClear()
    await wrapper.findAll('[data-testid="group-account-refresh"]')[0].trigger('click')
    await flushPromises()
    expect(getUsage).toHaveBeenCalledTimes(1)
    expect(getUsage).toHaveBeenCalledWith(1, 'active')

    getUsage.mockClear()
    await wrapper.get('[data-testid="group-accounts-refresh-all"]').trigger('click')
    await flushPromises()
    expect(list).toHaveBeenCalledTimes(2)
    expect(getUsage.mock.calls.map((c) => c[1])).toEqual(['active', 'active', 'active'])
  })

  it('shows a per-account error when a usage query fails', async () => {
    getUsage.mockImplementation((id: number) =>
      id === 3 ? Promise.reject(new Error('boom')) : Promise.resolve({ five_hour: null })
    )
    const wrapper = mountView()
    await flushPromises()

    const rows = wrapper.findAll('[data-testid="group-account-row"]')
    expect(rows[0].text()).toContain('groupAccounts.noUsage')
    expect(rows[2].text()).not.toContain('groupAccounts.noUsage')
    expect(rows[2].findAll('[data-testid="usage-bar"]')).toHaveLength(0)
  })

  it('shows the empty state when no exclusive group is assigned', async () => {
    list.mockResolvedValue([])
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="group-accounts-empty"]').exists()).toBe(true)
    expect(getUsage).not.toHaveBeenCalled()
  })
})
