<template>
  <div v-if="isBlindBoxOrder(order)" class="rounded-xl border border-green-200 bg-green-50 p-4 dark:border-green-900 dark:bg-green-950/20" data-test="blind-box-order-result">
    <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('payment.blindBox.name') }}</p>
    <template v-if="amount !== null">
      <p class="mt-2 text-3xl font-bold tabular-nums text-green-700 dark:text-green-400">${{ amount }}</p>
      <p class="mt-1 text-xs text-gray-600 dark:text-gray-300">{{ t('payment.blindBox.credited') }}</p>
    </template>
    <p v-else-if="!isBlindBoxAwaitingFulfillment(order) && ['CANCELLED', 'EXPIRED', 'FAILED'].includes(order.status)" class="mt-2 text-sm text-gray-600 dark:text-gray-300">{{ t('payment.blindBox.noResult') }}</p>
    <p v-else class="mt-2 text-sm text-gray-600 dark:text-gray-300">{{ t('payment.blindBox.awaitingResult') }}</p>
    <p v-if="order.blind_box?.rules_version" class="mt-2 break-words text-[11px] text-gray-400">{{ t('payment.blindBox.rulesVersion') }}: {{ order.blind_box.rules_version }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { completedBlindBoxAmount, isBlindBoxAwaitingFulfillment, isBlindBoxOrder, type BlindBoxOrder } from './blindBox'
const props = defineProps<{ order: BlindBoxOrder }>()
const { t } = useI18n()
const amount = computed(() => completedBlindBoxAmount(props.order))
</script>
