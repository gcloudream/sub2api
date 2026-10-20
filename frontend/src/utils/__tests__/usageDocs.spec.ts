import { describe, expect, it } from 'vitest'
import { buildCodexConfig, getPlatformGuide, isValidModelId, platformOptions } from '../usageDocs'

describe('public tutorial configuration generation', () => {
  it.each(['', 'YOUR_MODEL_ID', 'sk-example-key', 'SK_example', 'Bearer-token', 'gpt "injected"', 'gpt\nmodel_provider="other"', 'gpt;echo', '$(whoami)'])('rejects credentials and command/config injection: %s', id => {
    expect(isValidModelId(id)).toBe(false)
    expect(buildCodexConfig(id)).toContain('model = "YOUR_MODEL_ID"')
  })

  it.each(['gpt-example', 'gpt-example-codex', 'vendor/model:latest'])('binds the exact selected model consistently: %s', id => {
    expect(isValidModelId(id)).toBe(true)
    const config = buildCodexConfig(id)
    expect(config).toContain('model = "' + id + '"')
    expect(config).toContain('model_provider = "fantasy"')
    expect(config).toContain('[model_providers.fantasy]')
    expect(config.indexOf('model_provider =')).toBeLessThan(config.indexOf('[model_providers.fantasy]'))
    expect(config).toContain('env_key = "FANTASY_API_KEY"')
    expect(config).not.toContain('experimental_bearer_token')
    expect(config).not.toContain('requires_openai_auth = true')
    for (const option of platformOptions) {
      const command = getPlatformGuide(option.id).verifyResponses(id)
      const json = command.match(/'(\{"model":.+\})'/)?.[1]
      expect(json).toBeDefined()
      expect(JSON.parse(json!)).toMatchObject({ model: id, store: false, max_output_tokens: 512 })
      expect(command).toContain('https://gcd.cab/v1/responses')
      expect(command).not.toContain('/v1/v1/')
    }
  })

  it('keeps Windows PowerShell and Unix/WSL commands in separate environments', () => {
    const windows = getPlatformGuide('windows')
    const wsl = getPlatformGuide('wsl')
    expect(windows.verifyModels).toContain('Invoke-RestMethod')
    expect(windows.keySession).toContain('$env:FANTASY_API_KEY')
    expect(windows.verifyModels).not.toContain('curl ')
    expect(wsl.wslSetup).toContain('wsl --install')
    expect(wsl.install).not.toContain('wsl --install')
    expect(wsl.backup).toContain('$HOME/.codex')
    expect(wsl.backup).not.toContain('$env:CODEX_HOME')
    for (const platform of ['macos', 'linux', 'wsl'] as const) {
      const guide = getPlatformGuide(platform)
      expect(guide.backup).toContain('CODEX_HOME')
      expect(guide.edit).not.toContain('auth.json')
      expect(guide.verifyModels).toContain('Authorization: Bearer $FANTASY_API_KEY')
    }
  })
})
