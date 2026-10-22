/** Decimal arithmetic on Number's shortest decimal form, matching Go decimal.NewFromFloat. */
interface DecimalAmount {
  coefficient: bigint
  scale: number
}

function parseDecimal(value: number): DecimalAmount | null {
  if (!Number.isFinite(value)) return null
  const [mantissa, exponent = '0'] = String(value).toLowerCase().split('e')
  const [whole, fraction = ''] = mantissa.split('.')
  return { coefficient: BigInt(whole + fraction), scale: fraction.length - Number(exponent) }
}

function multiplyDecimals(values: DecimalAmount[]): DecimalAmount {
  return values.reduce((result, value) => ({
    coefficient: result.coefficient * value.coefficient,
    scale: result.scale + value.scale,
  }), { coefficient: 1n, scale: 0 })
}

function sumDecimals(values: DecimalAmount[]): DecimalAmount {
  const scale = Math.max(0, ...values.map(value => value.scale))
  return {
    coefficient: values.reduce((sum, value) => sum + value.coefficient * 10n ** BigInt(scale - value.scale), 0n),
    scale,
  }
}

/** Round half away from zero, without a binary product or epsilon before the rounding step. */
function roundDecimal(value: DecimalAmount, digits: number): number {
  if (!Number.isInteger(digits) || digits < 0 || digits > 100) return 0
  const negative = value.coefficient < 0n
  const magnitude = negative ? -value.coefficient : value.coefficient
  const placesToDrop = value.scale - digits
  let units: bigint
  if (placesToDrop > 0) {
    const divisor = 10n ** BigInt(placesToDrop)
    units = magnitude / divisor + (magnitude % divisor * 2n >= divisor ? 1n : 0n)
  } else {
    units = magnitude * 10n ** BigInt(-placesToDrop)
  }
  const text = units.toString().padStart(digits + 1, '0')
  const sign = negative && units !== 0n ? '-' : ''
  const result = Number(digits ? `${sign}${text.slice(0, -digits)}.${text.slice(-digits)}` : sign + text)
  return Number.isFinite(result) ? result : 0
}

export function roundDecimalProduct(factors: number[], digits = 2): number {
  const values = factors.map(parseDecimal)
  if (values.some(value => value === null)) return 0
  return roundDecimal(multiplyDecimals(values as DecimalAmount[]), digits)
}

export function roundDecimalSum(terms: number[], digits = 2): number {
  const values = terms.map(parseDecimal)
  if (values.some(value => value === null)) return 0
  return roundDecimal(sumDecimals(values as DecimalAmount[]), digits)
}

/** amount × (100 − percent) / 100, retaining decimal precision through the subtraction. */
export function roundDecimalDiscount(amount: number, percent: number, digits = 2): number {
  const payment = parseDecimal(amount)
  const discount = parseDecimal(-percent)
  if (!payment || !discount) return 0
  const remaining = sumDecimals([{ coefficient: 100n, scale: 0 }, discount])
  return roundDecimal(multiplyDecimals([payment, remaining, { coefficient: 1n, scale: 2 }]), digits)
}
