import { describe, expect, it } from 'vitest'
import { calculateRechargeCredit, getRechargeBonusPercent } from '../rechargeBonus'

const tiers = [
  { amount: 20, bonus_percent: 5 },
  { amount: 50, bonus_percent: 7 },
  { amount: 100, bonus_percent: 10 },
  { amount: 200, bonus_percent: 12 },
  { amount: 500, bonus_percent: 15 },
]

describe('recharge bonus preview', () => {
  it.each([
    [20, 120, 6, 126],
    [50, 300, 21, 321],
    [100, 600, 60, 660],
    [200, 1200, 144, 1344],
    [500, 3000, 450, 3450],
  ])('shows base, extra, and final credit for a %i recharge', (amount, base, bonus, total) => {
    expect(calculateRechargeCredit(amount, 6, tiers)).toMatchObject({ base, bonus, total })
  })

  it.each([1, 5, 9.9, 10, 19.99, 20.01, 20.001, 20.000000000000004, 49.99, 501])(
    'does not grant a nearby or non-tier amount %s a bonus', (amount) => {
      expect(getRechargeBonusPercent(amount, tiers)).toBe(0)
    },
  )

  it('uses API tiers rather than a separate client promotion list', () => {
    expect(calculateRechargeCredit(20, 6)).toEqual({ base: 120, bonusPercent: 0, bonus: 0, total: 120 })
    expect(calculateRechargeCredit(9.9, 6, [{ amount: 9.9, bonus_percent: 10 }])).toEqual({
      base: 59.4, bonusPercent: 10, bonus: 5.94, total: 65.34,
    })
  })

  it('rounds the base and bonus separately, matching server credit calculation', () => {
    expect(calculateRechargeCredit(20, 0.3146, tiers)).toEqual({
      base: 6.29, bonusPercent: 5, bonus: 0.31, total: 6.6,
    })
  })
})
