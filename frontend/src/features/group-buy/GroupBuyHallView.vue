<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-5">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 class="text-2xl font-bold">{{ t('groupBuy.hall') }}</h1>
          <p class="mt-1 text-sm text-gray-500">
            {{ t('groupBuy.activeTeams', { count: activeTeams.length }) }}
          </p>
        </div>
        <button class="btn btn-secondary" :disabled="loading" @click="load">
          {{ t('common.refresh') }}
        </button>
      </div>
      <div
        class="rounded-xl border border-sky-100 bg-sky-50 p-4 text-sm leading-6 text-sky-800 dark:border-sky-900 dark:bg-sky-950/30 dark:text-sky-200"
      >
        <p>{{ t('groupBuy.subtitle') }}</p>
        <p>{{ t('groupBuy.rules') }}</p>
      </div>
      <form class="flex flex-wrap gap-3" @submit.prevent="lookup()">
        <select
          v-model="groupId"
          class="input min-w-40"
          :aria-label="t('groupBuy.group')"
        >
          <option :value="0">{{ t('groupBuy.allGroups') }}</option>
          <option v-for="group in groups" :key="group.id" :value="group.id">
            {{ group.name }}
          </option>
        </select>
        <input
          v-model="code"
          class="input min-w-0 flex-1"
          :placeholder="t('groupBuy.teamCode')"
          :aria-label="t('groupBuy.teamCode')"
        />
        <button class="btn btn-primary" :disabled="!code.trim() || searching">
          {{ t('groupBuy.lookup') }}
        </button>
      </form>
      <p
        v-if="error"
        class="rounded-xl bg-red-50 p-4 text-sm text-red-700 dark:bg-red-950"
        role="alert"
      >
        {{ error }}
      </p>
      <div v-if="loading" class="py-12 text-center" role="status">
        {{ t('common.loading') }}
      </div>
      <template v-else>
        <div v-if="detail" class="space-y-3">
          <button class="text-sm text-primary-600" @click="clearDetail">
            {{ t('groupBuy.browse') }}
          </button>
          <div class="grid gap-5 md:grid-cols-2">
            <TeamCard :team="detail" :now="now" @join="join" />
          </div>
        </div>
        <div v-else-if="filteredTeams.length" class="grid gap-5 md:grid-cols-2">
          <TeamCard
            v-for="team in filteredTeams"
            :key="team.id"
            :team="team"
            :now="now"
            @join="join"
          />
        </div>
        <div v-else-if="!error" class="card p-12 text-center">
          <h2 class="text-lg font-semibold">{{ t('groupBuy.noTeams') }}</h2>
          <p class="my-3 text-sm text-gray-500">
            {{ t('groupBuy.noTeamsHint') }}
          </p>
          <RouterLink to="/purchase?tab=subscription" class="btn btn-primary">
            {{ t('groupBuy.purchase') }}
          </RouterLink>
        </div>
      </template>
    </div>
  </AppLayout>
</template>
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import { groupBuyAPI } from '@/api/groupBuy'
import type { GroupBuyTeam } from '@/types/groupBuy'
import TeamCard from './TeamCard.vue'
const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const teams = ref<GroupBuyTeam[]>([])
const detail = ref<GroupBuyTeam | null>(null)
const groupId = ref(0)
const code = ref('')
const loading = ref(true)
const searching = ref(false)
const error = ref('')
const now = ref(Date.now())
const activeTeams = computed(() =>
  teams.value.filter(
    (team) =>
      team.status === 'recruiting' &&
      Date.parse(team.closes_at) > now.value &&
      team.member_count < team.product.max_members
  )
)
const groups = computed(() =>
  Array.from(
    new Map(
      teams.value.map((team) => [
        team.product.group_id,
        { id: team.product.group_id, name: team.product.group_name }
      ])
    ).values()
  )
)
const filteredTeams = computed(() =>
  activeTeams.value.filter(
    (team) => !groupId.value || team.product.group_id === groupId.value
  )
)
async function load() {
  loading.value = true
  error.value = ''
  try {
    teams.value = await groupBuyAPI.teams()
    if (typeof route.query.team_code === 'string') {
      code.value = route.query.team_code
      await lookup(false)
    }
  } catch {
    error.value = t('groupBuy.loadFailed')
  } finally {
    loading.value = false
  }
}
async function lookup(updateRoute = true) {
  if (!code.value.trim()) return
  searching.value = true
  error.value = ''
  detail.value = null
  try {
    detail.value = await groupBuyAPI.team(code.value.trim())
    if (updateRoute)
      await router.replace({ query: { team_code: detail.value.code } })
  } catch (err: unknown) {
    error.value =
      ((err as { status?: number }).status ?? (err as { response?: { status?: number } }).response?.status) === 404
        ? t('groupBuy.noResults')
        : t('groupBuy.loadFailed')
  } finally {
    searching.value = false
  }
}
function join(team: GroupBuyTeam) {
  router.push({
    path: '/purchase',
    query: { tab: 'subscription', mode: 'join', team_code: team.code }
  })
}
function clearDetail() {
  detail.value = null
  error.value = ''
  router.replace({ query: {} })
}
watch(
  () => route.query.team_code,
  (value) => {
    if (typeof value === 'string' && value !== detail.value?.code) {
      code.value = value
      lookup(false)
    }
  }
)
let tick: ReturnType<typeof setInterval>
let poll: ReturnType<typeof setInterval>
onMounted(() => {
  load()
  tick = setInterval(() => {
    now.value = Date.now()
  }, 1000)
  poll = setInterval(() => {
    if (!loading.value && !searching.value) load()
  }, 30000)
})
onUnmounted(() => {
  clearInterval(tick)
  clearInterval(poll)
})
</script>
