// PROTOTYPE ONLY: illustrative quotas, never live customer or pricing data.
import type { MonthCard } from '../../src/types/groupBuy'

const base = {
  user_id: 900001,
  group_id: 1,
  group_name: 'Codex Pro 专属号池',
  platform: 'openai',
  freeze_allowed: true,
  starts_at: '2026-10-18T10:30:00+08:00',
  expires_at: '2026-11-17T10:30:00+08:00',
  weekly_window_start: '2026-10-25T10:30:00+08:00',
  weekly_window_end: '2026-11-01T10:30:00+08:00',
}

export const prototypeCards: MonthCard[] = [
  {
    ...base, id: 1, order_id: 90001, code: 'C9831990931a29d7842c2e52f',
    product_name: '牛魔顶级月卡', team_id: 1, team_code: 'Tc114d21215e6bda88ef2943',
    status: 'active', priority: 0, total_quota_usd: 660, total_used_usd: 226.8,
    weekly_quota_usd: 165, weekly_used_usd: 61.8,
  },
  {
    ...base, id: 2, order_id: 90002, code: 'C02b8e65da9d5e580a7745336',
    product_name: '牛王尊享月卡', team_id: 2, team_code: 'T44c17f0304a9fe8141b4273c',
    status: 'active', priority: 1, total_quota_usd: 400, total_used_usd: 60,
    weekly_quota_usd: 100, weekly_used_usd: 18.5,
    starts_at: '2026-10-23T15:20:00+08:00', expires_at: '2026-11-22T15:20:00+08:00',
    weekly_window_start: '2026-10-23T15:20:00+08:00', weekly_window_end: '2026-10-30T15:20:00+08:00',
  },
  {
    ...base, id: 3, order_id: 90003, code: 'C5e2bbdf2e25ad01c85d8a101',
    product_name: '奶牛轻享月卡', team_id: null, team_code: '',
    status: 'frozen', priority: 2, total_quota_usd: 180, total_used_usd: 72,
    weekly_quota_usd: 45, weekly_used_usd: 8.4,
    frozen_at: '2026-10-27T09:00:00+08:00', remaining_seconds: 18 * 86400,
  },
  {
    ...base, id: 4, order_id: 90004, code: 'Cf41c9d050d85743e57e5a8ab',
    product_name: '奶牛轻享月卡', team_id: 4, team_code: 'T1a6d9fb140f37c4963f0f782',
    status: 'expired', priority: 3, total_quota_usd: 180, total_used_usd: 145,
    weekly_quota_usd: 45, weekly_used_usd: 30,
    starts_at: '2026-09-25T10:00:00+08:00', expires_at: '2026-10-25T10:00:00+08:00',
    weekly_window_start: '2026-10-23T10:00:00+08:00', weekly_window_end: '2026-10-25T10:00:00+08:00',
  },
]

prototypeCards.forEach((card, i) => {
  card.original_price_cny = [198,128,58,58][i]
  card.purchase_price_cny = [178.2,128,58,58][i]
})

export const prototypeUser = {
  id: 900001, email: 'preview@example.test', username: 'varrge',
  role: 'user', account_type: 'personal', status: 'active',
  balance: 0, concurrency: 5, allowed_groups: [], created_at: '2026-08-01T00:00:00+08:00',
}

const logo = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><g fill="#fff3f2" stroke="#e8a4ad" stroke-width="1.4"><path d="M32 31C7 15 23-4 32 18C41-4 57 15 32 31Z"/><path d="M32 31C7 15 23-4 32 18C41-4 57 15 32 31Z" transform="rotate(72 32 32)"/><path d="M32 31C7 15 23-4 32 18C41-4 57 15 32 31Z" transform="rotate(144 32 32)"/><path d="M32 31C7 15 23-4 32 18C41-4 57 15 32 31Z" transform="rotate(216 32 32)"/><path d="M32 31C7 15 23-4 32 18C41-4 57 15 32 31Z" transform="rotate(288 32 32)"/></g><circle cx="32" cy="32" r="4" fill="#e8b4ad"/></svg>`

export const prototypeSettings = {
  site_name: 'v樱花 - Private', site_logo: `data:image/svg+xml,${encodeURIComponent(logo)}`,
  version: 'v0.2.18-custom.1', registration_enabled: false, backend_mode_enabled: false,
  payment_enabled: true, subscription_enabled: true, native_box_entry_enabled: true,
  activity_campaigns_enabled: false, image_generation_enabled: true, support_ticket_enabled: true,
  custom_menu_items: [], site_billing_mode: 'all', api_base_url: '', contact_info: '',
  doc_url: '', landing_page_enabled: false, public_usage_enabled: false,
}
