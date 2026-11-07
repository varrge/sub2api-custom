import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { reactive } from 'vue'
import { keysAPI } from '@/api/keys'
import { getImageTask, ImageGenerationError } from '@/api/imageGeneration'
import { resultImageFile } from '@/components/image-generation/image-result-file'
import { useImageStudioStore } from '@/stores/imageStudio'
import { useVideoStudioStore } from '@/stores/videoStudio'
import ImageGenerationView from '../ImageGenerationView.vue'

const { generate, showError } = vi.hoisted(() => ({ generate: vi.fn(), showError: vi.fn() }))
const publicSettings = reactive({ video_generation_enabled: undefined as boolean | undefined })
const route = reactive({ query: {} as Record<string, string | undefined> })

vi.mock('@/api/keys', () => ({ keysAPI: { list: vi.fn().mockResolvedValue({
  items: [{ id: 1, name: 'Studio', key: 'test-key', status: 'active', group: {
    name: 'Images', platform: 'openai', allow_image_generation: true,
  } }, { id: 2, name: 'Gemini', key: 'gemini-key', status: 'active', group: {
    name: 'Gemini', platform: 'gemini', allow_image_generation: true,
  } }], pages: 1,
}) } }))
vi.mock('@/api/imageGeneration', async importOriginal => ({
  ...await importOriginal<typeof import('@/api/imageGeneration')>(),
  listImageModels: vi.fn().mockImplementation(async (_key, platform) => platform === 'gemini' ? ['gemini-3-pro-image'] : ['gpt-image-1', 'gpt-image-1.5', 'gpt-image-2']),
  generateImages: generate,
  readImageAsyncCapability: vi.fn().mockResolvedValue(false),
  getImageTask: vi.fn(),
}))
vi.mock('@/components/image-generation/image-result-file', () => ({ resultImageFile: vi.fn() }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: publicSettings, showError, showSuccess: vi.fn() }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAuthenticated: true, user: { id: 42 } }) }))
vi.mock('@/stores/imageStudioStorage', () => ({ loadImageStudio: vi.fn().mockResolvedValue(null), saveImageStudio: vi.fn().mockResolvedValue(undefined) }))
vi.mock('@/stores/videoStudioStorage', () => ({ loadVideoStudio: vi.fn().mockResolvedValue(null), saveVideoStudio: vi.fn().mockResolvedValue(1) }))
vi.mock('@/api/videoGeneration', async importOriginal => ({
  ...await importOriginal<typeof import('@/api/videoGeneration')>(),
  listVideoModels: vi.fn().mockResolvedValue(['doubao-seedance-1-5-pro']),
}))
vi.mock('vue-router', async importOriginal => ({
  ...await importOriginal<typeof import('vue-router')>(),
  useRoute: () => route,
  useRouter: () => ({ replace: vi.fn(async ({ query }) => { route.query = query }) }),
}))
vi.mock('vue-i18n', async importOriginal => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

const SelectStub = {
  props: ['modelValue', 'options'],
  emits: ['update:modelValue'],
  template: `<select :value="modelValue" @change="$emit('update:modelValue', typeof modelValue === 'number' ? Number($event.target.value) : $event.target.value)">
    <option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option>
  </select>`,
}
let view: VueWrapper
let pinia: ReturnType<typeof createPinia>

function mountView(attachTo?: HTMLElement) {
  return mount(ImageGenerationView, { attachTo, global: { plugins: [pinia], stubs: {
    AppLayout: { template: '<main><slot /></main>' },
    Select: SelectStub, BaseDialog: true, ConfirmDialog: true, RouterLink: true, Icon: true, LoadingSpinner: true,
  } } })
}

beforeEach(async () => {
  publicSettings.video_generation_enabled = undefined
  route.query = {}
  const NativeURL = URL
  vi.stubGlobal('URL', class extends NativeURL {
    static createObjectURL = vi.fn(() => 'blob:reference-image')
    static revokeObjectURL = vi.fn()
  })
  generate.mockReset()
  vi.mocked(getImageTask).mockReset()
  vi.mocked(resultImageFile).mockReset()
  showError.mockReset()
  pinia = createPinia()
  view = mountView()
  await flushPromises()
  await view.get('#image-prompt').setValue('A quiet forest')
})
afterEach(() => {
  view.unmount()
  document.body.replaceChildren()
  useImageStudioStore(pinia).$dispose()
  useVideoStudioStore(pinia).$dispose()
  vi.unstubAllGlobals()
})

describe('image studio requests in progress', () => {
  it('shows video by default and blocks tab clicks and keyboard navigation when disabled', async () => {
    expect(view.find('#studio-video-tab').exists()).toBe(true)
    publicSettings.video_generation_enabled = false
    await flushPromises()
    expect(view.find('#studio-video-tab').exists()).toBe(false)
    await view.get('#studio-image-tab').trigger('keydown', { key: 'ArrowRight' })
    expect(view.find('#video-prompt').exists()).toBe(false)
    expect(view.get('#studio-image-tab').attributes('aria-selected')).toBe('true')
  })

  it('normalizes a disabled video deep link on initial load and later navigation', async () => {
    view.unmount()
    publicSettings.video_generation_enabled = false
    route.query = { tab: 'video', source: 'saved-link' }
    view = mountView()
    await flushPromises()
    expect(view.find('#studio-video-tab').exists()).toBe(false)
    expect(view.find('#video-prompt').exists()).toBe(false)
    expect(view.find('#image-prompt').exists()).toBe(true)
    expect(route.query).toEqual({ tab: undefined, source: 'saved-link' })
    route.query = { tab: 'video' }
    await flushPromises()
    expect(route.query.tab).toBeUndefined()
    expect(view.find('#video-prompt').exists()).toBe(false)
  })

  it('returns an open video tab to images when disabled and preserves its draft when re-enabled', async () => {
    await view.get('#studio-video-tab').trigger('click')
    await flushPromises()
    await view.get('#video-prompt').setValue('A slow camera pan')
    expect(route.query.tab).toBe('video')
    publicSettings.video_generation_enabled = false
    await flushPromises()
    expect(route.query.tab).toBeUndefined()
    expect(view.find('#video-prompt').exists()).toBe(false)
    expect(view.find('#image-prompt').exists()).toBe(true)
    publicSettings.video_generation_enabled = true
    await flushPromises()
    await view.get('#studio-video-tab').trigger('click')
    await flushPromises()
    expect((view.get('#video-prompt').element as HTMLTextAreaElement).value).toBe('A slow camera pan')
  })

  it('accepts prompt drops, clipboard images, and the lower-left upload button', async () => {
    const file = new File(['image'], 'first.png', { type: 'image/png' })
    const prompt = view.get('#image-prompt')
    const dataTransfer = { files: [file], types: ['Files'], dropEffect: 'none' }
    await prompt.trigger('dragenter', { dataTransfer })
    expect(view.get('.composer-card').text()).toContain('imageGeneration.dropReference')
    await prompt.trigger('dragover', { dataTransfer })
    expect(dataTransfer.dropEffect).toBe('copy')
    await prompt.trigger('drop', { dataTransfer })
    expect(useImageStudioStore(pinia).activeSession!.referenceFiles).toEqual([file])
    expect(view.get('.composer-card').text()).not.toContain('imageGeneration.dropReference')
    expect(view.get('.composer-card').text()).toContain('imageGeneration.referenceInputHint')
    const pasted = new File(['image'], 'pasted.webp', { type: 'image/webp' })
    await prompt.trigger('paste', { clipboardData: { files: [pasted] } })
    expect(useImageStudioStore(pinia).activeSession!.referenceFiles).toEqual([file, pasted])
    const input = view.get<HTMLInputElement>('input[type="file"]')
    const click = vi.spyOn(input.element, 'click')
    await view.get('.composer-icon-button').trigger('click')
    expect(click).toHaveBeenCalledOnce()
    const uploaded = new File(['image'], 'uploaded.jpg', { type: 'image/jpeg' })
    Object.defineProperty(input.element, 'files', { value: [uploaded] })
    await input.trigger('change')
    generate.mockResolvedValue([{ src: 'data:image/png;base64,QUJD' }])
    await view.get('form').trigger('submit')
    await flushPromises()
    await flushPromises()
    expect(generate).toHaveBeenCalledWith(expect.objectContaining({ referenceImages: [file, pasted, uploaded] }))
  })

  it('preserves the 4K draft and receives an image result while the video tab is open', async () => {
    let complete!: (images: { src: string }[]) => void
    generate.mockImplementation(() => new Promise(resolve => { complete = resolve }))
    await view.get('#image-model').setValue('gpt-image-2')
    await view.findAll('.resolution-card').find(button => button.text() === '4K')!.trigger('click')
    await view.get('form').trigger('submit')
    await flushPromises()
    await view.get('#studio-video-tab').trigger('click')
    await flushPromises()
    expect(view.get('#studio-video-tab').attributes('aria-selected')).toBe('true')
    expect(view.find('#video-prompt').exists()).toBe(true)
    expect(view.find('#image-prompt').exists()).toBe(false)
    complete([{ src: 'data:image/png;base64,QUJD' }])
    await flushPromises()
    await view.get('#studio-video-tab').trigger('keydown', { key: 'ArrowLeft' })
    expect(view.get('#studio-image-tab').attributes('aria-selected')).toBe('true')
    expect((view.get('#image-prompt').element as HTMLTextAreaElement).value).toBe('A quiet forest')
    expect(view.get('.resolution-card[aria-pressed="true"]').text()).toBe('4K')
    expect(view.get('.canvas-viewport img').attributes('src')).toBe('data:image/png;base64,QUJD')
    expect(generate).toHaveBeenCalledTimes(1)
  })
  it('selects 4K separately from quality and normalizes it when switching to a legacy model', async () => {
    await view.get('#image-model').setValue('gpt-image-2')
    await view.findAll('.resolution-card').find(button => button.text() === '4K')!.trigger('click')
    expect(useImageStudioStore(pinia).activeSession!.form.aspectRatio).toBe('1:1')
    expect(useImageStudioStore(pinia).activeSession!.form.quality).toBe('standard')
    generate.mockResolvedValue([{ src: 'data:image/png;base64,QUJD' }])
    await view.get('form').trigger('submit')
    await flushPromises()
    await flushPromises()
    expect(generate).toHaveBeenCalledWith(expect.objectContaining({ resolution: '4K', quality: 'standard' }))
    await view.findAll('.ratio-card').find(button => button.text() === '16:9')!.trigger('click')
    await view.findAll('.resolution-card').find(button => button.text() === '1K')!.trigger('click')
    expect(useImageStudioStore(pinia).activeSession!.form.aspectRatio).toBe('1:1')
    expect(view.findAll('.ratio-card').some(button => button.text() === '16:9')).toBe(false)
    await view.get('#image-model').setValue('gpt-image-1.5')
    expect(view.findAll('.resolution-card').map(button => button.text())).toEqual(['1K'])
    expect(useImageStudioStore(pinia).activeSession!.form.resolution).toBe('1K')
  })

  it('keeps Gemini 4K through remounts and hides unsupported quality controls', async () => {
    await view.get('#image-api-key').setValue('2')
    await flushPromises()
    await view.findAll('.resolution-card').find(button => button.text() === '4K')!.trigger('click')
    expect(view.findAll('.quality-card:not(.resolution-card)')).toHaveLength(0)
    view.unmount()
    view = mountView()
    await flushPromises()
    expect(view.get('.resolution-card[aria-pressed="true"]').text()).toBe('4K')
    await view.get('#image-api-key').setValue('1')
    await flushPromises()
    expect(useImageStudioStore(pinia).activeSession!.form.resolution).toBe('1K')
  })

  it('changes model defaults only when the selected key changes', async () => {
    await view.get('#image-model').setValue('gpt-image-1.5')
    await view.get('#image-api-key').setValue('2')
    await flushPromises()
    expect((view.get('#image-model').element as HTMLSelectElement).value).toBe('gemini-3-pro-image')
  })
  it('retains the draft and receives an in-flight result after leaving and remounting the page', async () => {
    let complete!: (images: { src: string }[]) => void
    generate.mockImplementation(() => new Promise(resolve => { complete = resolve }))
    await view.get('#image-model').setValue('gpt-image-1.5')
    await view.get('form').trigger('submit')
    await flushPromises()
    view.unmount()
    complete([{ src: 'data:image/png;base64,QUJD' }])
    await flushPromises()
    view = mountView()
    await flushPromises()
    expect((view.get('#image-prompt').element as HTMLTextAreaElement).value).toBe('A quiet forest')
    expect((view.get('#image-model').element as HTMLSelectElement).value).toBe('gpt-image-1.5')
    expect(view.findAll('.history-item-card')).toHaveLength(1)
    expect(view.get('.canvas-viewport img').attributes('src')).toBe('data:image/png;base64,QUJD')
    expect(generate).toHaveBeenCalledTimes(1)
  })

  it('keeps progress and completed history tied to the submitted settings while editing the next draft', async () => {
    let complete!: (images: { src: string }[]) => void
    generate.mockImplementation(() => new Promise(resolve => { complete = resolve }))
    await view.get('.composer-count-select').setValue('2')
    await view.findAll('.ratio-card').find(button => button.text() === '2:3')!.trigger('click')
    await view.get('form').trigger('submit')
    await flushPromises()

    await view.get('#image-model').setValue('gpt-image-1.5')
    await view.get('.composer-count-select').setValue('4')
    await view.findAll('.ratio-card').find(button => button.text() === '1:1')!.trigger('click')
    await view.get('#image-prompt').setValue('A seaside village')

    expect(generate).toHaveBeenCalledWith(expect.objectContaining({
      model: 'gpt-image-1', count: 2, aspectRatio: '2:3', prompt: 'A quiet forest',
    }))
    const progress = useImageStudioStore(pinia).activeSession!.pending
    expect(view.get('.task-status').text()).toContain('imageGeneration.progressCount')
    complete([{ src: 'data:image/png;base64,QUJD' }, { src: 'data:image/png;base64,REVG' }])
    await flushPromises()

    expect.soft(progress?.count).toBe(2)
    expect.soft(progress?.aspectRatio).toBe('2:3')
    expect(view.findAll('.history-item-card')).toHaveLength(2)
    for (const card of view.findAll('.history-item-card')) {
      expect.soft(card.text()).toContain('gpt-image-1')
      expect.soft(card.text()).not.toContain('gpt-image-1.5')
    }
    expect((view.get('#image-prompt').element as HTMLTextAreaElement).value).toBe('A seaside village')
    expect((view.get('#image-model').element as HTMLSelectElement).value).toBe('gpt-image-1.5')
    expect(view.find('.result-skeleton-card').exists()).toBe(false)
  })

  it('ignores duplicate submissions while pending and allows a retry after failure', async () => {
    let fail!: (error: Error) => void
    generate.mockImplementation(() => new Promise((_resolve, reject) => { fail = reject }))
    await view.get('form').trigger('submit')
    await flushPromises()
    await view.get('form').trigger('submit')
    await flushPromises()
    expect(generate).toHaveBeenCalledTimes(1)
    fail(new Error('Temporary upstream error'))
    await flushPromises()
    expect(showError).toHaveBeenCalledWith('Temporary upstream error')
    expect(view.get('.generate-button').attributes('disabled')).toBeUndefined()

    generate.mockResolvedValueOnce([{ src: 'data:image/png;base64,QUJD' }])
    await view.get('form').trigger('submit')
    await flushPromises()
    await flushPromises()
    expect(generate).toHaveBeenCalledTimes(2)
    expect(view.findAll('.history-item-card')).toHaveLength(1)
  })
})


describe('image studio result workflows', () => {
  async function createResult() {
    generate.mockResolvedValueOnce([{ src: 'data:image/png;base64,QUJD' }])
    await view.get('form').trigger('submit')
    await flushPromises()
    return useImageStudioStore(pinia).activeSession!.results[0]!
  }

  it('keeps the selected result visible during the next generation', async () => {
    const result = await createResult()
    generate.mockImplementation(() => new Promise(() => {}))
    await view.get('form').trigger('submit')
    await flushPromises()
    expect(view.get('.canvas-viewport img').attributes('src')).toBe(result.src)
    expect(view.get('.task-status').text()).toContain('imageGeneration.progressCount')
  })

  it('restores a complete run with its original key and model without generating', async () => {
    await view.get('#image-model').setValue('gpt-image-2')
    await view.findAll('.resolution-card').find(button => button.text() === '4K')!.trigger('click')
    await createResult()
    await view.get('#image-api-key').setValue('2')
    await flushPromises()
    await view.get('#image-prompt').setValue('Another draft')
    await view.get('.reuse-result').trigger('click')
    await flushPromises()
    expect(useImageStudioStore(pinia).activeSession!.form).toMatchObject({ apiKeyId: 1, model: 'gpt-image-2', resolution: '4K', prompt: 'A quiet forest' })
    expect(generate).toHaveBeenCalledTimes(1)
  })

  it('retains an unavailable historical key instead of silently selecting another group', async () => {
    const session = useImageStudioStore(pinia).activeSession!
    session.form.apiKeyId = 987
    session.form.model = 'historical-custom-image'
    await flushPromises()
    expect(session.form.apiKeyId).toBe(987)
    expect(view.text()).toContain('imageGeneration.savedKeyUnavailable')
    expect(view.get('.generate-button').attributes('disabled')).toBeDefined()
  })

  it('loads the new provider model when explicitly replacing an unavailable saved key', async () => {
    const session = useImageStudioStore(pinia).activeSession!
    session.form.apiKeyId = 987
    session.form.model = 'gpt-image-2'
    await flushPromises()
    await view.get('#image-api-key').setValue('2')
    await flushPromises()
    expect(session.form).toMatchObject({ apiKeyId: 2, model: 'gemini-3-pro-image' })
  })

  it('reorders reference files and reports references beyond the limit', async () => {
    const session = useImageStudioStore(pinia).activeSession!
    const files = [1, 2, 3, 4, 5].map(i => new File(['image'], `${i}.png`, { type: 'image/png' }))
    await view.get('#image-prompt').trigger('paste', { clipboardData: { files } })
    expect(session.referenceFiles.map(f => f.name)).toEqual(['1.png', '2.png', '3.png', '4.png'])
    expect(showError).toHaveBeenCalledWith('imageGeneration.tooManyReferences')
    await view.get('[aria-label="imageGeneration.moveReferenceRight"]').trigger('click')
    expect(session.referenceFiles.map(f => f.name)).toEqual(['2.png', '1.png', '3.png', '4.png'])
    await view.get('.reference-heading button').trigger('click')
    expect(session.referenceFiles).toHaveLength(0)
  })

  it('does not add a downloaded reference to a different session', async () => {
    await createResult()
    let finish!: (file: File) => void
    vi.mocked(resultImageFile).mockImplementation(() => new Promise(resolve => { finish = resolve }))
    await view.get('.reference-result').trigger('click')
    const other = useImageStudioStore(pinia).createSession()!
    finish(new File(['image'], 'result.png', { type: 'image/png' }))
    await flushPromises()
    expect(other.referenceFiles).toHaveLength(0)
  })

  it('filters history by prompt and exposes explicit session renaming', async () => {
    await createResult()
    await view.get('.history-search').setValue('forest')
    expect(view.findAll('.history-item-card')).toHaveLength(1)
    await view.get('.history-search').setValue('not in this prompt')
    expect(view.findAll('.history-item-card')).toHaveLength(0)
    await view.get('.history-search').setValue('')
    expect(view.find('[aria-label="imageGeneration.renameSession"]').exists()).toBe(true)
  })

  it('queries an original task with its exhausted key, never the current draft key', async () => {
    const response = await keysAPI.list(1, 100)
    const original = response.items[0]!
    const oldStatus = original.status
    original.status = 'quota_exhausted'
    try {
      vi.mocked(getImageTask).mockResolvedValue({ id: 'task-original', status: 'completed', images: [{ src: 'data:image/png;base64,QUJD' }], retryAfterMs: 2000 })
      const session = useImageStudioStore(pinia).activeSession!
      session.form.apiKeyId = 2
      session.runs.push({ id: 'stored-run', form: { ...session.form, apiKeyId: 1 }, platform: 'openai', referenceFiles: [], createdAt: '2026-11-07 12:00:00', status: 'processing', requested: 1, submitted: 1, completed: 0, taskId: 'task-original' })
      await flushPromises()
      expect(getImageTask).toHaveBeenCalledWith('test-key', 'task-original', expect.any(AbortSignal))
      expect(generate).not.toHaveBeenCalled()
      expect(session.results).toHaveLength(1)
    } finally { original.status = oldStatus }
  })

  it('requires explicit confirmation before a new request after a lost submission response', async () => {
    generate.mockImplementation(async options => {
      options.onRequest(1)
      throw new ImageGenerationError('imageGeneration.submitUnknown', 'unknown')
    })
    await view.get('form').trigger('submit')
    await flushPromises()
    expect(useImageStudioStore(pinia).activeSession!.interrupted).toBe(true)
    generate.mockResolvedValueOnce([{ src: 'data:image/png;base64,QUJD' }])
    await view.get('form').trigger('submit')
    await flushPromises()
    expect(generate).toHaveBeenCalledTimes(1)
    const confirm = view.findAllComponents({ name: 'ConfirmDialog' })[1]!
    expect(confirm.props('show')).toBe(true)
    confirm.vm.$emit('confirm')
    await flushPromises()
    expect(generate).toHaveBeenCalledTimes(2)
  })
})


describe('image workbench panels', () => {
  it('keeps save and conflict warnings outside every switchable panel', async () => {
    const store = useImageStudioStore(pinia)
    for (const error of ['save', 'conflict'] as const) {
      store.storageError = error
      await flushPromises()
      for (const tab of view.findAll('.mobile-panel-tabs button')) {
        await tab.trigger('click')
        const warning = view.get('.storage-notice[role="alert"]')
        expect(warning.text()).toBe(error === 'conflict' ? 'imageGeneration.sessionStorageConflict' : 'imageGeneration.sessionStorageError')
        expect(warning.element.closest('.studio-grid')).toBeNull()
      }
    }
  })

  it('moves mobile focus to the visible destination after submission, import and history actions', async () => {
    view.unmount()
    vi.stubGlobal('innerWidth', 390)
    view = mountView(document.body)
    await flushPromises()
    generate.mockResolvedValueOnce([{ src: 'data:image/png;base64,QUJD' }])
    view.get<HTMLTextAreaElement>('#image-prompt').element.focus()
    await view.get('#image-prompt').trigger('keydown', { key: 'Enter', ctrlKey: true })
    await flushPromises()
    expect(document.activeElement).toBe(view.get('#image-result-panel').element)

    vi.mocked(resultImageFile).mockResolvedValueOnce(new File(['image'], 'result.png', { type: 'image/png' }))
    view.get<HTMLButtonElement>('.reference-result').element.focus()
    await view.get('.reference-result').trigger('click')
    await flushPromises()
    expect(document.activeElement).toBe(view.get('#image-prompt').element)

    await view.findAll('.mobile-panel-tabs button')[2]!.trigger('click')
    view.get<HTMLButtonElement>('.history-result-select').element.focus()
    await view.get('.history-result-select').trigger('click')
    await flushPromises()
    expect(document.activeElement).toBe(view.get('#image-result-panel').element)

    await view.findAll('.mobile-panel-tabs button')[2]!.trigger('click')
    await view.get('.session-select').trigger('click')
    await flushPromises()
    expect(document.activeElement).toBe(view.get('#image-prompt').element)

    await view.findAll('.mobile-panel-tabs button')[2]!.trigger('click')
    await view.get('.history-title button').trigger('click')
    await flushPromises()
    expect(document.activeElement).toBe(view.get('#image-prompt').element)
    expect(generate).toHaveBeenCalledTimes(1)
  })

  it('adds a scene suggestion without discarding the existing prompt or generating', async () => {
    await view.get('[data-scene="product"]').trigger('click')
    expect(useImageStudioStore(pinia).activeSession!.form.prompt).toBe('A quiet forest\nimageGeneration.sceneProductPrompt')
    expect(generate).not.toHaveBeenCalled()
  })

  it('starts a new creation while retaining the previous session draft', async () => {
    const store = useImageStudioStore(pinia)
    const previous = store.activeSession!
    await view.get('.new-creation').trigger('click')
    await flushPromises()
    expect(store.activeId).not.toBe(previous.id)
    expect(store.sessions.find(session => session.id === previous.id)?.form.prompt).toBe('A quiet forest')
    expect(view.get('.studio-grid').attributes('data-panel')).toBe('compose')
  })

  it('submits with a keyboard shortcut and opens results, then reuse returns to creation', async () => {
    generate.mockResolvedValueOnce([{ src: 'data:image/png;base64,QUJD' }])
    await view.get('#image-prompt').trigger('keydown', { key: 'Enter', ctrlKey: true })
    await flushPromises()
    expect(generate).toHaveBeenCalledTimes(1)
    expect(view.get('.studio-grid').attributes('data-panel')).toBe('result')
    await view.get('.reuse-result').trigger('click')
    expect(view.get('.studio-grid').attributes('data-panel')).toBe('compose')
    expect(generate).toHaveBeenCalledTimes(1)
  })
})
