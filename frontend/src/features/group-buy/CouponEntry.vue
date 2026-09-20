<template>
  <div class="space-y-3">
    <button
      type="button"
      class="gb-coupon-toggle"
      :aria-expanded="expanded"
      aria-controls="month-card-coupon-input"
      :disabled="disabled"
      @click="expanded = !expanded"
    >{{ t('groupBuy.couponPrompt') }}</button>
    <form v-if="expanded" class="space-y-2" @submit.prevent="apply">
      <div class="flex gap-2">
        <input
          id="month-card-coupon-input"
          v-model="code"
          class="input min-w-0 flex-1 font-mono uppercase"
          :placeholder="t('groupBuy.couponPlaceholder')"
          :aria-label="t('groupBuy.couponCode')"
          :aria-invalid="!!error"
          aria-describedby="month-card-coupon-result"
          :disabled="disabled"
          maxlength="64"
          autocomplete="off"
          autocapitalize="characters"
          spellcheck="false"
        />
        <button type="submit" class="btn btn-secondary shrink-0" :disabled="disabled || busy || !code.trim()">
          {{ busy ? t('common.processing') : t('groupBuy.couponApply') }}
        </button>
      </div>
    </form>
    <div id="month-card-coupon-result" aria-live="polite">
      <p v-if="error" class="text-sm text-red-600 dark:text-red-400" role="alert">{{ error }}</p>
      <div v-else-if="modelValue" class="gb-coupon-applied">
        <span>{{ t('groupBuy.couponApplied', { code: modelValue.code, amount: cny(modelValue.discount_cny) }) }}</span>
        <button type="button" class="underline underline-offset-2" :disabled="disabled" @click="clear">{{ t('groupBuy.couponRemove') }}</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { groupBuyAPI } from '@/api/groupBuy'
import type { CouponQuote, MonthCardSelection } from '@/types/groupBuy'
import { extractApiErrorMessage } from '@/utils/apiError'
import { cny } from './model'
import './glass.css'

const props = defineProps<{ selection: MonthCardSelection; modelValue: CouponQuote | null; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [CouponQuote | null]; busy: [boolean] }>()
const { t } = useI18n()
const expanded = ref(false)
const code = ref('')
const error = ref('')
const busy = ref(false)
let version = 0

function invalidate() {
  version++
  error.value = ''
  busy.value = false
  emit('busy', false)
  emit('update:modelValue', null)
}
function clear() {
  code.value = ''
  invalidate()
}
watch(code, invalidate, { flush: 'sync' })
watch(() => props.selection, clear, { deep: true, flush: 'sync' })
onBeforeUnmount(() => { version++; emit('busy', false) })

async function apply() {
  if (props.disabled || busy.value || !code.value.trim()) return
  invalidate()
  const current = version
  busy.value = true
  emit('busy', true)
  try {
    const quote = await groupBuyAPI.previewCoupon(props.selection, code.value.trim())
    if (current === version) emit('update:modelValue', quote)
  } catch (err) {
    if (current === version) error.value = extractApiErrorMessage(err, t('groupBuy.couponFailed'))
  } finally {
    if (current === version) { busy.value = false; emit('busy', false) }
  }
}
</script>
