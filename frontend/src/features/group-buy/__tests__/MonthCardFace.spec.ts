import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import MonthCardFace from '../MonthCardFace.vue'
import { exactDate } from '../model'
import type { MonthCard } from '@/types/groupBuy'
vi.mock('vue-i18n',async()=>({...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),useI18n:()=>({locale:{value:'zh-CN'},t:(key:string,p:Record<string,unknown>={})=>`${key}:${p.name??p.days??p.amount??''}`})}))
enableAutoUnmount(afterEach)
const card = {
  id:1,code:'C-INDIVIDUAL-12345678',team_code:'T-TEAM-87654321',product_name:'轻享月卡',
  expires_at:'2026-11-30T00:00:00Z',status:'active',starts_at:'2026-11-01T00:00:00Z',
  group_name:'Codex Pro',platform:'openai',weekly_used_usd:61.8,weekly_quota_usd:165,
  total_used_usd:226.8,total_quota_usd:660,weekly_window_start:'2026-11-01T00:00:00Z',weekly_window_end:'2026-11-08T00:00:00Z'
} as MonthCard
const props = {card,holder:'User',brand:'Private',now:Date.parse('2026-11-02T00:00:00Z')}
describe('shared month card appearance',()=>{
  it('uses the identical default face across all product tiers, with per-card identity overlays',()=>{
    for(const name of ['轻享月卡','尊享月卡','顶级月卡']){
      const w=mount(MonthCardFace,{props:{...props,card:{...card,product_name:name}}})
      expect(w.get('[data-testid="month-card-face"]').attributes('data-appearance')).toBe('classic-copper')
      expect(w.text()).toContain(name);expect(w.text()).toContain('12345678');expect(w.text()).not.toContain('87654321')
    }
  })
  it('applies one image to any tier and falls back cleanly if it fails',async()=>{
    const appearance={id:'gift-art',name:'Gift',imageUrl:'/card-art/gift.webp',textColor:'#ffffff'}
    const w=mount(MonthCardFace,{props:{...props,appearance}})
    expect(w.get('img').attributes('src')).toBe('/card-art/gift.webp')
    await w.setProps({card:{...card,id:2,product_name:'顶级月卡'}})
    expect(w.get('img').attributes('src')).toBe('/card-art/gift.webp')
    await w.get('img').trigger('error')
    expect(w.find('img').exists()).toBe(false)
    expect(w.find('.month-card-engraving').exists()).toBe(true)
    await w.setProps({appearance:{...appearance,imageUrl:'/card-art/other.webp'}})
    expect(w.get('img').attributes('src')).toBe('/card-art/other.webp')
  })
  it('does not animate under reduced-motion settings',async()=>{
    const original=window.matchMedia
    window.matchMedia=vi.fn(query=>({matches:query.includes('prefers-reduced-motion'),media:query,onchange:null,
      addListener:vi.fn(),removeListener:vi.fn(),addEventListener:vi.fn(),removeEventListener:vi.fn(),dispatchEvent:vi.fn()}))
    try{
      const w=mount(MonthCardFace,{props})
      await w.get('.month-card-stage').trigger('pointermove',{pointerType:'mouse',clientX:1,clientY:1})
      expect(w.get('[data-testid="month-card-face"]').attributes('style')).not.toContain('rotateX')
    }finally{window.matchMedia=original}
  })
  it('keeps usable amount, both quota limits and the next reset inside the card',()=>{
    const w=mount(MonthCardFace,{props})
    const face=w.get('[data-testid="month-card-face"]')
    expect(face.attributes('role')).toBe('group')
    expect(face.get('[data-testid="card-available"]').text()).toBe('$103.20')
    expect(face.get('[data-testid="card-quotas"]').text()).toContain('$61.80 / $165.00')
    expect(face.get('[data-testid="card-quotas"]').text()).toContain('$226.80 / $660.00')
    expect(face.get('[data-testid="card-time-info"]').text()).toContain(exactDate(card.weekly_window_end))
    expect(face.find('button').exists()).toBe(false)
  })
  it('shows zero spendable quota and paused time for frozen cards, and the final-week message at expiry',async()=>{
    const w=mount(MonthCardFace,{props:{...props,card:{...card,status:'frozen',remaining_seconds:18*86400}}})
    expect(w.get('[data-testid="card-available"]').text()).toBe('$0.00')
    expect(w.get('[data-testid="card-time-info"]').text()).toContain('groupBuy.frozenRemaining:18')
    expect(w.get('[data-testid="card-time-info"]').text()).toContain('$103.20')
    expect(w.text()).not.toContain('groupBuy.nextReset')
    await w.setProps({card:{...card,weekly_window_end:card.expires_at}})
    expect(w.get('[data-testid="card-time-info"]').text()).toContain('groupBuy.finalWindow')
    expect(w.text()).not.toContain('groupBuy.nextReset')
  })
})
