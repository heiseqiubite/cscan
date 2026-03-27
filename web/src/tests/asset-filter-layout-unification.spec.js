import { shallowMount } from '@vue/test-utils'

import AssetAllView from '@/components/asset/AssetAllView.vue'
import DomainView from '@/components/asset/DomainView.vue'
import IPView from '@/components/asset/IPView.vue'
import SiteView from '@/components/asset/SiteView.vue'
import DirScanView from '@/components/asset/DirScanView.vue'
import VulView from '@/components/asset/VulView.vue'

const mountOptions = {
  global: {
    config: {
      globalProperties: {
        $t: (key) => key
      }
    }
  }
}

const targetPages = [
  { name: 'AssetAllView', component: AssetAllView, clearLabel: 'asset.clearData' },
  { name: 'DomainView', component: DomainView, clearLabel: 'asset.clearData' },
  { name: 'IPView', component: IPView, clearLabel: 'asset.clearData' },
  { name: 'SiteView', component: SiteView, clearLabel: 'asset.clearData' },
  { name: 'DirScanView', component: DirScanView, clearLabel: 'dirscan.clearData' },
  { name: 'VulView', component: VulView, clearLabel: 'vul.clearData' }
]

describe('asset filter layout unification', () => {
  it.each(targetPages)('toolbar order is search / Filters / Refresh in %s', ({ component }) => {
    const wrapper = shallowMount(component, mountOptions)
    const toolbar = wrapper.find('.toolbar')

    expect(toolbar.exists()).toBe(true)

    const children = Array.from(toolbar.element.children)
    expect(children.length).toBeGreaterThanOrEqual(3)

    const [first, second, third] = children

    expect(first.tagName.toLowerCase()).toContain('el-autocomplete')
    expect(second.tagName.toLowerCase()).toContain('el-button')
    expect(third.tagName.toLowerCase()).toContain('el-button')
  })

  it.each(targetPages)('filters panel is placed below toolbar and uses filters-panel class in %s', ({ component }) => {
    const wrapper = shallowMount(component, mountOptions)
    const toolbar = wrapper.find('.toolbar')
    const panel = wrapper.find('.filters-panel')

    expect(toolbar.exists()).toBe(true)
    expect(panel.exists()).toBe(true)

    const next = toolbar.element.nextElementSibling
    expect(next).toBe(panel.element)
  })

  it.each(targetPages)('ClearData actions live outside the filters panel in %s', ({ component }) => {
    const wrapper = shallowMount(component, mountOptions)
    const panel = wrapper.find('.filters-panel')

    expect(panel.exists()).toBe(true)

    // Buttons inside the advanced filters panel should not include a danger+plain ClearData action
    const panelButtons = panel.findAllComponents({ name: 'ElButton' })
    const dangerPlainInPanel = panelButtons.filter((btn) => {
      const props = btn.props()
      return props.type === 'danger' && props.plain
    })
    expect(dangerPlainInPanel.length).toBe(0)

    // Somewhere else in the view there should be at least one danger+plain button
    const allButtons = wrapper.findAllComponents({ name: 'ElButton' })
    const dangerPlainButtons = allButtons.filter((btn) => {
      const props = btn.props()
      return props.type === 'danger' && props.plain
    })
    expect(dangerPlainButtons.length).toBeGreaterThan(0)
  })
})
