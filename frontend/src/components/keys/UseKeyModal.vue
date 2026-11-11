<template>
  <BaseDialog
    :show="show"
    :title="t('keys.useKeyModal.title')"
    width="wide"
    @close="emit('close')"
  >
    <div class="space-y-4">
      <div v-if="groups && groups.length > 1" class="space-y-2">
        <label for="key-example-group" class="input-label">{{ t('keys.multiGroup.exampleGroup') }}</label>
        <select id="key-example-group" v-model.number="exampleGroupId" class="input" data-testid="key-example-group">
          <option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }} · {{ group.platform }}</option>
        </select>
        <p class="text-xs text-gray-500">{{ t('keys.multiGroup.exampleHint') }}</p>
      </div>
      <!-- No Group Assigned Warning -->
      <div v-if="!effectivePlatform" class="flex items-start gap-3 p-4 rounded-lg bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800">
        <svg class="w-5 h-5 text-yellow-500 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
        </svg>
        <div>
          <p class="text-sm font-medium text-yellow-800 dark:text-yellow-200">
            {{ t('keys.useKeyModal.noGroupTitle') }}
          </p>
          <p class="text-sm text-yellow-700 dark:text-yellow-300 mt-1">
            {{ t('keys.useKeyModal.noGroupDescription') }}
          </p>
        </div>
      </div>

      <!-- Platform-specific content -->
      <template v-else>
        <!-- Description -->
        <p class="text-sm text-gray-600 dark:text-gray-400">
          {{ platformDescription }}
        </p>

        <!-- Client Tabs -->
        <div v-if="clientTabs.length" class="overflow-x-auto border-b border-gray-200 dark:border-dark-700">
          <nav class="-mb-px flex min-w-max gap-4 sm:gap-6" aria-label="Client">
            <button
              v-for="tab in clientTabs"
              :key="tab.id"
              type="button"
              @click="activeClientTab = tab.id"
              :class="[
                'whitespace-nowrap py-2.5 px-1 border-b-2 font-medium text-sm transition-colors',
                activeClientTab === tab.id
                  ? 'border-primary-500 text-primary-600 dark:text-primary-400'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300'
              ]"
            >
              <span class="flex items-center gap-2">
                <component :is="tab.icon" class="w-4 h-4" />
                {{ tab.label }}
              </span>
            </button>
          </nav>
        </div>

        <!-- Codex Authentication Mode -->
        <div
          v-if="showCodexAuthMode"
          class="rounded-lg border border-gray-200 p-3 dark:border-dark-700"
        >
          <div class="mb-2">
            <p class="text-sm font-medium text-gray-900 dark:text-white">
              {{ t('keys.useKeyModal.openai.authModeTitle') }}
            </p>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t('keys.useKeyModal.openai.authModeDescription') }}
            </p>
          </div>
          <div
            class="grid grid-cols-2 gap-1 rounded-lg bg-gray-100 p-1 dark:bg-dark-700"
            role="radiogroup"
            :aria-label="t('keys.useKeyModal.openai.authModeTitle')"
          >
            <button
              type="button"
              role="radio"
              data-testid="codex-auth-mode-legacy"
              :aria-checked="codexAuthMode === 'legacy'"
              :class="[
                'rounded-md px-3 py-2 text-sm font-medium transition-colors',
                codexAuthMode === 'legacy'
                  ? 'bg-white text-primary-700 shadow-sm dark:bg-dark-800 dark:text-primary-300'
                  : 'text-gray-600 hover:text-gray-900 dark:text-dark-300 dark:hover:text-white'
              ]"
              @click="codexAuthMode = 'legacy'"
            >
              {{ t('keys.useKeyModal.openai.authModeLegacy') }}
            </button>
            <button
              type="button"
              role="radio"
              data-testid="codex-auth-mode-api-key"
              :aria-checked="codexAuthMode === 'api-key'"
              :class="[
                'rounded-md px-3 py-2 text-sm font-medium transition-colors',
                codexAuthMode === 'api-key'
                  ? 'bg-white text-primary-700 shadow-sm dark:bg-dark-800 dark:text-primary-300'
                  : 'text-gray-600 hover:text-gray-900 dark:text-dark-300 dark:hover:text-white'
              ]"
              @click="codexAuthMode = 'api-key'"
            >
              {{ t('keys.useKeyModal.openai.authModeApiKey') }}
            </button>
          </div>
          <div
            v-if="codexAuthMode === 'api-key'"
            data-testid="codex-api-key-restart-notice"
            class="mt-3 flex items-start gap-2 border-l-2 border-amber-400 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-800 dark:border-amber-500 dark:bg-amber-950/30 dark:text-amber-200"
          >
            <Icon name="exclamationCircle" size="sm" class="mt-0.5 flex-shrink-0" />
            <p>{{ t('keys.useKeyModal.openai.authModeApiKeyRestartNotice') }}</p>
          </div>
        </div>

        <!-- OS/Shell Tabs -->
        <div v-if="showShellTabs" class="overflow-x-auto border-b border-gray-200 dark:border-dark-700">
          <nav class="-mb-px flex min-w-max gap-4" aria-label="Tabs">
            <button
              v-for="tab in currentTabs"
              :key="tab.id"
              type="button"
              @click="activeTab = tab.id"
              :class="[
                'whitespace-nowrap py-2.5 px-1 border-b-2 font-medium text-sm transition-colors',
                activeTab === tab.id
                  ? 'border-primary-500 text-primary-600 dark:text-primary-400'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300'
              ]"
            >
              <span class="flex items-center gap-2">
                <component :is="tab.icon" class="w-4 h-4" />
                {{ tab.label }}
              </span>
            </button>
          </nav>
        </div>

        <!-- Code Blocks (Stacked for multi-file platforms) -->
        <div class="space-y-4">
          <div
            v-for="(file, index) in currentFiles"
            :key="index"
            class="relative"
          >
            <!-- File Hint (if exists) -->
            <p v-if="file.hint" class="text-xs text-amber-600 dark:text-amber-400 mb-1.5 flex items-center gap-1">
              <Icon name="exclamationCircle" size="sm" class="flex-shrink-0" />
              {{ file.hint }}
            </p>
            <div class="bg-gray-900 dark:bg-dark-900 rounded-xl overflow-hidden">
              <!-- Code Header -->
              <div class="flex items-center justify-between px-4 py-2 bg-gray-800 dark:bg-dark-800 border-b border-gray-700 dark:border-dark-700">
                <span class="min-w-0 truncate text-xs text-gray-400 font-mono">{{ file.path }}</span>
                <button
                  type="button"
                  @click="copyContent(file.content, index)"
                  class="flex flex-shrink-0 items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-lg transition-colors"
                  :class="copiedIndex === index
                    ? 'bg-green-500/20 text-green-400'
                    : 'bg-gray-700 hover:bg-gray-600 text-gray-300 hover:text-white'"
                >
                  <svg v-if="copiedIndex === index" class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                  </svg>
                  <svg v-else class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M15.666 3.888A2.25 2.25 0 0013.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 01-.75.75H9a.75.75 0 01-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 01-2.25 2.25H6.75A2.25 2.25 0 014.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 011.927-.184" />
                  </svg>
                  {{ copiedIndex === index ? t('keys.useKeyModal.copied') : t('keys.useKeyModal.copy') }}
                </button>
              </div>
              <!-- Code Content -->
              <pre class="p-4 text-sm font-mono text-gray-100 overflow-x-auto"><code v-if="file.highlighted" v-html="file.highlighted"></code><code v-else v-text="file.content"></code></pre>
            </div>
          </div>
        </div>

        <section
          v-if="showCodexModelCatalog"
          data-testid="codex-model-catalog"
          class="overflow-hidden rounded-lg border border-gray-200 bg-gray-50 dark:border-dark-700 dark:bg-dark-800/50"
        >
          <div class="flex flex-col gap-3 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
            <div class="min-w-0">
              <h3 class="text-sm font-medium text-gray-900 dark:text-white">
                {{ t('keys.useKeyModal.codexModelCatalog.title') }}
              </h3>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                {{ t('keys.useKeyModal.codexModelCatalog.description') }}
              </p>
              <p class="mt-1 truncate font-mono text-xs text-gray-700 dark:text-gray-300">
                {{ codexModelCatalogMode === 'remote' ? codexModelCatalogUrl : codexModelCatalogPath }}
              </p>
              <select
                v-model="codexModelCatalogMode"
                data-testid="codex-model-catalog-mode"
                :aria-label="t('keys.useKeyModal.codexModelCatalog.mode')"
                class="input mt-2 text-sm"
              >
                <option value="remote" :disabled="codexModelCatalogOversized">{{ t('keys.useKeyModal.codexModelCatalog.remote') }}</option>
                <option value="file">{{ t('keys.useKeyModal.codexModelCatalog.local') }}</option>
              </select>
              <p v-if="codexModelCatalogOversized" class="mt-2 text-xs text-amber-700 dark:text-amber-300">
                {{ t('keys.useKeyModal.codexModelCatalog.oversized') }}
              </p>
            </div>
            <button
              v-if="codexModelManifestState === 'ready'"
              type="button"
              class="btn btn-primary min-h-9 flex-shrink-0 px-3 text-xs"
              @click="downloadCodexModelManifest"
            >
              <Icon name="download" size="sm" class="mr-1.5" />
              {{ t('keys.useKeyModal.codexModelCatalog.download') }}
            </button>
            <button
              v-else
              type="button"
              data-testid="codex-model-catalog-fetch"
              class="btn btn-primary min-h-9 flex-shrink-0 px-3 text-xs"
              :disabled="codexModelManifestState === 'loading' || !apiKey"
              @click="loadCodexModelManifest"
            >
              <Icon
                name="refresh"
                size="sm"
                class="mr-1.5"
                :class="codexModelManifestState === 'loading' ? 'animate-spin' : ''"
              />
              {{ codexModelManifestState === 'error'
                ? t('keys.useKeyModal.codexModelCatalog.retry')
                : t('keys.useKeyModal.codexModelCatalog.fetch') }}
            </button>
          </div>
          <p
            v-if="codexModelManifestState === 'ready'"
            class="border-t border-gray-200 px-4 py-2 text-xs text-emerald-700 dark:border-dark-700 dark:text-emerald-300"
          >
            {{ t('keys.useKeyModal.codexModelCatalog.modelsCount', { count: codexModelManifestModelCount }) }}
          </p>
          <p
            v-else-if="codexModelManifestState === 'error'"
            class="border-t border-red-200 px-4 py-2 text-xs text-red-700 dark:border-red-900 dark:text-red-300"
          >
            {{ t('keys.useKeyModal.codexModelCatalog.errorDescription') }}
          </p>
        </section>

        <!-- Usage Note -->
        <div v-if="showPlatformNote" class="flex items-start gap-3 p-3 rounded-lg bg-blue-50 dark:bg-blue-900/20 border border-blue-100 dark:border-blue-800">
          <Icon name="infoCircle" size="md" class="text-blue-500 flex-shrink-0 mt-0.5" />
          <p class="text-sm text-blue-700 dark:text-blue-300">
            {{ platformNote }}
          </p>
        </div>
      </template>
    </div>

    <template #footer>
      <div class="flex items-center justify-between gap-3">
        <router-link v-if="tutorialEnabled" :to="{ path: '/tutorial', query: keyId ? { key_id: keyId } : {} }" class="text-sm text-primary-700 underline dark:text-primary-300" @click="emit('close')">{{ t('tutorial.title') }}</router-link>
        <button
          @click="emit('close')"
          class="btn btn-secondary"
        >
          {{ t('common.close') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useKeyConfig, type KeyConfigOptions } from '@/composables/useKeyConfig'
const props = defineProps<KeyConfigOptions>()
const emit = defineEmits<{ (e: 'close'): void }>()
const { t } = useI18n()
const tutorialEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.tutorial))
const { exampleGroupId, effectivePlatform, platformDescription, clientTabs, activeClientTab, showCodexAuthMode, codexAuthMode, showShellTabs, currentTabs, activeTab, currentFiles, copyContent, copiedIndex, showCodexModelCatalog, codexModelCatalogMode, codexModelCatalogUrl, codexModelCatalogPath, codexModelCatalogOversized, codexModelManifestState, codexModelManifestModelCount, downloadCodexModelManifest, loadCodexModelManifest, showPlatformNote, platformNote } = useKeyConfig(props)
</script>
