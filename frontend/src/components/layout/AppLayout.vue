<template>
  <div class="min-h-screen bg-gray-50 dark:bg-dark-950">
    <!-- Sidebar -->
    <AppSidebar />

    <!-- Main Content Area -->
    <div
      class="relative min-h-screen transition-all duration-300"
      :class="[sidebarCollapsed ? 'lg:ml-[72px]' : 'lg:ml-64']"
    >
      <!-- Header -->
      <AppHeader />

      <!-- Canvas Aurora Background (confined strictly under AppHeader and right of AppSidebar) -->
      <div
        class="pointer-events-none fixed inset-0 top-16 right-0 bottom-0 overflow-hidden transition-all duration-300"
        :class="[sidebarCollapsed ? 'lg:left-[72px]' : 'lg:left-64']"
        aria-hidden="true"
      >
        <div class="aurora-blob aurora-blob-1"></div>
        <div class="aurora-blob aurora-blob-2"></div>
        <div class="aurora-blob aurora-blob-3"></div>
      </div>

      <!-- Main Content -->
      <main class="relative z-10 p-4 md:p-6 lg:p-8">
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'

const appStore = useAppStore()
const authStore = useAuthStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isAdmin = computed(() => authStore.user?.role === 'admin')

const { replayTour } = useOnboardingTour({
  storageKey: isAdmin.value ? 'admin_guide' : 'user_guide',
  autoStart: true
})

const onboardingStore = useOnboardingStore()

onMounted(() => {
  onboardingStore.setReplayCallback(replayTour)
})

defineExpose({ replayTour })
</script>

<style scoped>
.aurora-blob {
  position: absolute;
  border-radius: 9999px;
  mix-blend-mode: normal;
  will-change: transform;
  pointer-events: none;
}

/* Light mode: subtle low-saturation teal & ice-blue */
.aurora-blob-1 {
  top: -10%;
  left: 15%;
  width: 38rem;
  height: 38rem;
  background: radial-gradient(circle, rgba(45, 212, 191, 0.16) 0%, rgba(20, 184, 166, 0.06) 50%, transparent 75%);
  filter: blur(60px);
  animation: aurora-drift-1 26s ease-in-out infinite alternate;
}

.aurora-blob-2 {
  top: 25%;
  right: -5%;
  width: 34rem;
  height: 34rem;
  background: radial-gradient(circle, rgba(56, 189, 248, 0.14) 0%, rgba(125, 211, 252, 0.05) 50%, transparent 75%);
  filter: blur(64px);
  animation: aurora-drift-2 22s ease-in-out infinite alternate;
}

.aurora-blob-3 {
  bottom: -15%;
  left: 30%;
  width: 42rem;
  height: 32rem;
  background: radial-gradient(circle, rgba(94, 234, 212, 0.12) 0%, rgba(14, 165, 233, 0.04) 50%, transparent 75%);
  filter: blur(72px);
  animation: aurora-drift-3 28s ease-in-out infinite alternate;
}

/* Dark mode: deep teal & navy palette */
:global(.dark) .aurora-blob-1 {
  background: radial-gradient(circle, rgba(13, 148, 136, 0.20) 0%, rgba(15, 118, 110, 0.08) 50%, transparent 75%);
  filter: blur(64px);
}

:global(.dark) .aurora-blob-2 {
  background: radial-gradient(circle, rgba(30, 58, 138, 0.24) 0%, rgba(14, 116, 144, 0.10) 50%, transparent 75%);
  filter: blur(68px);
}

:global(.dark) .aurora-blob-3 {
  background: radial-gradient(circle, rgba(15, 118, 110, 0.16) 0%, rgba(30, 64, 175, 0.08) 50%, transparent 75%);
  filter: blur(76px);
}

/* Drift animations */
@keyframes aurora-drift-1 {
  0% {
    transform: translate3d(0, 0, 0) scale(1) rotate(0deg);
  }
  50% {
    transform: translate3d(8%, 10%, 0) scale(1.08) rotate(12deg);
  }
  100% {
    transform: translate3d(-6%, 6%, 0) scale(0.95) rotate(-8deg);
  }
}

@keyframes aurora-drift-2 {
  0% {
    transform: translate3d(0, 0, 0) scale(1) rotate(0deg);
  }
  50% {
    transform: translate3d(-10%, -8%, 0) scale(1.10) rotate(-15deg);
  }
  100% {
    transform: translate3d(6%, -12%, 0) scale(0.92) rotate(10deg);
  }
}

@keyframes aurora-drift-3 {
  0% {
    transform: translate3d(0, 0, 0) scale(1);
  }
  50% {
    transform: translate3d(10%, -8%, 0) scale(1.05);
  }
  100% {
    transform: translate3d(-8%, 6%, 0) scale(0.96);
  }
}

/* Respect prefers-reduced-motion: stop animation completely but keep static gradient */
@media (prefers-reduced-motion: reduce) {
  .aurora-blob-1,
  .aurora-blob-2,
  .aurora-blob-3 {
    animation: none !important;
  }
}
</style>
