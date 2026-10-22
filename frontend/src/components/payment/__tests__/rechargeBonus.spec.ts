import { describe, expect, it } from 'vitest'
import { calculateRechargeCredit, getRechargeBonusPercent, normalizeFixedRechargeBonusTiers, quoteRechargeWithFixedBonus } from '../rechargeBonus'

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


describe('merged configured and Fantasy fixed offers', () => {
  it('recognizes legacy exact checkout tiers without converting threshold tiers to exact tiers', () => {
    expect(normalizeFixedRechargeBonusTiers([
      { amount: 20, bonus_percent: 5 },
      { min_amount: 20, bonus_percent: 10 },
      { amount: 20.001, bonus_percent: 5 },
    ])).toEqual([{ amount: 20, bonus_percent: 5 }])
  })

  it('keeps exact default credit and does not grant a nearby amount a bonus', () => {
    expect(quoteRechargeWithFixedBonus(20, { multiplier: 6, fixedTiers: tiers })).toMatchObject({
      payBase: 20, base: 120, percent: 5, bonus: 6, credited: 126,
    })
    expect(quoteRechargeWithFixedBonus(20.01, { multiplier: 6, fixedTiers: tiers })).toMatchObject({
      payBase: 20.01, base: 120.06, percent: 0, bonus: 0, credited: 120.06,
    })
  })

  it('configured tiers suppress fixed offers even below their first threshold', () => {
    expect(quoteRechargeWithFixedBonus(20, {
      multiplier: 6, fixedTiers: tiers, tiers: [{ min_amount: 100, bonus_percent: 20 }],
    })).toMatchObject({ base: 120, bonus: 0, credited: 120 })
  })

  it('configured discounts reduce payment while retaining the original credit without stacking fixed offers', () => {
    expect(quoteRechargeWithFixedBonus(20, {
      multiplier: 6, fixedTiers: tiers, tiers: [{ min_amount: 10, bonus_percent: 20 }], mode: 'discount',
    })).toMatchObject({ payBase: 16, base: 120, percent: 20, bonus: 24, credited: 120 })
  })
})


describe('Fantasy fixed offers use the backend decimal rounding contract', () => {
  it.each([
    [20, 1.00125, 20.03, 1, 21.03],
    [50, 1.0009, 50.05, 3.5, 53.55],
    [20, 1.001249, 20.02, 1, 21.02],
    [20, 1.0012499999999998, 20.02, 1, 21.02],
  ])('quotes %s with multiplier %s without losing or inventing a half cent', (amount, multiplier, base, bonus, total) => {
    expect(calculateRechargeCredit(amount, multiplier, tiers)).toMatchObject({ base, bonus, total })
    expect(quoteRechargeWithFixedBonus(amount, { multiplier, fixedTiers: tiers })).toMatchObject({ base, bonus, credited: total })
  })
})
