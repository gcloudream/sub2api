export type DocsPlatform = 'windows' | 'macos' | 'linux' | 'wsl'
export const docsUpdatedAt = '2026-10-20'
export const apiBase = 'https://gcd.cab/v1'
export const platformOptions: Array<{ id: DocsPlatform; label: string; terminal: string }> = [
  { id: 'windows', label: 'Windows', terminal: 'PowerShell（普通权限）' },
  { id: 'macos', label: 'macOS', terminal: '终端 · zsh / bash' },
  { id: 'linux', label: 'Linux', terminal: '终端 · bash / zsh' },
  { id: 'wsl', label: 'Linux / WSL2', terminal: 'WSL 发行版内的 Linux 终端' },
]
export const endpointRows = [
  { label: '网站与控制台', url: 'https://gcd.cab', description: '注册、充值、创建密钥和查看用量的网页。' },
  { label: 'Codex API Base URL', url: apiBase, description: '填入 model_providers.fantasy.base_url，保留末尾 /v1。' },
  { label: '可用模型列表', url: apiBase + '/models', description: 'GET 请求；需要你的幻想家 API 密钥。' },
  { label: 'Responses 接口', url: apiBase + '/responses', description: 'POST 请求；Codex 使用的模型协议，生成会消耗额度。' },
]
export const docsSections = [
  {
    id: 'prepare', eyebrow: '01 / PREPARE', title: '准备账户与 API 密钥',
    paragraphs: [
      '注册登录幻想家，按需充值，再进入「API 密钥」创建密钥。为 Codex 选择管理员提供的 OpenAI / Codex 分组，确认状态、配额和有效期正常。',
      '密钥绑定的分组决定可用模型、价格和路由。没有合适分组或模型时联系站点管理员，不要猜测模型名。',
      '这里需要幻想家 API 密钥，不是网站登录密码或 ChatGPT 密码。本页不收集、保存或验证密钥，也不会自动发送计费请求。',
    ],
    steps: ['登录控制台 → 按需充值。', 'API 密钥 → 创建密钥并选择 OpenAI / Codex 分组。', '妥善保存密钥；点击该密钥的「使用密钥」可查看专属配置和模型目录。'],
  },
  {
    id: 'address', eyebrow: '02 / ENDPOINTS', title: '区分网页地址与请求地址',
    paragraphs: [
      'Codex 的 Base URL 使用 https://gcd.cab/v1。客户端会追加 /responses，不要填写网页的 /home、/login，也不要拼成 /v1/v1。',
      '认证格式为 Authorization: Bearer 你的幻想家密钥。复制示例不会发送请求，只有在自己的终端运行命令时才会请求 API。',
    ],
  },
  {
    id: 'install', eyebrow: '03 / INSTALL', title: '安装 Codex CLI',
    paragraphs: [
      '下方统一使用官方 npm 包。先从 Node.js 官网安装当前 LTS（包含 npm），重新打开终端后检查版本。已有 Codex 时先运行 codex --version，不必重复安装。',
      'Windows 使用 PowerShell，macOS / Linux 使用 bash 或 zsh。npm 的全局目录应属于当前用户；EACCES 错误应先修复用户级 Node 安装，不要直接使用 sudo npm install。',
      'WSL2 先在管理员 PowerShell 中安装，再进入发行版执行 Linux 命令。Windows 与 WSL 的 Node、Codex、环境变量、配置文件需分别设置；WSL1 不适用于当前 Codex。',
    ],
  },
  {
    id: 'configure', eyebrow: '04 / CONFIGURE', title: '备份并合并配置',
    paragraphs: [
      '先备份 config.toml，再用本机编辑器合并。model、model_provider、web_search 等顶层项必须放在第一个 [表名] 之前；已有同名键或 [model_providers.fantasy] 时编辑原条目，不要重复追加。',
      '本方案使用独立的 fantasy 提供商和 FANTASY_API_KEY 环境变量，无需覆盖 auth.json，也无需退出原有 ChatGPT 登录。不要混用 env_key、直接 token 和命令认证。',
      '若设置过 CODEX_HOME，以该目录为准。用户级文件与项目 .codex/config.toml 不同；模型提供商配置放在用户目录，凭据不要提交到项目。',
    ],
  },
  {
    id: 'verify', eyebrow: '05 / VERIFY', title: '分三步确认接入成功',
    paragraphs: [
      '先获取模型列表，从 data[].id 复制精确模型 ID。列表由当前密钥分组决定；列表成功不代表每个模型都支持 Codex 或所有工具功能。',
      '手动发送一次短 Responses 请求会消耗少量站内额度。检查 HTTP 状态与响应 status；完成时 output 数组中应有 message / output_text，文本为 fantasy-ok，而非只寻找顶层 output_text。',
      '重新启动 Codex、新建会话，在 Codex 交互窗口输入 /status 确认提供商 fantasy 和选定模型，再发一句「只回复 fantasy-ok」。最后到幻想家「使用记录」核对密钥、模型、时间与额度记录，不靠模型自报身份判断路由。',
    ],
  },
]
export function isValidModelId(model: string): boolean {
  return model !== 'YOUR_MODEL_ID' && !/^(sk[-_]|bearer)/i.test(model) && /^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$/.test(model)
}
export function buildCodexConfig(model: string): string {
  const safe = isValidModelId(model) ? model : 'YOUR_MODEL_ID'
  return [
    '# 合并到用户级 config.toml，顶层项放在第一个 [表名] 之前',
    'model_provider = "fantasy"',
    'model = "' + safe + '"',
    '# 先验证基础对话，确认支持后再调整其他功能',
    'web_search = "disabled"',
    '',
    '[model_providers.fantasy]',
    'name = "幻想家"',
    'base_url = "' + apiBase + '"',
    'wire_api = "responses"',
    'env_key = "FANTASY_API_KEY"',
    'requires_openai_auth = false',
    'supports_websockets = false',
  ].join('\n')
}
const windowsDir = "$codexDir = if ($env:CODEX_HOME) { $env:CODEX_HOME } else { Join-Path $HOME '.codex' }"
const unixDir = 'codex_dir="$' + '{CODEX_HOME:-$HOME/.codex}"'
export function getPlatformGuide(platform: DocsPlatform) {
  const windows = platform === 'windows'
  const install = [
    '# 先从 https://nodejs.org/ 安装 Node.js LTS，重新打开终端',
    'node --version',
    'npm --version',
    'npm install -g @openai/codex@latest',
    'codex --version',
  ].join('\n')
  const wslPrelude = [
    '# 仅这两行在管理员 PowerShell 执行；已有 WSL2 可跳过',
    'wsl --install',
    'wsl',
    '',
    '# 进入 Linux 发行版，在该环境安装 Node.js LTS，再运行下方命令',
  ].join('\n')
  return {
    wslSetup: platform === 'wsl' ? wslPrelude : '',
    install,
    backup: windows ? [
      windowsDir,
      'New-Item -ItemType Directory -Force -Path $codexDir | Out-Null',
      '$codexDir = (Resolve-Path -LiteralPath $codexDir).Path',
      'Write-Host "配置目录：$codexDir"',
      "$configPath = Join-Path $codexDir 'config.toml'",
      'if (Test-Path $configPath) {',
      "  $backupPath = $configPath + '.bak-' + (Get-Date -Format 'yyyyMMdd-HHmmss')",
      '  Copy-Item -LiteralPath $configPath -Destination $backupPath',
      '  Write-Host "备份位置：$backupPath"',
      '}',
    ].join('\n') : [
      unixDir,
      'mkdir -p "$codex_dir"',
      'codex_dir="$(cd "$codex_dir" && pwd)"',
      'printf "配置目录：%s\\n" "$codex_dir"',
      'if [ -f "$codex_dir/config.toml" ]; then',
      '  backup_path="$codex_dir/config.toml.bak-$(date +%Y%m%d-%H%M%S)"',
      '  cp "$codex_dir/config.toml" "$backup_path"',
      '  printf "备份位置：%s\\n" "$backup_path"',
      'fi',
    ].join('\n'),
    edit: windows ? [
      windowsDir,
      "notepad (Join-Path $codexDir 'config.toml')",
      '# 文件名应为 config.toml，避免保存成 config.toml.txt',
    ].join('\n') : [
      unixDir,
      'touch "$codex_dir/config.toml"',
      platform === 'macos' ? 'open -a TextEdit "$codex_dir/config.toml"' : 'nano "$codex_dir/config.toml"',
      platform === 'macos' ? '# TextEdit 使用「格式 → 制作纯文本」，保留 .toml 扩展名' : '# nano：Ctrl+O 保存，Enter 确认文件名，Ctrl+X 退出',
    ].join('\n'),
    keySession: windows ? [
      "$secureKey = Read-Host '粘贴幻想家 API 密钥（输入不回显）' -AsSecureString",
      "$env:FANTASY_API_KEY = [System.Net.NetworkCredential]::new('', $secureKey).Password",
      'Remove-Variable secureKey',
      '# 当前 PowerShell 及其启动的 Codex 可以读取该变量',
    ].join('\n') : [
      '# bash / zsh：输入不回显，不把密钥作为命令写入历史',
      "printf '粘贴幻想家 API 密钥（输入不回显）：'",
      'read -r -s FANTASY_API_KEY',
      "printf '\\n'",
      'export FANTASY_API_KEY',
    ].join('\n'),
    verifyModels: windows ? [
      '$headers = @{ Authorization = "Bearer $env:FANTASY_API_KEY" }',
      "$models = Invoke-RestMethod -Method Get -Uri '" + apiBase + "/models' -Headers $headers",
      '$models.data | Select-Object id',
    ].join('\n') : [
      "curl --fail-with-body --silent --show-error '" + apiBase + "/models' \\",
      '  -H "Authorization: Bearer $FANTASY_API_KEY"',
      '# 查看 JSON 中 data 数组内的 id',
    ].join('\n'),
    verifyResponses: (model: string) => {
      const safe = isValidModelId(model) ? model : 'YOUR_MODEL_ID'
      const payload = JSON.stringify({ model: safe, input: 'Reply with exactly: fantasy-ok', max_output_tokens: 512, store: false })
      return windows ? [
        '# 此请求会消耗少量站内额度，确认后再运行',
        '$headers = @{ Authorization = "Bearer $env:FANTASY_API_KEY" }',
        "$body = '" + payload + "'",
        "Invoke-RestMethod -Method Post -Uri '" + apiBase + "/responses' -Headers $headers -ContentType 'application/json' -Body $body",
      ].join('\n') : [
        '# 此请求会消耗少量站内额度，确认后再运行',
        "curl --fail-with-body --silent --show-error '" + apiBase + "/responses' \\",
        '  -H "Authorization: Bearer $FANTASY_API_KEY" \\',
        "  -H 'Content-Type: application/json' \\",
        "  --data-raw '" + payload + "'",
      ].join('\n')
    },
    start: 'codex\n# /status 在 Codex 交互窗口输入，不是在系统 Shell 中运行',
    persistHint: windows
      ? '当前会话设置不自动永久保存。长期使用可在 Windows「环境变量 → 当前用户变量」新增 FANTASY_API_KEY，填入自己的密钥；完全退出并重新启动终端和桌面客户端，必要时重新登录 Windows。环境变量也应按凭据保护，勿把值发到聊天或仓库。'
      : '新终端可重新运行上面的不回显输入。长期使用可用本机编辑器在 ~/.zshrc（macOS 默认 zsh）或 ~/.bashrc（bash）设置同名变量，再打开新终端；密钥会以明文保存在文件中，应只允许本人访问且不要提交到仓库。桌面应用未必读取这些文件，需另外把变量传入桌面进程。',
    catalogPath: windows
      ? '示例 C:\\Users\\你的用户名\\.codex\\codex-models.json，以以下命令输出的实际绝对路径为准。'
      : platform === 'macos'
        ? '示例 /Users/你的用户名/.codex/codex-models.json，以以下命令输出为准。'
        : '示例 /home/你的用户名/.codex/codex-models.json；WSL 使用发行版内用户目录，以以下命令输出为准。',
    catalog: windows ? [
      windowsDir,
      '$codexDir = (Resolve-Path -LiteralPath $codexDir).Path',
      "Join-Path $codexDir 'codex-models.json'",
      '# 把「使用密钥」下载的目录保存到此实际位置，再填写绝对路径',
    ].join('\n') : [
      unixDir,
      'codex_dir="$(cd "$codex_dir" && pwd)"',
      'printf "%s/codex-models.json\\n" "$codex_dir"',
      '# 把「使用密钥」下载的目录保存到此实际位置，再填写绝对路径',
    ].join('\n'),
    restore: windows ? [
      windowsDir,
      "$backupPath = Read-Host '输入要恢复的 config.toml.bak 文件完整路径'",
      "if (Test-Path -LiteralPath $backupPath -PathType Leaf) { Copy-Item -LiteralPath $backupPath -Destination (Join-Path $codexDir 'config.toml') }",
      '# 仅在确需撤销本次修改时执行；不改 auth.json',
    ].join('\n') : [
      unixDir,
      "printf '输入要恢复的 config.toml.bak 文件完整路径：'",
      'read -r backup_path',
      'if [ -f "$backup_path" ]; then cp "$backup_path" "$codex_dir/config.toml"; fi',
      '# 仅在确需撤销本次修改时执行；不改 auth.json',
    ].join('\n'),
  }
}
export const troubleshooting = [
  { title: '找不到 codex / npm / node', symptom: 'command not found 或「不是内部或外部命令」。', solution: '安装 Node.js LTS 后重新打开终端，检查 node --version、npm --version、codex --version。macOS/Linux 用 command -v codex，Windows 用 Get-Command codex 检查 PATH。WSL 需要单独安装 Linux 版本。' },
  { title: 'Windows 提示脚本无法运行', symptom: 'PowerShell 阻止 npm.ps1 或 codex.ps1。', solution: '先试 npm.cmd install -g @openai/codex@latest 和 codex.cmd，或使用官方独立安装器，不必全局关闭 PowerShell 执行策略。' },
  { title: '401：认证未通过', symptom: 'invalid_api_key、Unauthorized 或缺少密钥。', solution: '检查幻想家密钥状态，避免多余引号、空格或 Bearer Bearer；env_key 填变量名 FANTASY_API_KEY。变量应存在于启动 Codex 的进程中，且本 provider 不应设 requires_openai_auth=true。报错截图勿暴露密钥。' },
  { title: '403：当前访问无权限', symptom: 'Forbidden、分组/模型/IP 访问限制。', solution: '按错误体检查分组、模型权限、有效期、IP 白名单及站点策略。更换登录账号不等于给当前密钥增加权限，可向管理员提供脱敏错误和时间。' },
  { title: '404：地址或模型不匹配', symptom: 'Not Found、model_not_found。', solution: 'Base URL 应为 https://gcd.cab/v1，不是 /home、/v1/responses 或 /v1/v1。核对 data[].id 的完整拼写、分组及 Responses/Codex 兼容性，别直接复制官网示例模型名。' },
  { title: '429：频率或额度限制', symptom: 'Too Many Requests、rate_limit 或 quota。', solution: '按 error.code/message 区分并发/速率限制与余额/配额不足。限流时降低并发并按 Retry-After 等待；额度不足时核对余额和密钥配额。不要不停重试或把所有429都当作需要充值。' },
  { title: 'TOML 失败 / 仍用原提供商', symptom: '重复 key/table、解析错误，或 /status 仍是旧 provider。', solution: '顶层项放在首个表之前，编辑已有条目而非重复追加。检查 CODEX_HOME、-c/--config、--profile 和管理策略覆盖，完全退出并新建会话，不要删除整个 .codex 目录。' },
  { title: '未知模型 / 模型目录不存在', symptom: '模型不被识别或无法读取 model_catalog_json。', solution: '从该密钥「使用密钥 → Codex CLI」获取并下载目录。仅在文件确实存在时填写实际绝对路径；Windows 用单引号保护反斜杠，不能在 TOML 内用 %USERPROFILE% 等未展开变量。/v1/models JSON 不能直接充当 Codex 目录。' },
  { title: 'CLI 可用，桌面不可用', symptom: 'GUI 缺少 FANTASY_API_KEY 或仍使用原登录。', solution: 'CLI 和 GUI 不一定共享环境变量。Windows 检查当前用户变量并彻底重启；macOS 按图形会话说明传入变量再重启。相同配置路径不代表相同凭据环境。' },
  { title: '超时、TLS 或上游不可用', symptom: '连接超时、证书错误、502/503。', solution: '核对本机时间、DNS、代理和 HTTPS 网络。解决证书信任问题，不使用 curl -k 或关闭校验。上游故障稍后重试，向站点提供脱敏时间和错误。' },
  { title: '输出不完整 / 工具不可用', symptom: 'status=incomplete、max_output_tokens 或工具报错。', solution: '明确因512输出上限中止时，可在确认预算后适当提高上限。文本成功只证明基础链路；工具、图片、多轮和WebSocket需模型及站点共同支持，初次先用 HTTP Responses。' },
]
export const officialSources = [
  { label: 'Codex 安装与开始使用', url: 'https://learn.chatgpt.com/docs/codex/cli' },
  { label: '网关 API / provider 凭据', url: 'https://learn.chatgpt.com/docs/enterprise/connect-to-a-gateway' },
  { label: '配置与优先级', url: 'https://learn.chatgpt.com/docs/config-file/config-basic' },
  { label: '完整配置参考', url: 'https://learn.chatgpt.com/docs/config-file/config-reference' },
  { label: '认证方式', url: 'https://learn.chatgpt.com/docs/auth' },
  { label: '环境变量与独立安装器', url: 'https://learn.chatgpt.com/docs/config-file/environment-variables' },
  { label: 'WSL2 配置', url: 'https://learn.chatgpt.com/docs/windows/wsl' },
  { label: '模型列表接口', url: 'https://developers.openai.com/api/reference/resources/models/methods/list' },
  { label: 'Responses 文本生成', url: 'https://developers.openai.com/api/docs/guides/text' },
  { label: 'API 常见错误', url: 'https://developers.openai.com/api/docs/guides/error-codes' },
  { label: 'Node.js 官方下载（选择 LTS）', url: 'https://nodejs.org/' },
]
