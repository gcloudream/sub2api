import { describe, expect, it } from 'vitest'
import { roundDecimalDiscount, roundDecimalProduct, roundDecimalSum } from '../decimalAmount'

describe('decimal payment amount rounding', () => {
  it.each([
    [20, 1.00125, 20.03],
    [50, 1.0009, 50.05],
    [20, 1.001249, 20.02],
    [20, 1.0012499999999998, 20.02],
    [50, 1.0008999999999997, 50.04],
    [1e-7, 1e5, 0.01],
    [1e21, 1e-21, 1],
  ])('multiplies %s by %s in decimal before rounding to %s', (amount, multiplier, expected) => {
    expect(roundDecimalProduct([amount, multiplier])).toBe(expected)
  })

  it('rounds decimal ties away from zero and does not promote a value genuinely below a tie', () => {
    expect(roundDecimalSum([1.005])).toBe(1.01)
    expect(roundDecimalSum([-1.005])).toBe(-1.01)
    expect(roundDecimalSum([1.0049999999999997])).toBe(1)
    expect(roundDecimalSum([20.03, 1])).toBe(21.03)
  })

  it('retains precision while subtracting the discount percentage', () => {
    expect(roundDecimalDiscount(10, 99.95)).toBe(0.01)
    expect(roundDecimalDiscount(20.01, 50)).toBe(10.01)
    expect(roundDecimalDiscount(101, 15, 0)).toBe(86)
  })

  it('returns zero for non-finite inputs', () => {
    expect(roundDecimalProduct([20, Infinity])).toBe(0)
    expect(roundDecimalSum([NaN])).toBe(0)
    expect(roundDecimalDiscount(10, NaN)).toBe(0)
  })
})
