<template>
  <AppLayout>
    <div class="space-y-5">
      <div class="flex flex-wrap justify-between gap-3">
        <h1 class="text-2xl font-bold">{{ t('groupBuy.admin') }}</h1>
        <div class="flex gap-2">
          <RouterLink
            to="/admin/orders?order_type=month_card"
            class="btn btn-secondary"
          >
            {{ t('groupBuy.ordersLink') }}
          </RouterLink>
          <button class="btn btn-secondary" :disabled="loading" @click="load">
            {{ t('common.refresh') }}
          </button>
        </div>
      </div>
      <div class="flex flex-wrap gap-2">
        <button
          v-for="key in ['products', 'teams', 'cards'] as const"
          :key="key"
          class="btn"
          :class="tab === key ? 'btn-primary' : 'btn-secondary'"
          @click="tab = key"
        >
          {{ t(`groupBuy.${key}`) }}
        </button>
      </div>
      <p
        v-if="error"
        class="rounded-lg bg-red-50 p-3 text-sm text-red-600 dark:bg-red-950"
        role="alert"
      >
        {{ error }}
      </p>
      <div v-if="loading" class="py-10 text-center">
        {{ t('common.loading') }}
      </div>
      <template v-else-if="tab === 'products'">
        <div class="flex justify-end">
          <button class="btn btn-primary" @click="editProduct()">
            {{ t('groupBuy.newProduct') }}
          </button>
        </div>
        <p
          class="rounded-xl bg-sky-50 p-4 text-sm text-sky-800 dark:bg-sky-950 dark:text-sky-200"
        >
          {{ t('groupBuy.productHint') }}
        </p>
        <div class="card overflow-x-auto">
          <table class="w-full text-left text-sm">
            <thead>
              <tr>
                <th class="p-4">{{ t('groupBuy.name') }}</th>
                <th class="p-4">{{ t('groupBuy.group') }}</th>
                <th class="p-4">{{ t('groupBuy.price') }}</th>
                <th class="p-4">{{ t('groupBuy.forSale') }}</th>
                <th class="p-4">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="product in products"
                :key="product.id"
                class="border-t border-gray-100 dark:border-dark-700"
              >
                <td class="p-4">{{ product.name }}</td>
                <td class="p-4">{{ product.group_name }}</td>
                <td class="p-4">{{ cny(product.price_cny) }}</td>
                <td class="p-4">
                  {{ product.for_sale ? t('common.yes') : t('common.no') }}
                </td>
                <td class="p-4">
                  <button
                    class="btn btn-secondary"
                    @click="editProduct(product)"
                  >
                    {{ t('common.edit') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
          <p v-if="!products.length" class="p-8 text-center text-gray-500">
            {{ t('groupBuy.noProducts') }}
          </p>
        </div>
      </template>
      <template v-else-if="tab === 'teams'">
        <form class="flex gap-2" @submit.prevent="inspectTeam(teamCode)">
          <input
            v-model="teamCode"
            class="input"
            :placeholder="t('groupBuy.teamCode')"
            :aria-label="t('groupBuy.teamCode')"
          />
          <button class="btn btn-primary" :disabled="!teamCode.trim()">
            {{ t('groupBuy.lookup') }}
          </button>
        </form>
        <div class="grid gap-4 md:grid-cols-2">
          <article v-for="team in teams" :key="team.id" class="card p-4">
            <h2 class="font-semibold">
              {{ team.product.name }} · {{ team.code }}
            </h2>
            <p class="my-2 text-sm">
              {{ t(`groupBuy.${team.status}`) }} · {{ team.member_count }} /
              {{ team.product.max_members }} · {{ usd(team.current_quota_usd) }}
            </p>
            <p class="text-xs text-gray-500">
              {{ t('groupBuy.closes') }} {{ exactDate(team.closes_at) }}
            </p>
            <button
              class="btn btn-secondary mt-3"
              @click="inspectTeam(team.code)"
            >
              {{ t('groupBuy.inspect') }}
            </button>
            <button
              v-if="team.status === 'recruiting'"
              class="btn btn-secondary mt-3 ml-2"
              :disabled="cancelling"
              @click="openCancellation(team)"
            >
              {{ t('groupBuy.cancelRecruitment') }}
            </button>
          </article>
        </div>
      </template>
      <template v-else>
        <form class="flex gap-2" @submit.prevent="loadCards">
          <input
            v-model.number="userId"
            type="number"
            min="1"
            class="input"
            :placeholder="t('groupBuy.userId')"
            :aria-label="t('groupBuy.userId')"
          />
          <button class="btn btn-primary" :disabled="cardsLoading || !userId">
            {{ t('common.search') }}
          </button>
        </form>
        <p class="text-sm text-amber-700 dark:text-amber-300">
          {{ t('groupBuy.refundHint') }}
        </p>
        <div class="grid gap-4 lg:grid-cols-2">
          <MonthCardCard v-for="card in cards" :key="card.id" :card="card">
            <div
              class="mt-4 border-t border-gray-100 pt-3 text-sm dark:border-dark-700"
            >
              <p>{{ t('groupBuy.userId') }}: {{ card.user_id }}</p>
              <RouterLink
                :to="{
                  path: '/admin/orders',
                  query: { order_id: card.order_id, order_type: 'month_card' }
                }"
                class="text-primary-600"
              >
                {{ t('groupBuy.order') }} #{{ card.order_id }} ·
                {{ t('groupBuy.ordersLink') }}
              </RouterLink>
            </div>
          </MonthCardCard>
        </div>
        <p v-if="!userId" class="text-sm text-gray-500">{{ t('groupBuy.selectUserHint') }}</p>
      <AllocationTable v-if="userId" :allocations="allocations" :cards="cards" />
      </template>
      <BaseDialog
        :show="!!draft"
        :title="t(draft?.id ? 'groupBuy.editProduct' : 'groupBuy.newProduct')"
        @close="draft = null"
      >
        <form
          v-if="draft"
          id="group-buy-product-form"
          class="space-y-4"
          @submit.prevent="saveProduct"
        >
          <label class="block text-sm">
            {{ t('groupBuy.name') }}
            <input
              v-model="draft.name"
              class="input mt-1 w-full"
              required
              maxlength="100"
            />
          </label>
          <label class="block text-sm">
            {{ t('groupBuy.group') }}
            <select
              v-model.number="draft.group_id"
              class="input mt-1 w-full"
              required
            >
              <option :value="0" disabled>{{ t('groupBuy.group') }}</option>
              <option
                v-for="group in groups.filter(
                  (group) => group.subscription_type === 'subscription'
                )"
                :key="group.id"
                :value="group.id"
              >
                {{ group.name }} · {{ group.platform }}
              </option>
            </select>
          </label>
          <label class="block text-sm">
            {{ t('groupBuy.description') }}
            <textarea
              v-model="draft.description"
              class="input mt-1 w-full"
              rows="2"
            />
          </label>
          <div class="grid grid-cols-2 gap-3">
            <label
              v-for="field in productFields"
              :key="field.key"
              class="block text-sm"
            >
              {{ t(`groupBuy.${field.label}`) }}
              <input
                v-model.number="draft[field.key]"
                type="number"
                :min="field.min"
                :step="field.step"
                class="input mt-1 w-full"
                required
              />
            </label>
          </div>
          <div class="space-y-2">
            <p class="text-sm font-medium">{{ t('groupBuy.tiers') }}</p>
            <div
              v-for="(tier, index) in draft.tiers"
              :key="index"
              class="flex items-center gap-2"
            >
              <label class="min-w-0 flex-1 text-xs">
                {{ t('groupBuy.tierMembers') }}
                <input
                  v-model.number="tier.members"
                  type="number"
                  min="2"
                  step="1"
                  required
                  class="input mt-1 w-full"
                />
              </label>
              <label class="min-w-0 flex-1 text-xs">
                {{ t('groupBuy.tierQuota') }}
                <input
                  v-model.number="tier.quota_usd"
                  type="number"
                  min="0.00000001"
                  step="0.00000001"
                  required
                  class="input mt-1 w-full"
                />
              </label>
              <button
                type="button"
                class="btn btn-secondary mt-5"
                :aria-label="t('common.delete')"
                @click="draft.tiers.splice(index, 1)"
              >
                ×
              </button>
            </div>
            <button
              type="button"
              class="btn btn-secondary"
              @click="
                draft.tiers.push({
                  members: (draft.tiers.at(-1)?.members ?? 1) + 1,
                  quota_usd:
                    (draft.tiers.at(-1)?.quota_usd ?? draft.base_quota_usd) + 1
                })
              "
            >
              {{ t('groupBuy.addTier') }}
            </button>
          </div>
          <label class="flex gap-2 text-sm">
            <input v-model="draft.for_sale" type="checkbox" />
            {{ t('groupBuy.forSale') }}
          </label>
          <p class="text-xs leading-5 text-gray-500">
            {{ t('groupBuy.rules') }} {{ t('groupBuy.productHint') }}
          </p>
          <p v-if="formError" class="text-sm text-red-600" role="alert">
            {{ formError }}
          </p>
        </form>
        <template #footer>
          <button
            class="btn btn-secondary"
            :disabled="saving"
            @click="draft = null"
          >
            {{ t('common.cancel') }}
          </button>
          <button
            form="group-buy-product-form"
            type="submit"
            class="btn btn-primary"
            :disabled="saving"
          >
            {{ t('common.save') }}
          </button>
        </template>
      </BaseDialog>
      <BaseDialog
        :show="!!teamDetail"
        :title="`${t('groupBuy.teamCode')} ${teamDetail?.code || ''}`"
        @close="teamDetail = null"
      >
        <div v-if="teamDetail" class="space-y-4 text-sm">
          <h3 class="font-semibold">
            {{ t('groupBuy.frozenRules') }} · {{ teamDetail.product.name }}
          </h3>
          <p>
            {{ teamDetail.product.group_name }} ·
            {{ cny(teamDetail.product.price_cny) }} ·
            {{ t('groupBuy.baseQuota') }}
            {{ usd(teamDetail.product.base_quota_usd) }}
          </p>
          <p v-for="tier in teamDetail.product.tiers" :key="tier.members">
            {{
              t('groupBuy.tier', {
                members: tier.members,
                amount: usd(tier.quota_usd)
              })
            }}
          </p>
          <p>
            {{ t('groupBuy.members') }} {{ teamDetail.member_count }} /
            {{ teamDetail.product.max_members }}
          </p>
          <p>
            {{ t('groupBuy.closes') }} {{ exactDate(teamDetail.closes_at) }}
          </p>
          <p>
            {{ t('groupBuy.endedReason') }}:
            {{ t(`groupBuy.${teamDetail.status}`) }}
          </p>
          <button v-if="teamDetail.status === 'recruiting'" class="btn btn-secondary" :disabled="cancelling" @click="openCancellation(teamDetail)">
            {{ t('groupBuy.cancelRecruitment') }}
          </button>
          <div
            v-for="card in teamDetail.cards || []"
            :key="card.id"
            class="rounded-lg border border-gray-100 p-3 dark:border-dark-700"
          >
            <p>
              {{ t('groupBuy.userId') }} {{ card.user_id }} · {{ card.code }} ·
              {{ t(`groupBuy.${card.status}`) }}
            </p>
            <RouterLink
              :to="{
                path: '/admin/orders',
                query: { order_id: card.order_id, order_type: 'month_card' }
              }"
              class="text-primary-600"
            >
              {{ t('groupBuy.order') }} #{{ card.order_id }}
            </RouterLink>
          </div>
        </div>
      </BaseDialog>
      <BaseDialog :show="!!cancelTarget" :title="t('groupBuy.cancelRecruitment')" @close="!cancelling && (cancelTarget = null)">
        <p class="text-sm leading-6">{{ t('groupBuy.cancelRecruitmentHint', { code: cancelTarget?.code || '' }) }}</p>
        <p v-if="cancelError" role="alert" class="mt-3 text-sm text-red-600">{{ cancelError }}</p>
        <template #footer>
          <button class="btn btn-secondary" :disabled="cancelling" @click="cancelTarget = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="cancelling" @click="cancelTeam">{{ t('groupBuy.cancelRecruitment') }}</button>
        </template>
      </BaseDialog>
    </div>
  </AppLayout>
</template>
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import MonthCardCard from './MonthCardCard.vue'
import AllocationTable from './AllocationTable.vue'
import { adminGroupBuyAPI } from '@/api/groupBuy'
import adminAPI from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type {
  GroupBuyProduct,
  GroupBuyTeam,
  MonthCard,
  ChargeAllocation
} from '@/types/groupBuy'
import type { AdminGroup } from '@/types'
import { exactDate, cny, usd } from './model'
import { extractApiErrorMessage } from '@/utils/apiError'
const { t } = useI18n()
const app = useAppStore()
const tab = ref<'products' | 'teams' | 'cards'>('products')
const loading = ref(true)
const error = ref('')
const formError = ref('')
const saving = ref(false)
const cardsLoading = ref(false)
const products = ref<GroupBuyProduct[]>([])
const teams = ref<GroupBuyTeam[]>([])
const groups = ref<AdminGroup[]>([])
const cards = ref<MonthCard[]>([])
const allocations = ref<ChargeAllocation[]>([])
const userId = ref<number | ''>('')
const teamCode = ref('')
const teamDetail = ref<GroupBuyTeam | null>(null)
const cancelTarget = ref<GroupBuyTeam | null>(null)
const cancelling = ref(false)
const cancelError = ref('')
const draft = ref<GroupBuyProduct | null>(null)
const productFields = [
  { key: 'price_cny', label: 'price', min: 0.01, step: 0.01 },
  { key: 'base_quota_usd', label: 'base', min: 0.00000001, step: 0.00000001 },
  { key: 'max_members', label: 'maxMembers', min: 2, step: 1 },
  { key: 'recruitment_hours', label: 'recruitmentHours', min: 1, step: 1 },
  { key: 'sort_order', label: 'sortOrder', min: 0, step: 1 }
] as const
async function load() {
  loading.value = true
  error.value = ''
  try {
    [products.value, teams.value, groups.value] = await Promise.all([
      adminGroupBuyAPI.products(),
      adminGroupBuyAPI.teams(),
      adminAPI.groups.getAll()
    ])
    if (userId.value) await loadCards()
  } catch (err) {
    error.value = extractApiErrorMessage(err, t('groupBuy.loadFailed'))
  } finally {
    loading.value = false
  }
}
async function loadCards() {
  if (!userId.value || !Number.isSafeInteger(userId.value) || userId.value <= 0) return
  cardsLoading.value = true
  error.value = ''
  try {
    [cards.value, allocations.value] = await Promise.all([
      adminGroupBuyAPI.cards(userId.value || undefined),
      adminGroupBuyAPI.allocations(userId.value || undefined)
    ])
  } catch (err) {
    error.value = extractApiErrorMessage(err, t('groupBuy.loadFailed'))
  } finally {
    cardsLoading.value = false
  }
}
function editProduct(product?: GroupBuyProduct) {
  formError.value = ''
  draft.value = product
    ? { ...product, tiers: product.tiers.map((tier) => ({ ...tier })) }
    : {
        id: 0,
        group_id: 0,
        group_name: '',
        platform: '',
        name: '',
        description: '',
        price_cny: 0,
        base_quota_usd: 0,
        tiers: [],
        max_members: 10,
        recruitment_hours: 48,
        for_sale: false,
        sort_order: 0
      }
}
async function saveProduct() {
  if (!draft.value || saving.value) return
  const product = draft.value
  let members = 1
  let quota = product.base_quota_usd
  const validTiers = product.tiers.every((tier) => {
    const valid =
      Number.isInteger(tier.members) &&
      tier.members > members &&
      tier.members <= product.max_members &&
      Number.isFinite(tier.quota_usd) &&
      tier.quota_usd > quota
    members = tier.members
    quota = tier.quota_usd
    return valid
  })
  if (
    !product.name.trim() ||
    !product.group_id ||
    !Number.isFinite(product.price_cny) ||
    product.price_cny <= 0 ||
    !Number.isFinite(product.base_quota_usd) ||
    product.base_quota_usd <= 0 ||
    !validTiers ||
    !Number.isInteger(product.max_members) ||
    product.max_members < 2 ||
    !Number.isInteger(product.recruitment_hours) ||
    product.recruitment_hours < 1
  ) {
    formError.value = t('groupBuy.invalidProduct')
    return
  }
  saving.value = true
  formError.value = ''
  try {
    await adminGroupBuyAPI.saveProduct(product)
    draft.value = null
    app.showSuccess(t('groupBuy.saved'))
    products.value = await adminGroupBuyAPI.products()
  } catch (err) {
    formError.value = extractApiErrorMessage(err, t('groupBuy.loadFailed'))
  } finally {
    saving.value = false
  }
}
async function inspectTeam(code: string) {
  error.value = ''
  try {
    teamDetail.value = await adminGroupBuyAPI.team(code.trim())
  } catch (err) {
    error.value = extractApiErrorMessage(err, t('groupBuy.loadFailed'))
  }
}
function openCancellation(team: GroupBuyTeam) {
  cancelError.value = ''
  cancelTarget.value = team
}
async function cancelTeam() {
  if (!cancelTarget.value || cancelling.value) return
  const code = cancelTarget.value.code
  cancelling.value = true
  cancelError.value = ''
  try {
    const team = await adminGroupBuyAPI.cancelTeam(code)
    teams.value = teams.value.map(item => item.code === code ? team : item)
    if (teamDetail.value?.code === code) teamDetail.value = { ...teamDetail.value, ...team }
    cancelTarget.value = null
    app.showSuccess(t('groupBuy.cancelled'))
  } catch (err) {
    cancelError.value = extractApiErrorMessage(err, t('groupBuy.cancelRecruitmentFailed'))
  } finally { cancelling.value = false }
}
onMounted(load)
</script>
