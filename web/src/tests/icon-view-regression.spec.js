import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AssetInventoryTab from '@/views/AssetManagement/AssetInventoryTab.vue'
import IconView from '@/components/asset/IconView.vue'
import ProTable from '@/components/common/ProTable.vue'

describe('icon view regression', () => {
  it('renders icon tab label from translated icon key instead of raw asset.iconHash key', () => {
    const wrapper = mount(AssetInventoryTab, {
      global: {
        stubs: {
          AssetInventoryCardView: true,
          AssetAllView: true,
          DomainView: true,
          IPView: true,
          SiteView: true,
          DirScanView: true,
          VulView: true,
          IconView: true,
          ElTabs: {
            template: '<div><slot /></div>'
          },
          ElTabPane: {
            template: '<div><slot name="label" /><slot /></div>'
          },
          ElIcon: {
            template: '<i><slot /></i>'
          }
        },
        config: {
          globalProperties: {
            $t: (key) => {
              if (key === 'asset.icon') return '图标'
              if (key === 'asset.iconHash') return 'asset.iconHash'
              return key
            }
          }
        }
      }
    })

    expect(wrapper.text()).toContain('图标')
    expect(wrapper.text()).not.toContain('asset.iconHash')
  })

  it('does not render top toolbar when neither search box nor header actions are needed', () => {
    const wrapper = mount(ProTable, {
      props: {
        columns: [],
        searchItems: []
      },
      global: {
        stubs: {
          ElCard: {
            template: '<div><slot /></div>'
          },
          ElTable: {
            template: '<div><slot /></div>'
          },
          ElTableColumn: {
            template: '<div><slot /></div>'
          },
          ElPagination: true,
          ElAutocomplete: true,
          ElButton: true,
          ElForm: true,
          ElFormItem: true,
          ElSelect: true,
          ElOption: true,
          ElDropdown: true,
          ElDropdownMenu: true,
          ElDropdownItem: true,
          ElTag: true,
          ElIcon: {
            template: '<i><slot /></i>'
          }
        }
      }
    })

    expect(wrapper.find('.pro-table-top-toolbar').exists()).toBe(false)
  })

  it('renders filter button inline in batch toolbar when only filters are available', () => {
    const wrapper = mount(ProTable, {
      props: {
        columns: [],
        searchItems: [{ label: 'Icon Hash', prop: 'icon_hash', type: 'input' }]
      },
      global: {
        stubs: {
          ElCard: {
            template: '<div><slot /></div>'
          },
          ElTable: {
            template: '<div><slot /></div>'
          },
          ElTableColumn: {
            template: '<div><slot /></div>'
          },
          ElPagination: true,
          ElAutocomplete: true,
          ElButton: {
            template: '<button><slot /></button>'
          },
          ElForm: true,
          ElFormItem: true,
          ElSelect: true,
          ElOption: true,
          ElDropdown: true,
          ElDropdownMenu: true,
          ElDropdownItem: true,
          ElTag: true,
          ElIcon: {
            template: '<i><slot /></i>'
          }
        },
        config: {
          globalProperties: {
            $t: (key) => key
          }
        }
      }
    })

    expect(wrapper.find('.pro-table-top-toolbar').exists()).toBe(false)
    expect(wrapper.find('.pro-table-batch-toolbar').text()).toContain('asset.assetInventoryTab.filters')
  })

  it('formats base64 screenshot as data url instead of using it as a raw src path', () => {
    const wrapper = mount(IconView, {
      global: {
        stubs: {
          ProTable: {
            template: '<div><slot name="screenshot" :row="row" /></div>',
            data() {
              return {
                row: {
                  iconHash: 'hash-1',
                  screenshot: 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wn8mWQAAAAASUVORK5CYII='
                }
              }
            }
          },
          ElDropdown: true,
          ElButton: true,
          ElDropdownMenu: true,
          ElDropdownItem: true,
          ElIcon: {
            template: '<i><slot /></i>'
          },
          ElTag: true
        },
        mocks: {
          $t: (key) => key
        }
      }
    })

    const img = wrapper.find('img.screenshot-image')
    expect(img.exists()).toBe(true)
    expect(img.attributes('src')).toMatch(/^data:image\//)
  })
})
