import { vi } from 'vitest'
import { config } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
import { createPinia } from 'pinia'

// Install Pinia and Element Plus so view components can use stores and UI components
const pinia = createPinia()
config.global.plugins = [pinia, ElementPlus]

// Register Element Plus icons so templates using them don't warn
config.global.components = config.global.components || {}
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  // Only register if not already provided
  if (!config.global.components[key]) {
    config.global.components[key] = component
  }
}

// Mock vue-router composables used in view components
vi.mock('vue-router', async (importOriginal) => {
  const actual = await importOriginal()

  return {
    ...actual,
    useRouter: () => ({
      push: vi.fn(),
      replace: vi.fn()
    }),
    useRoute: () => ({
      query: {}
    })
  }
})

// Stub vue-i18n so components can call useI18n without installing the plugin
vi.mock('vue-i18n', () => {
  const t = (key) => key

  return {
    useI18n: () => ({
      t,
      locale: { value: 'zh-CN' }
    })
  }
})

// Stub axios globally to avoid real HTTP calls in tests
vi.mock('axios', () => {
  const createInstance = () => {
    const resolvedValue = {
      code: 0,
      list: [],
      total: 0,
      stat: {}
    }

    // Axios instance behaves like both a function(config)
    // and an object with HTTP helpers like instance.post()
    const instance = vi.fn(() => Promise.resolve(resolvedValue))
    instance.post = vi.fn(() => Promise.resolve(resolvedValue))
    instance.interceptors = {
      request: { use: vi.fn() },
      response: { use: vi.fn() }
    }

    return instance
  }

  return {
    default: {
      create: createInstance
    }
  }
})
