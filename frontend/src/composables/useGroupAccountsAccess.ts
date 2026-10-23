import { computed, ref } from 'vue'
import { groupAccountsAPI } from '@/api/groupAccounts'
import { useAuthStore } from '@/stores/auth'

// 侧边栏「账号管理」入口：仅当当前用户被分配了至少一个专属分组时显示。
// 结果按用户缓存，切换登录用户后重新查询。

const loadedForUserId = ref<number | null>(null)
const hasGroups = ref(false)
let pendingLoad: Promise<boolean> | null = null

async function loadGroupAccountsAccess(force = false): Promise<boolean> {
  const authStore = useAuthStore()
  const userId = authStore.user?.id ?? null
  if (!authStore.isAuthenticated || userId === null) {
    loadedForUserId.value = null
    hasGroups.value = false
    return false
  }

  if (!force && loadedForUserId.value === userId) {
    return hasGroups.value
  }

  if (pendingLoad && !force) {
    return pendingLoad
  }

  pendingLoad = groupAccountsAPI
    .list()
    .then((groups) => {
      hasGroups.value = groups.length > 0
      loadedForUserId.value = userId
      return hasGroups.value
    })
    .catch(() => {
      hasGroups.value = false
      loadedForUserId.value = null
      return false
    })
    .finally(() => {
      pendingLoad = null
    })

  return pendingLoad
}

export function useGroupAccountsAccess() {
  const authStore = useAuthStore()
  const canViewGroupAccounts = computed(
    () => hasGroups.value && loadedForUserId.value !== null && loadedForUserId.value === (authStore.user?.id ?? null)
  )

  return {
    canViewGroupAccounts,
    refreshGroupAccountsAccess: loadGroupAccountsAccess
  }
}
