<template>
  <AppLayout>
    <div class="mx-auto max-w-[1440px] space-y-5">
      <header class="flex flex-wrap items-center justify-between gap-4">
        <div><h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('tutorial.adminTitle') }}</h1><p class="mt-2 text-sm text-gray-500">{{ t('tutorial.adminHint') }}</p></div>
        <div class="flex items-center gap-3"><span v-if="dirty" class="text-xs text-amber-700 dark:text-amber-300">{{ t('tutorial.unsaved') }}</span><router-link class="btn btn-secondary" to="/tutorial">{{ t('tutorial.openCenter') }}</router-link><button class="btn btn-primary" :disabled="loading || saving || !loaded || !dirty" @click="save">{{ t(saving ? 'common.saving' : 'tutorial.saveAll') }}</button></div>
      </header>
      <div class="flex flex-wrap items-center justify-between gap-3 rounded-xl bg-primary-50 p-4 text-sm text-primary-800 dark:bg-primary-950/30 dark:text-primary-200"><span>{{ t('tutorial.switchHint') }}</span><router-link to="/admin/settings" class="font-medium underline">{{ t('tutorial.settingsTitle') }}</router-link></div>
      <div v-if="error" role="alert" class="rounded-xl border border-red-200 p-4 text-sm text-red-600 dark:text-red-400">{{ error }} <button v-if="!loaded" class="ml-3 underline" @click="load">{{ t('tutorial.retry') }}</button></div>
      <p v-if="loading" role="status" class="p-8 text-center text-gray-500">{{ t('common.loading') }}</p>
      <fieldset v-else-if="loaded" :disabled="saving" class="min-w-0 space-y-5">
        <nav class="flex gap-2" :aria-label="t('tutorial.adminTitle')"><button v-for="tab in tabs" :key="tab" class="btn" :class="section === tab ? 'btn-primary' : 'btn-secondary'" :aria-pressed="section === tab" @click="section = tab">{{ t('tutorial.' + tab) }}</button></nav>
        <div v-if="section !== 'routes'" class="grid min-w-0 gap-5 lg:grid-cols-12">
          <aside class="card min-w-0 space-y-3 p-4 lg:col-span-3">
            <button class="btn btn-secondary w-full" :disabled="catalog.articles.length >= 50" @click="addArticle">+ {{ t('tutorial.addArticle') }}</button>
            <button v-for="article in listedArticles" :key="article.id" class="w-full rounded-xl border p-3 text-left" :class="article.id === selectedID ? 'border-primary-400 bg-primary-50 dark:bg-primary-950/30' : 'border-gray-200 dark:border-gray-700'" @click="selectedID = article.id"><span class="block break-words text-sm font-medium">{{ article.title || t('tutorial.untitled') }}</span><span class="mt-1 block text-xs text-gray-500">{{ t('tutorial.' + article.status) }} · {{ article.order }}</span></button>
            <p v-if="!listedArticles.length" class="p-4 text-center text-sm text-gray-500">{{ t('tutorial.noArticles') }}</p>
          </aside>
          <section v-if="selectedArticle" class="card min-w-0 space-y-4 p-5 lg:col-span-9">
            <div class="grid gap-4 sm:grid-cols-2"><label class="block text-sm">{{ t('tutorial.articleTitle') }}<input v-model="selectedArticle.title" class="input mt-2" maxlength="120" /></label><label class="block text-sm">{{ t('tutorial.status') }}<select v-model="selectedArticle.status" class="input mt-2"><option value="draft">{{ t('tutorial.draft') }}</option><option value="published">{{ t('tutorial.published') }}</option></select></label></div>
            <div class="grid gap-4 sm:grid-cols-3"><label class="block text-sm">{{ t('tutorial.platform') }}<select v-model="selectedArticle.platform" class="input mt-2"><option value="">{{ t('tutorial.allPlatforms') }}</option><option v-for="p in tutorialPlatforms" :key="p" :value="p">{{ p }}</option></select></label><label class="block text-sm">{{ t('tutorial.client') }}<select v-model="selectedArticle.client" class="input mt-2"><option value="">{{ t('tutorial.allClients') }}</option><option v-for="client in tutorialClients" :key="client" :value="client">{{ client }}</option></select></label><label class="block text-sm">{{ t('tutorial.order') }}<input v-model.number="selectedArticle.order" class="input mt-2" type="number" min="-10000" max="10000" /></label></div>
            <label class="block text-sm">{{ t('tutorial.markdown') }}<textarea v-model="selectedArticle.markdown" class="input mt-2 min-h-72 font-mono text-xs" spellcheck="false" /></label>
            <ImageUpload :key="selectedArticle.id" model-value="" size="sm" :hint="t('tutorial.imageHint')" :upload-label="t('tutorial.insertImage')" @update:model-value="insertImage" />
            <details class="rounded-xl border border-gray-200 dark:border-gray-700" open><summary class="cursor-pointer px-4 py-3 text-sm font-medium">{{ t('tutorial.preview') }}</summary><TutorialMarkdown :content="selectedArticle.markdown" class="border-t border-gray-200 p-5 dark:border-gray-700" /></details>
            <button class="btn btn-secondary text-red-600" @click="deleteTarget = { type: 'article', id: selectedArticle.id }">{{ t('common.delete') }}</button>
          </section>
        </div>
        <section v-else class="space-y-4">
          <button class="btn btn-secondary" :disabled="catalog.routes.length >= 20" @click="addRoute">+ {{ t('tutorial.addRoute') }}</button>
          <p class="text-sm text-gray-500">{{ t('tutorial.routeHint') }}</p>
          <div v-for="route in catalog.routes" :key="route.id" class="card space-y-4 p-5">
            <div class="grid gap-4 sm:grid-cols-2"><label class="text-sm">{{ t('tutorial.routeTitle') }}<input v-model="route.title" class="input mt-2" maxlength="120" /></label><label class="text-sm">{{ t('tutorial.routeUrl') }}<input v-model="route.url" class="input mt-2 font-mono text-sm" type="url" placeholder="https://api.example.com" /></label></div>
            <div class="grid items-end gap-4 sm:grid-cols-4"><label class="text-sm">{{ t('tutorial.platform') }}<select v-model="route.platform" class="input mt-2"><option value="">{{ t('tutorial.allPlatforms') }}</option><option v-for="p in tutorialPlatforms" :key="p" :value="p">{{ p }}</option></select></label><label class="text-sm">{{ t('tutorial.status') }}<select v-model="route.status" class="input mt-2"><option value="draft">{{ t('tutorial.draft') }}</option><option value="published">{{ t('tutorial.published') }}</option></select></label><label class="text-sm">{{ t('tutorial.order') }}<input v-model.number="route.order" class="input mt-2" type="number" min="-10000" max="10000" /></label><button class="btn btn-secondary text-red-600" @click="deleteTarget = { type: 'route', id: route.id }">{{ t('common.delete') }}</button></div>
          </div>
        </section>
      </fieldset>
    </div>
    <ConfirmDialog :show="!!deleteTarget" :title="t('common.delete')" :message="t('tutorial.deleteHint')" danger @confirm="remove" @cancel="deleteTarget = null" />
    <ConfirmDialog :show="!!leaveResolver" :title="t('tutorial.unsaved')" :message="t('tutorial.leaveHint')" danger @confirm="resolveLeave(true)" @cancel="resolveLeave(false)" />
  </AppLayout>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import ImageUpload from '@/components/common/ImageUpload.vue'
import TutorialMarkdown from '@/components/tutorial/TutorialMarkdown.vue'
import { tutorialAPI, type TutorialCatalog } from '@/api/tutorial'
import { tutorialClients, tutorialPlatforms } from '@/utils/tutorial'
import { useAppStore } from '@/stores/app'
const { t } = useI18n()
const app = useAppStore()
const tabs = ['tutorial', 'faq', 'routes'] as const
const section = ref<typeof tabs[number]>('tutorial')
const catalog = ref<TutorialCatalog>({ revision: 0, articles: [], routes: [] })
const saved = ref(''), selectedID = ref(''), error = ref('')
const loading = ref(true), saving = ref(false), loaded = ref(false)
const deleteTarget = ref<{ type: 'article' | 'route'; id: string } | null>(null)
const leaveResolver = ref<((value: boolean) => void) | null>(null)
const dirty = computed(() => loaded.value && JSON.stringify(catalog.value) !== saved.value)
const listedArticles = computed(() => catalog.value.articles.filter(a => a.kind === section.value).sort((a,b) => a.order-b.order))
const selectedArticle = computed(() => listedArticles.value.find(a => a.id === selectedID.value))
let alive = true
watch(listedArticles, articles => { if (!articles.some(a => a.id === selectedID.value)) selectedID.value = articles[0]?.id || '' })
async function load() {
  loading.value = true; error.value = ''
  try { const result = await tutorialAPI.adminGet(); if (!alive) return; catalog.value = result; saved.value = JSON.stringify(result); loaded.value = true }
  catch (e) { if (alive) error.value = (e as { message?: string })?.message || t('tutorial.loadFailed') }
  finally { if (alive) loading.value = false }
}
async function save() {
  if (saving.value || !loaded.value) return
  saving.value = true; error.value = ''
  try { const result = await tutorialAPI.save(catalog.value); if (!alive) return; catalog.value = result; saved.value = JSON.stringify(result); app.showSuccess(t('tutorial.saved')) }
  catch (e) { if (alive) error.value = (e as { status?: number })?.status === 409 ? t('tutorial.conflict') : (e as { message?: string })?.message || t('tutorial.saveFailed') }
  finally { if (alive) saving.value = false }
}
function addArticle() {
  if (catalog.value.articles.length >= 50) return
  const id = crypto.randomUUID()
  catalog.value.articles.push({ id, title: '', kind: section.value === 'faq' ? 'faq' : 'tutorial', platform: '', client: '', markdown: '', status: 'draft', order: listedArticles.value.length })
  selectedID.value = id
}
function addRoute() {
  if (catalog.value.routes.length >= 20) return
  catalog.value.routes.push({ id: crypto.randomUUID(), title: '', url: '', platform: '', status: 'draft', order: catalog.value.routes.length })
}
function insertImage(value: string) {
  if (!value || !selectedArticle.value) return
  if (!/^data:image\/(png|jpeg|gif|webp);base64,/.test(value)) { error.value = t('tutorial.imageHint'); return }
  selectedArticle.value.markdown += `\n\n![image](${value})\n`
}
function remove() {
  if (deleteTarget.value?.type === 'article') catalog.value.articles = catalog.value.articles.filter(a => a.id !== deleteTarget.value?.id)
  else catalog.value.routes = catalog.value.routes.filter(r => r.id !== deleteTarget.value?.id)
  deleteTarget.value = null
}
function resolveLeave(value: boolean) { leaveResolver.value?.(value); leaveResolver.value = null }
onBeforeRouteLeave(() => saving.value ? false : !dirty.value || new Promise<boolean>(resolve => { leaveResolver.value = resolve }))
function beforeUnload(event: BeforeUnloadEvent) { if (dirty.value || saving.value) { event.preventDefault(); event.returnValue = '' } }
onMounted(() => { window.addEventListener('beforeunload', beforeUnload); void load() })
onBeforeUnmount(() => { alive = false; resolveLeave(false); window.removeEventListener('beforeunload', beforeUnload) })
</script>
