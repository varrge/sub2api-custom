<template>
  <div class="landing" :class="{ 'is-dark': isDark }" data-testid="marketing-home">
    <a href="#home-main" class="skip-link">{{ t('home.marketing.skip') }}</a>
    <header class="landing-header">
      <nav class="page-width nav-inner" :aria-label="t('home.marketing.navigation')">
        <a href="#home-main" class="brand">
          <img :src="siteLogo || '/logo.svg'" alt="" width="28" height="28" />
          <span>{{ siteName }}</span>
        </a>
        <div class="nav-links">
          <a href="#home-highlights">{{ t('home.marketing.nav.highlights') }}</a>
          <RouterLink v-if="showModelPlazaEntry" to="/model-plaza">{{ t('nav.modelPlaza') }}</RouterLink>
          <RouterLink v-if="showPurchaseEntry" to="/purchase" data-testid="home-purchase-cta">{{ t('home.marketing.pricing.plans') }}</RouterLink>
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">{{ t('home.docs') }}</a>
        </div>
        <div class="nav-actions">
          <LocaleSwitcher />
          <button type="button" class="theme-button" :aria-label="isDark ? t('home.switchToLight') : t('home.switchToDark')" @click="$emit('toggleTheme')">
            <Icon :name="isDark ? 'sun' : 'moon'" size="sm" />
          </button>
          <RouterLink :to="entryPath" class="button button-entry">{{ isAuthenticated ? t('home.dashboard') : t('home.login') }}</RouterLink>
        </div>
      </nav>
    </header>

    <main id="home-main" tabindex="-1" class="page-width">
      <section class="hero" aria-labelledby="home-title">
        <p class="eyebrow">{{ siteSubtitle.trim() || t('home.marketing.eyebrow') }}</p>
        <h1 id="home-title">{{ t('home.marketing.heroFirst') }}<br />{{ t('home.marketing.heroSecond') }}</h1>
        <p class="hero-description">{{ t('home.marketing.description') }}</p>
        <div class="actions">
          <RouterLink :to="entryPath" class="button button-primary" data-testid="home-primary-cta">{{ entryLabel }} <span aria-hidden="true">↗</span></RouterLink>
          <a href="#home-highlights" class="button button-secondary">{{ t('home.marketing.explore') }} <span aria-hidden="true">↓</span></a>
        </div>
      </section>

      <HomeShowcase :site-name="siteName" />

      <section id="home-highlights" class="features-section" :aria-label="t('home.marketing.nav.highlights')">
        <article class="feature-card feature-models">
          <div>
            <p class="feature-index">01 / {{ t('home.marketing.models.label') }}</p>
            <h2>{{ t('home.marketing.models.title') }}</h2>
            <p class="feature-description">{{ t('home.marketing.models.description') }}</p>
          </div>
          <div class="model-visual" aria-hidden="true">
            <span>{ } &nbsp; ✳ &nbsp; ↗</span>
            <small>CODE / CREATE / BUILD</small>
          </div>
          <RouterLink v-if="showModelPlazaEntry" to="/model-plaza" class="text-link">{{ t('home.marketing.browseModels') }} <span aria-hidden="true">↗</span></RouterLink>
        </article>
        <div class="feature-stack">
          <article class="feature-card">
            <p class="feature-index">02 / {{ t('home.marketing.tools.label') }}</p>
            <h2>{{ t('home.marketing.tools.title') }}</h2>
            <p class="feature-description">{{ t('home.marketing.tools.description') }}</p>
            <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="text-link">{{ t('home.viewDocs') }} <span aria-hidden="true">↗</span></a>
          </article>
          <article class="feature-card">
            <p class="feature-index">03 / {{ t('home.marketing.usage.label') }}</p>
            <h2>{{ t('home.marketing.usage.title') }}</h2>
            <p class="feature-description">{{ t('home.marketing.usage.description') }}</p>
            <RouterLink v-if="showPurchaseEntry" to="/purchase" class="text-link">{{ t('home.marketing.pricing.plans') }} <span aria-hidden="true">↗</span></RouterLink>
          </article>
        </div>
      </section>

      <section class="closing" aria-labelledby="closing-title">
        <p class="eyebrow">{{ t('home.marketing.closing.eyebrow') }}</p>
        <h2 id="closing-title">{{ t('home.marketing.closing.title') }}</h2>
        <p class="closing-description">{{ t('home.marketing.closing.description') }}</p>
        <RouterLink :to="entryPath" class="button button-primary">{{ entryLabel }} <span aria-hidden="true">↗</span></RouterLink>
      </section>
    </main>

    <footer class="footer">
      <div class="page-width footer-content">
        <p>&copy; {{ new Date().getFullYear() }} {{ siteName }}</p>
        <div class="footer-links">
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">{{ t('home.docs') }}</a>
          <RouterLink v-if="showModelPlazaEntry" to="/model-plaza">{{ t('nav.modelPlaza') }}</RouterLink>
          <RouterLink v-if="showPurchaseEntry" to="/purchase">{{ t('home.marketing.pricing.plans') }}</RouterLink>
          <RouterLink :to="entryPath">{{ entryLabel }}</RouterLink>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import HomeShowcase from './HomeShowcase.vue'

const props = defineProps<{
  siteName: string
  siteLogo: string
  siteSubtitle: string
  docUrl: string
  isAuthenticated: boolean
  dashboardPath: string
  showModelPlazaEntry: boolean
  showPurchaseEntry: boolean
  isDark: boolean
}>()
defineEmits<{ toggleTheme: [] }>()
const { t } = useI18n()
const entryPath = computed(() => props.isAuthenticated ? props.dashboardPath : '/login')
const entryLabel = computed(() => props.isAuthenticated ? t('home.goToDashboard') : t('home.getStarted'))
</script>

<style scoped>
.landing {
  --ink: #1d1d1f;
  --muted: #68686e;
  --accent: #785436;
  --border: #dfdfe3;
  --card: #fff;
  --soft: #f5f5f7;
  --header: #ffffffd9;
  --model-surface: linear-gradient(140deg, #e8e1d7, #f7f5f0);
  --glass: linear-gradient(125deg, #ffffffed, #f4f0e9d9);
  --glass-ink: #5e5347;
  --glass-border: #ffffff;
  --glass-shadow: #55422a24;
  --inset: #ffffff80;
  min-height: 100vh;
  background: linear-gradient(#fff, #f6f6f9);
  color: var(--ink);
  color-scheme: light;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', sans-serif;
  overflow-x: clip;
}
.landing.is-dark {
  --ink: #f1eee9;
  --muted: #b2adb7;
  --accent: #e5b99a;
  --border: #37343b;
  --card: #242228;
  --soft: #1b191f;
  --header: #17151bd9;
  --model-surface: linear-gradient(140deg, #36302b, #211e24);
  --glass: linear-gradient(125deg, #343039ed, #211e25ed);
  --glass-ink: #e5d2be;
  --glass-border: #6a5e53;
  --glass-shadow: #00000050;
  --inset: #ffffff0a;
  color-scheme: dark;
  background: linear-gradient(#151319, #1c1920);
}
.page-width { width: min(1200px, calc(100% - 48px)); margin-inline: auto; }
.landing-header { position: sticky; top: 0; z-index: 30; background: var(--header); border-bottom: 1px solid var(--border); backdrop-filter: blur(20px); -webkit-backdrop-filter: blur(20px); }
.nav-inner { min-height: 64px; display: flex; align-items: center; justify-content: space-between; gap: 24px; }
.brand { display: flex; align-items: center; gap: 10px; min-width: 0; font-size: 16px; font-weight: 650; }
.brand img { flex-shrink: 0; object-fit: contain; border-radius: 6px; }
.brand span { max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.nav-links { display: flex; gap: 22px; align-items: center; margin-left: auto; color: var(--muted); font-size: 13px; }
.nav-actions { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }
.theme-button { display: grid; place-items: center; min-width: 40px; min-height: 40px; border-radius: 50%; color: var(--muted); }
.theme-button:hover { background: var(--soft); }
.nav-links a { padding-block: 12px; }
.nav-links a:hover, .footer-links a:hover { color: var(--accent); }
.button { display: inline-flex; align-items: center; justify-content: center; gap: 6px; min-height: 46px; padding: 12px 28px; border-radius: 99px; font-size: 15px; font-weight: 500; line-height: 1.4; text-align: center; transition: background .2s, box-shadow .2s; }
.button-entry { min-height: 40px; padding: 9px 18px; background: var(--ink); color: var(--card); font-size: 13px; }
.button-primary { background: #785436; color: #fff; box-shadow: 0 3px 8px #78543610; }
.button-primary:hover { background: #62452e; box-shadow: 0 6px 16px #78543624; }
.is-dark .button-primary { background: #e5b99a; color: #302217; }
.is-dark .button-primary:hover { background: #f0cdb0; }
.button-secondary { background: var(--soft); border: 1px solid var(--border); color: var(--ink); }
.button-secondary:hover { background: var(--card); }
.hero { padding: 88px 0 28px; text-align: center; }
.eyebrow { max-width: 700px; margin: 0 auto 22px; color: var(--accent); font-size: 11px; font-weight: 650; letter-spacing: .16em; overflow-wrap: anywhere; }
.hero h1 { margin-bottom: 22px; font-size: clamp(42px, 5.4vw, 78px); font-weight: 750; letter-spacing: -.04em; line-height: 1.15; text-wrap: balance; background: linear-gradient(135deg, var(--ink) 30%, var(--muted)); background-clip: text; -webkit-text-fill-color: transparent; }
.hero-description { max-width: 640px; margin: 0 auto 32px; font-size: 18px; line-height: 1.65; white-space: pre-line; color: var(--muted); }
.actions { display: flex; flex-wrap: wrap; align-items: center; justify-content: center; gap: 16px; }
.features-section { display: grid; grid-template-columns: 1fr 1.2fr; gap: 24px; margin-bottom: 80px; scroll-margin-top: 90px; }
.feature-card { min-width: 0; padding: 40px; border-radius: 20px; border: 1px solid var(--border); background: var(--card); box-shadow: 0 4px 20px #00000004; }
.feature-index { color: var(--accent); font-size: 11px; letter-spacing: .14em; margin-bottom: 28px; }
.feature-card h2 { font-size: 30px; line-height: 1.35; font-weight: 650; letter-spacing: -.025em; margin-bottom: 12px; text-wrap: balance; white-space: pre-line; }
.feature-description { font-size: 15px; line-height: 1.75; color: var(--muted); }
.feature-models { display: flex; flex-direction: column; background: var(--model-surface); border-color: transparent; }
.model-visual { display: flex; flex-direction: column; gap: 10px; padding: 30px; margin: auto 0 0; background: var(--inset); border-radius: 14px; }
.model-visual span { font-size: 36px; line-height: 1.4; letter-spacing: -.03em; }
.model-visual small { color: var(--muted); font-family: ui-monospace, monospace; font-size: 11px; }
.feature-models > div:first-child { margin-bottom: 60px; }
.feature-stack { display: flex; flex-direction: column; gap: 24px; }
.text-link { display: inline-flex; align-items: center; gap: 5px; min-height: 40px; margin-top: 18px; font-size: 13px; color: var(--accent); }
.text-link:hover { text-decoration: underline; text-underline-offset: 4px; }
.closing { text-align: center; padding: 80px 0 100px; border-top: 1px solid var(--border); }
.closing h2 { margin-bottom: 18px; font-size: clamp(30px, 4vw, 48px); line-height: 1.3; letter-spacing: -.035em; font-weight: 650; text-wrap: balance; }
.closing-description { max-width: 520px; color: var(--muted); margin: 0 auto 28px; font-size: 16px; line-height: 1.6; }
.footer { border-top: 1px solid var(--border); padding: 28px 0; background: var(--soft); }
.footer-content { display: flex; justify-content: space-between; align-items: center; gap: 24px; color: var(--muted); font-size: 12px; }
.footer-content p { overflow-wrap: anywhere; min-width: 0; }
.footer-links { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 8px 24px; }
.footer-links a { display: inline-flex; align-items: center; min-height: 40px; }
a:focus-visible, button:focus-visible { outline: 2px solid var(--accent); outline-offset: 4px; }
.skip-link { position: absolute; z-index: 40; left: 16px; top: 8px; padding: 12px 18px; border-radius: 8px; background: var(--ink); color: var(--card); transform: translateY(-150%); }
.skip-link:focus { transform: none; }
@media (max-width: 1100px) {
  .nav-links { display: none; }
  .nav-inner { gap: 16px; }
}
@media (max-width: 768px) {
  .page-width { width: calc(100% - 36px); }
  .brand span { max-width: 140px; font-size: 14px; }
  .hero { padding-top: 64px; }
  .hero h1 { font-size: clamp(34px, 6vw, 48px); }
  .hero-description { font-size: 16px; }
  .eyebrow { font-size: 10px; letter-spacing: .1em; }
  .features-section { grid-template-columns: 1fr; gap: 20px; margin-bottom: 56px; }
  .feature-card { padding: 28px; }
  .feature-card h2 { font-size: 27px; }
  .feature-stack { gap: 20px; }
  .feature-models > div:first-child { margin-bottom: 36px; }
  .actions { gap: 10px; }
  .actions .button { padding-inline: 20px; }
  .closing { padding: 64px 0; }
  .footer-content { flex-direction: column; gap: 12px; text-align: center; }
  .footer-links { justify-content: center; }
}
@media (max-width: 400px) {
  .page-width { width: calc(100% - 28px); }
  .nav-inner { gap: 8px; }
  .brand { flex: 1; gap: 6px; }
  .brand span { max-width: 78px; }
  .brand img { width: 24px; height: 24px; }
  .nav-actions { gap: 2px; }
  .button-entry { padding-inline: 12px; }
  .hero h1 { font-size: 34px; }
  .actions .button { padding-inline: 16px; font-size: 14px; }
  .feature-card { padding: 24px; }
}
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after { transition: none !important; }
}
@media (forced-colors: active) {
  .hero h1 { background: none; -webkit-text-fill-color: CanvasText; }
}
</style>
