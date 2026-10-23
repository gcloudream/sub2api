/**
 * User-facing group accounts API (账号管理)
 * Read-only view of upstream accounts in the exclusive groups assigned to the current user.
 */

import { apiClient } from './client'

export type GroupAccountStatus =
  | 'active'
  | 'paused'
  | 'rate_limited'
  | 'overloaded'
  | 'temp_unschedulable'
  | 'error'
  | 'disabled'
  | 'expired'

export interface GroupAccount {
  id: number
  name: string
  platform: string
  type: string
  status: GroupAccountStatus
  status_until?: string | null
  /** true: first load can use the sampled snapshot without querying upstream */
  passive_usage: boolean
}

export interface GroupAccountGroup {
  id: number
  name: string
  platform: string
  accounts: GroupAccount[]
}

export interface GroupAccountUsageWindow {
  utilization: number
  resets_at: string | null
  remaining_seconds: number
}

export interface GroupAccountUsage {
  source?: 'passive' | 'active'
  updated_at?: string | null
  five_hour?: GroupAccountUsageWindow | null
  seven_day?: GroupAccountUsageWindow | null
  seven_day_sonnet?: GroupAccountUsageWindow | null
  seven_day_fable?: GroupAccountUsageWindow | null
}

export async function listGroupAccounts(): Promise<GroupAccountGroup[]> {
  const { data } = await apiClient.get<GroupAccountGroup[]>('/user/group-accounts')
  return data ?? []
}

export async function getGroupAccountUsage(
  accountId: number,
  source: 'passive' | 'active' = 'active'
): Promise<GroupAccountUsage> {
  const { data } = await apiClient.get<GroupAccountUsage>(`/user/group-accounts/${accountId}/usage`, {
    params: { source }
  })
  return data
}

export const groupAccountsAPI = {
  list: listGroupAccounts,
  getUsage: getGroupAccountUsage
}

export default groupAccountsAPI
