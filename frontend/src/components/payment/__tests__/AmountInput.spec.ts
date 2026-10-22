import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import AmountInput from '../AmountInput.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => key === 'payment.extraBonus'
      ? `额外赠送${params?.percent}%`
      : params && 'amount' in params ? `${key} ${String(params.amount)}` : key,
  }),
}))
enableAutoUnmount(afterEach)

function mountInput(value: number | null = null) {
  return mount(AmountInput, { props: { modelValue: value } })
}

describe('recharge amount input', () => {
  it.each(['10abc', '10.555', '-10', '1e2'])('restores the accepted amount after rejecting %s', async (value) => {
    const wrapper = mountInput(10)
    const input = wrapper.get('input')
    await input.setValue(value)
    expect((input.element as HTMLInputElement).value).toBe('10')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('restores the last typed amount rather than a stale prop', async () => {
    const wrapper = mountInput()
    const input = wrapper.get('input')
    await input.setValue('12.50')
    await input.setValue('12.500')
    expect((input.element as HTMLInputElement).value).toBe('12.50')
    expect(wrapper.emitted('update:modelValue')).toEqual([[12.5]])
  })

  it('preserves decimal editing and allows clearing the amount', async () => {
    const wrapper = mountInput()
    const input = wrapper.get('input')
    for (const value of ['0', '0.', '0.5', '0.50', '']) await input.setValue(value)
    expect(wrapper.emitted('update:modelValue')).toEqual([[null], [null], [0.5], [0.5], [null]])
    expect((input.element as HTMLInputElement).value).toBe('')
  })
})

describe('AmountInput bonus offers', () => {
  it('keeps the preset order and shows the API bonus beside its amount in green', () => {
    const wrapper = mount(AmountInput, {
      props: {
        modelValue: null,
        amounts: [9.9, 1, 5, 10, 20, 50, 100, 200, 500],
        fixedBonusTiers: [{ amount: 20, bonus_percent: 5 }, { amount: 50, bonus_percent: 7 }],
      },
    })

    const buttons = wrapper.findAll('button')
    expect(buttons.map(button => button.text().replace(/\s/g, ''))).toEqual([
      '9.9', '1', '5', '10', '20额外赠送5%', '50额外赠送7%', '100', '200', '500',
    ])
    expect(buttons[4].find('.text-green-600').text()).toBe('额外赠送5%')
    expect(buttons[5].find('.text-green-600').text()).toBe('额外赠送7%')
    expect(buttons[6].find('.text-green-600').exists()).toBe(false)
  })

  it('emits the same actual amount when selecting a promoted preset or typing it', async () => {
    const wrapper = mount(AmountInput, {
      props: { modelValue: null, amounts: [20], fixedBonusTiers: [{ amount: 20, bonus_percent: 5 }] },
    })
    await wrapper.get('button').trigger('click')
    await wrapper.get('input').setValue('20.00')
    expect(wrapper.emitted('update:modelValue')).toEqual([[20], [20]])
  })
})

describe('AmountInput explicit mystery box selection', () => {
  const config = { enabled: true, price: 9.9, currency: 'CNY', min: 50, max: 100, daily_limit: 1, rules_version: 'test-v1', probabilities: [] }

  it('replaces only the matching preset with the API mystery box, preserving the other presets and bonuses', async () => {
    const wrapper = mount(AmountInput, { props: {
      modelValue: null, amounts: [9.9, 1, 5, 10, 20, 50, 100, 200, 500],
      blindBox: config, currency: 'CNY', fixedBonusTiers: [{ amount: 20, bonus_percent: 5 }],
    } })
    const buttons = wrapper.findAll('button')
    expect(buttons).toHaveLength(9)
    expect(buttons[0].text()).toContain('payment.blindBox.name')
    expect(buttons[0].text()).not.toContain('50–100')
    expect(buttons[1].text()).toBe('1')
    expect(buttons[4].text()).toContain('额外赠送5%')
    expect(wrapper.text()).toContain('¥')
    await buttons[0].trigger('click')
    expect(wrapper.emitted('select-mode')).toEqual([['blind_box']])
    expect(wrapper.emitted('update:modelValue')).toEqual([[9.9]])
    expect((wrapper.get('input').element as HTMLInputElement).value).toBe('')
  })

  it('exits mystery box mode when typing 9.9 or choosing a normal preset', async () => {
    const wrapper = mount(AmountInput, { props: { modelValue: 9.9, amounts: [9.9, 1, 5], blindBox: config, rechargeMode: 'blind_box' } })
    await wrapper.get('input').setValue('9.9')
    await wrapper.findAll('button')[1].trigger('click')
    expect(wrapper.emitted('select-mode')).toEqual([['balance'], ['balance']])
    expect(wrapper.emitted('update:modelValue')).toEqual([[9.9], [1]])
  })

  it('keeps the ordinary preset while the feature is disabled', () => {
    const wrapper = mount(AmountInput, { props: { modelValue: null, amounts: [9.9, 1, 5], blindBox: { ...config, enabled: false } } })
    expect(wrapper.findAll('button')[0].text()).toBe('9.9')
  })
})

describe('recharge bonus hints on quick amounts', () => {
  const tiers = [
    { min_amount: 100, bonus_percent: 20 },
    { min_amount: 500, bonus_percent: 30 },
  ]

  it('renders no badge or second line when no tiers are configured', () => {
    const wrapper = mount(AmountInput, { props: { modelValue: null, amounts: [50, 100, 500] } })
    expect(wrapper.find('[data-testid="quick-amount-bonus-badge"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="quick-amount-credited"]').exists()).toBe(false)
  })

  it('bonus mode: price tag only on amounts that hit a tier, credited totals on every button', () => {
    const wrapper = mount(AmountInput, { props: { modelValue: null, amounts: [50, 100, 500], bonusTiers: tiers } })
    const below = wrapper.get('[data-testid="quick-amount-50"]')
    const first = wrapper.get('[data-testid="quick-amount-100"]')
    const second = wrapper.get('[data-testid="quick-amount-500"]')

    expect(below.find('[data-testid="quick-amount-bonus-badge"]').exists()).toBe(false)
    expect(below.get('[data-testid="quick-amount-credited"]').text()).toContain('$50.00')
    expect(first.get('[data-testid="quick-amount-bonus-badge"]').text()).toBe('+20%')
    expect(first.get('[data-testid="quick-amount-credited"]').text()).toContain('$120.00')
    expect(second.get('[data-testid="quick-amount-bonus-badge"]').text()).toBe('+30%')
    expect(second.get('[data-testid="quick-amount-credited"]').text()).toContain('$650.00')
  })

  it('bonus mode: matches tiers by the entered amount but credits by the multiplier', () => {
    const wrapper = mount(AmountInput, {
      props: { modelValue: null, amounts: [1000], bonusTiers: tiers, multiplier: 0.14 },
    })
    const button = wrapper.get('[data-testid="quick-amount-1000"]')
    expect(button.get('[data-testid="quick-amount-bonus-badge"]').text()).toBe('+30%')
    // 1000 × 0.14 = 140 base, +30% = 182
    expect(button.get('[data-testid="quick-amount-credited"]').text()).toContain('$182.00')
  })

  it('discount mode: tag reads N% OFF and the second line shows the discounted payment', () => {
    const wrapper = mount(AmountInput, {
      props: { modelValue: null, amounts: [50, 500], bonusTiers: tiers, bonusMode: 'discount', currency: 'USD' },
    })
    const below = wrapper.get('[data-testid="quick-amount-50"]')
    const hit = wrapper.get('[data-testid="quick-amount-500"]')
    expect(below.find('[data-testid="quick-amount-bonus-badge"]').exists()).toBe(false)
    expect(below.get('[data-testid="quick-amount-credited"]').text()).toContain('50.00')
    expect(hit.get('[data-testid="quick-amount-bonus-badge"]').text()).toBe('30% OFF')
    // 500 × (1 − 30%) = 350
    expect(hit.get('[data-testid="quick-amount-credited"]').text()).toContain('350.00')
  })
})
