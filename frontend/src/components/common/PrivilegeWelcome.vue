<template>
  <div
    class="pw"
    :class="[`pw-stage-${phase}`, `pw-effect-${activeEffect}`]"
    role="status"
    aria-live="polite"
    data-testid="privilege-welcome"
    :data-effect="activeEffect"
    :aria-hidden="phase === 'waiting'"
    :inert="phase === 'waiting'"
  >
    <!-- A · 顶部暖金胶囊 -->
    <div v-if="activeEffect === 'capsule'" class="pw-capsule">
      <span class="pw-sheen" aria-hidden="true"></span>
      <svg class="pw-mark" viewBox="0 0 16 16" fill="none" aria-hidden="true">
        <path d="M8 1.5 9.9 6.1 14.5 8 9.9 9.9 8 14.5 6.1 9.9 1.5 8 6.1 6.1 8 1.5Z" fill="currentColor" opacity="0.9" />
      </svg>
      <span class="pw-text">{{ message }}</span>
      <span class="pw-sub">{{ localText('很高兴再次见到您', 'Great to see you again') }}</span>
      <button
        type="button"
        class="pw-close"
        data-testid="privilege-welcome-close"
        :aria-label="localText('关闭欢迎提示', 'Dismiss welcome message')"
        @click="emit('close')"
      >
        <svg viewBox="0 0 12 12" fill="none" aria-hidden="true">
          <path d="M2 2l8 8M10 2l-8 8" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" />
        </svg>
      </button>
    </div>

    <!-- B · 右上角滑入身份卡 -->
    <div v-else-if="activeEffect === 'slide'" class="pw-card">
      <span class="pw-card-accent" aria-hidden="true"></span>
      <svg class="pw-card-mark" viewBox="0 0 20 20" fill="none" aria-hidden="true">
        <rect x="2" y="3.5" width="16" height="13" rx="2.5" stroke="currentColor" stroke-width="1.3" />
        <circle cx="7" cy="9" r="1.8" fill="currentColor" opacity="0.85" />
        <path d="M4.4 13.4c.5-1.5 1.5-2.2 2.6-2.2s2.1.7 2.6 2.2" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" />
        <path d="M12 8h4M12 11h4" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" opacity="0.7" />
      </svg>
      <span class="pw-card-body">
        <span class="pw-text pw-card-text">{{ message }}</span>
        <span class="pw-sub pw-card-sub">{{ localText('很高兴再次见到您', 'Great to see you again') }}</span>
      </span>
      <button
        type="button"
        class="pw-close pw-card-close"
        data-testid="privilege-welcome-close"
        :aria-label="localText('关闭欢迎提示', 'Dismiss welcome message')"
        @click="emit('close')"
      >
        <svg viewBox="0 0 12 12" fill="none" aria-hidden="true">
          <path d="M2 2l8 8M10 2l-8 8" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" />
        </svg>
      </button>
    </div>

    <!-- C · 顶部光环徽章 -->
    <div v-else-if="activeEffect === 'halo'" class="pw-badge-wrap">
      <span class="pw-halo" aria-hidden="true"></span>
      <div class="pw-badge">
        <svg class="pw-badge-mark" viewBox="0 0 16 16" fill="none" aria-hidden="true">
          <path d="M8 1.5 9.9 6.1 14.5 8 9.9 9.9 8 14.5 6.1 9.9 1.5 8 6.1 6.1 8 1.5Z" fill="currentColor" opacity="0.9" />
        </svg>
        <span class="pw-badge-body">
          <span class="pw-text pw-badge-text">{{ message }}</span>
          <span class="pw-sub pw-badge-sub">{{ localText('很高兴再次见到您', 'Great to see you again') }}</span>
        </span>
        <button
          type="button"
          class="pw-close pw-badge-close"
          data-testid="privilege-welcome-close"
          :aria-label="localText('关闭欢迎提示', 'Dismiss welcome message')"
          @click="emit('close')"
        >
          <svg viewBox="0 0 12 12" fill="none" aria-hidden="true">
            <path d="M2 2l8 8M10 2l-8 8" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" />
          </svg>
        </button>
      </div>
    </div>

    <!-- D · 顶部流光横幅 -->
    <div v-else class="pw-banner">
      <span class="pw-ribbon-sheen" aria-hidden="true"></span>
      <span class="pw-banner-edge" aria-hidden="true"></span>
      <svg class="pw-mark pw-banner-mark" viewBox="0 0 16 16" fill="none" aria-hidden="true">
        <path d="M8 1.5 9.9 6.1 14.5 8 9.9 9.9 8 14.5 6.1 9.9 1.5 8 6.1 6.1 8 1.5Z" fill="currentColor" opacity="0.9" />
      </svg>
      <span class="pw-text pw-banner-text">{{ message }}</span>
      <span class="pw-sub pw-banner-sub">{{ localText('很高兴再次见到您', 'Great to see you again') }}</span>
      <button
        type="button"
        class="pw-close pw-banner-close"
        data-testid="privilege-welcome-close"
        :aria-label="localText('关闭欢迎提示', 'Dismiss welcome message')"
        @click="emit('close')"
      >
        <svg viewBox="0 0 12 12" fill="none" aria-hidden="true">
          <path d="M2 2l8 8M10 2l-8 8" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" />
        </svg>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { WelcomePhase } from '@/composables/useWelcomePlayback'
import { normalizeWelcomeEffect, type WelcomeEffect } from '@/utils/welcomeEffects'

const props = defineProps<{
  message: string
  phase: WelcomePhase
  effect?: WelcomeEffect
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { locale } = useI18n()
const localText = (zh: string, en: string) => (locale.value.toLowerCase().startsWith('zh') ? zh : en)

const activeEffect = computed(() => normalizeWelcomeEffect(props.effect))
</script>

<style scoped>
.pw {
  pointer-events: none;
  position: fixed;
  top: 5rem;
  left: 50%;
  z-index: 40;
  max-width: calc(100vw - 2rem);
  transform: translateX(-50%);
  transition-property: opacity, transform;
  transition-timing-function: ease-out;
}

.pw button {
  pointer-events: auto;
}

/* ===== 共用文字与关闭按钮 ===== */
.pw-mark {
  width: 12px;
  height: 12px;
  flex-shrink: 0;
  color: var(--gold-fg);
}

.dark .pw-mark {
  color: var(--gold);
}

.pw-text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.02em;
  color: #6b4a1d;
}

.dark .pw-text {
  color: var(--gold);
}

.pw-sub {
  flex-shrink: 0;
  font-size: 12px;
  color: #a3855a;
}

.dark .pw-sub {
  color: rgba(229, 185, 154, 0.6);
}

.pw-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 20px;
  height: 20px;
  border-radius: 9999px;
  color: var(--gold-fg);
  opacity: 0.65;
  transition:
    opacity 0.15s ease,
    background-color 0.15s ease,
    box-shadow 0.15s ease;
}

.dark .pw-close {
  color: var(--gold);
}

.pw-close svg {
  width: 10px;
  height: 10px;
}

.pw-close:hover {
  opacity: 1;
  background-color: rgba(148, 104, 47, 0.12);
}

.pw-close:focus-visible {
  outline: none;
  opacity: 1;
  box-shadow: 0 0 0 2px rgba(148, 104, 47, 0.45);
}

.dark .pw-close:hover {
  background-color: rgba(229, 185, 154, 0.14);
}

.dark .pw-close:focus-visible {
  box-shadow: 0 0 0 2px rgba(229, 185, 154, 0.5);
}

/* ===== A · 暖金胶囊 ===== */
.pw-capsule {
  position: relative;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  overflow: hidden;
  max-width: 100%;
  padding: 0.375rem 0.625rem 0.375rem 0.875rem;
  border-radius: 9999px;
  border: 1px solid rgba(148, 104, 47, 0.35);
  background: rgba(255, 252, 247, 0.92);
  box-shadow: 0 4px 16px rgba(148, 104, 47, 0.14);
  backdrop-filter: blur(8px);
  white-space: nowrap;
}

.dark .pw-capsule {
  border-color: rgba(229, 185, 154, 0.3);
  background: rgba(30, 24, 18, 0.88);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.3);
}

.pw-capsule .pw-sub {
  padding-left: 0.5rem;
  border-left: 1px solid rgba(148, 104, 47, 0.25);
}

.dark .pw-capsule .pw-sub {
  border-left-color: rgba(229, 185, 154, 0.25);
}

/* 一次性金色高光（仅入场播放一次，结束后保持透明） */
.pw-sheen {
  position: absolute;
  inset: 0;
  border-radius: inherit;
  pointer-events: none;
  opacity: 0;
}

.pw-stage-entering .pw-sheen,
.pw-stage-dwell .pw-sheen {
  animation: pw-sheen 900ms ease-out 1 both;
}

@keyframes pw-sheen {
  0% {
    opacity: 0;
    background: linear-gradient(105deg, transparent 30%, rgba(229, 185, 154, 0.35) 50%, transparent 70%);
    transform: translateX(-30%);
  }
  25% {
    opacity: 1;
  }
  100% {
    opacity: 0;
    background: linear-gradient(105deg, transparent 30%, rgba(229, 185, 154, 0.35) 50%, transparent 70%);
    transform: translateX(30%);
  }
}

/* ===== B · 侧滑身份卡 ===== */
.pw-effect-slide {
  top: 4.5rem;
  right: 1rem;
  left: auto;
  transform: none;
}

.pw-card {
  position: relative;
  display: flex;
  align-items: center;
  gap: 0.625rem;
  max-width: min(20rem, 100%);
  padding: 0.625rem 0.625rem 0.625rem 0.875rem;
  border-radius: 0.875rem;
  border: 1px solid rgba(148, 104, 47, 0.3);
  background: rgba(255, 252, 247, 0.94);
  box-shadow:
    0 10px 30px rgba(148, 104, 47, 0.18),
    0 2px 6px rgba(148, 104, 47, 0.1);
  backdrop-filter: blur(10px);
}

.dark .pw-card {
  border-color: rgba(229, 185, 154, 0.28);
  background: rgba(30, 24, 18, 0.9);
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.4);
}

.pw-card-accent {
  position: absolute;
  top: 0.625rem;
  bottom: 0.625rem;
  left: 0;
  width: 3px;
  border-radius: 9999px;
  background: linear-gradient(180deg, var(--gold), rgba(148, 104, 47, 0.4));
}

.pw-card-mark {
  width: 26px;
  height: 26px;
  flex-shrink: 0;
  color: var(--gold-fg);
}

.dark .pw-card-mark {
  color: var(--gold);
}

.pw-card-body {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 1px;
}

.pw-card-text {
  white-space: nowrap;
}

.pw-card-sub {
  font-size: 11px;
  white-space: nowrap;
}

/* ===== C · 光环徽章 ===== */
.pw-effect-halo {
  top: 4.25rem;
}

.pw-badge-wrap {
  position: relative;
  display: flex;
  justify-content: center;
}

.pw-halo {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 4.5rem;
  height: 4.5rem;
  border-radius: 9999px;
  border: 2px solid rgba(229, 185, 154, 0.7);
  transform: translate(-50%, -50%);
  pointer-events: none;
  opacity: 0;
}

.dark .pw-halo {
  border-color: rgba(229, 185, 154, 0.55);
}

.pw-stage-entering .pw-halo,
.pw-stage-dwell .pw-halo {
  animation: pw-halo 1100ms ease-out 1 both;
}

@keyframes pw-halo {
  0% {
    opacity: 0.9;
    transform: translate(-50%, -50%) scale(0.5);
  }
  100% {
    opacity: 0;
    transform: translate(-50%, -50%) scale(2.4);
  }
}

.pw-badge {
  position: relative;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  max-width: 100%;
  padding: 0.5rem 0.625rem 0.5rem 0.75rem;
  border-radius: 0.875rem;
  border: 1px solid rgba(148, 104, 47, 0.4);
  background: linear-gradient(160deg, rgba(255, 252, 247, 0.96), rgba(250, 240, 224, 0.92));
  box-shadow:
    0 6px 20px rgba(148, 104, 47, 0.2),
    inset 0 1px 0 rgba(255, 255, 255, 0.7);
  backdrop-filter: blur(8px);
  white-space: nowrap;
}

.dark .pw-badge {
  border-color: rgba(229, 185, 154, 0.35);
  background: linear-gradient(160deg, rgba(38, 30, 22, 0.94), rgba(26, 21, 16, 0.92));
  box-shadow:
    0 6px 20px rgba(0, 0, 0, 0.35),
    inset 0 1px 0 rgba(229, 185, 154, 0.12);
}

.pw-stage-entering .pw-badge,
.pw-stage-dwell .pw-badge {
  animation: pw-badge-in 420ms cubic-bezier(0.34, 1.4, 0.64, 1) 1 both;
}

@keyframes pw-badge-in {
  0% {
    opacity: 0;
    transform: scale(0.6);
  }
  100% {
    opacity: 1;
    transform: scale(1);
  }
}

.pw-badge-mark {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
  color: var(--gold-fg);
}

.dark .pw-badge-mark {
  color: var(--gold);
}

.pw-badge-body {
  display: flex;
  min-width: 0;
  flex-direction: column;
}

.pw-badge-text {
  white-space: nowrap;
}

.pw-badge-sub {
  font-size: 11px;
  white-space: nowrap;
}

/* ===== D · 流光横幅 ===== */
.pw-effect-ribbon {
  top: 3.5rem;
}

.pw-banner {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.625rem;
  overflow: hidden;
  max-width: 100%;
  padding: 0.625rem 2rem;
  border-radius: 0.75rem;
  border: 1px solid rgba(148, 104, 47, 0.32);
  background: linear-gradient(90deg, rgba(255, 252, 247, 0.9), rgba(250, 238, 218, 0.94), rgba(255, 252, 247, 0.9));
  box-shadow: 0 6px 20px rgba(148, 104, 47, 0.16);
  backdrop-filter: blur(8px);
  white-space: nowrap;
}

.dark .pw-banner {
  border-color: rgba(229, 185, 154, 0.28);
  background: linear-gradient(90deg, rgba(28, 23, 17, 0.9), rgba(40, 31, 22, 0.94), rgba(28, 23, 17, 0.9));
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.35);
}

.pw-banner-edge {
  position: absolute;
  top: 0;
  right: 0;
  left: 0;
  height: 2px;
  background: linear-gradient(90deg, transparent, var(--gold), transparent);
  opacity: 0.8;
  pointer-events: none;
}

.pw-stage-entering .pw-banner,
.pw-stage-dwell .pw-banner {
  animation: pw-banner-in 450ms ease-out 1 both;
}

@keyframes pw-banner-in {
  0% {
    opacity: 0;
    transform: scaleX(0.35);
  }
  100% {
    opacity: 1;
    transform: scaleX(1);
  }
}

/* 一次性流光扫过（仅入场播放一次，结束后保持透明） */
.pw-ribbon-sheen {
  position: absolute;
  inset: 0;
  pointer-events: none;
  opacity: 0;
}

.pw-stage-entering .pw-ribbon-sheen,
.pw-stage-dwell .pw-ribbon-sheen {
  animation: pw-ribbon-sheen 1000ms ease-out 1 both;
}

@keyframes pw-ribbon-sheen {
  0% {
    opacity: 0;
    background: linear-gradient(100deg, transparent 35%, rgba(255, 255, 255, 0.75) 50%, transparent 65%);
    transform: translateX(-60%);
  }
  20% {
    opacity: 1;
  }
  100% {
    opacity: 0;
    background: linear-gradient(100deg, transparent 35%, rgba(255, 255, 255, 0.75) 50%, transparent 65%);
    transform: translateX(60%);
  }
}

.dark .pw-stage-entering .pw-ribbon-sheen,
.dark .pw-stage-dwell .pw-ribbon-sheen {
  animation-name: pw-ribbon-sheen-dark;
}

@keyframes pw-ribbon-sheen-dark {
  0% {
    opacity: 0;
    background: linear-gradient(100deg, transparent 35%, rgba(229, 185, 154, 0.4) 50%, transparent 65%);
    transform: translateX(-60%);
  }
  20% {
    opacity: 1;
  }
  100% {
    opacity: 0;
    background: linear-gradient(100deg, transparent 35%, rgba(229, 185, 154, 0.4) 50%, transparent 65%);
    transform: translateX(60%);
  }
}

.pw-banner .pw-sub {
  padding-left: 0.625rem;
  border-left: 1px solid rgba(148, 104, 47, 0.25);
}

.dark .pw-banner .pw-sub {
  border-left-color: rgba(229, 185, 154, 0.25);
}

/* ===== 阶段状态 ===== */
/* waiting 保留隐形节点作为 entering 过渡起点 */
.pw-stage-waiting {
  visibility: hidden;
  opacity: 0;
}

.pw-effect-capsule.pw-stage-waiting {
  transform: translate(-50%, -10px);
}

.pw-effect-halo.pw-stage-waiting,
.pw-effect-ribbon.pw-stage-waiting {
  transform: translate(-50%, -6px);
}

.pw-effect-slide.pw-stage-waiting {
  transform: translateX(calc(100% + 1.5rem));
}

.pw-stage-entering {
  transition-duration: 400ms;
}

.pw-stage-entering,
.pw-stage-dwell {
  opacity: 1;
}

.pw-effect-capsule.pw-stage-entering,
.pw-effect-capsule.pw-stage-dwell,
.pw-effect-halo.pw-stage-entering,
.pw-effect-halo.pw-stage-dwell,
.pw-effect-ribbon.pw-stage-entering,
.pw-effect-ribbon.pw-stage-dwell {
  transform: translate(-50%, 0);
}

.pw-effect-slide.pw-stage-entering,
.pw-effect-slide.pw-stage-dwell {
  transform: translateX(0);
}

.pw-stage-exiting {
  transition-duration: 250ms;
  transition-timing-function: ease-in;
  opacity: 0;
}

.pw-effect-capsule.pw-stage-exiting {
  transform: translate(-50%, -4px);
}

.pw-effect-halo.pw-stage-exiting,
.pw-effect-ribbon.pw-stage-exiting {
  transform: translate(-50%, -4px);
}

.pw-effect-slide.pw-stage-exiting {
  transform: translateX(calc(100% + 1.5rem));
}

/* 375px 手机：隐藏副文案，避免溢出 */
@media (max-width: 480px) {
  .pw-sub {
    display: none;
  }

  .pw-effect-slide {
    right: 0.75rem;
    max-width: calc(100vw - 1.5rem);
  }

  .pw-banner {
    padding: 0.5rem 0.875rem;
  }
}

/* 减少动态：不滑动、不高光、不缩放，瞬间出现/消失 */
@media (prefers-reduced-motion: reduce) {
  .pw {
    transition-duration: 1ms !important;
  }

  .pw-effect-capsule.pw-stage-waiting,
  .pw-effect-capsule.pw-stage-exiting,
  .pw-effect-halo.pw-stage-waiting,
  .pw-effect-halo.pw-stage-exiting,
  .pw-effect-ribbon.pw-stage-waiting,
  .pw-effect-ribbon.pw-stage-exiting {
    transform: translateX(-50%);
  }

  .pw-effect-slide.pw-stage-waiting,
  .pw-effect-slide.pw-stage-exiting {
    transform: none;
  }

  .pw-stage-entering .pw-sheen,
  .pw-stage-dwell .pw-sheen,
  .pw-stage-entering .pw-halo,
  .pw-stage-dwell .pw-halo,
  .pw-stage-entering .pw-badge,
  .pw-stage-dwell .pw-badge,
  .pw-stage-entering .pw-banner,
  .pw-stage-dwell .pw-banner,
  .pw-stage-entering .pw-ribbon-sheen,
  .pw-stage-dwell .pw-ribbon-sheen {
    animation: none;
  }
}
</style>
