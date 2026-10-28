<template>
  <div class="wallet-orders">
    <section v-for="group in draft" :key="group.id" class="wallet-order-group" :aria-label="group.name">
      <p class="wallet-order-group-name">{{ group.name }}</p>
      <VueDraggable v-model="group.items" :animation="180" handle=".wallet-grip" :disabled="saving" :force-fallback="true"
        :delay-on-touch-only="true" :delay="100" :touch-start-threshold="5" ghost-class="wallet-drag-ghost"
        class="wallet-order-list" role="list" @start="dragging = true; emit('dragging', true)" @end="finish(group)">
        <article v-for="(item, index) in group.items" :key="refKey(item)" class="wallet-order-item" role="listitem"
          :class="{ selected: item.card?.id === selectedId, frozen: item.card?.status === 'frozen', legacy: item.kind === 'legacy' }"
          :data-testid="`wallet-item-${refKey(item)}`">
          <div class="wallet-order-top">
            <span>{{ String(index + 1).padStart(2, '0') }}</span>
            <button type="button" class="wallet-grip" :disabled="saving" :aria-label="t('groupBuy.wallet.dragCard', { name: item.card?.product_name ?? t('groupBuy.legacy') })"
              :title="t('groupBuy.wallet.keyboardSort')" @keydown.up.prevent="move(group,index,index-1)" @keydown.left.prevent="move(group,index,index-1)"
              @keydown.down.prevent="move(group,index,index+1)" @keydown.right.prevent="move(group,index,index+1)">
              <svg viewBox="0 0 20 20" width="18" height="18" fill="currentColor" aria-hidden="true"><circle v-for="point in dots" :key="point.join(',')" :cx="point[0]" :cy="point[1]" r="1.4" /></svg>
            </button>
          </div>
          <button v-if="item.card" type="button" class="wallet-card-select" :aria-pressed="item.card.id === selectedId"
            :data-testid="`wallet-select-${item.card.id}`" @click="emit('select', item.card.id)">
            <strong>{{ item.card.product_name }}</strong>
            <span class="wallet-card-tail">{{ t('groupBuy.cardCode') }} ···· {{ item.card.code.slice(-6) }}</span>
            <span class="wallet-card-available" :class="{ 'wallet-card-paused': item.card.status === 'frozen' }">
              {{ item.card.status === 'frozen' ? t('groupBuy.frozenInOrder') : usd(spendableQuota(item.card, Date.now())) }}
            </span>
          </button>
          <div v-else class="wallet-legacy-info"><strong>{{ t('groupBuy.legacy') }} #{{ item.id }}</strong><span>{{ t('groupBuy.wallet.legacyHint') }}</span></div>
          <div v-if="item.card" class="wallet-price" @mouseenter="hoverPrice = item.id" @mouseleave="hoverPrice = null">
            <button type="button" :aria-label="t('groupBuy.wallet.priceDetails')" :data-testid="`wallet-price-${item.id}`"
              :aria-expanded="priceOpen === item.id || hoverPrice === item.id" :aria-controls="`wallet-price-tip-${item.id}`"
              @click="priceOpen = priceOpen === item.id ? null : item.id" @focus="hoverPrice = item.id"
              @blur="priceOpen = null; hoverPrice = null" @keydown.esc.stop="priceOpen = null; hoverPrice = null">
              {{ item.card.purchase_price_cny != null ? cny(item.card.purchase_price_cny) : t('groupBuy.wallet.priceDetails') }} <span aria-hidden="true">ⓘ</span>
            </button>
            <div v-if="priceOpen === item.id || hoverPrice === item.id" :id="`wallet-price-tip-${item.id}`" class="wallet-price-tooltip" role="tooltip">
              <p>{{ t('groupBuy.wallet.originalPrice') }} <strong>{{ item.card.original_price_cny != null ? cny(item.card.original_price_cny) : t('groupBuy.wallet.noPrice') }}</strong></p>
              <p>{{ t('groupBuy.wallet.purchasePrice') }} <strong>{{ item.card.purchase_price_cny != null ? cny(item.card.purchase_price_cny) : t('groupBuy.wallet.noPrice') }}</strong></p>
              <small>{{ t('groupBuy.wallet.feeHint') }}</small>
            </div>
          </div>
        </article>
      </VueDraggable>
    </section>
  </div>
</template>
<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { VueDraggable } from 'vue-draggable-plus'
import type { EntitlementRef } from '@/types/groupBuy'
import { cny, refKey, usd } from './model'
import { spendableQuota, type WalletGroup } from './walletModel'
const props = defineProps<{ groups: WalletGroup[]; selectedId: number | null; saving: boolean }>()
const emit = defineEmits<{ select: [id: number]; reorder: [groupId: number, items: EntitlementRef[]]; dragging: [value: boolean] }>()
const { t } = useI18n()
const dots = [[7,4],[13,4],[7,10],[13,10],[7,16],[13,16]]
const draft = ref<WalletGroup[]>([])
const dragging = ref(false)
const priceOpen = ref<number | null>(null), hoverPrice = ref<number | null>(null)
function sync() { draft.value = props.groups.map(group => ({ ...group, items: [...group.items] })) }
watch(() => props.groups, () => { if (!dragging.value) sync() }, { immediate: true })
watch(() => props.saving, value => { if (!value && !dragging.value) sync() })
function finish(group: WalletGroup) {
  dragging.value = false
  emit('dragging', false)
  if (props.saving) { sync(); return }
  emit('reorder', group.id, group.items.map(({kind,id}) => ({kind,id})))
}
function move(group: WalletGroup, from: number, to: number) {
  if (props.saving || to < 0 || to >= group.items.length) return
  const [item] = group.items.splice(from,1)
  group.items.splice(to,0,item)
  finish(group)
}
</script>
<style scoped>
.wallet-orders{display:flex;flex-direction:column;gap:22px}.wallet-order-group-name{font-size:12px;color:#958a80;margin-bottom:12px}
.wallet-order-list{display:flex;flex-wrap:wrap;gap:14px;align-items:stretch}
.wallet-order-item{position:relative;width:240px;min-width:0;padding:14px 16px 12px;border:1px solid #e7d6c4;background:#fffdfa;border-radius:17px;box-shadow:0 3px 8px #67411c03;transition:border-color .15s,box-shadow .15s}
.wallet-order-item.selected{border:2px solid #b17a50;padding:13px 15px 11px;box-shadow:0 4px 18px #b17a500d}
.wallet-order-top{display:flex;align-items:center;justify-content:space-between;color:#b2a59a;font:11px ui-monospace,monospace;margin-bottom:6px}
.wallet-grip{display:grid;place-items:center;cursor:grab;touch-action:none;width:30px;height:26px;border-radius:6px;color:#a79585;background:#f6efe7}
.wallet-grip:active{cursor:grabbing}.wallet-grip:disabled{opacity:.45;cursor:wait}
.wallet-card-select{display:flex;flex-direction:column;align-items:flex-start;text-align:left;width:100%;gap:7px}
.wallet-card-select strong,.wallet-legacy-info strong{font-size:14px;font-weight:600;color:#353132;line-height:1.5;overflow-wrap:anywhere}
.wallet-card-tail{color:#9b9591;font-size:11px;font-family:ui-monospace,monospace}
.wallet-card-available{color:#a9754e;font-size:17px;font-weight:600;font-variant-numeric:tabular-nums;margin-top:3px}
.wallet-card-paused{color:#63859b;font-size:13px}.wallet-legacy-info{display:flex;flex-direction:column;gap:12px}.wallet-legacy-info span{font-size:12px;color:#99928b;line-height:1.7}
.wallet-order-item.legacy{background:#f6f4f1;border-style:dashed}
.wallet-price{font-size:11px;color:#ab9784;margin-top:10px;border-top:1px solid #eee5db;padding-top:8px}
.wallet-price>button{cursor:pointer;display:flex;justify-content:space-between;width:100%;text-align:left}
.wallet-price-tooltip{position:absolute;left:10px;right:10px;bottom:38px;z-index:10;padding:13px;border-radius:12px;background:#fffdf9;border:1px solid #e1cbb4;box-shadow:0 8px 24px #3d24192b;color:#706256}
.wallet-price-tooltip p{display:flex;justify-content:space-between;gap:10px;margin-bottom:7px;line-height:1.5}.wallet-price-tooltip strong{white-space:nowrap;font-weight:600}.wallet-price-tooltip small{display:block;font-size:10px;color:#a2968a;line-height:1.5}
.wallet-drag-ghost{opacity:.35;background:#eddbc8}
button:focus-visible{outline:2px solid #b17a50;outline-offset:3px}
:global(.dark) .wallet-order-item{background:#292521;border-color:#5b493b}:global(.dark) .wallet-order-item.selected{border-color:#c7996f}
:global(.dark) .wallet-card-select strong,:global(.dark) .wallet-legacy-info strong{color:#eee4da}:global(.dark) .wallet-grip{background:#3b3027;color:#c4ac94}
:global(.dark) .wallet-price-tooltip{background:#302820;border-color:#6b503a;color:#e5d0bc}:global(.dark) .wallet-price{border-color:#554435}
@media(max-width:600px){.wallet-order-list{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px}.wallet-order-item{width:auto;padding:11px}.wallet-order-item.selected{padding:10px}.wallet-card-select strong{font-size:13px}.wallet-card-paused{font-size:12px}.wallet-price-tooltip{left:3px;right:3px;padding:10px}.wallet-price-tooltip p{flex-wrap:wrap;gap:3px}}
@media(max-width:360px){.wallet-order-list{grid-template-columns:1fr}.wallet-order-top{margin-bottom:2px}}
@media(prefers-reduced-motion:reduce){.wallet-order-item{transition:none}}
</style>
