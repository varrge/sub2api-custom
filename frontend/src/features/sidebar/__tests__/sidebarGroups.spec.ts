import { describe, expect, it, vi } from 'vitest'
import type { CustomMenuItem, SidebarGroup, SidebarGroupsConfig } from '@/types'
import { applySidebarGroups, sidebarPageOptions } from '../sidebarGroups'

interface NavItem {
  path: string
  label: string
  icon?: unknown
  children?: NavItem[]
  badge?: () => number
  expandOnly?: boolean
}

const folderIcon = { name: 'FolderIcon' }
const page = (path: string): NavItem => ({ path, label: path })
const group = (id: string, items: string[], visibility: 'user' | 'admin' = 'user'): SidebarGroup => ({
  id, label: `Group ${id}`, visibility, items,
})

describe('applySidebarGroups', () => {
  it('leaves the original navigation intact when no groups are configured', () => {
    const items = [page('/dashboard'), page('/keys')]

    expect(applySidebarGroups(items, undefined, 'user', folderIcon)).toBe(items)
    expect(applySidebarGroups(items, { groups: [] }, 'user', folderIcon)).toEqual(items)
  })

  it('groups only entries already authorized by the caller', () => {
    const keys = page('/keys')
    const dashboard = page('/dashboard')
    const config = { groups: [group('tools', ['/admin/users', '/image-generation', '/keys', '/custom/hidden'])] }

    const result = applySidebarGroups([dashboard, keys], config, 'user', folderIcon)

    expect(result).toEqual([
      { path: '/__sidebar_group__/tools', label: 'Group tools', icon: folderIcon, expandOnly: true, children: [keys] },
      dashboard,
    ])
    expect(result[0].children?.[0]).toBe(keys)
  })

  it('uses configured group and page order, then preserves unassigned page order', () => {
    const items = ['/dashboard', '/keys', '/usage', '/tickets', '/redeem'].map(page)
    const config = { groups: [group('support', ['/tickets']), group('tools', ['/usage', '/keys'])] }

    const result = applySidebarGroups(items, config, 'user', folderIcon)

    expect(result.map(item => item.path)).toEqual([
      '/__sidebar_group__/support', '/__sidebar_group__/tools', '/dashboard', '/redeem',
    ])
    expect(result[1].children?.map(item => item.path)).toEqual(['/usage', '/keys'])
    expect(items.map(item => item.path)).toEqual(['/dashboard', '/keys', '/usage', '/tickets', '/redeem'])
  })

  it('renders each assigned page once and ignores repeated group IDs', () => {
    const config = { groups: [
      group('first', ['/keys', '/keys']),
      group('first', ['/usage']),
      group('second', ['/keys', '/usage', '/usage']),
    ] }

    const result = applySidebarGroups(['/keys', '/usage', '/tickets'].map(page), config, 'user', folderIcon)

    expect(result.map(item => item.path)).toEqual(['/__sidebar_group__/first', '/__sidebar_group__/second', '/tickets'])
    expect(result.flatMap(item => item.children?.map(child => child.path) ?? [item.path]))
      .toEqual(['/keys', '/usage', '/tickets'])
  })

  it('keeps user and admin assignments independent even for a shared page path', () => {
    const config = { groups: [group('admin', ['/keys'], 'admin'), group('user', ['/keys'])] }

    expect(applySidebarGroups([page('/keys')], config, 'user', folderIcon)[0].path).toBe('/__sidebar_group__/user')
    expect(applySidebarGroups([page('/keys')], config, 'admin', folderIcon)[0].path).toBe('/__sidebar_group__/admin')
  })

  it('omits empty, unknown-only, and invalid groups without losing navigation', () => {
    const items = ['/dashboard', '/keys'].map(page)
    const config = { groups: [
      group('empty', []), group('unknown', ['/missing']),
      { ...group('blank', ['/keys']), label: '   ' },
      { ...group('', ['/dashboard']) },
      { ...group('malformed', []), items: null },
      null,
    ] } as unknown as SidebarGroupsConfig

    expect(applySidebarGroups(items, config, 'user', folderIcon)).toEqual(items)
  })

  it('preserves native nested admin groups and live unread badges', () => {
    let unread = 3
    const tickets = { ...page('/admin/tickets'), badge: () => unread }
    const payment = {
      ...page('/admin/orders'),
      children: [page('/admin/orders/list'), page('/admin/orders/settings')],
    }
    const config = { groups: [group('operations', ['/admin/orders', '/admin/tickets'], 'admin')] }

    const result = applySidebarGroups([tickets, payment], config, 'admin', folderIcon)
    const children = result[0].children!

    expect(children[0]).toBe(payment)
    expect(children[0].children).toBe(payment.children)
    expect(children[1]).toBe(tickets)
    expect(children[1].badge?.()).toBe(3)
    unread = 7
    expect(children[1].badge?.()).toBe(7)
  })
})

describe('sidebarPageOptions', () => {
  it('offers translated built-ins and only custom pages in the requested scope', () => {
    const menus: CustomMenuItem[] = [
      { id: 'late', label: 'Later user page', visibility: 'user', sort_order: 9, icon_svg: '', url: '/late' },
      { id: 'admin', label: 'Admin page', visibility: 'admin', sort_order: 0, icon_svg: '', url: '/admin' },
      { id: 'early', label: 'Earlier user page', visibility: 'user', sort_order: 1, icon_svg: '', url: '/early' },
    ]
    const translate = vi.fn((key: string) => `translated:${key}`)

    const user = sidebarPageOptions('user', menus, translate)
    const admin = sidebarPageOptions('admin', menus, translate)

    expect(user).toContainEqual({ path: '/dashboard', label: 'translated:nav.dashboard' })
    expect(admin).toContainEqual({ path: '/admin/orders', label: 'translated:nav.orderManagement' })
    expect(user.filter(item => item.path.startsWith('/custom/')).map(item => item.path)).toEqual(['/custom/early', '/custom/late'])
    expect(admin.filter(item => item.path.startsWith('/custom/')).map(item => item.path)).toEqual(['/custom/admin'])
    expect(user.some(item => item.path.startsWith('/admin/'))).toBe(false)
    expect(admin.some(item => item.path === '/admin/orders/list')).toBe(false)
    expect(menus.map(item => item.id)).toEqual(['late', 'admin', 'early'])
  })
})
