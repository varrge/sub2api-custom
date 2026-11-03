<template>
  <SharedCreditBadge
    v-if="currentSummary"
    :summary="currentSummary"
    :loading="loading"
    :failed="failed"
    @refresh="refresh"
  />
</template>

<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref } from 'vue'
import { useEventListener } from '@vueuse/core'
import { getSharedCreditSummary, type UserSharedCreditSummary } from '@/api/sharedCredit'
import SharedCreditBadge from './SharedCreditBadge.vue'

const props = defineProps<{ userId: number }>()
const summary = ref<UserSharedCreditSummary | null>(null)
const loading = ref(false)
const failed = ref(false)
const now = ref(Date.now())
let serverOffset = 0
let disposed = false
let suspended = false
let generation = 0
let request: AbortController | undefined
let clockTimer: ReturnType<typeof setInterval> | undefined
let refreshTimer: ReturnType<typeof setInterval> | undefined

const currentSummary = computed(() => {
  const value = summary.value
  if (!value?.visible || !value.ends_at || now.value >= Date.parse(value.ends_at)) return null
  if (value.status === 'scheduled' && (!value.starts_at || now.value >= Date.parse(value.starts_at))) {
    return { ...value, status: Number(value.remaining_usd) > 0 ? 'active' as const : 'exhausted' as const }
  }
  return value
})

async function refresh() {
  if (disposed || suspended || loading.value || document.visibilityState === 'hidden') return
  loading.value = true
  const requestGeneration = generation
  request = new AbortController()
  try {
    const data = await getSharedCreditSummary(request.signal)
    if (disposed || generation !== requestGeneration) return
    serverOffset = Date.parse(data.server_time) - Date.now()
    now.value = Date.now() + serverOffset
    summary.value = data
    failed.value = false
  } catch {
    if (!disposed && generation === requestGeneration) failed.value = true
  } finally {
    if (!disposed && generation === requestGeneration) loading.value = false
  }
}

useEventListener(document, 'visibilitychange', () => {
  now.value = Date.now() + serverOffset
  void refresh()
})
useEventListener(window, 'focus', () => { void refresh() })
useEventListener(window, 'storage', (event) => {
  if (event.key !== null && event.key !== 'auth_user') return
  const userID = (value: string | null) => {
    try { return value ? JSON.parse(value).id : null } catch { return null }
  }
  if (event.key === null || userID(event.oldValue) !== userID(event.newValue)) {
    // Hide a different account's data while the local auth store catches up.
    generation++
    request?.abort()
    summary.value = null
    loading.value = false
    suspended = userID(event.newValue) !== props.userId
    if (!suspended) void refresh()
  }
})
onMounted(() => {
  void refresh()
  clockTimer = setInterval(() => { now.value = Date.now() + serverOffset }, 1000)
  refreshTimer = setInterval(() => { void refresh() }, 60_000)
})
function stop() {
  disposed = true
  request?.abort()
  clearInterval(clockTimer)
  clearInterval(refreshTimer)
}
onBeforeUnmount(stop)
</script>
