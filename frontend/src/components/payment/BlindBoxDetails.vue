<template>
  <section class="mt-5 space-y-3 rounded-xl border border-green-200 bg-green-50/60 p-4 text-sm dark:border-green-900 dark:bg-green-950/20" data-test="blind-box-details">
    <div class="flex flex-wrap items-baseline justify-between gap-2">
      <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('payment.blindBox.name') }}</h3>
      <span class="font-semibold text-green-700 dark:text-green-400">{{ formatPaymentAmount(config.price, config.currency) }}</span>
    </div>
    <p class="text-gray-700 dark:text-gray-200">{{ t('payment.blindBox.range', { min: config.min, max: config.max }) }}</p>
    <p class="text-xs leading-relaxed text-gray-600 dark:text-gray-300">{{ t('payment.blindBox.finalAmountHint') }}</p>
    <div class="rounded-lg bg-white/80 p-3 text-xs leading-relaxed dark:bg-dark-800">
      <p>{{ t('payment.blindBox.standardComparison', { price: formatPaymentAmount(config.price, config.currency), amount: standardAmount.toFixed(2) }) }}</p>
      <p class="mt-1">{{ t('payment.blindBox.expectedAmount', { amount: expectedAmount.toFixed(4) }) }}</p>
      <p class="mt-1 font-medium text-amber-700 dark:text-amber-300">{{ t('payment.blindBox.belowStandard', { percent: percent(belowProbability) }) }}</p>
    </div>
    <details class="text-gray-700 dark:text-gray-200" open>
      <summary class="cursor-pointer font-medium">{{ t('payment.blindBox.probabilities') }}</summary>
      <table class="mt-2 w-full table-fixed text-center text-xs">
        <thead><tr class="border-b border-green-200 dark:border-green-900">
          <th class="py-2 font-medium">{{ t('payment.blindBox.creditRange') }}</th>
          <th class="py-2 font-medium">{{ t('payment.blindBox.rangeProbability') }}</th>
          <th class="py-2 font-medium">{{ t('payment.blindBox.integerProbability') }}</th>
        </tr></thead>
        <tbody><tr v-for="range in config.probabilities" :key="range.min">
          <td class="py-1.5">{{ range.min === range.max ? range.min : `${range.min}–${range.max}` }}</td>
          <td class="py-1.5">{{ percent(range.probability) }}%</td>
          <td class="py-1.5">{{ percent(range.per_integer_probability) }}%</td>
        </tr></tbody>
      </table>
    </details>
    <p class="text-xs leading-relaxed text-gray-600 dark:text-gray-300">{{ t('payment.blindBox.dailyLimit', { count: config.daily_limit }) }}</p>
    <p class="text-xs leading-relaxed text-gray-600 dark:text-gray-300">{{ t('payment.blindBox.savedResult') }}</p>
    <p class="break-words text-[11px] text-gray-400">{{ t('payment.blindBox.rulesVersion') }}: {{ config.rules_version }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { BlindBoxConfig } from '@/types/payment'
import { formatPaymentAmount } from './currency'
import { blindBoxBelowProbability, blindBoxExpectedAmount } from './blindBox'

const props = defineProps<{ config: BlindBoxConfig; standardAmount: number }>()
const { t } = useI18n()
const expectedAmount = computed(() => blindBoxExpectedAmount(props.config))
const belowProbability = computed(() => blindBoxBelowProbability(props.config, props.standardAmount))
function percent(value: number): string { return Number((value * 100).toFixed(4)).toString() }
</script>
