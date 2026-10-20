import type { BlindBoxConfig, BlindBoxOrderInfo, RechargeMode } from '@/types/payment'

export interface BlindBoxOrder {
  recharge_mode?: RechargeMode
  blind_box?: BlindBoxOrderInfo
  status: string
  paid_at?: string | null
}

export function isBlindBoxOrder(order: BlindBoxOrder | null | undefined): boolean {
  return order?.recharge_mode === 'blind_box'
}

export function blindBoxExpectedAmount(config: BlindBoxConfig): number {
  return config.probabilities.reduce((sum, range) => sum + (range.min + range.max) / 2 * range.probability, 0)
}

export function blindBoxBelowProbability(config: BlindBoxConfig, standardAmount: number): number {
  return config.probabilities.reduce((sum, range) => {
    const below = Math.max(0, Math.min(range.max, Math.ceil(standardAmount) - 1) - range.min + 1)
    return sum + below * range.per_integer_probability
  }, 0)
}

// Only a completed order may claim that the saved draw has reached the balance.
// Pending / failed orders never show the placeholder amount or a client-side draw.
export function completedBlindBoxAmount(order: BlindBoxOrder | null | undefined): number | null {
  const box = order?.blind_box
  if (!isBlindBoxOrder(order) || order?.status !== 'COMPLETED' || !box?.drawn) return null
  const amount = box.amount
  return typeof amount === 'number' && Number.isInteger(amount) && amount >= box.min && amount <= box.max
    ? amount : null
}

export function isBlindBoxAwaitingFulfillment(order: BlindBoxOrder | null | undefined): boolean {
  if (!isBlindBoxOrder(order) || !order) return false
  if (order.status === 'PAID' || order.status === 'RECHARGING') return true
  if (order.status === 'COMPLETED') return completedBlindBoxAmount(order) === null
  // A fulfillment failure after confirmed payment is recoverable. It must not
  // reopen the payment prompt or be presented as an unpaid expired order.
  return order.status === 'FAILED' && (order.blind_box?.drawn === true || !!order.paid_at)
}
