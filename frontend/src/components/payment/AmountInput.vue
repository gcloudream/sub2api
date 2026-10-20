<template>
  <div class="space-y-4">
    <!-- Quick Amount Buttons -->
    <div>
      <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
        {{ t('payment.quickAmounts') }}
      </label>
      <div class="grid grid-cols-3 gap-2">
        <button
          v-for="preset in presets"
          :key="preset.blindBox ? 'blind-box' : preset.amount"
          type="button"
          :class="[
            'flex h-[52px] min-w-0 items-center justify-center rounded-lg border-2 px-2 text-center text-base font-medium leading-5 transition-colors sm:px-4',
            modelValue === preset.amount && (rechargeMode === 'blind_box') === preset.blindBox
              ? 'border-primary-500 bg-primary-50 text-primary-700 dark:border-primary-400 dark:bg-primary-900/40 dark:text-primary-300'
              : 'border-gray-200 bg-white text-gray-700 hover:border-gray-300 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200 dark:hover:border-dark-500',
          ]"
          @click="selectAmount(preset.amount, preset.blindBox)"
        >
          <span class="inline-flex max-w-full flex-wrap items-baseline justify-center gap-x-1.5">
            <span>{{ preset.amount }}</span>
            <template v-if="preset.blindBox && blindBox">
              <span class="text-xs leading-3">{{ t('payment.blindBox.name') }}</span>
              <span class="text-xs font-normal leading-3">{{ blindBox.min }}–{{ blindBox.max }}</span>
            </template>
            <span
              v-if="!preset.blindBox && bonusPercent(preset.amount) > 0"
              class="text-xs font-medium text-green-600 dark:text-green-400"
            >{{ t('payment.extraBonus', { percent: bonusPercent(preset.amount) }) }}</span>
          </span>
        </button>
      </div>
    </div>

    <!-- Custom Amount Input -->
    <div>
      <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
        {{ t('payment.customAmount') }}
      </label>
      <div class="relative">
        <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-dark-500">
          {{ currencySymbol(currency) }}
        </span>
        <input
          type="text"
          inputmode="decimal"
          :value="customText"
          :placeholder="placeholderText"
          class="input w-full py-3 pl-8 pr-4"
          @input="handleInput"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { currencySymbol } from './currency'
import type { BlindBoxConfig, RechargeMode, RechargeBonusTier } from '@/types/payment'
import { getRechargeBonusPercent } from './rechargeBonus'

const props = withDefaults(defineProps<{
  amounts?: number[]
  bonusTiers?: RechargeBonusTier[]
  blindBox?: BlindBoxConfig
  rechargeMode?: RechargeMode
  currency?: string
  modelValue: number | null
  min?: number
  max?: number
}>(), {
  amounts: () => [10, 20, 50, 100, 200, 500, 1000, 2000, 5000],
  bonusTiers: () => [],
  rechargeMode: 'balance',
  currency: 'USD',
  min: 0,
  max: 0,
})

const emit = defineEmits<{
  'update:modelValue': [value: number | null]
  'select-mode': [value: RechargeMode]
}>()

const { t } = useI18n()

const customText = ref('')

function bonusPercent(amount: number) {
  return getRechargeBonusPercent(amount, props.bonusTiers)
}

// 0 = no limit
const filteredAmounts = computed(() =>
  props.amounts.filter((a) => (props.min <= 0 || a >= props.min) && (props.max <= 0 || a <= props.max))
)

const presets = computed(() => {
  const amounts = filteredAmounts.value
    .filter(amount => !props.blindBox?.enabled || amount !== props.blindBox.price)
    .map(amount => ({ amount, blindBox: false }))
  const box = props.blindBox
  if (box?.enabled && (props.min <= 0 || box.price >= props.min) && (props.max <= 0 || box.price <= props.max)) {
    amounts.unshift({ amount: box.price, blindBox: true })
  }
  return amounts
})

const placeholderText = computed(() => {
  if (props.min > 0 && props.max > 0) return `${props.min} - ${props.max}`
  if (props.min > 0) return `≥ ${props.min}`
  if (props.max > 0) return `≤ ${props.max}`
  return t('payment.enterAmount')
})

const AMOUNT_PATTERN = /^\d*(\.\d{0,2})?$/

function selectAmount(amt: number, blindBox = false) {
  customText.value = blindBox ? '' : String(amt)
  emit('select-mode', blindBox ? 'blind_box' : 'balance')
  emit('update:modelValue', amt)
}

function handleInput(e: Event) {
  const input = e.target as HTMLInputElement
  const val = input.value
  if (!AMOUNT_PATTERN.test(val)) {
    input.value = customText.value
    return
  }
  customText.value = val
  emit('select-mode', 'balance')
  if (val === '') {
    emit('update:modelValue', null)
    return
  }
  const num = parseFloat(val)
  if (!isNaN(num) && num > 0) {
    emit('update:modelValue', num)
  } else {
    emit('update:modelValue', null)
  }
}

watch(() => props.modelValue, (v) => {
  if (props.rechargeMode === 'blind_box') return
  if (v !== null && String(v) !== customText.value) {
    customText.value = String(v)
  }
}, { immediate: true })
</script>
