<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('groupAccounts.hint') }}</p>
        <button
          type="button"
          class="btn btn-secondary"
          data-testid="group-accounts-refresh-all"
          :disabled="loading || refreshingAll"
          @click="refreshAll"
        >
          <Icon name="refresh" size="md" :class="refreshingAll ? 'animate-spin' : ''" />
          <span>{{ t('groupAccounts.refreshAll') }}</span>
        </button>
      </div>

      <div v-if="loading" class="card p-10 text-center text-sm text-gray-500 dark:text-gray-400">
        {{ t('common.loading') }}
      </div>

      <div
        v-else-if="groups.length === 0"
        class="card p-10 text-center text-sm text-gray-500 dark:text-gray-400"
        data-testid="group-accounts-empty"
      >
        {{ t('groupAccounts.empty') }}
      </div>

      <section v-for="group in groups" v-else :key="group.id" class="card overflow-hidden">
        <header class="flex items-center justify-between gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <div class="flex min-w-0 items-center gap-2">
            <PlatformIcon :platform="group.platform as GroupPlatform" size="sm" />
            <h2 class="truncate text-base font-semibold text-gray-900 dark:text-white">{{ group.name }}</h2>
          </div>
          <span class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('groupAccounts.accountCount', { count: group.accounts.length }) }}
          </span>
        </header>

        <div v-if="group.accounts.length === 0" class="px-5 py-6 text-sm text-gray-500 dark:text-gray-400">
          {{ t('groupAccounts.noAccounts') }}
        </div>

        <ul v-else class="divide-y divide-gray-100 dark:divide-dark-700">
          <li
            v-for="account in group.accounts"
            :key="account.id"
            class="grid gap-3 px-5 py-4 md:grid-cols-[minmax(0,14rem)_minmax(0,10rem)_minmax(0,1fr)_auto] md:items-center"
            data-testid="group-account-row"
          >
            <div class="min-w-0">
              <p class="truncate font-medium text-gray-900 dark:text-white" :title="account.name">{{ account.name }}</p>
              <p class="text-xs text-gray-400 dark:text-gray-500">{{ platformTypeLabel(account) }}</p>
            </div>

            <div class="min-w-0">
              <span :class="['badge', statusClass(account.status)]" data-testid="group-account-status">
                {{ t(`groupAccounts.status.${account.status}`) }}
              </span>
              <p v-if="account.status_until" class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                {{ t('groupAccounts.until', { time: formatDateTimeToMinute(account.status_until) }) }}
              </p>
            </div>

            <div class="min-w-0 space-y-1">
              <template v-if="usageOf(account.id).usage && hasWindows(usageOf(account.id).usage)">
                <UsageProgressBar
                  v-for="w in windowsOf(usageOf(account.id).usage)"
                  :key="w.label"
                  :label="w.label"
                  :utilization="w.window.utilization"
                  :resets-at="w.window.resets_at"
                  :color="w.color"
                />
                <p v-if="usageOf(account.id).usage?.updated_at" class="text-[11px] text-gray-400 dark:text-gray-500">
                  {{ t('groupAccounts.updatedAt', { time: formatDateTimeToMinute(usageOf(account.id).usage?.updated_at) }) }}
                </p>
                <p
                  v-if="usageOf(account.id).error"
                  class="text-xs text-amber-600 dark:text-amber-400"
                  data-testid="group-account-usage-error"
                >
                  {{ usageOf(account.id).error }}
                </p>
              </template>
              <p v-else-if="usageOf(account.id).loading" class="text-xs text-gray-400">{{ t('common.loading') }}</p>
              <p
                v-else-if="usageOf(account.id).error"
                class="text-xs text-amber-600 dark:text-amber-400"
                data-testid="group-account-usage-error"
              >
                {{ usageOf(account.id).error }}
              </p>
              <p v-else class="text-xs text-gray-400">{{ t('groupAccounts.noUsage') }}</p>
            </div>

            <div class="flex md:justify-end">
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                data-testid="group-account-refresh"
                :disabled="usageOf(account.id).loading"
                :title="t('groupAccounts.refreshUsage')"
                @click="loadUsage(account, 'active')"
              >
                <Icon name="refresh" size="sm" :class="usageOf(account.id).loading ? 'animate-spin' : ''" />
                <span class="md:hidden">{{ t('groupAccounts.refreshUsage') }}</span>
              </button>
            </div>
          </li>
        </ul>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import UsageProgressBar from '@/components/account/UsageProgressBar.vue'
import {
  groupAccountsAPI,
  type GroupAccount,
  type GroupAccountGroup,
  type GroupAccountStatus,
  type GroupAccountUsage,
  type GroupAccountUsageWindow
} from '@/api/groupAccounts'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTimeToMinute } from '@/utils/format'
import type { GroupPlatform } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()

const groups = ref<GroupAccountGroup[]>([])
const loading = ref(true)
const refreshingAll = ref(false)

interface UsageState {
  usage: GroupAccountUsage | null
  loading: boolean
  error: string
}
const usageStates = reactive<Record<number, UsageState>>({})

// 首屏并发上限：账号多时避免一次性打出过多用量查询。
const USAGE_CONCURRENCY = 3

function usageOf(accountId: number): UsageState {
  if (!usageStates[accountId]) {
    usageStates[accountId] = { usage: null, loading: false, error: '' }
  }
  return usageStates[accountId]
}

type WindowColor = 'indigo' | 'emerald' | 'purple' | 'amber'
interface WindowRow {
  label: string
  window: GroupAccountUsageWindow
  color: WindowColor
}

function windowsOf(usage: GroupAccountUsage | null): WindowRow[] {
  if (!usage) return []
  const rows: Array<[string, GroupAccountUsageWindow | null | undefined, WindowColor]> = [
    ['5h', usage.five_hour, 'indigo'],
    ['7d', usage.seven_day, 'emerald'],
    ['7d S', usage.seven_day_sonnet, 'purple'],
    ['7d F', usage.seven_day_fable, 'amber']
  ]
  return rows
    .filter((row): row is [string, GroupAccountUsageWindow, WindowColor] => !!row[1])
    .map(([label, window, color]) => ({ label, window, color }))
}

function hasWindows(usage: GroupAccountUsage | null): boolean {
  return windowsOf(usage).length > 0
}

function statusClass(status: GroupAccountStatus): string {
  switch (status) {
    case 'active':
      return 'badge-success'
    case 'rate_limited':
    case 'overloaded':
    case 'temp_unschedulable':
      return 'badge-warning'
    case 'error':
    case 'expired':
      return 'badge-danger'
    default:
      return 'badge-gray'
  }
}

function platformTypeLabel(account: GroupAccount): string {
  const platform = account.platform === 'anthropic' ? 'Claude' : account.platform === 'openai' ? 'OpenAI' : account.platform
  return account.type ? `${platform} · ${account.type}` : platform
}

async function loadUsage(account: GroupAccount, source: 'passive' | 'active'): Promise<void> {
  const state = usageOf(account.id)
  if (state.loading) return
  state.loading = true
  state.error = ''
  try {
    state.usage = await groupAccountsAPI.getUsage(account.id, source)
  } catch {
    // 上游错误细节只记在服务端日志；失败时保留上一次的用量数据并提示。
    state.error = t('groupAccounts.usageFailed')
  } finally {
    state.loading = false
  }
}

async function loadAllUsage(source: (account: GroupAccount) => 'passive' | 'active'): Promise<void> {
  const queue = groups.value.flatMap((g) => g.accounts)
  const workers = Array.from({ length: Math.min(USAGE_CONCURRENCY, queue.length) }, async () => {
    while (queue.length > 0) {
      const account = queue.shift()
      if (account) await loadUsage(account, source(account))
    }
  })
  await Promise.all(workers)
}

async function loadGroups(): Promise<boolean> {
  try {
    groups.value = await groupAccountsAPI.list()
    return true
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
    return false
  }
}

async function refreshAll(): Promise<void> {
  refreshingAll.value = true
  try {
    if (await loadGroups()) {
      await loadAllUsage(() => 'active')
    }
  } finally {
    refreshingAll.value = false
  }
}

onMounted(async () => {
  const ok = await loadGroups()
  loading.value = false
  if (ok) {
    // 首屏优先读已采样快照，不主动请求上游；用户点击刷新时再查询。
    await loadAllUsage((account) => (account.passive_usage ? 'passive' : 'active'))
  }
})
</script>
