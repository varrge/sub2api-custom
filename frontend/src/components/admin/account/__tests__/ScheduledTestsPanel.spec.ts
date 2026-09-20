import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ScheduledTestsPanel from '../ScheduledTestsPanel.vue'

const { list, create, update } = vi.hoisted(() => ({ list: vi.fn(), create: vi.fn(), update: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { scheduledTests: { listByAccount: list, create, update } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }) }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (s: string) => s }) }))
const basePlan = { id: 1, account_id: 5, model_id: 'gpt-5', cron_expression: '*/5 * * * *', enabled: true, max_results: 20, auto_recover: true, only_when_model_limited: true, last_run_at: null, next_run_at: null }
const global = { stubs: {
 BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' },
 Select: { props: ['modelValue'], emits: ['update:modelValue'], template: '<input data-test="model" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />' },
 Input: { props: ['modelValue','placeholder'], emits: ['update:modelValue'], template: '<input :placeholder="placeholder" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />' },
 Icon: true, HelpTooltip: true, ConfirmDialog: true
} }
async function open() {
 const w=mount(ScheduledTestsPanel,{props:{show:false,accountId:5,modelOptions:[{value:'gpt-5',label:'GPT-5'}]},global})
 await w.setProps({show:true});await flushPromises();return w
}
beforeEach(()=>{vi.clearAllMocks();list.mockResolvedValue([]);create.mockResolvedValue(basePlan);update.mockImplementation(async (_id,data)=>({...basePlan,...data}))})
describe('conditional scheduled account tests',()=>{
 it('creates a conditional plan, preserving the auto-recovery option',async()=>{
  const w=await open();await w.findAll('button').find(b=>b.text()==='admin.scheduledTests.addPlan')!.trigger('click')
  expect(w.get('[data-test="newPlan-only-when-limited"]').attributes('aria-checked')).toBe('false')
  await w.get('[data-test="model"]').setValue('gpt-5')
  await w.get('input[placeholder="*/30 * * * *"]').setValue('*/5 * * * *')
  await w.get('[data-test="newPlan-only-when-limited"]').trigger('click')
  await w.findAll('button').find(b=>b.text()==='common.save')!.trigger('click');await flushPromises()
  expect(create).toHaveBeenCalledWith(expect.objectContaining({model_id:'gpt-5',only_when_model_limited:true,auto_recover:false}))
  w.unmount()
 })
 it('restores the condition during edit and can turn it off without changing auto recovery',async()=>{
  list.mockResolvedValue([basePlan]);const w=await open()
  expect(w.text()).toContain('admin.scheduledTests.onlyWhenModelLimited')
  await w.get('[title="admin.scheduledTests.editPlan"]').trigger('click')
  expect(w.get('[data-test="editForm-only-when-limited"]').attributes('aria-checked')).toBe('true')
  await w.get('[data-test="editForm-only-when-limited"]').trigger('click')
  await w.findAll('button').find(b=>b.text()==='common.save')!.trigger('click');await flushPromises()
  expect(update).toHaveBeenCalledWith(1,expect.objectContaining({only_when_model_limited:false,auto_recover:true}))
  w.unmount()
 })
 it('treats pre-upgrade plans as unconditional',async()=>{
  list.mockResolvedValue([{...basePlan,only_when_model_limited:undefined}]);const w=await open()
  await w.get('[title="admin.scheduledTests.editPlan"]').trigger('click')
  expect(w.get('[data-test="editForm-only-when-limited"]').attributes('aria-checked')).toBe('false');w.unmount()
 })
})
