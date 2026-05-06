import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import GlobalSearch from '../GlobalSearch.vue'
import { createRouter, createMemoryHistory } from 'vue-router'

// Mock the API request module
vi.mock('@/services/api', () => ({
  default: {
    get: vi.fn()
  }
}))

describe('GlobalSearch', () => {
  let router: any

  beforeEach(() => {
    router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/', component: { template: '<div>Home</div>' } },
        { path: '/pipeline/templates', component: { template: '<div>Templates</div>' } },
        { path: '/pipeline/credentials', component: { template: '<div>Credentials</div>' } },
        { path: '/pipeline/variables', component: { template: '<div>Variables</div>' } },
        { path: '/healthcheck', component: { template: '<div>Health</div>' } },
        { path: '/healthcheck/ssl-cert', component: { template: '<div>SSL</div>' } },
        { path: '/feature-flags', component: { template: '<div>Flags</div>' } },
        { path: '/system/monitor', component: { template: '<div>Monitor</div>' } }
      ]
    })
  })

  it('应该渲染搜索按钮', () => {
    const wrapper = mount(GlobalSearch, {
      global: {
        plugins: [router]
      }
    })
    expect(wrapper.find('button').exists()).toBe(true)
  })

  it('应该能够搜索菜单项 - 模板市场', async () => {
    const wrapper = mount(GlobalSearch, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.find('button').trigger('click')
    await nextTick()

    const input = wrapper.find('input')
    await input.setValue('模板')
    await nextTick()
    
    await new Promise(resolve => setTimeout(resolve, 400))
    await nextTick()

    const searchResults = wrapper.find('.search-results')
    expect(searchResults.exists()).toBe(true)
  })

  it('应该能够搜索菜单项 - 构建缓存', async () => {
    const wrapper = mount(GlobalSearch, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.find('button').trigger('click')
    await nextTick()

    const input = wrapper.find('input')
    await input.setValue('缓存')
    await nextTick()
    
    await new Promise(resolve => setTimeout(resolve, 400))
    await nextTick()

    const searchResults = wrapper.find('.search-results')
    expect(searchResults.exists()).toBe(true)
  })

  it('应该能够搜索菜单项 - 构建统计', async () => {
    const wrapper = mount(GlobalSearch, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.find('button').trigger('click')
    await nextTick()

    const input = wrapper.find('input')
    await input.setValue('统计')
    await nextTick()
    
    await new Promise(resolve => setTimeout(resolve, 400))
    await nextTick()

    const searchResults = wrapper.find('.search-results')
    expect(searchResults.exists()).toBe(true)
  })

  it('应该能够搜索菜单项 - 凭证管理', async () => {
    const wrapper = mount(GlobalSearch, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.find('button').trigger('click')
    await nextTick()

    const input = wrapper.find('input')
    await input.setValue('凭证')
    await nextTick()
    
    await new Promise(resolve => setTimeout(resolve, 400))
    await nextTick()

    const searchResults = wrapper.find('.search-results')
    expect(searchResults.exists()).toBe(true)
  })

  it('应该能够搜索菜单项 - 变量管理', async () => {
    const wrapper = mount(GlobalSearch, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.find('button').trigger('click')
    await nextTick()

    const input = wrapper.find('input')
    await input.setValue('变量')
    await nextTick()
    
    await new Promise(resolve => setTimeout(resolve, 400))
    await nextTick()

    const searchResults = wrapper.find('.search-results')
    expect(searchResults.exists()).toBe(true)
  })

  it('应该能够搜索菜单项 - SSL证书检查', async () => {
    const wrapper = mount(GlobalSearch, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.find('button').trigger('click')
    await nextTick()

    const input = wrapper.find('input')
    await input.setValue('ssl')
    await nextTick()
    
    await new Promise(resolve => setTimeout(resolve, 400))
    await nextTick()

    const searchResults = wrapper.find('.search-results')
    expect(searchResults.exists()).toBe(true)
  })

  it('应该能够搜索菜单项 - 功能开关', async () => {
    const wrapper = mount(GlobalSearch, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.find('button').trigger('click')
    await nextTick()

    const input = wrapper.find('input')
    await input.setValue('功能开关')
    await nextTick()
    
    await new Promise(resolve => setTimeout(resolve, 400))
    await nextTick()

    const searchResults = wrapper.find('.search-results')
    expect(searchResults.exists()).toBe(true)
  })

  it('应该能够搜索菜单项 - 系统监控', async () => {
    const wrapper = mount(GlobalSearch, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.find('button').trigger('click')
    await nextTick()

    const input = wrapper.find('input')
    await input.setValue('监控')
    await nextTick()
    
    await new Promise(resolve => setTimeout(resolve, 400))
    await nextTick()

    const searchResults = wrapper.find('.search-results')
    expect(searchResults.exists()).toBe(true)
  })

  it('应该支持模糊搜索', async () => {
    const wrapper = mount(GlobalSearch, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.find('button').trigger('click')
    await nextTick()

    const input = wrapper.find('input')
    await input.setValue('cache')
    await nextTick()
    
    await new Promise(resolve => setTimeout(resolve, 400))
    await nextTick()

    const searchResults = wrapper.find('.search-results')
    expect(searchResults.exists()).toBe(true)
  })

  it('空搜索应该显示快捷导航', async () => {
    const wrapper = mount(GlobalSearch, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.find('button').trigger('click')
    await nextTick()

    const quickNav = wrapper.find('.quick-nav')
    expect(quickNav.exists()).toBe(true)
  })

  it('应该在ESC键时关闭弹窗', async () => {
    const wrapper = mount(GlobalSearch, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.find('button').trigger('click')
    await nextTick()

    const input = wrapper.find('input')
    await input.trigger('keydown', { key: 'Escape' })
    await nextTick()

    // 弹窗应该关闭
    expect(wrapper.vm.showModal).toBe(false)
  })
})
