import { defineStore } from 'pinia'
import { computed, onScopeDispose, ref, toRaw, watch } from 'vue'
import { generateImages, type GeneratedImage, type GenerateImageOptions, type ImageAspectRatio, type ImageQuality } from '@/api/imageGeneration'
import type { GroupPlatform } from '@/types'
import { useAuthStore } from './auth'
import { loadImageStudio, saveImageStudio, type SavedImageStudio } from './imageStudioStorage'

export interface ImageStudioForm {
  apiKeyId: number | null
  model: string
  prompt: string
  aspectRatio: ImageAspectRatio
  quality: ImageQuality
  count: number
}

export interface ImageStudioResult extends GeneratedImage {
  id: string
  model: string
  createdAt: string
  aspectRatio: string
  mode: 'text' | 'image'
  prompt: string
}

export interface ImageStudioSession {
  id: string
  title: string
  form: ImageStudioForm
  referenceFiles: File[]
  results: ImageStudioResult[]
  activeResultId: string | null
  pending: { count: number; aspectRatio: ImageAspectRatio } | null
  interrupted: boolean
  error: string
}

export const useImageStudioStore = defineStore('imageStudio', () => {
  const auth = useAuthStore()
  const sessions = ref<ImageStudioSession[]>([])
  const activeId = ref<string | null>(null)
  const ready = ref(false)
  const storageError = ref<'load' | 'save' | 'conflict' | null>(null)
  const activeSession = computed(() => sessions.value.find(session => session.id === activeId.value) ?? null)
  let owner: number | null = null
  let generation = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  let writes = Promise.resolve()
  let canSave = false
  let dirty = false
  let revision = { value: 0 }

  function createSession() {
    if (!ready.value) return null
    const session: ImageStudioSession = {
      id: crypto.randomUUID(), title: '',
      form: { apiKeyId: null, model: '', prompt: '', aspectRatio: 'auto', quality: 'standard', count: 1 },
      referenceFiles: [], results: [], activeResultId: null, pending: null, interrupted: false, error: '',
    }
    sessions.value.unshift(session)
    activeId.value = session.id
    return session
  }

  function deleteSession(id: string) {
    if (sessions.value.find(session => session.id === id)?.pending) return
    const index = sessions.value.findIndex(session => session.id === id)
    if (index < 0) return
    sessions.value.splice(index, 1)
    if (activeId.value === id) activeId.value = sessions.value[0]?.id ?? null
    if (!sessions.value.length) createSession()
    void flush()
  }

  function flush(): Promise<void> {
    clearTimeout(timer)
    if (!ready.value || !canSave || !dirty || owner === null) return writes
    const userId = owner
    const epoch = generation
    const version = revision
    // IndexedDB keeps large image data and File references out of localStorage.
    // A snapshot excludes API key secrets: only the selected key ID is retained.
    let snapshot: SavedImageStudio
    try {
      snapshot = structuredClone({ sessions: toRaw(sessions.value), activeId: activeId.value })
    } catch {
      storageError.value = 'save'
      return writes
    }
    dirty = false
    writes = writes.then(async () => {
      if (epoch === generation && !canSave) return
      version.value = await saveImageStudio(userId, { ...snapshot, revision: version.value })
    }).then(() => {
      if (epoch === generation && canSave) storageError.value = null
    }).catch(error => {
      if (epoch !== generation) return
      dirty = true
      if (error instanceof Error && error.message === 'image-studio-conflict') {
        canSave = false
        storageError.value = 'conflict'
      } else storageError.value = 'save'
    })
    return writes
  }

  watch([sessions, activeId], () => {
    if (!ready.value || !canSave) return
    dirty = true
    clearTimeout(timer)
    timer = setTimeout(() => void flush(), 250)
  }, { deep: true, flush: 'sync' })

  watch(() => auth.isAuthenticated ? auth.user?.id ?? null : null, async userId => {
    void flush()
    const epoch = ++generation
    owner = userId
    ready.value = false
    canSave = false
    dirty = false
    sessions.value = []
    activeId.value = null
    storageError.value = null
    revision = { value: 0 }
    if (userId === null) return
    try {
      await writes
      const saved = await loadImageStudio(userId)
      if (epoch !== generation) return
      if (saved) {
        revision.value = saved.revision ?? 0
        if (!Array.isArray(saved.sessions)) throw new Error('Invalid image studio history')
        sessions.value = saved.sessions.map(session => ({
          ...session,
          interrupted: Boolean(session.pending) || session.interrupted,
          pending: null,
        }))
        activeId.value = saved.activeId
      }
      canSave = true
    } catch {
      if (epoch !== generation) return
      storageError.value = 'load'
    }
    if (epoch !== generation) return
    ready.value = true
    if (!sessions.value.length) createSession()
    if (!activeSession.value) activeId.value = sessions.value[0]!.id
  }, { immediate: true, flush: 'sync' })

  async function generate(apiKey: string, platform: GroupPlatform): Promise<number> {
    const session = activeSession.value
    if (!session || session.pending || owner === null) return 0
    const epoch = generation
    const request: GenerateImageOptions = {
      ...session.form,
      prompt: session.form.prompt.trim(),
      apiKey, platform, referenceImages: [...session.referenceFiles],
    }
    session.title ||= request.prompt.slice(0, 48)
    session.error = ''
    session.interrupted = false
    session.pending = { count: request.count, aspectRatio: request.aspectRatio }
    void flush()
    const isCurrent = () => epoch === generation && sessions.value.some(item => item.id === session.id)
    try {
      const images = await generateImages(request)
      if (!isCurrent()) return 0
      if (!images.length) throw new Error('imageGeneration.noImageReturned')
      const results = images.map((image): ImageStudioResult => ({
        ...image, id: crypto.randomUUID(), model: request.model, createdAt: new Date().toLocaleString(),
        aspectRatio: request.aspectRatio, mode: request.referenceImages.length ? 'image' : 'text', prompt: request.prompt,
      }))
      session.results.unshift(...results)
      session.activeResultId = results[0]!.id
      return results.length
    } catch (error) {
      if (!isCurrent()) return 0
      session.error = error instanceof Error ? error.message : 'imageGeneration.generateFailed'
      throw error
    } finally {
      if (isCurrent()) {
        session.pending = null
        void flush()
      }
    }
  }

  const onPageHide = () => void flush()
  window.addEventListener('pagehide', onPageHide)
  onScopeDispose(() => {
    void flush()
    window.removeEventListener('pagehide', onPageHide)
  })

  return { sessions, activeId, activeSession, ready, storageError, createSession, deleteSession, generate, flush }
})
