import { mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import UsageDocsView from '@/views/UsageDocsView.vue'
import { getPlatformGuide, type DocsPlatform } from '@/utils/usageDocs'

const mocks = vi.hoisted(() => ({
  copy: vi.fn(),
  fetchSettings: vi.fn(),
  app: {
    publicSettingsLoaded: true,
    cachedPublicSettings: { site_name: '幻想家', site_logo: '' },
    siteName: '幻想家',
    siteLogo: '',
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ ...mocks.app, fetchPublicSettings: mocks.fetchSettings }),
  useAuthStore: () => ({ isAuthenticated: false, isAdmin: false }),
}))
vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard: mocks.copy }),
}))

function render() {
  return mount(UsageDocsView, {
    global: {
      stubs: {
        RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
        Icon: { template: '<svg aria-hidden="true" />' },
      },
    },
  })
}

function codeBlock(wrapper: VueWrapper, label: string) {
  const block = wrapper.findAll('.docs-code-block').find(item => item.find('.docs-code-header > span').text() === label)
  expect(block, `Missing code block: ${label}`).toBeDefined()
  return block!
}

describe('UsageDocsView', () => {
  let wrapper: VueWrapper | undefined
  let fetchSpy: ReturnType<typeof vi.fn>
  let xhrOpenSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    vi.clearAllMocks()
    mocks.copy.mockResolvedValue(true)
    fetchSpy = vi.fn()
    vi.stubGlobal('fetch', fetchSpy)
    xhrOpenSpy = vi.spyOn(XMLHttpRequest.prototype, 'open')
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = undefined
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('renders publicly without a key input, model request, or generated paid command', () => {
    wrapper = render()
    expect(wrapper.get('h1').text()).toContain('第一次对话')
    expect(wrapper.findAll('input')).toHaveLength(1)
    expect(wrapper.get('input').attributes('id')).toBe('docs-model-id')
    expect(wrapper.find('input[type="password"]').exists()).toBe(false)
    expect(wrapper.find('a[href="/login"]').exists()).toBe(true)
    expect(wrapper.find('a[href="/keys"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('--data-raw')
    expect(wrapper.text()).not.toContain('Invoke-RestMethod -Method Post')
    expect(mocks.fetchSettings).not.toHaveBeenCalled()
    expect(fetchSpy).not.toHaveBeenCalled()
    expect(xhrOpenSpy).not.toHaveBeenCalled()
  })

  it.each<DocsPlatform>(['windows', 'macos', 'linux', 'wsl'])('switches backup and verification commands together for %s', async (platform) => {
    wrapper = render()
    await wrapper.get(`#platform-${platform}`).trigger('click')
    expect(wrapper.get(`#platform-${platform}`).attributes('aria-selected')).toBe('true')
    expect(codeBlock(wrapper, '确认目录并备份 config.toml').get('pre').text()).toBe(getPlatformGuide(platform).backup)
    expect(codeBlock(wrapper, '查询可用模型（不生成内容）').get('pre').text()).toBe(getPlatformGuide(platform).verifyModels)
    if (platform === 'windows') {
      expect(codeBlock(wrapper, '确认目录并备份 config.toml').text()).toContain('$env:CODEX_HOME')
      expect(codeBlock(wrapper, '查询可用模型（不生成内容）').text()).toContain('Invoke-RestMethod')
    } else {
      expect(codeBlock(wrapper, '确认目录并备份 config.toml').text()).toContain('$HOME/.codex')
      expect(codeBlock(wrapper, '查询可用模型（不生成内容）').text()).toContain('curl ')
    }
    if (platform === 'wsl') {
      const setup = codeBlock(wrapper, 'Windows 管理员 PowerShell · 安装 / 启动 WSL2').get('pre').text()
      expect(setup).toContain('wsl --install')
      expect(setup).not.toContain('npm install')
      expect(codeBlock(wrapper, '安装与版本检查').text()).not.toContain('wsl --install')
    }
    expect(fetchSpy).not.toHaveBeenCalled()
    expect(xhrOpenSpy).not.toHaveBeenCalled()
  })

  it('does not generate configuration or a paid request from an invalid model ID', async () => {
    wrapper = render()
    const input = wrapper.get('#docs-model-id')
    await input.setValue('model"; injected command')
    expect(input.attributes('aria-invalid')).toBe('true')
    expect(wrapper.findAll('.docs-code-header > span').some(node => node.text().startsWith('config.toml ·'))).toBe(false)
    expect(wrapper.text()).not.toContain('--data-raw')
    expect(wrapper.text()).not.toContain('Invoke-RestMethod -Method Post')
    expect(fetchSpy).not.toHaveBeenCalled()
  })

  it('keeps the selected model identical in config and both platform Responses commands', async () => {
    wrapper = render()
    await wrapper.get('#docs-model-id').setValue('test-codex-model')
    const config = codeBlock(wrapper, 'config.toml · 幻想家 Responses 接入').get('pre').text()
    expect(config).toContain('model = "test-codex-model"')
    expect(config).toContain('base_url = "https://gcd.cab/v1"')
    expect(config).toContain('env_key = "FANTASY_API_KEY"')
    expect(config).toContain('requires_openai_auth = false')
    expect(codeBlock(wrapper, '手动验证 Responses（可能计费）').get('pre').text()).toContain('"model":"test-codex-model"')
    await wrapper.get('#platform-windows').trigger('click')
    expect(codeBlock(wrapper, 'config.toml · 幻想家 Responses 接入').get('pre').text()).toBe(config)
    const windowsRequest = codeBlock(wrapper, '手动验证 Responses（可能计费）').get('pre').text()
    expect(windowsRequest).toContain('"model":"test-codex-model"')
    expect(windowsRequest).toContain('Invoke-RestMethod -Method Post')
    expect(fetchSpy).not.toHaveBeenCalled()
    expect(xhrOpenSpy).not.toHaveBeenCalled()
  })

  it('copies only the command text without running a request', async () => {
    wrapper = render()
    await wrapper.get('#docs-model-id').setValue('test-codex-model')
    const block = codeBlock(wrapper, '手动验证 Responses（可能计费）')
    const command = block.get('pre').text()
    await block.get('button').trigger('click')
    expect(mocks.copy).toHaveBeenCalledTimes(1)
    expect(mocks.copy).toHaveBeenCalledWith(command)
    expect(fetchSpy).not.toHaveBeenCalled()
    expect(xhrOpenSpy).not.toHaveBeenCalled()
  })
})
