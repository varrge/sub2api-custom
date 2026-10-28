import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import MonthCardFace from '../MonthCardFace.vue'
import type { MonthCard } from '@/types/groupBuy'
vi.mock('vue-i18n',()=>({useI18n:()=>({locale:{value:'zh-CN'},t:(key:string,p:Record<string,unknown>={})=>`${key}:${p.name??p.days??''}`})}))
enableAutoUnmount(afterEach)
const card = {
  id:1,code:'C-INDIVIDUAL-12345678',team_code:'T-TEAM-87654321',product_name:'轻享月卡',
  expires_at:'2026-11-30T00:00:00Z',status:'active'
} as MonthCard
const props = {card,holder:'User',brand:'Private'}
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
})
