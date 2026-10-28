import { describe, expect, it } from 'vitest'
import type { MonthCard, EntitlementRef } from '@/types/groupBuy'
import { cardDaysRemaining, completeWalletOrder, spendableQuota, visibleWalletCards, walletGroups } from '../walletModel'
import { cardArtworkURL } from '../cardAppearance'
const now = Date.parse('2026-10-28T04:00:00Z')
const base = {
  id:1,user_id:1,group_id:3,order_id:1,code:'C1',group_name:'Pool',product_name:'Monthly',
  platform:'openai',team_code:'T1',team_id:1,status:'active',total_quota_usd:100,total_used_usd:90,
  weekly_quota_usd:25,weekly_used_usd:10,starts_at:'2026-10-18T04:00:00Z',expires_at:'2026-11-17T04:00:00Z',
  weekly_window_start:'2026-10-25T04:00:00Z',weekly_window_end:'2026-11-01T04:00:00Z',priority:0
} satisfies MonthCard
describe('wallet entitlement boundaries', () => {
  it('hides expired/revoked at the deadline but keeps frozen and exhausted cards', () => {
    const cards:MonthCard[] = [base,{...base,id:2,status:'expired'},{...base,id:3,status:'revoked'},
      {...base,id:4,expires_at:new Date(now).toISOString()},
      {...base,id:5,status:'frozen',expires_at:'2020-01-01T00:00:00Z',remaining_seconds:86400},
      {...base,id:6,weekly_used_usd:25}]
    expect(visibleWalletCards(cards,now).map(c=>c.id)).toEqual([1,5,6])
    expect(spendableQuota(base,now)).toBe(10)
    expect(spendableQuota(cards[4],now)).toBe(0)
    expect(cardDaysRemaining(cards[4],now)).toBe(1)
    expect(spendableQuota({...base,starts_at:'2027-01-01T00:00:00Z'},now)).toBe(0)
  })
  it('keeps same-group subscriptions and frozen cards in the draggable billing list', () => {
    const refs:EntitlementRef[]=[{kind:'legacy',id:1},{kind:'card',id:1},{kind:'card',id:2}]
    const groups=walletGroups([base,{...base,id:2,status:'frozen',remaining_seconds:86400},
      {...base,id:3,group_id:4}], [{group_id:3,items:refs}],now)
    expect(groups.map(g=>g.id)).toEqual([3,4])
    expect(groups[0].items.map(({kind,id})=>({kind,id}))).toEqual(refs)
  })
  it('preserves non-displayed refs and rejects duplicates, foreign refs and incomplete sorts', () => {
    const a:EntitlementRef={kind:'card',id:1},b:EntitlementRef={kind:'card',id:2},l:EntitlementRef={kind:'legacy',id:1}
    expect(completeWalletOrder([a,l,b],[a,b],[b,a])).toEqual([b,l,a])
    expect(completeWalletOrder([a,l,b],[a,l,b],[b,l,a])).toEqual([b,l,a])
    expect(completeWalletOrder([a,b],[a,b],[a,a])).toBeNull()
    expect(completeWalletOrder([a,b],[a,b],[a,l])).toBeNull()
    expect(completeWalletOrder([a,b],[a,b],[a])).toBeNull()
  })
  it('accepts same-origin or HTTPS artwork only', () => {
    expect(cardArtworkURL('/skins/sakura.webp')).toBe('/skins/sakura.webp')
    expect(cardArtworkURL('https://assets.example.test/card.png')).toBeTruthy()
    for(const input of ['javascript:alert(1)','//evil.test/a','http://evil.test/a','/\\evil.test/a','data:text/html,test']) expect(cardArtworkURL(input)).toBeUndefined()
  })
})
