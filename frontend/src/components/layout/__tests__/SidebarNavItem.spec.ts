import { describe, expect, it } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'

import SidebarNavItem, { type SidebarNavEntry } from '../SidebarNavItem.vue'

const RouterLinkStub = defineComponent({
  props: ['to'],
  setup(props, { attrs, slots }) {
    return () => h('a', { ...attrs, 'data-to': String(props.to) }, slots.default?.())
  },
})

function mountItem(item: SidebarNavEntry, options: {
  currentPath?: string
  collapsed?: boolean
  overrides?: Map<string, boolean>
} = {}) {
  const currentPath = options.currentPath ?? '/dashboard'
  const overrides = options.overrides ?? new Map<string, boolean>()

  const isGroupActive = (entry: SidebarNavEntry): boolean =>
    !!entry.children?.some(child => child.path === currentPath || isGroupActive(child))
  const isExpanded = (entry: SidebarNavEntry): boolean =>
    overrides.get(entry.path) ?? isGroupActive(entry)

  return mount(SidebarNavItem, {
    props: {
      item,
      collapsed: options.collapsed ?? false,
      currentPath,
      isExpanded,
      isGroupActive,
      isActive: (path: string) => currentPath === path || currentPath.startsWith(path + '/'),
    },
    global: { stubs: { RouterLink: RouterLinkStub } },
  })
}

describe('SidebarNavItem', () => {
  it('renders a plain item with label and emits menu-click', async () => {
    const view = mountItem({ path: '/keys', label: 'API Keys', icon: null })
    const link = view.get('[data-to="/keys"]')
    expect(link.text()).toContain('API Keys')
    await link.trigger('click')
    expect(view.emitted('menu-click')).toEqual([['/keys']])
  })

  it('marks the active item and keeps onboarding anchors', () => {
    const view = mountItem(
      { path: '/admin/groups', label: 'Groups', icon: null },
      { currentPath: '/admin/groups' },
    )
    const link = view.get('[data-to="/admin/groups"]')
    expect(link.attributes('id')).toBe('sidebar-group-manage')
    expect(link.classes()).toContain('sidebar-link-active')
  })

  it('sets the my-keys tour anchor only on /keys', () => {
    const view = mountItem({ path: '/keys', label: 'API Keys', icon: null })
    expect(view.get('[data-to="/keys"]').attributes('data-tour')).toBe('sidebar-my-keys')
  })

  it('renders badge values for plain items', () => {
    const view = mountItem({ path: '/tickets', label: 'Tickets', icon: null, badge: () => 3 })
    expect(view.text()).toContain('3')
  })

  it('renders custom SVG icons through the sanitizer', () => {
    const view = mountItem({
      path: '/custom/abc',
      label: 'Custom',
      icon: null,
      iconSvg: '<svg viewBox="0 0 24 24" onload="alert(1)"><path d="M0 0h24v24H0z"/></svg>',
    })
    const icon = view.get('.sidebar-svg-icon')
    expect(icon.html()).toContain('<svg')
    expect(icon.html()).not.toContain('onload')
  })

  it('renders group children when expanded and forwards group-click', async () => {
    const group: SidebarNavEntry = {
      path: '/__sidebar_group__/g1',
      label: 'My Group',
      icon: null,
      expandOnly: true,
      children: [
        { path: '/keys', label: 'API Keys', icon: null },
        { path: '/usage', label: 'Usage', icon: null },
      ],
    }
    const overrides = new Map([['/__sidebar_group__/g1', true]])
    const view = mountItem(group, { overrides })

    // expanded via override -> children rendered
    expect(view.get('[data-to="/keys"]').text()).toContain('API Keys')

    const button = view.get('button')
    expect(button.text()).toContain('My Group')
    await button.trigger('click')
    expect(view.emitted('group-click')).toHaveLength(1)
  })

  it('auto-expands a custom group wrapper when a nested native-group child is active', () => {
    const nested: SidebarNavEntry = {
      path: '/__sidebar_group__/g1',
      label: 'Ops',
      icon: null,
      expandOnly: true,
      children: [
        {
          path: '/admin/channels',
          label: 'Channels',
          icon: null,
          expandOnly: true,
          children: [{ path: '/admin/channels/monitor', label: 'Monitor', icon: null }],
        },
      ],
    }
    const view = mountItem(nested, { currentPath: '/admin/channels/monitor' })
    // both levels auto-expand through the recursive isGroupActive
    expect(view.get('[data-to="/admin/channels/monitor"]').text()).toContain('Monitor')
    expect(view.findAll('button').length).toBe(2)
  })

  it('hides children while collapsed but keeps the group button usable', () => {
    const overrides = new Map([['/g', true]])
    const view = mountItem(
      {
        path: '/g',
        label: 'Group',
        icon: null,
        children: [{ path: '/keys', label: 'API Keys', icon: null }],
      },
      { collapsed: true, overrides },
    )
    expect(view.find('[data-to="/keys"]').exists()).toBe(false)
    const button = view.get('button')
    expect(button.attributes('title')).toBe('Group')
  })
})
