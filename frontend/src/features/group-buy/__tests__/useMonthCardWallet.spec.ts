import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, reactive } from 'vue'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { useMonthCardWallet } from '../useMonthCardWallet'
import { groupBuyAPI as api } from '@/api/groupBuy'
import type { EntitlementOrder, MonthCard } from '@/types/groupBuy'
const shared=vi.hoisted(()=>({auth:{user:{id:1,account_type:'personal'},isEnterprise:false},update:vi.fn()}))
vi.mock('@/stores/auth',()=>({useAuthStore:()=>shared.auth}))
vi.mock('@/stores/groupBuy',()=>({useGroupBuyStore:()=>({updateMonthCard:shared.update})}))
vi.mock('vue-i18n',async()=>({...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),useI18n:()=>({t:(key:string)=>key})}))
vi.mock('@/api/groupBuy',()=>({groupBuyAPI:{cards:vi.fn(),orders:vi.fn(),allocations:vi.fn(),setOrder:vi.fn(),freezeCard:vi.fn(),thawCard:vi.fn()}}))
const card:MonthCard={id:1,user_id:1,group_id:3,order_id:1,code:'C1',product_name:'Monthly',group_name:'Pool',platform:'openai',
  team_code:'T1',team_id:1,status:'active',total_quota_usd:100,total_used_usd:10,weekly_quota_usd:25,weekly_used_usd:5,
  starts_at:'2026-10-01T00:00:00Z',expires_at:'2026-10-31T00:00:00Z',weekly_window_start:'2026-10-22T00:00:00Z',weekly_window_end:'2026-10-29T00:00:00Z',priority:0}
let cards:MonthCard[],orders:EntitlementOrder[], wallet:ReturnType<typeof useMonthCardWallet>
const refs=[{kind:'card' as const,id:1},{kind:'legacy' as const,id:1},{kind:'card' as const,id:2}]
function deferred<T>(){let resolve!:(v:T)=>void;const promise=new Promise<T>(r=>{resolve=r});return{promise,resolve}}
async function view(){mount(defineComponent({setup(){wallet=useMonthCardWallet();return()=>null}}));await flushPromises()}
enableAutoUnmount(afterEach)
beforeEach(()=>{
  vi.resetAllMocks();vi.useFakeTimers();vi.setSystemTime(new Date('2026-10-28T00:00:00Z'))
  shared.auth=reactive({user:{id:1,account_type:'personal'},isEnterprise:false})
  cards=[card,{...card,id:2,code:'C2'}];orders=[{group_id:3,items:refs}]
  vi.mocked(api.cards).mockImplementation(async()=>cards)
  vi.mocked(api.orders).mockImplementation(async()=>orders)
  vi.mocked(api.allocations).mockResolvedValue([])
})
afterEach(()=>vi.useRealTimers())
describe('month card wallet lifecycle',()=>{
  it('saves complete mixed order on drop and retains the selected card',async()=>{
    vi.mocked(api.setOrder).mockImplementation(async order=>{orders=[order]})
    await view();wallet.selectedId.value=2
    await wallet.reorder(3,[refs[2],refs[1],refs[0]]);await flushPromises()
    expect(api.setOrder).toHaveBeenCalledWith({group_id:3,items:[refs[2],refs[1],refs[0]]})
    expect(wallet.selected.value?.id).toBe(2);expect(wallet.orderSaved.value).toBe(true)
  })
  it('rolls back a failed save and does not report success',async()=>{
    vi.mocked(api.setOrder).mockRejectedValue(new Error('conflict'))
    await view();await wallet.reorder(3,[refs[2],refs[1],refs[0]]);await flushPromises()
    expect(wallet.groups.value[0].items.map(i=>i.kind+':'+i.id)).toEqual(['card:1','legacy:1','card:2'])
    expect(wallet.orderError.value).toBe('groupBuy.orderChanged');expect(wallet.orderSaved.value).toBe(false)
  })
  it('blocks duplicate saves and polling during a pending sort',async()=>{
    const pending=deferred<void>();vi.mocked(api.setOrder).mockReturnValue(pending.promise)
    await view();void wallet.reorder(3,[refs[2],refs[1],refs[0]])
    await wallet.reorder(3,[refs[0],refs[2],refs[1]]);await vi.advanceTimersByTimeAsync(30000)
    expect(api.setOrder).toHaveBeenCalledTimes(1);expect(api.cards).toHaveBeenCalledTimes(1)
    pending.resolve();await flushPromises()
  })
  it('clears prior success when a stale drag includes a card that expired',async()=>{
    cards=[{...card,expires_at:'2026-10-28T00:00:02Z'},cards[1]]
    vi.mocked(api.setOrder).mockImplementation(async order=>{orders=[order]})
    await view();await wallet.reorder(3,[refs[2],refs[1],refs[0]]);await flushPromises()
    expect(wallet.orderSaved.value).toBe(true)
    wallet.dragging.value=true
    await vi.advanceTimersByTimeAsync(2100)
    wallet.dragging.value=false
    await wallet.reorder(3,[refs[0],refs[1],refs[2]])
    expect(api.setOrder).toHaveBeenCalledTimes(1)
    expect(wallet.orderSaved.value).toBe(false)
    expect(wallet.orderError.value).toBe('groupBuy.orderChanged')
  })
  it('removes a card at expiry and selects another without deleting billing history',async()=>{
    cards=[{...card,expires_at:'2026-10-28T00:00:01Z'},cards[1]]
    await view();await vi.advanceTimersByTimeAsync(1100)
    expect(wallet.cards.value.map(c=>c.id)).toEqual([2]);expect(wallet.selected.value?.id).toBe(2)
    expect(wallet.allCards.value).toHaveLength(2)
  })
  it('discards old-user reads and freezes when accounts switch',async()=>{
    const pending=deferred<MonthCard[]>();vi.mocked(api.cards).mockReturnValueOnce(pending.promise)
    await view();cards=[{...card,id:3,user_id:2}];shared.auth.user={id:2,account_type:'personal'}
    await flushPromises();pending.resolve([card]);await flushPromises()
    expect(wallet.cards.value.map(c=>c.id)).toEqual([3])
    const freezing=deferred<MonthCard>();vi.mocked(api.freezeCard).mockReturnValue(freezing.promise)
    void wallet.freeze(cards[0]);cards=[];shared.auth.user={id:3,account_type:'personal'}
    await flushPromises();freezing.resolve({...card,id:3,user_id:2,status:'frozen'});await flushPromises()
    expect(wallet.cards.value).toHaveLength(0);expect(shared.update).not.toHaveBeenCalled()
  })
  it('preserves frozen cards beyond wall-clock expiry, and respects freeze policy',async()=>{
    cards=[{...card,status:'frozen',remaining_seconds:3*86400,expires_at:'2020-01-01T00:00:00Z',freeze_allowed:false}]
    await view();expect(wallet.cards.value).toHaveLength(1)
    await wallet.freeze(cards[0]);expect(api.thawCard).not.toHaveBeenCalled()
  })
})
