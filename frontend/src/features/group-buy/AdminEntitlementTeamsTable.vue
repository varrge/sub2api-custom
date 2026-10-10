<template>
  <section class="gb-panel overflow-hidden p-5 sm:p-6">
    <p class="mb-4 text-xs leading-5 text-gray-500 dark:text-gray-400">
      {{ t('groupBuy.entitlementActiveHint') }}
    </p>
    <p v-if="loading" class="py-10 text-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('common.loading') }}
    </p>
    <p v-else-if="!items.length" class="py-10 text-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('groupBuy.noEntitlementTeams') }}
    </p>
    <div v-else class="-mx-5 overflow-x-auto px-5 sm:-mx-6 sm:px-6">
      <table class="gb-table w-full text-left text-sm text-gray-700 dark:text-gray-300">
        <thead>
          <tr>
            <th scope="col">{{ t('groupBuy.teamProduct') }}</th>
            <th scope="col">{{ t('groupBuy.groupPlatform') }}</th>
            <th scope="col">{{ t('groupBuy.recruitmentStatus') }}</th>
            <th scope="col">{{ t('groupBuy.membersActiveCards') }}</th>
            <th scope="col">{{ t('groupBuy.totalUsage') }}</th>
            <th scope="col">{{ t('groupBuy.weeklyUsage') }}</th>
            <th scope="col">{{ t('groupBuy.latestExpiry') }}</th>
            <th scope="col">{{ t('common.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="team in items" :key="team.id">
            <td>
              <span class="gb-code whitespace-nowrap">{{ team.code }}</span>
              <p class="mt-1 font-medium text-gray-900 dark:text-white">
                {{ team.product.name }}
              </p>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                {{ cny(team.product.price_cny) }}
              </p>
            </td>
            <td>
              <p>{{ team.product.group_name }}</p>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                {{ team.product.platform }}
              </p>
            </td>
            <td>
              <span class="gb-pill" :class="statusPill(team.status)">
                {{ t(`groupBuy.${team.status}`) }}
              </span>
            </td>
            <td>
              <p>{{ team.member_count }} / {{ team.product.max_members }}</p>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                {{ t('groupBuy.activeCards') }}: {{ team.active_cards }}
              </p>
            </td>
            <td>
              <div class="min-w-40">
                <p class="mb-1 whitespace-nowrap text-xs text-gray-500 dark:text-gray-400">
                  {{ usd(team.total_used_usd) }} / {{ usd(team.total_quota_usd) }}
                </p>
                <div class="gb-progress">
                  <div
                    class="gb-progress-fill"
                    :class="{ 'gb-progress-fill-warn': team.total_used_usd >= team.total_quota_usd && team.total_quota_usd > 0 }"
                    :style="{ width: `${progress(team.total_used_usd, team.total_quota_usd)}%` }"
                  />
                </div>
              </div>
            </td>
            <td>
              <div class="min-w-40">
                <p class="mb-1 whitespace-nowrap text-xs text-gray-500 dark:text-gray-400">
                  {{ usd(team.weekly_used_usd) }} / {{ usd(team.weekly_quota_usd) }}
                </p>
                <div class="gb-progress">
                  <div
                    class="gb-progress-fill"
                    :class="{ 'gb-progress-fill-warn': team.weekly_used_usd >= team.weekly_quota_usd && team.weekly_quota_usd > 0 }"
                    :style="{ width: `${progress(team.weekly_used_usd, team.weekly_quota_usd)}%` }"
                  />
                </div>
              </div>
            </td>
            <td class="whitespace-nowrap">
              <template v-if="team.expires_at">{{ exactDate(team.expires_at) }}</template>
              <span v-else class="text-gray-400 dark:text-gray-500">—</span>
            </td>
            <td>
              <button
                type="button"
                class="btn btn-secondary btn-sm whitespace-nowrap"
                @click="emit('inspect', team)"
              >
                {{ t('groupBuy.inspect') }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { AdminTeamEntitlement, GroupBuyTeam } from '@/types/groupBuy'
import { cny, exactDate, progress, usd } from './model'
import './glass.css'

defineOptions({ name: 'AdminEntitlementTeamsTable' })
defineProps<{ items: AdminTeamEntitlement[]; loading: boolean }>()
const emit = defineEmits<{ inspect: [team: AdminTeamEntitlement] }>()
const { t } = useI18n()

function statusPill(status: GroupBuyTeam['status']) {
  if (status === 'recruiting') return 'gb-pill-teal'
  if (status === 'full') return 'gb-pill-sky'
  return 'gb-pill-gray'
}
</script>
