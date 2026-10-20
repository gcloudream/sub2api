import { afterEach, describe, expect, it } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { baseCompile } from '@intlify/message-compiler'
import zh from '@/i18n/locales/zh'
import en from '@/i18n/locales/en'
import BlindBoxDetails from '../BlindBoxDetails.vue'
import BlindBoxOrderResult from '../BlindBoxOrderResult.vue'
import type { BlindBoxConfig } from '@/types/payment'

enableAutoUnmount(afterEach)
const config: BlindBoxConfig = {
  enabled: true, price: 9.9, currency: 'CNY', min: 50, max: 100, daily_limit: 1, rules_version: 'test-rules-v1',
  probabilities: [
    { min: 50, max: 59, probability: .35, per_integer_probability: .035 },
    { min: 60, max: 69, probability: .6, per_integer_probability: .06 },
    { min: 70, max: 79, probability: .04, per_integer_probability: .004 },
    { min: 80, max: 89, probability: .007, per_integer_probability: .0007 },
    { min: 90, max: 99, probability: .002, per_integer_probability: .0002 },
    { min: 100, max: 100, probability: .001, per_integer_probability: .001 },
  ],
}

function i18n(locale = 'zh') {
  return createI18n({
    legacy: false, locale, messages: { zh, en },
    // Vitest uses the runtime-only build without the production JIT define.
    // Supply its locked official compiler so real dictionary interpolation is exercised.
    messageCompiler: (message) => typeof message === 'string'
      ? new Function(`return (${baseCompile(message, { mode: 'arrow' }).code})`)()
      : () => '',
  })
}

describe('mystery box purchase disclosure', () => {
  it('renders the full API probability table, fixed alternative, expected credit, and lower-credit risk before purchase', () => {
    const wrapper = mount(BlindBoxDetails, { props: { config, standardAmount: 59.4 }, global: { plugins: [i18n()] } })
    expect(wrapper.get('details').attributes()).toHaveProperty('open')
    expect(wrapper.findAll('tbody tr').map(row => row.text().replace(/\s/g, ''))).toEqual([
      '50–5935%3.5%', '60–6960%6%', '70–794%0.4%', '80–890.7%0.07%', '90–990.2%0.02%', '1000.1%0.1%',
    ])
    expect(wrapper.text()).toContain('61.6355')
    expect(wrapper.text()).toContain('$59.40')
    expect(wrapper.text()).toContain('35% 的概率低于')
    expect(wrapper.text()).toContain('取消或过期仍占当天次数')
    expect(wrapper.text()).toContain('test-rules-v1')
    expect(wrapper.text()).not.toContain('payment.blindBox.')
  })

  it('keeps English disclosures complete', () => {
    const wrapper = mount(BlindBoxDetails, { props: { config, standardAmount: 59.4 }, global: { plugins: [i18n('en')] } })
    expect(wrapper.text()).toContain('35% chance of receiving less')
    expect(wrapper.text()).toContain('61.6355')
    expect(wrapper.text()).not.toContain('payment.blindBox.')
  })

  it('renders no credit result for cancelled or pending orders', async () => {
    const order = { recharge_mode: 'blind_box' as const, status: 'PENDING', blind_box: { rules_version: 'test-v1', min: 50, max: 100, drawn: false } }
    const wrapper = mount(BlindBoxOrderResult, { props: { order }, global: { plugins: [i18n()] } })
    expect(wrapper.text()).toContain('尚无可展示的已入账结果')
    expect(wrapper.text()).not.toContain('$0')
    await wrapper.setProps({ order: { ...order, status: 'CANCELLED' } })
    expect(wrapper.text()).toContain('订单未完成')
    expect(wrapper.text()).not.toContain('额度已入账')
  })
})
