<template>
  <main class="auth-entry dark">
    <div class="auth-sky" aria-hidden="true">
      <svg class="auth-constellation" viewBox="0 0 1440 900" preserveAspectRatio="xMidYMid slice">
        <g fill="none" stroke="currentColor" stroke-width="1">
          <path d="M100 200 300 150 500 250 700 180 850 320 1100 200 1250 400 1400 300" />
          <path d="m300 150-50 250 200 50m250-270-100 240m500-220-120 280m420-180-100-150" />
        </g>
        <g fill="currentColor" stroke="none">
          <circle cx="100" cy="200" r="4" /><circle cx="300" cy="150" r="3" />
          <circle cx="500" cy="250" r="5" /><circle cx="700" cy="180" r="5" />
          <circle cx="850" cy="320" r="3" /><circle cx="1100" cy="200" r="5" />
          <circle cx="1250" cy="400" r="4" /><circle cx="600" cy="420" r="4" />
          <circle cx="250" cy="400" r="3" /><circle cx="450" cy="450" r="4" />
        </g>
      </svg>
    </div>

    <div class="auth-scene">
      <section class="auth-story" aria-labelledby="auth-story-title">
        <p class="auth-eyebrow">{{ siteName }} / AI GATEWAY</p>
        <h1 id="auth-story-title">{{ t('auth.scene.headline') }}<br />{{ t('auth.scene.headlineEnd') }}</h1>
        <p class="auth-description">{{ siteSubtitle || t('auth.scene.description', { siteName }) }}</p>
        <ul class="auth-features">
          <li v-for="feature in ['models', 'keys', 'usage']" :key="feature">
            <span aria-hidden="true">✦</span> {{ t(`auth.scene.${feature}`) }}
          </li>
        </ul>
      </section>

      <section class="auth-panel" :aria-label="siteName">
        <div class="auth-content">
          <div class="auth-brand">
            <img :src="siteLogo || '/logo.svg'" :alt="siteName" width="44" height="44" />
          </div>
          <slot />
          <div v-if="$slots.footer" class="auth-footer"><slot name="footer" /></div>
          <p class="auth-copyright">&copy; {{ new Date().getFullYear() }} {{ siteName }}. All rights reserved.</p>
        </div>
      </section>
    </div>
  </main>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{ siteName: string; siteLogo: string; siteSubtitle?: string }>()
const { t } = useI18n()
</script>

<style scoped>
.auth-entry {
  --auth-gap: 12px;
  --auth-control: 42px;
  height: 100dvh;
  color-scheme: dark;
  color: #f4f1f6;
  background: #08070e;
  position: relative;
  isolation: isolate;
}
.auth-sky {
  position: absolute;
  inset: 0;
  overflow: hidden;
  z-index: -1;
  background:
    radial-gradient(ellipse at 88% 12%, #32334f 0%, transparent 55%),
    radial-gradient(ellipse at 58% 64%, #342442 0%, transparent 55%),
    radial-gradient(ellipse at 22% 100%, #3d3235 0%, transparent 45%),
    #08070e;
}
.auth-constellation { width: 100%; height: 100%; color: #e5b99a; }
.auth-constellation g:first-child { opacity: .1; }
.auth-constellation g:last-child { opacity: .35; }
.auth-scene {
  width: min(1440px, 100%);
  height: 100%;
  margin-inline: auto;
  padding: 32px clamp(28px, 6vw, 88px);
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(390px, 480px);
  align-items: center;
  gap: clamp(40px, 7vw, 100px);
}
.auth-eyebrow { color: #e5b99a; font-size: 13px; font-weight: 700; letter-spacing: .14em; overflow-wrap: anywhere; }
.auth-story h1 { margin-top: 20px; font-size: clamp(36px, 3.6vw, 54px); line-height: 1.25; letter-spacing: -.025em; color: #f1e5de; font-weight: 750; }
.auth-description { max-width: 480px; margin-top: 28px; color: #c0b9c7; font-size: 17px; line-height: 1.8; overflow-wrap: anywhere; }
.auth-features { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 36px; }
.auth-features li { border: 1px solid #ffffff12; border-radius: 99px; background: #ffffff06; padding: 8px 14px; color: #e5b99a; font-size: 12px; }
.auth-panel {
  height: min(740px, calc(100dvh - 64px));
  width: 100%;
  padding: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid #e5b99a33;
  border-radius: 20px;
  background: #100f1b8c;
  box-shadow: 0 24px 70px #00000030;
  backdrop-filter: blur(24px);
  -webkit-backdrop-filter: blur(24px);
}
.auth-content { width: 100%; max-width: 360px; flex-shrink: 0; }
.auth-brand { width: 44px; height: 44px; margin: 0 auto 14px; }
.auth-brand img { width: 100%; height: 100%; object-fit: contain; border-radius: 12px; }
.auth-footer { margin-top: 24px; text-align: center; font-size: 13px; line-height: 20px; }
.auth-copyright { margin-top: 16px; color: #9790a3; text-align: center; font-size: 11px; line-height: 16px; overflow-wrap: anywhere; }
.auth-content :deep(.auth-view) { display: flex; flex-direction: column; gap: 16px; }
.auth-content :deep(.auth-form) { display: flex; flex-direction: column; gap: var(--auth-gap); }
.auth-content :deep(.auth-heading h2) { color: #f6f2f8; font-size: 24px; line-height: 30px; }
.auth-content :deep(.auth-heading p) { color: #aca5b9; font-size: 13px; line-height: 18px; margin-top: 6px; }
.auth-content :deep(.input-label) { color: #e0dae8; font-size: 13px; line-height: 16px; margin-bottom: 6px; }
.auth-content :deep(.input) {
  height: var(--auth-control);
  padding-block: 0;
  font-size: 14px;
  border-radius: 10px;
  border-color: #ffffff1c;
  background: #ffffff07;
  color: #f4f1f6;
  box-shadow: none;
}
.auth-content :deep(.input::placeholder) { color: #938b9f; }
.auth-content :deep(.input:focus) { border-color: #e5b99a; box-shadow: 0 0 0 3px #e5b99a20; }
.auth-content :deep(.input.border-green-500) { border-color: #4ade80; }
.auth-content :deep(.input-error), .auth-content :deep(.input.border-red-500) { border-color: #f87171; }
.auth-content :deep(.input-hint) { color: #aca5b9; font-size: 11px; line-height: 15px; margin-top: 5px; }
.auth-content :deep(.btn) { min-height: var(--auth-control); padding: 8px 12px; border-radius: 10px; font-size: 14px; line-height: 20px; transform: none; }
.auth-content :deep(.btn-primary) { background: linear-gradient(110deg, #f2d7c0, #b88a60); color: #382414; border-color: #f2d7c033; box-shadow: 0 6px 20px #e5b99a16; }
.auth-content :deep(.btn-secondary) { background: #ffffff05; color: #d5cddf; border-color: #ffffff20; box-shadow: none; }
.auth-content :deep(a) { color: #e5b99a; }
.auth-content :deep(a:hover) { color: #f4d7c2; }
.auth-content :deep(button:focus-visible), .auth-content :deep(a:focus-visible) { outline: 2px solid #e5b99a; outline-offset: 3px; }
.auth-content :deep(.auth-code-fields) { display: grid; grid-template-columns: repeat(auto-fit, minmax(130px, 1fr)); gap: 12px; }
.auth-content :deep(.auth-code-fields:empty) { display: none; }
.auth-content :deep(.auth-code-fields .input-label) { font-size: 12px; }
.auth-content :deep(.auth-code-fields .input-label span) { font-size: 10px; }
.auth-content :deep(.auth-code-feedback) { font-size: 11px; line-height: 16px; }
.auth-content :deep(.auth-alternative-trigger) { margin-top: 0; min-height: 24px; padding: 0; background: transparent; border: none; font-size: 13px; color: #e5b99a; }
.auth-content :deep(.auth-alternative-trigger:hover) { text-decoration: underline; }
.auth-panel:has(.auth-code-fields) .auth-brand,
.auth-panel:has(.auth-code-fields) .auth-copyright { display: none; }

@media (max-width: 991px) {
  .auth-story { display: none; }
  .auth-scene { display: flex; justify-content: center; padding: 16px; }
  .auth-panel { max-width: 440px; height: min(740px, calc(100dvh - 32px)); padding: 24px; }
  .auth-content :deep(.input) { font-size: 16px; }
}
@media (max-height: 800px) {
  .auth-entry { --auth-gap: 10px; --auth-control: 38px; }
  .auth-panel { padding-block: 20px; }
  .auth-brand { width: 36px; height: 36px; margin-bottom: 10px; }
  .auth-content :deep(.auth-view) { gap: 16px; }
  .auth-content :deep(.auth-heading h2) { font-size: 22px; line-height: 28px; }
  .auth-content :deep(.btn) { padding-block: 6px; }
  .auth-copyright { display: none; }
}
@media (max-height: 680px) {
  .auth-entry { --auth-gap: 8px; --auth-control: 36px; }
  .auth-scene { padding-block: 12px; }
  .auth-panel { height: calc(100dvh - 24px); padding-block: 16px; }
  .auth-brand, .auth-content :deep(.auth-heading p) { display: none; }
  .auth-content :deep(.auth-view) { gap: 12px; }
  .auth-content :deep(.input-label) { margin-bottom: 4px; }
  .auth-content :deep(.btn) { padding-block: 5px; }
  .auth-footer { margin-top: 20px; }
}
@media (max-width: 375px) {
  .auth-scene { padding-inline: 8px; }
  .auth-panel { padding-inline: 12px; border-radius: 16px; }
  .auth-content :deep(.auth-code-fields) { gap: 8px; }
}
@media (max-height: 600px) {
  .auth-entry { --auth-gap: 6px; --auth-control: 34px; }
  .auth-content :deep(.auth-view) { gap: 10px; }
  .auth-content :deep(.btn) { padding-block: 4px; }
  .auth-content :deep(.input-hint) { margin-top: 3px; }
  .auth-footer { margin-top: 16px; }
  .auth-panel:has(.auth-code-fields) { --auth-gap: 4px; }
  .auth-panel:has(.auth-code-fields) :deep(.auth-view) { gap: 8px; }
  .auth-panel:has(.auth-code-fields) :deep(.input-label) { margin-bottom: 3px; }
  .auth-panel:has(.auth-code-fields) :deep(.input-hint) { line-height: 14px; margin-top: 2px; }
  .auth-panel:has(.auth-code-fields) .auth-footer { margin-top: 12px; }
}
.auth-content :deep(.auth-alternative-trigger) { padding: 0; }
</style>
