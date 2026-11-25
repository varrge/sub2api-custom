<template>
  <form class="space-y-6" data-testid="shake-editor" @submit.prevent="submit">
    <section class="grid gap-4 sm:grid-cols-3">
      <label class="space-y-1.5 text-sm sm:col-span-3">
        <span class="font-medium text-gray-700 dark:text-dark-200">{{ t('shakeAdmin.name') }}</span>
        <input v-model.trim="form.name" class="input" maxlength="100" required data-testid="shake-name" />
      </label>
      <label class="space-y-1.5 text-sm">
        <span class="font-medium text-gray-700 dark:text-dark-200">{{ t('shakeAdmin.startsAt') }}</span>
        <input v-model="startsAt" type="datetime-local" class="input" required :disabled="startLocked" data-testid="shake-start" />
      </label>
      <label class="space-y-1.5 text-sm">
        <span class="font-medium text-gray-700 dark:text-dark-200">{{ t('shakeAdmin.endsAt') }}</span>
        <input v-model="endsAt" type="datetime-local" class="input" required data-testid="shake-end" />
      </label>
      <p class="self-end text-xs text-gray-500 dark:text-dark-400">
        {{ startLocked ? t('shakeAdmin.startLockedHint') : t('shakeAdmin.timeHint') }}
      </p>
    </section>

    <section>
      <div class="mb-2 flex items-center justify-between">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('shakeAdmin.rulesTitle') }}</h3>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="startLocked" data-testid="shake-add-rule" @click="addRule">
          <Icon name="plus" size="sm" />{{ t('shakeAdmin.addRule') }}
        </button>
      </div>
      <p class="mb-3 text-xs text-gray-500 dark:text-dark-400">{{ t('shakeAdmin.rulesHint') }} {{ startLocked ? t('shakeAdmin.rulesLockedHint') : '' }}</p>
      <p v-if="form.rules.length === 0" class="rounded-lg bg-gray-50 p-3 text-xs text-gray-500 dark:bg-dark-800">{{ t('shakeAdmin.noRules') }}</p>
      <div v-for="(rule, i) in form.rules" :key="i" class="mb-2 grid gap-2 rounded-lg bg-gray-50 p-3 dark:bg-dark-800 sm:grid-cols-7" :data-testid="`shake-rule-${i}`">
        <select v-model="rule.source" :disabled="startLocked" class="input" :aria-label="t('shakeAdmin.ruleSource')" @change="normalizeRule(rule)">
          <option value="order">{{ t('shakeAdmin.sourceOrder') }}</option>
          <option value="usage">{{ t('shakeAdmin.sourceUsage') }}</option>
        </select>
        <select v-if="rule.source === 'order'" v-model="rule.order_type" :disabled="startLocked" class="input" :aria-label="t('shakeAdmin.orderType')">
          <option value="subscription">{{ t('luckyBox.ruleOrder_subscription') }}</option>
          <option value="month_card">{{ t('luckyBox.ruleOrder_month_card') }}</option>
          <option value="balance">{{ t('luckyBox.ruleOrder_balance') }}</option>
        </select>
        <select v-else v-model="rule.billing" :disabled="startLocked" class="input" :aria-label="t('shakeAdmin.billing')">
          <option value="balance">{{ t('shakeAdmin.billingBalance') }}</option>
          <option value="subscription">{{ t('shakeAdmin.billingSubscription') }}</option>
        </select>
        <select v-model="rule.mode" class="input" :disabled="startLocked || rule.source === 'usage'" :aria-label="t('shakeAdmin.mode')">
          <option value="per_order">{{ t('shakeAdmin.modePerOrder') }}</option>
          <option value="threshold">{{ t('shakeAdmin.modeThreshold') }}</option>
        </select>
        <input v-model.number="rule.group_id" :disabled="startLocked" type="number" min="1" class="input" :placeholder="t('shakeAdmin.groupAll')" :aria-label="t('shakeAdmin.groupId')" />
        <input v-if="rule.mode === 'threshold'" v-model.number="rule.threshold" type="number" min="0.01" step="0.01" class="input" :placeholder="thresholdLabel(rule)" :aria-label="thresholdLabel(rule)" required />
        <span v-else class="self-center text-xs text-gray-500">{{ t('shakeAdmin.perOrderHint') }}</span>
        <div class="flex gap-2">
          <input v-model.number="rule.quantity" type="number" min="1" max="100" class="input w-20" :aria-label="t('shakeAdmin.quantity')" required />
          <select v-model="rule.card_tier" class="input" :aria-label="t('shakeAdmin.cardTier')">
            <option v-for="tier in tiers" :key="tier" :value="tier">{{ t(`luckyBox.tier${cap(tier)}`) }}</option>
          </select>
        </div>
        <button type="button" class="btn btn-secondary btn-sm justify-self-end" :aria-label="t('shakeAdmin.remove')" :disabled="startLocked" @click="form.rules.splice(i, 1)">
          <Icon name="trash" size="sm" />
        </button>
      </div>
    </section>

    <section>
      <div class="mb-2 flex items-center justify-between">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('shakeAdmin.prizesTitle') }}</h3>
        <button type="button" class="btn btn-secondary btn-sm" data-testid="shake-add-prize" @click="addPrize">
          <Icon name="plus" size="sm" />{{ t('shakeAdmin.addPrize') }}
        </button>
      </div>
      <p class="mb-3 text-xs text-gray-500 dark:text-dark-400">{{ t('shakeAdmin.prizesHint') }}</p>
      <div class="overflow-x-auto">
        <table class="w-full min-w-[760px] text-sm">
          <thead class="text-left text-xs text-gray-500 dark:text-dark-400">
            <tr>
              <th class="py-1 pr-2">{{ t('shakeAdmin.prizeName') }}</th>
              <th class="py-1 pr-2">{{ t('shakeAdmin.prizeType') }}</th>
              <th class="py-1 pr-2">{{ t('shakeAdmin.amount') }}</th>
              <th class="py-1 pr-2">{{ t('shakeAdmin.rarity') }}</th>
              <th v-for="tier in tiers" :key="tier" class="py-1 pr-2">{{ t(`luckyBox.tier${cap(tier)}`) }}</th>
              <th class="py-1 pr-2">{{ t('shakeAdmin.displayText') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(prize, i) in form.prizes" :key="prize.id || i" :data-testid="`shake-prize-${i}`">
              <td class="py-1 pr-2"><input v-model.trim="prize.name" class="input" maxlength="100" required /></td>
              <td class="py-1 pr-2">
                <select v-model="prize.type" class="input">
                  <option value="balance">{{ t('shakeAdmin.typeBalance') }}</option>
                  <option value="extra_draw">{{ t('shakeAdmin.typeExtra') }}</option>
                  <option value="empty">{{ t('shakeAdmin.typeEmpty') }}</option>
                </select>
              </td>
              <td class="py-1 pr-2"><input v-model.number="prize.amount" type="number" min="0.01" step="0.01" class="input w-24" :disabled="prize.type !== 'balance'" :required="prize.type === 'balance'" /></td>
              <td class="py-1 pr-2">
                <select v-model="prize.rarity" class="input">
                  <option v-for="r in rarities" :key="r" :value="r">{{ t(`luckyBox.rarity${cap(r)}`) }}</option>
                </select>
              </td>
              <td v-for="tier in tiers" :key="tier" class="py-1 pr-2">
                <input v-model.number="prize.weights[tier]" type="number" min="0" step="any" class="input w-20" required />
                <span class="block text-[11px] text-gray-500">{{ pct(prize, tier) }}</span>
              </td>
              <td class="py-1 pr-2"><input v-model.trim="prize.display_text" class="input" maxlength="200" /></td>
              <td class="py-1"><button type="button" class="btn btn-secondary btn-sm" :aria-label="t('shakeAdmin.remove')" :disabled="form.prizes.length <= 1" @click="form.prizes.splice(i, 1)"><Icon name="trash" size="sm" /></button></td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-for="tier in emptyTiers" :key="tier" class="mt-2 text-xs text-red-600 dark:text-red-400">
        {{ t('shakeAdmin.tierEmpty', { tier: t(`luckyBox.tier${cap(tier)}`) }) }}
      </p>
    </section>

    <div class="flex justify-end gap-2">
      <button type="button" class="btn btn-secondary" @click="emit('cancel')">{{ t('common.cancel') }}</button>
      <button type="submit" class="btn btn-primary" :disabled="saving || emptyTiers.length > 0" data-testid="shake-save">{{ saving ? t('common.saving') : t('common.save') }}</button>
    </div>
  </form>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { ShakeConfig, ShakePrize, ShakeRule } from '@/api/admin/shake'
import type { BoxCardTier } from '@/api/luckyBox'
import { beijingDateTimeLocalToRfc3339, toBeijingDateTimeLocal } from '@/utils/beijingDateTime'

const props = defineProps<{ initial?: ShakeConfig; startLocked?: boolean; saving?: boolean }>()
const emit = defineEmits<{ (e: 'save', config: ShakeConfig): void; (e: 'cancel'): void }>()
const { t } = useI18n()

const tiers: BoxCardTier[] = ['low', 'medium', 'high']
const rarities = ['common', 'rare', 'epic', 'legendary'] as const
const cap = (v: string) => v.charAt(0).toUpperCase() + v.slice(1)

const defaults = (): ShakeConfig => ({
  name: '',
  starts_at: '',
  ends_at: '',
  rules: [
    { source: 'order', order_type: 'subscription', mode: 'per_order', group_id: null, quantity: 1, card_tier: 'medium' },
    { source: 'usage', billing: 'balance', mode: 'threshold', group_id: null, threshold: 10, quantity: 1, card_tier: 'low' }
  ],
  prizes: [
    { name: '$1 余额', type: 'balance', amount: 1, weights: { low: 30, medium: 30, high: 20 }, rarity: 'rare', display_text: '', icon: '', sort_order: 0 },
    { name: '再来一次', type: 'extra_draw', amount: null, weights: { low: 10, medium: 10, high: 10 }, rarity: 'epic', display_text: '', icon: '', sort_order: 1 },
    { name: '谢谢参与', type: 'empty', amount: null, weights: { low: 60, medium: 40, high: 20 }, rarity: 'common', display_text: '', icon: '', sort_order: 2 }
  ]
})

const form = reactive<ShakeConfig>(JSON.parse(JSON.stringify(props.initial ?? defaults())))
const startsAt = ref(toBeijingDateTimeLocal(form.starts_at))
const endsAt = ref(toBeijingDateTimeLocal(form.ends_at))

const normalizeRule = (rule: ShakeRule) => {
  if (rule.source === 'usage') {
    rule.mode = 'threshold'
    rule.billing = rule.billing ?? 'balance'
    rule.threshold = rule.threshold || 10
  } else {
    rule.order_type = rule.order_type ?? 'subscription'
  }
}
const thresholdLabel = (rule: ShakeRule) => t(rule.source === 'order' && rule.order_type === 'month_card' ? 'shakeAdmin.thresholdCny' : 'shakeAdmin.threshold')
const addRule = () => form.rules.push({ source: 'usage', billing: 'balance', mode: 'threshold', group_id: null, threshold: 10, quantity: 1, card_tier: 'low' })
const addPrize = () => form.prizes.push({ name: '', type: 'empty', amount: null, weights: { low: 1, medium: 1, high: 1 }, rarity: 'common', display_text: '', icon: '', sort_order: form.prizes.length })

const total = (tier: BoxCardTier) => form.prizes.reduce((sum, p) => sum + (Number(p.weights[tier]) || 0), 0)
const pct = (prize: ShakePrize, tier: BoxCardTier) => {
  const sum = total(tier)
  return sum > 0 ? `${(((Number(prize.weights[tier]) || 0) / sum) * 100).toFixed(2)}%` : '—'
}
const emptyTiers = computed(() => tiers.filter((tier) => !(total(tier) > 0)))

const submit = () => {
  const config: ShakeConfig = JSON.parse(JSON.stringify(form))
  config.starts_at = beijingDateTimeLocalToRfc3339(startsAt.value) ?? ''
  config.ends_at = beijingDateTimeLocalToRfc3339(endsAt.value) ?? ''
  config.rules = config.rules.map((r) => {
    const out: ShakeRule = { source: r.source, mode: r.source === 'usage' ? 'threshold' : r.mode, quantity: Number(r.quantity), card_tier: r.card_tier }
    if (r.source === 'order') out.order_type = r.order_type
    else out.billing = r.billing
    if (r.group_id) out.group_id = Number(r.group_id)
    if (out.mode === 'threshold') out.threshold = Number(r.threshold)
    return out
  })
  config.prizes = config.prizes.map((p, i) => ({ ...p, sort_order: i, amount: p.type === 'balance' ? Number(p.amount) : null }))
  emit('save', config)
}
</script>
