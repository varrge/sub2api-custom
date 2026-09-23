<template>
  <BaseDialog :show="show" :title="dialogTitle" width="normal" @close="$emit('close')">
    <div v-if="product" class="pdd space-y-4 text-sm">
      <div class="pdd-meta"><p>{{ product.group_name }} · {{ product.platform }}</p><p>{{ t('groupBuy.recruitmentHours') }}: {{ product.recruitment_hours }} · {{ t('groupBuy.maxMembers') }}: {{ product.max_members }}</p></div>
      <p v-if="product.description" class="pdd-text">{{ product.description }}</p>
      <section>
        <h4 class="pdd-heading">{{ t('groupBuy.tierLadderTitle') }}</h4>
        <ol class="pdd-list">
          <li v-for="(step, index) in steps" :key="step.members" class="pdd-stage">
            <div class="pdd-stage-head"><span class="pdd-stage-name">{{ index === 0 ? t('groupBuy.initialPurchase') : t('groupBuy.reachMembers', { members: step.members }) }}</span><span class="pdd-stage-quota">{{ usd(step.quota_usd) }}</span></div>
            <div class="pdd-stage-sub"><span>{{ t('groupBuy.weeklyQuota') }} {{ usd(step.quota_usd / 4) }}</span><span aria-hidden="true">·</span><span>{{ t('groupBuy.effectiveRate') }} {{ effectiveRate(step.quota_usd) }}</span><template v-if="index > 0"><span aria-hidden="true">·</span><span class="pdd-diff">{{ t('groupBuy.tierIncrease', { amount: usd(step.increase) }) }}</span></template></div>
          </li>
        </ol>
        <p class="pdd-note">{{ t('groupBuy.quotaGrowthHint') }}</p><p class="pdd-note">{{ t('groupBuy.effectiveRateHint') }}</p>
      </section>
      <section><h4 class="pdd-heading">{{ t('groupBuy.viewRules') }}</h4><p class="pdd-text">{{ t('groupBuy.rules') }}</p><p class="pdd-text">{{ t('groupBuy.soloHint') }} {{ t('groupBuy.createHint') }}</p></section>
    </div>
  </BaseDialog>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { GroupBuyProduct } from '@/types/groupBuy'
import { quotaSteps, usd } from './model'
const props = defineProps<{ show: boolean; product: GroupBuyProduct | null }>()
defineEmits<{ close: [] }>()
const { t } = useI18n()
const dialogTitle = computed(() => props.product ? `${props.product.name} · ${t('groupBuy.cardDetails')}` : t('groupBuy.cardDetails'))
const steps = computed(() => props.product ? quotaSteps(props.product) : [])
function effectiveRate(quota: number) { return props.product && quota > 0 ? (props.product.price_cny / quota).toFixed(4) : '—' }
</script>
<style scoped>
.pdd { min-width: 0; overflow-wrap: anywhere; }
.pdd-meta,.pdd-text,.pdd-note { color: rgb(100 116 139); line-height: 1.6; }
.pdd-meta { display: flex; flex-direction: column; gap: .125rem; font-size: .8125rem; }
.pdd-heading { font-size: .75rem; font-weight: 600; letter-spacing: .02em; color: rgb(110 110 118); }
.pdd-text,.pdd-note { margin-top: .375rem; font-size: .8125rem; }
.pdd-list { margin: .5rem 0 0; padding: 0; list-style: none; border: 1px solid rgb(229 229 233 / .8); border-radius: .75rem; overflow: hidden; background: rgb(255 255 255 / .5); }
.pdd-stage { min-width: 0; padding: .5rem .75rem; } .pdd-stage + .pdd-stage { border-top: 1px solid rgb(245 245 247 / .8); }
.pdd-stage-head { display: flex; min-width: 0; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: .125rem .5rem; }
.pdd-stage-name { min-width: 0; font-weight: 500; color: rgb(55 65 81); } .pdd-stage-quota { min-width: 0; font-weight: 700; font-variant-numeric: tabular-nums; color: rgb(138 92 36); }
.pdd-stage-sub { display: flex; min-width: 0; flex-wrap: wrap; gap: .125rem .375rem; margin-top: .125rem; font-size: .75rem; line-height: 1.5; font-variant-numeric: tabular-nums; color: rgb(100 116 139); }
.pdd-diff { color: rgb(138 92 36); }
.dark .pdd-meta,.dark .pdd-text,.dark .pdd-note,.dark .pdd-stage-sub { color: rgb(148 163 184); } .dark .pdd-heading { color: rgb(161 161 170); } .dark .pdd-stage-name { color: rgb(203 213 225); } .dark .pdd-stage-quota,.dark .pdd-diff { color: rgb(229 185 154); } .dark .pdd-list { border-color: rgb(46 46 53 / .7); background: rgb(18 18 21 / .4); } .dark .pdd-stage + .pdd-stage { border-top-color: rgb(46 46 53 / .4); }
</style>
