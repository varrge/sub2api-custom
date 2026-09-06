import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { useImageStudioStore } from '@/stores/imageStudio'
import ImageGenerationView from '../ImageGenerationView.vue'

const { generate, showError } = vi.hoisted(() => ({ generate: vi.fn(), showError: vi.fn() }))

vi.mock('@/api/keys', () => ({ keysAPI: { list: vi.fn().mockResolvedValue({
  items: [{ id: 1, name: 'Studio', key: 'test-key', status: 'active', group: {
    name: 'Images', platform: 'openai', allow_image_generation: true,
  } }, { id: 2, name: 'Gemini', key: 'gemini-key', status: 'active', group: {
    name: 'Gemini', platform: 'gemini', allow_image_generation: true,
  } }], pages: 1,
}) } }))
vi.mock('@/api/imageGeneration', async importOriginal => ({
  ...await importOriginal<typeof import('@/api/imageGeneration')>(),
  listImageModels: vi.fn().mockImplementation(async (_key, platform) => platform === 'gemini' ? ['gemini-3-pro-image'] : ['gpt-image-1', 'gpt-image-1.5']),
  generateImages: generate,
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAuthenticated: true, user: { id: 42 } }) }))
vi.mock('@/stores/imageStudioStorage', () => ({ loadImageStudio: vi.fn().mockResolvedValue(null), saveImageStudio: vi.fn().mockResolvedValue(undefined) }))
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

function mountView() {
  return mount(ImageGenerationView, { global: { plugins: [pinia], stubs: {
    AppLayout: { template: '<main><slot /></main>' },
    Select: SelectStub, BaseDialog: true, ConfirmDialog: true, RouterLink: true, Icon: true, LoadingSpinner: true,
  } } })
}

beforeEach(async () => {
  generate.mockReset()
  showError.mockReset()
  pinia = createPinia()
  view = mountView()
  await flushPromises()
  await view.get('#image-prompt').setValue('A quiet forest')
})
afterEach(() => { view.unmount(); useImageStudioStore(pinia).$dispose() })

describe('image studio requests in progress', () => {
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

    await view.get('#image-model').setValue('gpt-image-1.5')
    await view.get('.composer-count-select').setValue('4')
    await view.findAll('.ratio-card').find(button => button.text() === '1:1')!.trigger('click')
    await view.get('#image-prompt').setValue('A seaside village')

    expect(generate).toHaveBeenCalledWith(expect.objectContaining({
      model: 'gpt-image-1', count: 2, aspectRatio: '2:3', prompt: 'A quiet forest',
    }))
    const progressCount = view.findAll('.result-skeleton-card').length
    const progressRatio = view.get('.result-skeleton-card').attributes('style')
    complete([{ src: 'data:image/png;base64,QUJD' }, { src: 'data:image/png;base64,REVG' }])
    await flushPromises()

    expect.soft(progressCount).toBe(2)
    expect.soft(progressRatio).toContain('2 / 3')
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
    await view.get('form').trigger('submit')
    expect(generate).toHaveBeenCalledTimes(1)
    fail(new Error('Temporary upstream error'))
    await flushPromises()
    expect(showError).toHaveBeenCalledWith('Temporary upstream error')
    expect(view.get('.generate-button').attributes('disabled')).toBeUndefined()

    generate.mockResolvedValueOnce([{ src: 'data:image/png;base64,QUJD' }])
    await view.get('form').trigger('submit')
    await flushPromises()
    expect(generate).toHaveBeenCalledTimes(2)
    expect(view.findAll('.history-item-card')).toHaveLength(1)
  })
})
