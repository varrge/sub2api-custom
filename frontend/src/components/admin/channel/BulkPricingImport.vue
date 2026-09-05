<template>
  <button type="button" class="btn btn-secondary shrink-0 whitespace-nowrap" @click="open">
    <Icon name="download" size="sm" class="mr-1.5" />
    {{ t('admin.channels.bulkPricing.open') }}
  </button>
  <BaseDialog :show="show" :title="t('admin.channels.bulkPricing.title')" width="wide" :z-index="60" @close="close">
    <p class="mb-4 text-sm leading-6 text-gray-500 dark:text-dark-300">{{ t('admin.channels.bulkPricing.hint') }}</p>
    <div v-if="platform === 'composite'" class="mb-4">
      <label class="input-label" for="bulk-pricing-platform">{{ t('admin.channels.bulkPricing.platform') }}</label>
      <select id="bulk-pricing-platform" v-model="catalogPlatform" class="input" @change="loadCatalog">
        <option v-for="provider in providers" :key="provider" :value="provider">{{ provider }}</option>
      </select>
    </div>
    <div class="mb-3 flex flex-wrap items-center gap-3">
      <input v-model="search" type="search" class="input min-w-0 flex-1" :aria-label="t('admin.channels.bulkPricing.search')" :placeholder="t('admin.channels.bulkPricing.search')" />
      <button type="button" class="btn btn-secondary" :disabled="loading || !visibleRows.length" @click="selectVisible">
        {{ t('admin.channels.bulkPricing.selectVisible') }}
      </button>
      <button type="button" class="text-sm text-gray-500 hover:text-primary-600" :disabled="loading" @click="selected = []">
        {{ t('admin.channels.bulkPricing.clear') }}
      </button>
    </div>
    <p v-if="loading" role="status" class="py-6 text-center text-sm text-gray-500">
      {{ t('admin.channels.bulkPricing.loading', { loaded, total }) }}
    </p>
    <div v-else-if="error" role="alert" class="rounded-xl bg-red-50 p-4 text-sm text-red-600 dark:bg-red-900/20">
      {{ error }}
      <button type="button" class="ml-2 underline" @click="loadCatalog">{{ t('admin.channels.bulkPricing.retry') }}</button>
    </div>
    <div v-else class="max-h-[45vh] overflow-auto rounded-xl border border-gray-200 dark:border-dark-600">
      <table v-if="visibleRows.length" class="w-full text-left text-sm">
        <thead class="sticky top-0 bg-gray-50 text-xs text-gray-500 dark:bg-dark-800 dark:text-dark-300">
          <tr>
            <th class="p-3">{{ t('admin.channels.bulkPricing.model') }}</th>
            <th class="whitespace-nowrap p-3 text-right">{{ t('admin.channels.bulkPricing.input') }}</th>
            <th class="whitespace-nowrap p-3 text-right">{{ t('admin.channels.bulkPricing.output') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
          <tr v-for="row in visibleRows" :key="row.model">
            <td class="p-3">
              <label class="flex cursor-pointer items-start gap-2.5">
                <input v-model="selected" type="checkbox" :value="row.model" class="mt-0.5 shrink-0" />
                <span class="break-all font-medium text-gray-800 dark:text-gray-100">{{ row.model }}</span>
              </label>
            </td>
            <td class="whitespace-nowrap p-3 text-right tabular-nums">{{ formatPrice(row.pricing.input_price) }}</td>
            <td class="whitespace-nowrap p-3 text-right tabular-nums">{{ formatPrice(row.pricing.output_price) }}</td>
          </tr>
        </tbody>
      </table>
      <p v-else class="p-8 text-center text-sm text-gray-500">{{ t('admin.channels.bulkPricing.empty') }}</p>
    </div>
    <p v-if="!loading && unavailable" class="mt-3 text-xs text-amber-600 dark:text-amber-400">
      {{ t('admin.channels.bulkPricing.unavailable', { count: unavailable }) }}
    </p>
    <template #footer>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <span class="text-xs text-gray-500">{{ t('admin.channels.bulkPricing.unit') }}</span>
        <div class="flex gap-2">
          <button type="button" class="btn btn-secondary" @click="close">{{ t('common.cancel') }}</button>
          <button type="button" class="btn btn-primary" :disabled="loading || !!error || !selected.length" @click="apply">
            {{ t('admin.channels.bulkPricing.apply', { count: selected.length }) }}
          </button>
        </div>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import channelsAPI from '@/api/admin/channels'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { canImportDefaultPricing, hasDefaultTokenPricing, mergeDefaultPricing, type DefaultPricingRow } from './bulk-pricing'
import { perTokenToMTok, type PricingFormEntry } from './types'

const props = defineProps<{ platform: string; entries: PricingFormEntry[] }>()
const emit = defineEmits<{ update: [entries: PricingFormEntry[]] }>()
const { t } = useI18n()
const appStore = useAppStore()
const providers = ['anthropic', 'openai', 'gemini', 'grok', 'deepseek', 'kimi', 'zhipu']
const show = ref(false)
const loading = ref(false)
const catalogPlatform = ref('anthropic')
const search = ref('')
const error = ref('')
const rows = ref<DefaultPricingRow[]>([])
const selected = ref<string[]>([])
const loaded = ref(0)
const total = ref(0)
const unavailable = ref(0)
let request = 0

const visibleRows = computed(() => rows.value.filter(row =>
  row.model.toLowerCase().includes(search.value.trim().toLowerCase()) && canImportDefaultPricing(row.model, props.entries)
))

function open() {
  show.value = true
  search.value = ''
  catalogPlatform.value = props.platform === 'composite' ? 'anthropic' : props.platform
  void loadCatalog()
}

function close() {
  request++
  show.value = false
  loading.value = false
}

watch(() => props.platform, close)
onBeforeUnmount(() => { request++ })

async function loadCatalog() {
  const current = ++request
  loading.value = true
  rows.value = []
  selected.value = []
  error.value = ''
  loaded.value = 0
  total.value = 0
  unavailable.value = 0
  try {
    const catalog = await channelsAPI.syncPricingModels(catalogPlatform.value)
    if (current !== request) return
    const names = [...new Set(catalog.models)].filter(name => canImportDefaultPricing(name, props.entries))
    total.value = names.length
    // Bound requests to the existing read-only endpoint; closing cancels subsequent batches.
    for (let offset = 0; offset < names.length; offset += 6) {
      const batch = names.slice(offset, offset + 6)
      const results = await Promise.allSettled(batch.map(model => channelsAPI.getModelDefaultPricing(model)))
      if (current !== request) return
      results.forEach((result, index) => {
        if (result.status === 'fulfilled' && hasDefaultTokenPricing(result.value)) {
          rows.value.push({ model: batch[index], pricing: result.value })
        } else {
          unavailable.value++
        }
      })
      loaded.value += batch.length
    }
  } catch {
    if (current === request) error.value = t('admin.channels.bulkPricing.failed')
  } finally {
    if (current === request) loading.value = false
  }
}

function selectVisible() {
  selected.value = [...new Set([...selected.value, ...visibleRows.value.map(row => row.model)])]
}

function apply() {
  if (loading.value || error.value) return
  const result = mergeDefaultPricing(props.entries, rows.value.filter(row => selected.value.includes(row.model)))
  if (result.count) {
    emit('update', result.entries)
    appStore.showSuccess(t('admin.channels.bulkPricing.success', { count: result.count }))
  }
  close()
}

function formatPrice(price?: number) {
  if (price == null) return '—'
  return `$${perTokenToMTok(price)}`
}
</script>
