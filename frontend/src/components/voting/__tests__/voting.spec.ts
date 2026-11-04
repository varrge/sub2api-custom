import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import VotingBoard from '../VotingBoard.vue'
import VotingManager from '../VotingManager.vue'
import type { Poll, PollInput } from '@/api/voting'

vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key, locale: { value: 'zh' } })
}))

const fixture = (): Poll => ({
  id: 1, title: '选择方案', description: '可以多选', image: '', status: 'open', revision: 1,
  voter_count: 0, has_voted: false, created_at: '2026-11-04T00:00:00Z', updated_at: '2026-11-04T00:00:00Z', my_option_ids: [],
  options: [{ id: 1, title: '方案 A', image: '', votes: 0 }, { id: 2, title: '方案 B', image: '', votes: 0 }]
})
const input = (): PollInput => ({ title: '选择方案', description: '', image: '', status: 'open', revision: 1, options: [{ title: '方案 A', image: '' }, { title: '方案 B', image: '' }] })
const wrappers: VueWrapper[] = []
const global = () => ({ stubs: { teleport: true } })
afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()) })

describe('voting presentation', () => {
  it('uses multi-select checkboxes and confirms before emitting a final submission', async () => {
    const poll = fixture()
    const view = mount(VotingBoard, { props: { polls: [poll], poll, selection: [] }, global: global() })
    wrappers.push(view)
    await view.get('[data-testid="vote-option-1"]').setValue(true)
    expect(view.emitted('update:selection')?.[0]).toEqual([[1]])
    await view.setProps({ selection: [1] })
    await view.get('[data-testid="vote-option-2"]').setValue(true)
    expect(view.emitted('update:selection')?.[1]).toEqual([[1, 2]])
    await view.setProps({ selection: [1, 2] })
    await view.get('[data-testid="vote-submit"]').trigger('click')
    expect(view.emitted('submit')).toBeUndefined()
    expect(view.find('[role="dialog"]').exists()).toBe(false)
    expect(view.get('[role="status"]').text()).toContain('voting.board.confirmSubmitMessage')
    expect(view.get<HTMLInputElement>('[data-testid="vote-option-1"]').element.disabled).toBe(true)
    await view.get('[data-testid="vote-confirm"]').trigger('click')
    expect(view.emitted('submit')).toHaveLength(1)
  })

  it.each(['voted', 'closed', 'submitting'])('locks the controls when %s', async state => {
    const poll = fixture()
    if (state === 'voted') { poll.has_voted = true; poll.my_option_ids = [1] }
    if (state === 'closed') poll.status = 'closed'
    const view = mount(VotingBoard, { props: { polls: [poll], poll, selection: [1], submitting: state === 'submitting' }, global: global() })
    wrappers.push(view)
    expect(view.get<HTMLInputElement>('[data-testid="vote-option-2"]').element.disabled).toBe(true)
    expect(view.find('[data-testid="vote-submit"]').exists()).toBe(false)
  })

  it('shows detail errors even with an existing list', () => {
    const view = mount(VotingBoard, { props: { polls: [fixture()], poll: null, error: '详情加载失败' }, global: global() })
    wrappers.push(view)
    expect(view.get('[role="alert"]').text()).toContain('详情加载失败')
  })

  it('locks voted options and image upload controls and shows server errors inside the form', () => {
    const poll = { ...fixture(), voter_count: 3 }
    const view = mount(VotingManager, { props: { polls: [poll], poll, form: input(), formOpen: true, editingId: 1, formVoteCount: 3, error: '配置已变化' }, global: global() })
    wrappers.push(view)
    const dialog = view.get('[role="dialog"]')
    expect(dialog.get('[role="alert"]').text()).toContain('配置已变化')
    const uploadInputs = dialog.findAll('input[type="file"]')
    expect(uploadInputs).toHaveLength(3)
    expect(uploadInputs[1].element.closest('fieldset')?.disabled).toBe(true)
    expect(dialog.get('select').find('option[value="draft"]').exists()).toBe(false)
    expect(view.findAll('[role="dialog"]')).toHaveLength(1)
  })

  it('does not open result modal when a poll is loaded for editing', async () => {
    const view = mount(VotingManager, { props: { polls: [fixture()], poll: null, form: input(), formOpen: false, editingId: null, formVoteCount: 0 }, global: global() })
    wrappers.push(view)
    await view.setProps({ poll: fixture() })
    expect(view.find('[role="dialog"]').exists()).toBe(false)
    await view.setProps({ formOpen: true, editingId: 1 })
    expect(view.findAll('[role="dialog"]')).toHaveLength(1)
  })
})
