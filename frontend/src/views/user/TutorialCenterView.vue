<template>
  <AppLayout>
    <div class="mx-auto max-w-[1440px] space-y-5">
      <header class="flex flex-wrap items-center justify-between gap-4">
        <div>
          <p class="text-xs font-semibold uppercase tracking-widest text-primary-700 dark:text-primary-300">{{ t('tutorial.eyebrow') }}</p>
          <h1 class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{{ t('tutorial.title') }}</h1>
          <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ t('tutorial.subtitle') }}</p>
        </div>
        <button class="btn btn-secondary gap-2" :disabled="!d.ready" @click="d.share">
          <Icon :name="d.copied === 'share' ? 'check' : 'link'" size="sm" />{{ t(d.copied === 'share' ? 'tutorial.copied' : 'tutorial.share') }}
        </button>
      </header>
      <div v-if="d.loading" role="status" class="panel p-8 text-center text-gray-500">{{ t('common.loading') }}</div>
      <div v-else-if="d.error" role="alert" class="panel space-y-3 p-6">
        <p class="text-red-600 dark:text-red-400">{{ d.error }}</p><button class="btn btn-secondary" @click="d.load">{{ t('tutorial.retry') }}</button>
      </div>
      <div v-else class="grid min-w-0 grid-cols-1 items-start gap-5 lg:grid-cols-12">
        <aside class="panel min-w-0 divide-y divide-gray-200 dark:divide-gray-800 lg:col-span-4">
          <section class="space-y-4 p-5">
            <div class="flex items-center gap-2 text-sm font-semibold"><Icon name="cog" size="sm" />{{ t('tutorial.configuration') }}</div>
            <div>
              <label for="tutorial-key" class="field-label">{{ t('tutorial.key') }}</label>
              <select id="tutorial-key" v-model="d.selectedKeyId" class="input" @change="d.selectedKeyId === null && d.loadKey()">
                <option :value="null">{{ t('tutorial.placeholder') }}</option>
                <option v-for="key in d.keys" :key="key.id" :value="key.id" :disabled="!tutorialKeyUsable(key)">{{ key.name }}{{ tutorialKeyUsable(key) ? '' : ' · ' + t('tutorial.unavailable') }}</option>
              </select>
              <p v-if="d.keyLoading" role="status" class="help-text">{{ t('common.loading') }}</p>
              <div v-else-if="d.keyError" role="alert" class="mt-2 text-xs text-red-600"><p>{{ d.keyError }}</p><button class="underline" @click="d.loadKey">{{ t('tutorial.retry') }}</button></div>
              <label v-if="d.selectedKey" class="mt-3 flex items-center gap-2 text-xs text-gray-600 dark:text-gray-300">
                <input v-model="d.revealKey" type="checkbox" class="rounded border-gray-300" />{{ t('tutorial.insertKey') }}
              </label>
              <p class="help-text">{{ t(d.revealKey ? 'tutorial.secretHint' : 'tutorial.placeholderHint') }}</p>
              <router-link v-if="!d.keys.length" to="/keys" class="text-xs text-primary-700 underline dark:text-primary-300">{{ t('tutorial.createKey') }}</router-link>
            </div>
            <div>
              <span class="field-label">{{ t('tutorial.platform') }}</span>
              <div class="grid grid-cols-2 gap-2">
                <button v-for="platform in d.platforms" :key="platform" type="button" :aria-pressed="d.platform === platform" class="choice truncate" :class="{ selected: d.platform === platform }" @click="d.platform = platform">{{ platform }}</button>
              </div>
              <p v-if="!d.platforms.length && !d.keyLoading" class="help-text">{{ t('tutorial.noGroups') }}</p>
            </div>
            <div>
              <label for="tutorial-group" class="field-label">{{ t('tutorial.group') }}</label>
              <select id="tutorial-group" v-model="d.groupID" class="input" :disabled="!d.groups.length"><option v-for="g in d.groups" :key="g.id" :value="g.id">{{ g.name }}</option></select>
              <p class="help-text">{{ t('tutorial.groupHint') }}</p>
            </div>
          </section>
          <section class="space-y-4 p-5">
            <div>
              <span class="field-label">{{ t('tutorial.client') }}</span>
              <div class="flex flex-wrap gap-2">
                <button v-for="client in d.config.clientTabs" :key="client.id" class="choice" :class="{ selected: d.config.activeClientTab === client.id }" :aria-pressed="d.config.activeClientTab === client.id" @click="d.config.activeClientTab = client.id">{{ client.label }}</button>
              </div>
            </div>
            <div v-if="d.config.showShellTabs">
              <span class="field-label">{{ t('tutorial.os') }}</span>
              <div class="flex flex-wrap gap-2"><button v-for="os in d.config.currentTabs" :key="os.id" class="choice" :class="{ selected: d.config.activeTab === os.id }" :aria-pressed="d.config.activeTab === os.id" @click="d.config.activeTab = os.id">{{ os.label }}</button></div>
            </div>
            <div>
              <label for="tutorial-model" class="field-label">{{ t('tutorial.model') }}</label>
              <select id="tutorial-model" v-model="d.model" class="input font-mono text-xs" :disabled="d.modelsLoading || !d.models.length"><option v-for="model in d.models" :key="model" :value="model">{{ model }}</option></select>
              <p v-if="d.modelsLoading" role="status" class="help-text">{{ t('common.loading') }}</p>
              <div v-else-if="d.modelsError" role="alert" class="mt-2 text-xs text-red-600"><p>{{ d.modelsError }}</p><button class="underline" @click="d.loadModels">{{ t('tutorial.retry') }}</button></div>
              <p v-else-if="d.group && !d.models.length" class="help-text">{{ t('tutorial.noModels') }}</p>
            </div>
          </section>
          <section class="space-y-3 p-5">
            <label for="tutorial-route" class="field-label">{{ t('tutorial.route') }}</label>
            <select id="tutorial-route" v-model="d.routeID" class="input"><option v-for="route in d.routes" :key="route.id" :value="route.id">{{ route.title }}</option></select>
            <div class="flex min-w-0 items-center gap-2 rounded-xl bg-gray-50 p-3 dark:bg-dark-950">
              <code class="min-w-0 flex-1 break-all text-xs text-gray-600 dark:text-gray-300">{{ d.endpoint || t('tutorial.noEndpoint') }}</code>
              <button class="shrink-0 rounded p-1 text-primary-700 dark:text-primary-300" :aria-label="t('tutorial.copyAddress')" :disabled="!d.endpoint" @click="d.copy(d.endpoint, 'address')"><Icon :name="d.copied === 'address' ? 'check' : 'copy'" size="sm" /></button>
            </div>
          </section>
        </aside>
        <main class="min-w-0 space-y-5 lg:col-span-8">
          <section class="panel min-w-0 overflow-hidden">
            <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 bg-gray-50 px-4 py-3 dark:border-gray-800 dark:bg-dark-950">
              <div class="flex min-w-0 flex-1 gap-2 overflow-x-auto" :aria-label="t('tutorial.files')">
                <button v-for="(file, index) in d.files" :key="file.path" class="choice shrink-0 font-mono text-xs" :class="{ selected: d.fileIndex === index }" :aria-pressed="d.fileIndex === index" @click="d.fileIndex = index">{{ file.path }}</button>
                <span v-if="!d.files.length" class="text-sm text-gray-500">{{ t('tutorial.files') }}</span>
              </div>
              <div class="flex gap-2">
                <button class="btn btn-secondary gap-1.5 px-3 text-xs" :disabled="!d.file" @click="d.file && d.copy(d.file.content, 'config')"><Icon :name="d.copied === 'config' ? 'check' : 'copy'" size="xs" />{{ t('tutorial.copy') }}</button>
                <button class="btn btn-primary gap-1.5 px-3 text-xs" :disabled="!d.file" @click="d.download"><Icon name="download" size="xs" />{{ t('tutorial.download') }}</button>
              </div>
            </div>
            <template v-if="d.file">
              <p v-if="d.file.hint" class="border-b border-gray-200 px-5 py-3 text-xs leading-5 text-gray-500 dark:border-gray-800 dark:text-gray-400">{{ d.file.hint }}</p>
              <pre class="min-h-72 max-w-full overflow-x-auto bg-gray-950 p-5 text-xs leading-7 text-gray-100"><code>{{ d.file.content }}</code></pre>
            </template>
            <div v-else class="flex min-h-72 items-center justify-center p-8 text-center text-sm text-gray-500">{{ t('tutorial.configureFirst') }}</div>
          </section>
          <section v-if="d.group" class="panel p-5">
            <h2 class="flex items-center gap-2 text-sm font-semibold"><Icon name="terminal" size="sm" />{{ t('tutorial.startTitle') }}</h2>
            <p class="mt-3 text-sm leading-6 text-gray-600 dark:text-gray-300">{{ d.config.platformDescription }}</p>
            <ol class="mt-3 list-decimal space-y-2 pl-5 text-sm leading-6 text-gray-600 dark:text-gray-300"><li>{{ t('tutorial.startOne') }}</li><li>{{ t('tutorial.startTwo') }}</li><li>{{ t('tutorial.startThree') }}</li></ol>
            <p v-if="d.config.showPlatformNote" class="mt-4 rounded-xl bg-primary-50 p-3 text-xs leading-6 text-primary-800 dark:bg-primary-950/40 dark:text-primary-200">{{ d.config.platformNote }}</p>
          </section>
          <details v-for="article in d.guides" :key="article.id" class="panel overflow-hidden" open>
            <summary class="cursor-pointer px-5 py-4 text-sm font-semibold">{{ article.title }}</summary>
            <TutorialMarkdown :content="article.markdown" class="border-t border-gray-200 p-5 dark:border-gray-800" />
          </details>
          <section v-if="d.faqs.length" class="panel p-5">
            <h2 class="mb-3 text-sm font-semibold">{{ t('tutorial.faq') }}</h2>
            <details v-for="article in d.faqs" :key="article.id" class="border-t border-gray-200 py-3 dark:border-gray-800"><summary class="cursor-pointer text-sm font-medium">{{ article.title }}</summary><TutorialMarkdown :content="article.markdown" class="pt-2" /></details>
          </section>
        </main>
      </div>
    </div>
  </AppLayout>
</template>
<script setup lang="ts">
import { reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import TutorialMarkdown from '@/components/tutorial/TutorialMarkdown.vue'
import { useTutorialCenter } from '@/composables/useTutorialCenter'
import { tutorialKeyUsable } from '@/utils/tutorial'
const { t } = useI18n()
const d = reactive(useTutorialCenter())
</script>
<style scoped>
.panel { @apply rounded-2xl border border-gray-200 bg-white text-gray-900 dark:border-gray-800 dark:bg-dark-900 dark:text-gray-100; }
.field-label { @apply mb-2 block text-xs font-semibold text-gray-500 dark:text-gray-400; }
.help-text { @apply mt-2 text-xs leading-5 text-gray-500 dark:text-gray-400; }
.choice { @apply rounded-lg border border-gray-200 px-3 py-2 text-left text-xs font-medium transition hover:border-primary-400 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:border-gray-700; }
.choice.selected { @apply border-primary-400 bg-primary-50 text-primary-800 dark:border-primary-600 dark:bg-primary-950/50 dark:text-primary-200; }
</style>
