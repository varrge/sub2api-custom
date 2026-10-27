<template>
  <div v-if="isEligible && (currentTreasure || error)" class="fixed bottom-6 left-4 z-50 max-w-[calc(100vw-2rem)] lg:left-[80px] select-none" data-testid="hidden-whale-widget">
    <button
      v-if="currentTreasure"
      type="button"
      @click="captureWhale"
      :disabled="finding"
      class="flex items-center gap-2 rounded-xl bg-gradient-to-br from-[#E2AD64] to-[#B86943] border border-[#C79865]/40 px-3 py-2.5 text-white shadow-xl hover:shadow-2xl transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-[#C79865]"
      data-testid="hidden-whale-button"
      :aria-label="t('activityCampaigns.shared.findWhale')"
    >
      <span class="text-xl animate-bounce-gentle">🐳</span>
      <span class="text-xs font-semibold tracking-wide pr-1">{{ t('activityCampaigns.shared.findWhale') }}</span>
    </button>
    <p v-if="error" role="alert" class="mt-2 rounded-lg bg-white p-2 text-sm text-red-600 shadow dark:bg-dark-900" data-testid="hidden-whale-error">{{ error }}</p>
  </div>

  <!-- Congratulatory Modal -->
  <BaseDialog :show="showSuccessModal" :title="t('activityCampaigns.wave3.congratsTitle')" width="narrow" :z-index="100" @close="closeModal">
    <div class="text-center space-y-4" data-testid="whale-success-modal">
      <div class="text-5xl animate-bounce-gentle py-2">🐳</div>
      <p class="text-sm italic text-gray-600 dark:text-dark-300" data-testid="whale-success-message">
        {{ t('activityCampaigns.shared.foundMessage', { line: funLine }) }}
      </p>
      <button
        type="button"
        @click="closeModal"
        class="btn btn-primary w-full py-2.5 rounded-xl text-sm font-semibold"
        data-testid="whale-success-close"
      >
        {{ t('activityCampaigns.wave3.close') }}
      </button>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { isFeatureFlagEnabled, FeatureFlags } from '@/utils/featureFlags'
import { activityCampaignsAPI, waveThreeID } from '@/api/activityCampaigns'
import BaseDialog from '@/components/common/BaseDialog.vue'

const { t } = useI18n()
const route = useRoute()
const authStore = useAuthStore()

const currentTreasure = ref<{ id: string; name: string; token: string } | null>(null)
const finding = ref(false)
const showSuccessModal = ref(false)
const funLine = ref('')
const error = ref('')

let disposed = false
let contextGeneration = 0
let visitGeneration = 0
const paths = ['/dashboard', '/keys', '/usage', '/group-buy', '/profile']

const isEligible = computed(() => {
  return (
    isFeatureFlagEnabled(FeatureFlags.activityCampaigns) &&
    authStore.isAuthenticated &&
    !!authStore.user?.id &&
    !authStore.isEnterprise &&
    paths.includes(route.path)
  )
})

async function checkRouteChallenge() {
  const generation = contextGeneration
  const request = ++visitGeneration
  currentTreasure.value = null
  if (!isEligible.value) {
    currentTreasure.value = null
    return
  }

  const path = route.path
  const current = () => !disposed && generation === contextGeneration && request === visitGeneration && isEligible.value

  try {
    const res = await activityCampaignsAPI.treasureVisit(path)
    if (!current()) return
    currentTreasure.value = res.treasure || null
  } catch {
    // Silent catch, no toasts or errors for visit failures
    if (!current()) return
    currentTreasure.value = null
  }
}

async function captureWhale() {
  if (!currentTreasure.value || finding.value || !isEligible.value) return
  const { id, token } = currentTreasure.value
  const generation = contextGeneration
  const current = () => !disposed && generation === contextGeneration && isEligible.value
  finding.value = true
  error.value = ''
  try {
    const state = await activityCampaignsAPI.treasureFind(id, token)
    if (!current()) return

    // Extract line
    const match = state.treasures?.find((t) => t.id === id)
    if (!match?.found) throw new Error('Unconfirmed treasure discovery')
    funLine.value = match?.line || ''
    showSuccessModal.value = true
    currentTreasure.value = null
  } catch {
    if (!current()) return
    // A lost response may follow a committed find. Confirm before replacing
    // its token, so the user's discovery is not presented as a failure.
    try {
      const state = await activityCampaignsAPI.get(waveThreeID)
      if (!current()) return
      const found = state.treasures?.find(treasure => treasure.id === id && treasure.found)
      if (found) {
        funLine.value = found.line
        showSuccessModal.value = true
        currentTreasure.value = null
        return
      }
    } catch { /* Refresh the challenge below if confirmation is unavailable. */ }
    if (!current()) return
    error.value = t('activityCampaigns.shared.findFailed')
    // Token expires in 10 minutes; failed token refresh visit then let retry.
    await checkRouteChallenge()
  } finally {
    if (current()) {
      finding.value = false
    }
  }
}

function closeModal() {
  showSuccessModal.value = false
  funLine.value = ''
}

// Watchers
watch(
  () => [route.path, authStore.user?.id, isEligible.value],
  () => {
    contextGeneration++
    visitGeneration++
    currentTreasure.value = null
    finding.value = false
    showSuccessModal.value = false
    funLine.value = ''
    error.value = ''
    void checkRouteChallenge()
  },
  { immediate: true, flush: 'sync' }
)

onUnmounted(() => {
  disposed = true
  contextGeneration++
  visitGeneration++
})
</script>

<style scoped>
@keyframes bounceGentle {
  0%, 100% {
    transform: translateY(0);
  }
  50% {
    transform: translateY(-4px);
  }
}
.animate-bounce-gentle {
  animation: bounceGentle 2s infinite ease-in-out;
}
@media (prefers-reduced-motion: reduce) {
  .animate-bounce-gentle {
    animation: none;
  }
}
</style>
