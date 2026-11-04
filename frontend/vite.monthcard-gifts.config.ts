// Local preview only. All API requests terminate here; writes stay in memory.
import { defineConfig, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'node:path'
import { createHash } from 'node:crypto'
import { prototypeCards, prototypeSettings, prototypeUser } from './dev/monthcard-wallet/fixtures'
import type { AdminMonthCard, GiftCardBatch, GiftCardConfig, GiftCardIssue, MonthCard } from './src/types/groupBuy'

const user = { ...prototypeUser, role: 'admin' }
const groups = [{ id: 1, name: 'Codex Pro 专属号池', platform: 'openai', status: 'active', subscription_type: 'subscription' }]
const now = Date.now()
const cards: MonthCard[] = prototypeCards.slice(0, 2).map((card, index) => ({
  ...card, starts_at: new Date(now - 2 * 86400000).toISOString(), expires_at: new Date(now + 28 * 86400000).toISOString(),
  weekly_window_start: new Date(now - 2 * 86400000).toISOString(), weekly_window_end: new Date(now + 5 * 86400000).toISOString(), priority: index,
}))
const batches: GiftCardBatch[] = []
const receipts = new Map<string, GiftCardBatch>()
const giftRecipients = new Map<number, AdminMonthCard[]>()
let order = cards.map(card => ({ kind: 'card', id: card.id }))
const token = (cfg: GiftCardConfig) => createHash('sha256').update(JSON.stringify({ name: cfg.name, group_id: cfg.group_id, source_group_id: cfg.source_group_id, include_frozen: cfg.include_frozen, quota_usd: cfg.quota_usd, valid_days: cfg.valid_days })).digest('hex')

function previewAPI(): Plugin {
  return {
    name: 'month-card-gift-preview',
    configureServer(server) {
      server.middlewares.use(async (req, res, next) => {
        const url = new URL(req.url ?? '/', 'http://preview.local')
        const path = url.pathname
        if (!path.startsWith('/api/') && !path.startsWith('/setup/')) return next()
        res.setHeader('Content-Type', 'application/json; charset=utf-8')
        res.setHeader('Cache-Control', 'no-store')
        const reply = (data: unknown) => res.end(JSON.stringify({ code: 0, data }))
        if (req.method !== 'GET' && req.method !== 'HEAD') {
          try {
            const chunks: Buffer[] = []
            for await (const chunk of req) chunks.push(Buffer.from(chunk))
            const input = JSON.parse(Buffer.concat(chunks).toString() || '{}')
            if (req.method === 'POST' && path === '/api/v1/admin/group-buy/gifts/preview') {
              return reply({ recipient_count: 3, total_quota_usd: String(Number(input.quota_usd) * 3), preview_token: token(input) })
            }
            if (req.method === 'POST' && path === '/api/v1/admin/group-buy/gifts') {
              const request = input as GiftCardIssue
              if (receipts.has(request.request_id)) return reply(receipts.get(request.request_id))
              if (request.preview_token !== token(request)) throw Error('配置已变化，请重新预览')
              const created = new Date().toISOString(), expires = new Date(Date.now() + request.valid_days * 86400000).toISOString()
              const batch: GiftCardBatch = { ...request, id: batches.length + 1, group_name: groups[0].name, actor_user_id: user.id, recipient_count: 3, created_at: created, expires_at: expires }
              const card: MonthCard = { ...cards[0], id: cards.length + 1, code: `G-preview-${batch.id}`, order_id: 0, team_id: null, team_code: '', card_kind: 'gift', product_name: request.name, total_quota_usd: Number(request.quota_usd), weekly_quota_usd: Number(request.quota_usd), total_used_usd: 0, weekly_used_usd: 0, starts_at: created, expires_at: expires, weekly_window_start: created, weekly_window_end: expires, freeze_allowed: false, original_price_cny: 0, purchase_price_cny: 0, priority: order.length }
              cards.push(card); order.push({ kind: 'card', id: card.id }); batches.unshift(batch); receipts.set(request.request_id, batch)
              giftRecipients.set(batch.id, Array.from({ length: 3 }, (_, i) => ({
                ...card, id: i === 0 ? card.id : batch.id * 1000000 + i,
                user_id: i === 0 ? user.id : user.id + i,
                username: i === 0 ? user.username : `演示领取人 ${i + 1}`,
                email: i === 0 ? user.email : `gift${i + 1}@example.test`,
                code: i === 0 ? card.code : `${card.code}-${i + 1}`,
              })))
              return reply(batch)
            }
            if (req.method === 'PUT' && path === '/api/v1/group-buy/order') { order = input.items; return reply({ updated: true }) }
            throw Error('此预览仅支持模拟赠卡及排序')
          } catch (error) {
            res.statusCode = 400
            return res.end(JSON.stringify({ code: 400, message: error instanceof Error ? error.message : 'Invalid input' }))
          }
        }
        if (path === '/setup/status') return reply({ needs_setup: false })
        if (path === '/api/v1/settings/public') return reply(prototypeSettings)
        if (path === '/api/v1/auth/me' || path === '/api/v1/user/profile') return reply(user)
        if (path === '/api/v1/admin/groups/all') return reply(groups)
        if (path === '/api/v1/admin/group-buy/freeze-policy') return reply({ enabled: false, starts_at: null, ends_at: null })
        if (path === '/api/v1/admin/group-buy/gifts') return reply({ items: batches, total: batches.length, page: 1, page_size: 20, pages: Math.ceil(batches.length / 20) })
        const giftUsage = path.match(/^\/api\/v1\/admin\/group-buy\/gifts\/(\d+)\/cards(?:\/(\d+)\/allocations)?$/)
        if (giftUsage) {
          const recipients = giftRecipients.get(Number(giftUsage[1]))?.map(card => ({ ...card, status: Date.parse(card.expires_at) <= Date.now() ? 'expired' as const : card.status }))
          const card = recipients?.find(item => item.id === Number(giftUsage[2]))
          if (!recipients || (giftUsage[2] && !card)) {
            res.statusCode = 404
            return res.end(JSON.stringify({ code: 404, message: '赠卡不存在' }))
          }
          const page = Math.max(1, Number(url.searchParams.get('page')) || 1)
          if (giftUsage[2]) return reply({ card, items: [], total: 0, page, page_size: 20, pages: 1 })
          const search = (url.searchParams.get('search') || '').trim().toLowerCase(), validity = url.searchParams.get('validity') || 'all'
          const filtered = recipients.filter(card => `${card.username} ${card.email} ${card.code} ${card.user_id}`.toLowerCase().includes(search) && (validity === 'all' || (validity === 'active') === (card.status === 'active')))
          return reply({ items: filtered.slice((page - 1) * 20, page * 20), total: filtered.length, page, page_size: 20, pages: Math.max(1, Math.ceil(filtered.length / 20)) })
        }
        if (path === '/api/v1/group-buy/cards') return reply(cards)
        if (path === '/api/v1/group-buy/order') return reply([{ group_id: 1, items: order }])
        if (path.includes('summary')) return reply({ active_count: 0, subscriptions: [], unread_count: 0 })
        if (path.includes('unread')) return reply({ count: 0 })
        if (path.includes('version')) return reply({ current_version: 'local-gift-preview', has_update: false })
        return reply([])
      })
    },
    transformIndexHtml() {
      return [{ tag: 'script', injectTo: 'head-prepend', children: `window.__APP_CONFIG__=${JSON.stringify(prototypeSettings)};
localStorage.setItem('auth_token','gift-preview-only');
localStorage.setItem('auth_user',${JSON.stringify(JSON.stringify(user))});
localStorage.removeItem('refresh_token');
localStorage.setItem('sub2api_locale','zh');
localStorage.setItem('admin_guide_${user.id}_admin_v4_interactive','true');` }]
    },
  }
}

export default defineConfig({
  plugins: [vue(), previewAPI()],
  resolve: { alias: { '@': resolve(__dirname, 'src'), 'vue-i18n': 'vue-i18n/dist/vue-i18n.runtime.esm-bundler.js' } },
  define: { __INTLIFY_JIT_COMPILATION__: true },
  server: { host: '127.0.0.1', port: 18141, strictPort: true },
})
