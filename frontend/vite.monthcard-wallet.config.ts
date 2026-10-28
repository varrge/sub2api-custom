// Development-only preview: actual wallet UI, mock APIs and in-memory writes.
import { defineConfig, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'node:path'
import { prototypeCards, prototypeSettings, prototypeUser } from './dev/monthcard-wallet/fixtures'

const demoOrders = [{ group_id: 1, items: [{kind:'card',id:1},{kind:'legacy',id:501},{kind:'card',id:2},{kind:'card',id:3}] }]
function demoAPI(): Plugin {
  return {
    name: 'month-card-prototype-fixtures',
    configureServer(server) {
      server.middlewares.use(async (req, res, next) => {
        const path = new URL(req.url ?? '/', 'http://prototype.local').pathname
        if (!path.startsWith('/api/') && !path.startsWith('/setup/')) return next()
        res.setHeader('Content-Type', 'application/json; charset=utf-8')
        res.setHeader('Cache-Control', 'no-store')
        if (req.method !== 'GET' && req.method !== 'HEAD') {
          try {
            const chunks: Buffer[] = []
            for await (const chunk of req) chunks.push(Buffer.from(chunk))
            const input = JSON.parse(Buffer.concat(chunks).toString() || '{}')
            if (req.method === 'PUT' && path === '/api/v1/group-buy/order') {
              const order = demoOrders.find(order => order.group_id === input.group_id)
              const keys = (items: {kind:string;id:number}[]) => items.map(i=>`${i.kind}:${i.id}`).sort().join(',')
              if (!order || !Array.isArray(input.items) || keys(order.items)!==keys(input.items)) throw Error('排序不完整')
              order.items = input.items
              return res.end(JSON.stringify({code:0,data:{updated:true}}))
            }
            const match = path.match(/^\/api\/v1\/group-buy\/cards\/(\d+)\/(freeze|thaw)$/)
            if (req.method === 'POST' && match) {
              const card = prototypeCards.find(c=>c.id===Number(match[1]))
              if (!card) throw Error('月卡不存在')
              if (match[2] === 'freeze') {
                card.remaining_seconds=Math.max(0,Math.floor((Date.parse(card.expires_at)-Date.now())/1000))
                card.frozen_at=new Date().toISOString(); card.status='frozen'
              } else {
                card.expires_at=new Date(Date.now()+(card.remaining_seconds??0)*1000).toISOString()
                card.frozen_at=null; card.status='active'
              }
              return res.end(JSON.stringify({code:0,data:card}))
            }
            throw Error('演示仅支持本地排序和冻结')
          } catch (e) {
            res.statusCode=400
            return res.end(JSON.stringify({code:400,message:e instanceof Error ? e.message : 'Invalid input'}))
          }
        }
        let data: unknown = []
        if (path === '/setup/status') data = { needs_setup: false, step: 'complete' }
        else if (path === '/api/v1/settings/public') data = prototypeSettings
        else if (path === '/api/v1/auth/me' || path === '/api/v1/user/profile') data = prototypeUser
        else if (path === '/api/v1/group-buy/cards') data = prototypeCards
        else if (path === '/api/v1/group-buy/order') data = demoOrders
        else if (path === '/api/v1/user/privileges') data = { title: null, welcome: null }
        else if (path.includes('summary')) data = { active_count: 0, subscriptions: [], unread_count: 0 }
        else if (path.includes('unread')) data = { count: 0, unread_count: 0 }
        else if (path.includes('version')) data = { current_version: '0.2.18-custom.1', has_update: false }
        res.end(JSON.stringify({ code: 0, message: 'success', data }))
      })
    },
    transformIndexHtml() {
      return [{
        tag: 'script', injectTo: 'head-prepend',
        children: `window.__APP_CONFIG__=${JSON.stringify(prototypeSettings)};
localStorage.setItem('auth_token','prototype-only-no-server-access');
localStorage.setItem('auth_user',${JSON.stringify(JSON.stringify(prototypeUser))});
localStorage.removeItem('refresh_token');
localStorage.setItem('sub2api_locale','zh');
localStorage.setItem('user_guide_900001_user_v4_interactive','true');`,
      }]
    },
  }
}

export default defineConfig({
  plugins: [vue(), demoAPI()],
  resolve: { alias: { '@': resolve(__dirname, 'src'), 'vue-i18n': 'vue-i18n/dist/vue-i18n.runtime.esm-bundler.js' } },
  define: { __INTLIFY_JIT_COMPILATION__: true },
  server: { host: '127.0.0.1', port: 18140, strictPort: true },
})
