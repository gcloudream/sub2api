import { describe, expect, it } from 'vitest'
import { blindBoxBelowProbability, blindBoxExpectedAmount, completedBlindBoxAmount } from '../blindBox'
import type { BlindBoxConfig } from '@/types/payment'

export const blindBoxFixture: BlindBoxConfig = {
  enabled: true, price: 9.9, currency: 'CNY', min: 50, max: 100, daily_limit: 1, rules_version: 'test-v1',
  probabilities: [
    { min: 50, max: 59, probability: .35, per_integer_probability: .035 },
    { min: 60, max: 69, probability: .6, per_integer_probability: .06 },
    { min: 70, max: 79, probability: .04, per_integer_probability: .004 },
    { min: 80, max: 89, probability: .007, per_integer_probability: .0007 },
    { min: 90, max: 99, probability: .002, per_integer_probability: .0002 },
    { min: 100, max: 100, probability: .001, per_integer_probability: .001 },
  ],
}

describe('blind box presentation calculations', () => {
  it('derives the expected credit and below-standard probability from the server distribution', () => {
    expect(blindBoxExpectedAmount(blindBoxFixture)).toBeCloseTo(61.6355, 8)
    expect(blindBoxBelowProbability(blindBoxFixture, 59.4)).toBeCloseTo(.35, 8)
    const differentRules = { ...blindBoxFixture, probabilities: [{ min: 70, max: 70, probability: 1, per_integer_probability: 1 }] }
    expect(blindBoxExpectedAmount(differentRules)).toBe(70)
    expect(blindBoxBelowProbability(differentRules, 59.4)).toBe(0)
  })

  it('uses a saved integer result only after the order is completed', () => {
    const order = { recharge_mode: 'blind_box' as const, status: 'COMPLETED', blind_box: { rules_version: 'test-v1', min: 50, max: 100, drawn: true, amount: 63 } }
    expect(completedBlindBoxAmount(order)).toBe(63)
    for (const status of ['PENDING', 'PAID', 'RECHARGING', 'CANCELLED', 'FAILED']) {
      expect(completedBlindBoxAmount({ ...order, status })).toBeNull()
    }
    expect(completedBlindBoxAmount({ ...order, blind_box: { ...order.blind_box, drawn: false } })).toBeNull()
    for (const amount of [0, 49, 101, 63.5]) {
      expect(completedBlindBoxAmount({ ...order, blind_box: { ...order.blind_box, amount } })).toBeNull()
    }
  })
})
