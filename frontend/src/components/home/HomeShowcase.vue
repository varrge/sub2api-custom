<template>
  <figure class="showcase" :aria-label="t('home.marketing.demo.title')">
    <div class="api-type" aria-hidden="true">API</div>
    <div class="connection-panel">
      <div class="panel-heading">
        <span class="panel-brand">{{ siteName }}</span>
        <span class="panel-caption">{{ t('home.marketing.demo.title') }}</span>
      </div>
      <div
        class="scenarios"
        role="group"
        :aria-label="t('home.marketing.demo.choose')"
      >
        <button
          v-for="scenario in scenarios"
          :key="scenario"
          type="button"
          :aria-pressed="selected === scenario"
          aria-controls="home-demo-content"
          @click="selected = scenario"
        >
          {{ t(`home.marketing.demo.${scenario}.label`) }}
        </button>
      </div>
      <div
        id="home-demo-content"
        class="demo-content"
        aria-live="polite"
        aria-atomic="true"
      >
        <span class="demo-prompt">{{
          t(`home.marketing.demo.${selected}.prompt`)
        }}</span>
        <div class="connection" aria-hidden="true">
          <span></span><i></i><span></span>
        </div>
        <span class="demo-result">{{
          t(`home.marketing.demo.${selected}.result`)
        }}</span>
      </div>
      <div class="panel-footer">
        <span>{{ t('home.marketing.demo.application') }}</span>
        <span class="connection-label">{{
          t('home.marketing.demo.connection')
        }}</span>
        <span>{{ t('home.marketing.demo.model') }}</span>
      </div>
    </div>
    <figcaption>{{ t('home.marketing.demo.caption') }}</figcaption>
  </figure>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

defineProps<{ siteName: string }>()
const { t } = useI18n()
const scenarios = ['code', 'write', 'build'] as const
const selected = ref<(typeof scenarios)[number]>('code')
</script>

<style scoped>
.showcase {
  position: relative;
  width: min(100%, 960px);
  margin: 32px auto 0;
  padding: 110px 30px 0;
}
.showcase::before {
  content: '';
  position: absolute;
  inset: 0;
  background: radial-gradient(ellipse, #b9886130, transparent 66%);
  pointer-events: none;
}
.api-type {
  position: absolute;
  top: -24px;
  left: 0;
  width: 100%;
  color: #b6b6bc;
  background: linear-gradient(
    170deg,
    #f9ece1 8%,
    #bcbec4 30%,
    #37383e 54%,
    #d0c5bc 82%
  );
  background-clip: text;
  -webkit-text-fill-color: transparent;
  font-size: clamp(150px, 23vw, 300px);
  line-height: 1;
  letter-spacing: 0.065em;
  font-weight: 750;
  text-align: center;
  user-select: none;
}
.connection-panel {
  position: relative;
  width: min(100%, 720px);
  margin: 0 auto;
  border: 1px solid #777078;
  border-top-color: #c5a48b;
  border-radius: 26px;
  padding: 28px 32px 22px;
  background: linear-gradient(140deg, #2c2c30, #161619 65%, #202024);
  box-shadow:
    0 34px 70px #0008,
    inset 0 1px 0 #ffffff16;
  transform: rotate(-3deg);
}
.panel-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding-bottom: 19px;
  border-bottom: 1px solid #ffffff17;
}
.panel-brand {
  font-size: 21px;
  font-weight: 650;
  overflow-wrap: anywhere;
  text-align: left;
}
.panel-caption {
  flex-shrink: 0;
  font-size: 12px;
  letter-spacing: 0.07em;
  color: #b2b2bc;
}
.scenarios {
  display: flex;
  gap: 8px;
  margin: 20px 0;
}
.scenarios button {
  min-height: 44px;
  flex: 1;
  border-radius: 24px;
  padding: 8px 12px;
  color: #bdbdc5;
  font-size: 14px;
  transition:
    color 0.2s,
    background 0.2s;
}
.scenarios button:hover {
  color: #fff;
  background: #ffffff0c;
}
.scenarios button[aria-pressed='true'] {
  color: #241b16;
  background: #e5b99a;
}
.scenarios button:focus-visible {
  outline: 2px solid #f5d4bc;
  outline-offset: 4px;
}
.demo-content {
  display: grid;
  grid-template-columns: 1fr 70px 1fr;
  gap: 16px;
  align-items: center;
  min-height: 62px;
  font-size: 15px;
  line-height: 1.6;
}
.demo-prompt {
  text-align: left;
  color: #d3d3d8;
}
.demo-result {
  text-align: right;
  color: #e8bfa1;
}
.connection {
  display: flex;
  align-items: center;
  gap: 6px;
}
.connection span {
  height: 1px;
  background: #b78e71;
  flex: 1;
}
.connection i {
  width: 6px;
  height: 6px;
  background: #e5b99a;
  border-radius: 50%;
  box-shadow: 0 0 16px #e5b99a88;
}
.panel-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-top: 23px;
  padding-top: 15px;
  border-top: 1px solid #ffffff12;
  color: #9999a3;
  font-size: 12px;
}
.connection-label {
  letter-spacing: 0.12em;
}
figcaption {
  position: relative;
  margin: 44px auto 0;
  max-width: 580px;
  color: #a3a3ac;
  font-size: 12px;
  line-height: 1.7;
}
@media (max-width: 600px) {
  .showcase {
    margin-top: 44px;
    padding: 68px 0 0;
  }
  .api-type {
    top: -18px;
    font-size: 155px;
  }
  .connection-panel {
    padding: 20px 18px 17px;
    border-radius: 20px;
    transform: rotate(-2deg);
  }
  .panel-heading {
    gap: 8px;
    padding-bottom: 14px;
  }
  .panel-brand {
    font-size: 17px;
  }
  .panel-caption {
    font-size: 10px;
  }
  .scenarios {
    gap: 4px;
    margin: 14px 0;
  }
  .scenarios button {
    padding: 6px;
    font-size: 12px;
  }
  .demo-content {
    grid-template-columns: 1fr 24px 1fr;
    gap: 10px;
    min-height: 75px;
    font-size: 13px;
  }
  .panel-footer {
    font-size: 10px;
  }
  .connection-label {
    letter-spacing: 0;
  }
  figcaption {
    margin-top: 30px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .scenarios button {
    transition: none;
  }
}
</style>
