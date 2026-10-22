import type { FixedRechargeBonusTier, RechargeBonusTier } from '@/types/payment'
import { quoteRechargeBonus, type RechargeBonusQuoteOptions } from '@/utils/rechargeBonus'
import { roundDecimalProduct, roundDecimalSum } from '@/utils/decimalAmount'

function amountInCents(amount: number): number | null {
  if (!Number.isFinite(amount) || amount <= 0) return null
  const cents = amount * 100
  const rounded = Math.round(cents)
  // Do not let rounding turn an unsupported amount into an eligible tier.
  return Number.isSafeInteger(rounded) && Math.abs(cents - rounded) < 1e-7 ? rounded : null
}

export function getRechargeBonusPercent(amount: number, tiers: FixedRechargeBonusTier[] = []): number {
  const cents = amountInCents(amount)
  if (cents === null) return 0
  const tier = tiers.find((candidate) => candidate.amount === amount && amountInCents(candidate.amount) === cents
    && Number.isFinite(candidate.bonus_percent) && candidate.bonus_percent > 0)
  return tier?.bonus_percent ?? 0
}

export function calculateRechargeCredit(amount: number, multiplier: number, tiers: FixedRechargeBonusTier[] = []) {
  const base = roundDecimalProduct([amount, multiplier])
  const bonusPercent = getRechargeBonusPercent(amount, tiers)
  const bonus = roundDecimalProduct([base, bonusPercent, 0.01])
  return { base, bonusPercent, bonus, total: roundDecimalSum([base, bonus]) }
}

/** Accept the legacy checkout amount shape without treating threshold offers as fixed offers. */
export function normalizeFixedRechargeBonusTiers(raw: unknown): FixedRechargeBonusTier[] {
  if (!Array.isArray(raw)) return []
  return raw.flatMap((item) => {
    if (!item || typeof item !== 'object') return []
    const tier = item as { amount?: unknown; min_amount?: unknown; bonus_percent?: unknown }
    if (tier.min_amount !== undefined || typeof tier.amount !== 'number' || amountInCents(tier.amount) === null
      || typeof tier.bonus_percent !== 'number' || !Number.isFinite(tier.bonus_percent) || tier.bonus_percent <= 0) return []
    return [{ amount: tier.amount, bonus_percent: tier.bonus_percent }]
  })
}

/** Configured upstream tiers take priority; the Fantasy fixed offers never stack with them. */
export function quoteRechargeWithFixedBonus(
  amount: number,
  options: RechargeBonusQuoteOptions & { tiers?: RechargeBonusTier[]; fixedTiers?: FixedRechargeBonusTier[] } = {},
) {
  const quote = quoteRechargeBonus(options.tiers ?? [], amount, options)
  if (options.tiers?.length) return quote
  const credit = calculateRechargeCredit(amount, options.multiplier ?? 1, options.fixedTiers)
  return { ...quote, mode: 'bonus' as const, percent: credit.bonusPercent, bonus: credit.bonus, credited: credit.total }
}
