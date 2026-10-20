import type { RechargeBonusTier } from '@/types/payment'

function amountInCents(amount: number): number | null {
  if (!Number.isFinite(amount) || amount <= 0) return null
  const cents = amount * 100
  const rounded = Math.round(cents)
  // Do not let rounding turn an unsupported amount into an eligible tier.
  return Number.isSafeInteger(rounded) && Math.abs(cents - rounded) < 1e-7 ? rounded : null
}

export function getRechargeBonusPercent(amount: number, tiers: RechargeBonusTier[] = []): number {
  const cents = amountInCents(amount)
  if (cents === null) return 0
  const tier = tiers.find((candidate) => candidate.amount === amount && amountInCents(candidate.amount) === cents
    && Number.isFinite(candidate.bonus_percent) && candidate.bonus_percent > 0)
  return tier?.bonus_percent ?? 0
}

export function calculateRechargeCredit(amount: number, multiplier: number, tiers: RechargeBonusTier[] = []) {
  const base = Math.round(amount * multiplier * 100) / 100
  const bonusPercent = getRechargeBonusPercent(amount, tiers)
  const bonus = Math.round(base * bonusPercent) / 100
  return { base, bonusPercent, bonus, total: Math.round((base + bonus) * 100) / 100 }
}
