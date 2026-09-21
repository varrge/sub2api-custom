<template>
  <div
    class="landing"
    :class="{ 'is-dark': isDark }"
    data-testid="marketing-home"
  >
    <a href="#home-main" class="skip-link">{{ t('home.marketing.skip') }}</a>
    <header class="landing-header dark">
      <nav
        class="page-width nav-inner"
        :aria-label="t('home.marketing.navigation')"
      >
        <a href="#home-main" class="brand">
          <img :src="siteLogo || '/logo.svg'" alt="" width="28" height="28" />
          <span>{{ siteName }}</span>
        </a>
        <div class="nav-sections">
          <a href="#home-highlights">{{
            t('home.marketing.nav.highlights')
          }}</a>
          <a href="#home-start">{{ t('home.marketing.nav.start') }}</a>
          <a href="#home-pricing">{{ t('home.marketing.nav.pricing') }}</a>
        </div>
        <div class="nav-actions">
          <LocaleSwitcher />
          <button
            type="button"
            class="theme-button"
            :aria-label="
              isDark ? t('home.switchToLight') : t('home.switchToDark')
            "
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="$emit('toggleTheme')"
          >
            <Icon :name="isDark ? 'sun' : 'moon'" size="sm" />
          </button>
          <router-link :to="entryPath" class="button button-small">
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main id="home-main" tabindex="-1">
      <section class="hero page-width" aria-labelledby="home-title">
        <p class="eyebrow">
          {{ siteSubtitle.trim() || t('home.marketing.eyebrow') }}
        </p>
        <h1 id="home-title">
          {{ t('home.marketing.heroFirst') }}<br /><span>{{
            t('home.marketing.heroSecond')
          }}</span>
        </h1>
        <p class="hero-description">{{ t('home.marketing.description') }}</p>
        <div class="actions hero-actions">
          <router-link
            :to="entryPath"
            class="button"
            data-testid="home-primary-cta"
            >{{ entryLabel }}<span aria-hidden="true">↗</span></router-link
          >
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="text-link"
            >{{ t('home.marketing.browseModels')
            }}<span aria-hidden="true">↗</span></router-link
          >
          <a v-else href="#home-start" class="text-link"
            >{{ t('home.marketing.nav.start')
            }}<span aria-hidden="true">↓</span></a
          >
        </div>
        <HomeShowcase :site-name="siteName" />
        <a href="#home-highlights" class="discover"
          >{{ t('home.marketing.discover')
          }}<span aria-hidden="true">↓</span></a
        >
      </section>

      <section
        id="home-highlights"
        class="highlights section-pad"
        aria-labelledby="highlights-title"
      >
        <div class="page-width">
          <p class="eyebrow accent">
            {{ t('home.marketing.highlights.eyebrow') }}
          </p>
          <h2 id="highlights-title">
            {{ t('home.marketing.highlights.title') }}
          </h2>
          <p class="section-description">
            {{ t('home.marketing.highlights.description') }}
          </p>
          <div class="feature-grid">
            <article class="feature-card">
              <p class="card-label">
                01 / {{ t('home.marketing.models.label') }}
              </p>
              <h3>{{ t('home.marketing.models.title') }}</h3>
              <p>{{ t('home.marketing.models.description') }}</p>
              <div class="model-visual" aria-hidden="true">
                <span>{ }</span><span>✳</span><span>↗</span>
                <div class="model-line"></div>
                <strong>API</strong>
              </div>
              <router-link
                v-if="showModelPlazaEntry"
                to="/model-plaza"
                class="text-link"
                >{{ t('home.marketing.browseModels')
                }}<span aria-hidden="true">↗</span></router-link
              >
            </article>
            <article class="feature-card">
              <p class="card-label">
                02 / {{ t('home.marketing.tools.label') }}
              </p>
              <h3>{{ t('home.marketing.tools.title') }}</h3>
              <p>{{ t('home.marketing.tools.description') }}</p>
              <div class="config-visual" aria-hidden="true">
                <span><i>01</i> API endpoint</span><span><i>02</i> API key</span
                ><span><i>03</i> Model<span class="config-cursor"></span></span>
              </div>
              <a href="#home-start" class="text-link"
                >{{ t('home.marketing.nav.start')
                }}<span aria-hidden="true">↓</span></a
              >
            </article>
            <article class="feature-card">
              <p class="card-label">
                03 / {{ t('home.marketing.usage.label') }}
              </p>
              <h3>{{ t('home.marketing.usage.title') }}</h3>
              <p>{{ t('home.marketing.usage.description') }}</p>
              <div class="usage-visual" aria-hidden="true">
                <span
                  v-for="(height, index) in [28, 54, 40, 76, 59, 95, 69]"
                  :key="index"
                  :style="{ height: `${height}%` }"
                ></span>
              </div>
              <span class="visual-caption">{{
                t('home.marketing.usage.caption')
              }}</span>
            </article>
          </div>
        </div>
      </section>

      <section
        id="home-start"
        class="getting-started section-pad"
        aria-labelledby="start-title"
      >
        <div class="page-width">
          <p class="eyebrow">{{ t('home.marketing.start.eyebrow') }}</p>
          <h2 id="start-title">{{ t('home.marketing.start.title') }}</h2>
          <p class="section-description">
            {{ t('home.marketing.start.description') }}
          </p>
          <ol class="steps">
            <li v-for="(step, index) in ['account', 'key', 'app']" :key="step">
              <span class="step-number">0{{ index + 1 }}</span>
              <h3>{{ t(`home.marketing.start.${step}.title`) }}</h3>
              <p>{{ t(`home.marketing.start.${step}.description`) }}</p>
            </li>
          </ol>
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-link"
            >{{ t('home.viewDocs') }}<span aria-hidden="true">↗</span></a
          >
          <router-link v-else :to="entryPath" class="text-link"
            >{{ entryLabel }}<span aria-hidden="true">↗</span></router-link
          >
        </div>
      </section>

      <section
        id="home-pricing"
        class="pricing section-pad"
        aria-labelledby="pricing-title"
      >
        <div class="page-width pricing-layout">
          <div>
            <p class="eyebrow accent">
              {{ t('home.marketing.pricing.eyebrow') }}
            </p>
            <h2 id="pricing-title">{{ t('home.marketing.pricing.title') }}</h2>
            <p class="section-description">
              {{ t('home.marketing.pricing.description') }}
            </p>
            <div class="actions">
              <router-link
                v-if="showModelPlazaEntry"
                to="/model-plaza"
                class="text-link"
                >{{ t('home.marketing.pricing.models')
                }}<span aria-hidden="true">↗</span></router-link
              >
              <router-link
                v-if="showPurchaseEntry"
                to="/purchase"
                class="button button-outline"
                data-testid="home-purchase-cta"
                >{{ t('home.marketing.pricing.plans')
                }}<span aria-hidden="true">↗</span></router-link
              >
            </div>
          </div>
          <dl class="pricing-facts">
            <div v-for="fact in ['model', 'quota', 'records']" :key="fact">
              <dt>{{ t(`home.marketing.pricing.${fact}.title`) }}</dt>
              <dd>{{ t(`home.marketing.pricing.${fact}.description`) }}</dd>
            </div>
          </dl>
        </div>
      </section>

      <section class="faq-section section-pad" aria-labelledby="faq-title">
        <div class="page-width faq-layout">
          <h2 id="faq-title">{{ t('home.marketing.faq.title') }}</h2>
          <div class="faq-list">
            <details
              v-for="question in ['models', 'tools', 'billing']"
              :key="question"
            >
              <summary>
                {{ t(`home.marketing.faq.${question}.question`)
                }}<span aria-hidden="true">+</span>
              </summary>
              <p>{{ t(`home.marketing.faq.${question}.answer`) }}</p>
            </details>
          </div>
        </div>
      </section>

      <section class="closing page-width" aria-labelledby="closing-title">
        <p class="eyebrow accent">{{ siteName }}</p>
        <h2 id="closing-title">{{ t('home.marketing.closing.title') }}</h2>
        <p class="section-description">
          {{ t('home.marketing.closing.description') }}
        </p>
        <router-link :to="entryPath" class="button"
          >{{ entryLabel }}<span aria-hidden="true">↗</span></router-link
        >
      </section>
    </main>
    <footer class="page-width footer">
      <p>&copy; {{ new Date().getFullYear() }} {{ siteName }}</p>
      <div>
        <a
          v-if="docUrl"
          :href="docUrl"
          target="_blank"
          rel="noopener noreferrer"
          >{{ t('home.docs') }}</a
        >
        <router-link v-if="showModelPlazaEntry" to="/model-plaza">{{
          t('nav.modelPlaza')
        }}</router-link>
        <router-link :to="entryPath">{{
          isAuthenticated ? t('home.dashboard') : t('home.login')
        }}</router-link>
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
const entryPath = computed(() =>
  props.isAuthenticated ? props.dashboardPath : '/login'
)
const entryLabel = computed(() =>
  props.isAuthenticated ? t('home.goToDashboard') : t('home.getStarted')
)
</script>

<style scoped>
.landing {
  --copper: #e5b99a;
  --muted: #a3a3ad;
  --reading-bg: #f5f5f7;
  --reading-text: #222225;
  --reading-muted: #686872;
  --reading-border: #d7d7dc;
  background: #080809;
  color: #f5f5f7;
  font-family:
    -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', sans-serif;
  -webkit-font-smoothing: antialiased;
}
.landing.is-dark {
  --reading-bg: #19191d;
  --reading-text: #f5f5f7;
  --reading-muted: #aeafb8;
  --reading-border: #38383f;
}
.page-width {
  width: min(1200px, calc(100% - 80px));
  margin-inline: auto;
}
.section-pad {
  padding-block: 104px;
}
.landing section,
#home-main {
  scroll-margin-top: 96px;
}
.landing a,
.landing button {
  -webkit-tap-highlight-color: transparent;
}
.landing a:focus-visible,
.landing button:focus-visible,
.landing summary:focus-visible {
  outline: 2px solid currentColor;
  outline-offset: 5px;
  border-radius: 6px;
}
.skip-link {
  position: fixed;
  top: -80px;
  left: 20px;
  z-index: 60;
  padding: 12px 20px;
  color: #1d1714;
  background: var(--copper);
  border-radius: 8px;
}
.skip-link:focus {
  top: 12px;
}
.landing-header {
  position: sticky;
  top: 0;
  z-index: 30;
  background: #080809e8;
  border-bottom: 1px solid #ffffff12;
  backdrop-filter: blur(20px);
}
.nav-inner {
  display: flex;
  align-items: center;
  gap: 28px;
  min-height: 76px;
}
.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  margin-right: auto;
  font-size: 20px;
  font-weight: 650;
  letter-spacing: -0.03em;
}
.brand img {
  flex: 0 0 28px;
  border-radius: 8px;
  object-fit: contain;
}
.brand span {
  max-width: 260px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.nav-sections,
.nav-actions {
  display: flex;
  align-items: center;
  gap: 24px;
}
.nav-sections a {
  display: inline-flex;
  align-items: center;
  min-height: 44px;
  color: #bdbdc6;
  font-size: 13px;
  white-space: nowrap;
}
.nav-sections a:hover {
  color: #fff;
}
.nav-actions {
  gap: 8px;
  flex-shrink: 0;
}
.nav-actions :deep(button) {
  min-height: 44px;
}
.theme-button {
  display: grid;
  place-items: center;
  width: 40px;
  min-height: 44px;
  color: #bdbdc6;
}
.button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 22px;
  min-height: 50px;
  padding: 12px 26px;
  border-radius: 28px;
  background: var(--copper);
  color: #241b16;
  font-size: 15px;
  font-weight: 550;
  transition:
    background 0.2s,
    transform 0.2s;
}
.button:hover {
  background: #f1ccb1;
  transform: translateY(-2px);
}
.button-small {
  min-height: 40px;
  padding: 9px 21px;
  font-size: 13px;
  white-space: nowrap;
}
.button-outline {
  border: 1px solid #655449;
  color: var(--copper);
  background: transparent;
}
.button-outline:hover {
  background: #e5b99a12;
}
.hero {
  text-align: center;
  padding-top: 76px;
  padding-bottom: 32px;
}
.eyebrow {
  color: var(--muted);
  font-size: 13px;
  font-weight: 550;
  letter-spacing: 0.08em;
  line-height: 1.6;
  overflow-wrap: anywhere;
}
.accent {
  color: var(--copper);
}
h1 {
  margin: 25px 0 23px;
  font-size: clamp(44px, 5.6vw, 80px);
  line-height: 1.17;
  font-weight: 650;
  letter-spacing: -0.045em;
  text-wrap: balance;
}
h1 span {
  color: var(--copper);
}
.hero-description {
  max-width: 670px;
  margin: 0 auto;
  color: #b1b1ba;
  font-size: 19px;
  line-height: 1.7;
  text-wrap: balance;
}
.actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 26px;
  margin-top: 32px;
}
.hero-actions {
  justify-content: center;
}
.text-link {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  min-height: 44px;
  color: var(--copper);
  font-size: 14px;
}
.text-link:hover {
  text-decoration: underline;
  text-underline-offset: 5px;
}
.discover {
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 16px;
  margin-top: 25px;
  color: var(--muted);
  font-size: 11px;
  letter-spacing: 0.06em;
}
.discover span {
  font-size: 19px;
}
h2 {
  margin-top: 14px;
  font-size: clamp(32px, 3.7vw, 50px);
  line-height: 1.22;
  font-weight: 650;
  letter-spacing: -0.035em;
  text-wrap: balance;
}
.section-description {
  margin-top: 18px;
  color: var(--muted);
  font-size: 18px;
  line-height: 1.7;
  text-wrap: pretty;
}
.highlights {
  background: #121214;
}
.feature-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 22px;
  margin-top: 48px;
}
.feature-card {
  display: flex;
  flex-direction: column;
  min-width: 0;
  padding: 32px 28px 26px;
  background: #202023;
  border: 1px solid #ffffff04;
  border-radius: 26px;
}
.card-label {
  color: var(--copper);
  font-size: 12px;
  letter-spacing: 0.08em;
}
h3 {
  margin-top: 19px;
  font-size: 25px;
  font-weight: 600;
  line-height: 1.4;
  letter-spacing: -0.025em;
  text-wrap: balance;
}
.feature-card > p:not(.card-label) {
  margin-top: 15px;
  color: #b0b0ba;
  font-size: 14px;
  line-height: 1.8;
}
.feature-card > .text-link,
.visual-caption {
  margin-top: auto;
}
.visual-caption {
  display: flex;
  align-items: center;
  min-height: 44px;
  color: #a3a3ad;
  font-size: 12px;
}
.model-visual {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-wrap: wrap;
  gap: 15px;
  height: 158px;
  margin: 30px 0 18px;
  padding: 5px 20px;
}
.model-visual > span {
  display: grid;
  place-items: center;
  width: 46px;
  height: 46px;
  border: 1px solid #65564d;
  border-radius: 13px;
  color: #dfc6b4;
  font-size: 24px;
}
.model-line {
  flex-basis: 100%;
  height: 24px;
  margin-inline: 45px;
  border: 1px solid #726053;
  border-top: 0;
  border-radius: 0 0 10px 10px;
}
.model-visual strong {
  color: var(--copper);
  font-size: 17px;
  letter-spacing: 0.2em;
}
.config-visual {
  display: flex;
  flex-direction: column;
  gap: 14px;
  margin: 36px 0 20px;
  min-height: 152px;
  padding: 24px 20px;
  background: #121214;
  border: 1px solid #34343b;
  border-radius: 15px;
  color: #bdbdc7;
  font:
    13px ui-monospace,
    monospace;
}
.config-visual i {
  margin-right: 14px;
  color: #757580;
  font-style: normal;
}
.config-visual > span:last-child {
  color: var(--copper);
}
.config-cursor {
  display: inline-block;
  width: 6px;
  height: 12px;
  margin-left: 9px;
  background: var(--copper);
  vertical-align: middle;
}
.usage-visual {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 10px;
  height: 152px;
  margin: 36px 0 20px;
  padding-top: 10px;
  border-bottom: 1px solid #4b4037;
}
.usage-visual span {
  flex: 1;
  background: linear-gradient(#d9b398, #8a6b55);
  border-radius: 5px 5px 0 0;
}
.getting-started,
.faq-section {
  background: var(--reading-bg);
  color: var(--reading-text);
}
.getting-started .eyebrow,
.getting-started .section-description {
  color: var(--reading-muted);
}
.getting-started .text-link {
  color: #8a5230;
}
.is-dark .getting-started .text-link {
  color: var(--copper);
}
.steps {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  margin-block: 52px 36px;
  list-style: none;
}
.steps li {
  padding: 0 36px;
  border-left: 1px solid var(--reading-border);
}
.steps li:first-child {
  border-left: 0;
  padding-left: 0;
}
.steps li:last-child {
  padding-right: 0;
}
.step-number {
  color: #966c4f;
  font-size: 14px;
  font-weight: 600;
}
.is-dark .step-number {
  color: var(--copper);
}
.steps p {
  margin-top: 15px;
  color: var(--reading-muted);
  font-size: 15px;
  line-height: 1.8;
}
.pricing-layout {
  display: grid;
  grid-template-columns: 1.1fr 1fr;
  gap: 100px;
  align-items: start;
}
.pricing-facts > div {
  padding-block: 24px;
  border-bottom: 1px solid #2c2c31;
}
.pricing-facts > div:first-child {
  padding-top: 0;
}
.pricing-facts dt {
  font-size: 19px;
  font-weight: 550;
}
.pricing-facts dd {
  color: var(--muted);
  font-size: 14px;
  line-height: 1.8;
  margin-top: 10px;
}
.faq-layout {
  display: grid;
  grid-template-columns: 1fr 1.5fr;
  gap: 100px;
}
.faq-layout h2 {
  margin-top: 0;
}
.faq-list details {
  border-bottom: 1px solid var(--reading-border);
  padding-block: 22px;
}
.faq-list details:first-child {
  padding-top: 0;
}
.faq-list summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 22px;
  min-height: 44px;
  cursor: pointer;
  font-size: 17px;
  font-weight: 550;
  list-style: none;
}
.faq-list summary::-webkit-details-marker {
  display: none;
}
.faq-list summary span {
  font-size: 25px;
  font-weight: 300;
  transition: transform 0.2s;
}
.faq-list details[open] summary span {
  transform: rotate(45deg);
}
.faq-list details p {
  padding: 10px 32px 3px 0;
  font-size: 15px;
  line-height: 1.9;
  color: var(--reading-muted);
}
.closing {
  text-align: center;
  padding-block: 116px 104px;
}
.closing .button {
  margin-top: 32px;
}
.footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 25px;
  flex-wrap: wrap;
  padding-block: 26px;
  border-top: 1px solid #28282e;
  color: var(--muted);
  font-size: 12px;
  overflow-wrap: anywhere;
}
.footer > div {
  display: flex;
  flex-wrap: wrap;
  gap: 24px;
}
.footer a {
  display: inline-flex;
  align-items: center;
  min-height: 44px;
}
@media (max-width: 1000px) {
  .nav-sections {
    display: none;
  }
  .feature-grid {
    gap: 16px;
  }
  .feature-card {
    padding: 26px 20px;
  }
  .feature-card h3 {
    font-size: 22px;
  }
  .pricing-layout,
  .faq-layout {
    gap: 50px;
  }
}
@media (max-width: 760px) {
  .page-width {
    width: calc(100% - 40px);
  }
  .nav-inner {
    gap: 8px;
    min-height: 68px;
  }
  .brand {
    gap: 7px;
    font-size: 17px;
  }
  .brand img {
    width: 24px;
    height: 24px;
    flex-basis: 24px;
  }
  .nav-actions {
    gap: 2px;
  }
  .button-small {
    padding-inline: 15px;
    min-height: 44px;
  }
  .theme-button {
    width: 34px;
  }
  .hero {
    padding-top: 56px;
    padding-bottom: 16px;
  }
  h1 {
    font-size: clamp(37px, 7.7vw, 56px);
    margin-top: 22px;
  }
  .hero-description {
    font-size: 16px;
    max-width: 480px;
  }
  .hero-actions {
    gap: 12px 24px;
    margin-top: 26px;
  }
  .eyebrow {
    font-size: 12px;
  }
  .section-pad {
    padding-block: 68px;
  }
  .section-description {
    font-size: 16px;
  }
  .feature-grid {
    grid-template-columns: 1fr;
    margin-top: 32px;
    gap: 18px;
  }
  .feature-card {
    padding: 28px;
  }
  .feature-card h3 {
    font-size: 27px;
  }
  .model-visual,
  .config-visual,
  .usage-visual {
    width: 100%;
    max-width: 350px;
    margin-inline: auto;
  }
  .steps {
    grid-template-columns: 1fr;
    gap: 28px;
    margin-top: 34px;
  }
  .steps li,
  .steps li:first-child {
    border-left: 0;
    border-top: 1px solid var(--reading-border);
    padding: 28px 0 0;
  }
  .steps li:first-child {
    border: 0;
    padding-top: 0;
  }
  .steps h3 {
    margin-top: 10px;
  }
  .steps p {
    margin-top: 10px;
  }
  .pricing-layout,
  .faq-layout {
    grid-template-columns: 1fr;
    gap: 36px;
  }
  .closing {
    padding-block: 78px;
  }
  .closing h2 {
    font-size: 34px;
  }
  .footer {
    gap: 8px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .button,
  .faq-list summary span {
    transition: none;
  }
}
</style>
