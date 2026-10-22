<template>
  <div
    class="wep"
    role="radiogroup"
    :aria-label="localText('欢迎动效样式', 'Welcome effect style')"
  >
    <label
      v-for="option in options"
      :key="option.value"
      class="wep-option"
      :class="{ 'wep-option-selected': selected === option.value }"
      :data-effect="option.value"
    >
      <input
        type="radio"
        class="wep-input"
        :name="groupName"
        :value="option.value"
        :checked="selected === option.value"
        :disabled="disabled"
        @change="select(option.value)"
      />
      <span class="wep-card">
        <span class="wep-letter" aria-hidden="true">{{ option.letter }}</span>
        <span class="wep-texts">
          <span class="wep-name">{{ localText(option.zh, option.en) }}</span>
          <span class="wep-desc">{{ localText(option.descZh, option.descEn) }}</span>
        </span>
      </span>
    </label>
  </div>
</template>

<script lang="ts">
let pickerSeq = 0
</script>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { WelcomeEffect } from '@/utils/welcomeEffects'

const props = defineProps<{
  modelValue?: WelcomeEffect
  disabled?: boolean
  name?: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: WelcomeEffect): void
}>()

const { locale } = useI18n()
const localText = (zh: string, en: string) => (locale.value.toLowerCase().startsWith('zh') ? zh : en)

const groupName = props.name ?? `welcome-effect-picker-${++pickerSeq}`

const selected = computed(() => props.modelValue ?? 'capsule')

const options: { value: WelcomeEffect; letter: string; zh: string; en: string; descZh: string; descEn: string }[] = [
  { value: 'capsule', letter: 'A', zh: '暖金胶囊', en: 'Warm-gold capsule',
    descZh: '顶部胶囊轻落，金色高光掠过', descEn: 'A top capsule that settles in with a golden sheen' },
  { value: 'slide', letter: 'B', zh: '侧滑卡片', en: 'Slide-in card',
    descZh: '右上角滑入的精致身份卡', descEn: 'A refined identity card sliding in from the top right' },
  { value: 'halo', letter: 'C', zh: '光环徽章', en: 'Halo badge',
    descZh: '顶部徽章缩放亮相，光环扩散一次', descEn: 'A top badge scaling in with a one-time halo' },
  { value: 'ribbon', letter: 'D', zh: '流光横幅', en: 'Light ribbon',
    descZh: '顶部横幅展开，流光扫过一次', descEn: 'A top banner unfolding with a single light sweep' }
]

function select(value: WelcomeEffect) {
  if (props.disabled) return
  emit('update:modelValue', value)
}
</script>

<style scoped>
.wep {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 0.5rem;
}

.wep-option {
  position: relative;
  cursor: pointer;
}

.wep-input {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  margin: 0;
  opacity: 0;
  cursor: pointer;
}

.wep-input:disabled {
  cursor: not-allowed;
}

.wep-card {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  height: 100%;
  padding: 0.5rem 0.625rem;
  border-radius: 0.625rem;
  border: 1px solid rgba(148, 104, 47, 0.25);
  background: rgba(255, 255, 255, 0.6);
  transition:
    border-color 0.15s ease,
    background-color 0.15s ease,
    box-shadow 0.15s ease;
}

.dark .wep-card {
  border-color: rgba(229, 185, 154, 0.2);
  background: rgba(255, 255, 255, 0.03);
}

.wep-letter {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 1.25rem;
  height: 1.25rem;
  border-radius: 9999px;
  font-size: 11px;
  font-weight: 700;
  color: var(--gold-fg);
  background: rgba(148, 104, 47, 0.1);
}

.dark .wep-letter {
  color: var(--gold);
  background: rgba(229, 185, 154, 0.12);
}

.wep-texts {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 1px;
}

.wep-name {
  font-size: 12px;
  font-weight: 600;
  color: #374151;
}

.dark .wep-name {
  color: #e5e7eb;
}

.wep-desc {
  font-size: 11px;
  line-height: 1.35;
  color: #6b7280;
}

.dark .wep-desc {
  color: #9ca3af;
}

.wep-option-selected .wep-card {
  border-color: rgba(148, 104, 47, 0.6);
  background: rgba(229, 185, 154, 0.12);
  box-shadow: 0 0 0 1px rgba(148, 104, 47, 0.35);
}

.dark .wep-option-selected .wep-card {
  border-color: rgba(229, 185, 154, 0.55);
  background: rgba(229, 185, 154, 0.1);
  box-shadow: 0 0 0 1px rgba(229, 185, 154, 0.35);
}

.wep-option-selected .wep-letter {
  color: #fff;
  background: var(--gold-fg);
}

.dark .wep-option-selected .wep-letter {
  color: #1f2937;
  background: var(--gold);
}

.wep-input:focus-visible + .wep-card {
  box-shadow: 0 0 0 2px rgba(148, 104, 47, 0.45);
}

.dark .wep-input:focus-visible + .wep-card {
  box-shadow: 0 0 0 2px rgba(229, 185, 154, 0.5);
}

.wep-input:disabled + .wep-card {
  opacity: 0.55;
}
</style>
