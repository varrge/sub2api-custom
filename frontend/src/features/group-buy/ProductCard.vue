<template>
  <article
    class="group relative flex flex-col justify-between overflow-hidden rounded-2xl border border-slate-200/90 bg-white shadow-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-teal-400/60 hover:shadow-lg dark:border-dark-700/80 dark:bg-dark-800 dark:hover:border-teal-500/50"
  >
    <!-- Top accent bar -->
    <div class="h-1 w-full bg-gradient-to-r from-teal-500 via-emerald-400 to-teal-400 dark:from-teal-400 dark:via-emerald-500 dark:to-teal-500" />

    <div class="flex flex-1 flex-col p-5 sm:p-6">
      <!-- Header meta: Group/Platform badge & Recruitment duration -->
      <div class="flex items-center justify-between gap-2">
        <span
          class="inline-flex items-center gap-1.5 rounded-md bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-700 dark:bg-dark-700 dark:text-slate-300 truncate"
          :title="`${product.group_name} · ${product.platform}`"
        >
          <svg aria-hidden="true" class="h-3.5 w-3.5 shrink-0 text-slate-400 dark:text-dark-400" viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M2 4.75C2 3.784 2.784 3 3.75 3h12.5c.966 0 1.75.784 1.75 1.75v10.5A1.75 1.75 0 0116.25 17H3.75A1.75 1.75 0 012 15.25V4.75zM3.5 6v9.25c0 .138.112.25.25.25h12.5a.25.25 0 00.25-.25V6H3.5z" clip-rule="evenodd" />
          </svg>
          <span class="truncate">{{ product.group_name }} · {{ product.platform }}</span>
        </span>

        <span
          v-if="product.recruitment_hours"
          class="inline-flex shrink-0 items-center gap-1 rounded-md border border-teal-100 bg-teal-50/80 px-2.5 py-1 text-xs font-medium text-teal-700 dark:border-teal-900/60 dark:bg-teal-950/50 dark:text-teal-300"
        >
          <svg aria-hidden="true" class="h-3.5 w-3.5" viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm.75-13a.75.75 0 00-1.5 0v5c0 .414.336.75.75.75h4a.75.75 0 000-1.5h-3.25V5z" clip-rule="evenodd" />
          </svg>
          <span>{{ t('groupBuy.recruitmentHoursFormatted', { hours: product.recruitment_hours }) }}</span>
        </span>
      </div>

      <!-- Title and description -->
      <div class="mt-3">
        <h3
          class="text-lg sm:text-xl font-bold tracking-tight text-slate-900 dark:text-white break-words"
          :title="product.name"
        >
          {{ product.name }}
        </h3>
        <p
          v-if="product.description"
          class="mt-1 text-xs sm:text-sm leading-relaxed text-slate-500 dark:text-dark-400 break-words line-clamp-2"
          :title="product.description"
        >
          {{ product.description }}
        </p>
      </div>

      <!-- Pricing headline -->
      <div class="mt-4 flex flex-wrap items-baseline gap-1">
        <span class="min-w-0 text-3xl font-extrabold tracking-tight text-slate-900 [overflow-wrap:anywhere] dark:text-white">
          {{ cny(product.price_cny) }}
        </span>
        <span class="text-xs font-medium text-slate-500 dark:text-dark-400">
          {{ t('groupBuy.per30Days') }}
        </span>
      </div>

      <!-- Allowance metrics: Base quota & Weekly limit -->
      <div class="mt-4 grid grid-cols-[repeat(auto-fit,minmax(min(100%,8rem),1fr))] gap-3 rounded-xl border border-slate-100 bg-slate-50/80 p-3 text-xs dark:border-dark-700/60 dark:bg-dark-700/40">
        <div class="min-w-0">
          <span class="block text-[11px] font-medium text-slate-500 dark:text-dark-400">
            {{ t('groupBuy.baseQuota') }}
          </span>
          <span class="mt-0.5 block text-base font-bold text-teal-700 [overflow-wrap:anywhere] dark:text-teal-300">
            {{ usd(product.base_quota_usd) }}
          </span>
        </div>
        <div class="min-w-0">
          <span class="block text-[11px] font-medium text-slate-500 dark:text-dark-400">
            {{ t('groupBuy.weeklyQuota') }}
          </span>
          <div class="mt-0.5 flex flex-wrap items-baseline gap-1">
            <span class="min-w-0 text-sm sm:text-base font-semibold text-slate-700 [overflow-wrap:anywhere] dark:text-slate-200">
              {{ usd(weeklyQuotaUsd) }}
            </span>
            <span class="shrink-0 text-[10px] text-slate-400 dark:text-dark-400">
              {{ t('groupBuy.perWeek') }}
            </span>
          </div>
        </div>
      </div>

      <!-- Tier Ladder Section -->
      <div v-if="sortedTiers.length > 0" class="mt-4 flex-1">
        <div class="mb-2 flex items-center justify-between">
          <span class="text-xs font-semibold text-slate-500 dark:text-dark-300">
            {{ t('groupBuy.tierLadderTitle') }}
          </span>
          <span class="text-[11px] text-slate-400 dark:text-dark-400">
            {{ t('groupBuy.tierLadderPerCard') }}
          </span>
        </div>

        <div class="space-y-1.5">
          <!-- Base tier row -->
          <div
            class="flex flex-wrap items-center justify-between gap-x-3 gap-y-1 rounded-lg border border-slate-100 bg-slate-50/50 px-3 py-1.5 text-xs text-slate-600 dark:border-dark-700/60 dark:bg-dark-700/30 dark:text-slate-300"
          >
            <span class="inline-flex items-center gap-1.5 font-medium">
              <span class="h-1.5 w-1.5 rounded-full bg-slate-300 dark:bg-dark-500" />
              {{ t('groupBuy.baseTierLabel') }}
            </span>
            <span class="min-w-0 font-semibold text-slate-700 [overflow-wrap:anywhere] dark:text-slate-200">
              {{ usd(product.base_quota_usd) }}
              <span class="font-normal text-[11px] text-slate-400 dark:text-dark-400">{{ t('groupBuy.perCardSuffix') }}</span>
            </span>
          </div>

          <!-- Configured tiers rows -->
          <div
            v-for="tier in sortedTiers"
            :key="tier.members"
            :class="[
              'flex flex-wrap items-center justify-between gap-x-3 gap-y-1 rounded-lg px-3 py-1.5 text-xs transition-colors',
              isHighestTier(tier)
                ? 'border border-teal-200/80 bg-teal-50/60 text-teal-950 dark:border-teal-800/60 dark:bg-teal-950/40 dark:text-teal-200'
                : 'border border-slate-100 bg-slate-50/50 text-slate-700 dark:border-dark-700/60 dark:bg-dark-700/30 dark:text-slate-300'
            ]"
          >
            <div class="flex flex-wrap items-center gap-1.5">
              <span
                :class="[
                  'h-1.5 w-1.5 rounded-full',
                  isHighestTier(tier) ? 'bg-teal-500 dark:bg-teal-400' : 'bg-slate-300 dark:bg-dark-500'
                ]"
              />
              <span class="font-medium">
                {{ t('groupBuy.tierMembersCount', { count: tier.members }) }}
              </span>
              <span
                v-if="isHighestTier(tier)"
                class="rounded bg-teal-600 px-1.5 py-0.5 text-[10px] font-semibold text-white dark:bg-teal-500 dark:text-dark-900"
              >
                {{ t('groupBuy.maxTierBadge') }}
              </span>
            </div>

            <div class="min-w-0 font-semibold [overflow-wrap:anywhere]">
              <span :class="isHighestTier(tier) ? 'text-teal-700 dark:text-teal-300 font-bold' : 'text-slate-800 dark:text-slate-200'">
                {{ usd(tier.quota_usd) }}
              </span>
              <span class="font-normal text-[11px] text-slate-400 dark:text-dark-400"> {{ t('groupBuy.perCardSuffix') }}</span>
            </div>
          </div>
        </div>
      </div>
      <div v-else class="flex-1" />

      <!-- Action Buttons -->
      <div class="mt-5 grid grid-cols-2 gap-3">
        <!-- Solo Buy Button (Secondary) -->
        <button
          type="button"
          class="flex flex-col items-center justify-center rounded-xl border border-slate-200 bg-white py-2.5 px-3 text-center transition-all hover:bg-slate-50 hover:border-slate-300 active:scale-[0.98] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-slate-400 dark:border-dark-600 dark:bg-dark-800 dark:text-slate-200 dark:hover:bg-dark-700 dark:hover:border-dark-500"
          @click="emit('select', { product, mode: 'solo' })"
        >
          <span class="text-sm font-semibold text-slate-800 dark:text-slate-100">
            {{ t('groupBuy.solo') }}
          </span>
          <span class="mt-0.5 text-[11px] text-slate-400 dark:text-dark-400">
            {{ t('groupBuy.soloActionSub') }}
          </span>
        </button>

        <!-- Create Team Button (Primary CTA) -->
        <button
          type="button"
          class="flex flex-col items-center justify-center rounded-xl bg-teal-600 py-2.5 px-3 text-center text-white shadow-sm transition-all hover:bg-teal-500 active:bg-teal-700 active:scale-[0.98] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-teal-500 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-dark-900"
          @click="emit('select', { product, mode: 'create' })"
        >
          <span class="flex items-center gap-1 text-sm font-bold text-white">
            <svg aria-hidden="true" class="h-4 w-4" viewBox="0 0 20 20" fill="currentColor">
              <path d="M13 6a3 3 0 11-6 0 3 3 0 016 0zM18 8a2 2 0 11-4 0 2 2 0 014 0zM14 15a4 4 0 00-8 0v3h8v-3zM6 8a2 2 0 11-4 0 2 2 0 014 0zM16 18v-3a5.972 5.972 0 00-.75-2.906A3.005 3.005 0 0119 15v3h-3zM4.75 12.094A5.973 5.973 0 004 15v3H1v-3a3 3 0 013.75-2.906z" />
            </svg>
            {{ t('groupBuy.create') }}
          </span>
          <span class="mt-0.5 text-[11px] text-teal-100">
            {{ t('groupBuy.createActionSub') }}
          </span>
        </button>
      </div>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { GroupBuyProduct, GroupBuyTier, MonthCardSelection } from '@/types/groupBuy'
import { cny, usd } from './model'

const props = defineProps<{ product: GroupBuyProduct }>()
const emit = defineEmits<{ select: [selection: MonthCardSelection] }>()
const { t } = useI18n()

const weeklyQuotaUsd = computed(() => props.product.base_quota_usd / 4)

const sortedTiers = computed<GroupBuyTier[]>(() => {
  if (!props.product.tiers || !props.product.tiers.length) return []
  return [...props.product.tiers].sort((a, b) => a.members - b.members)
})

const maxTierMembers = computed(() => {
  if (!sortedTiers.value.length) return 0
  return sortedTiers.value[sortedTiers.value.length - 1].members
})

function isHighestTier(tier: GroupBuyTier) {
  return tier.members === maxTierMembers.value
}
</script>
