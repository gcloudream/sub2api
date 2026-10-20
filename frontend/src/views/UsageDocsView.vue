<template>
  <div class="usage-docs lux-ambient" data-testid="usage-docs">
    <a class="docs-skip" href="#docs-main">跳转到教程正文</a>
    <div class="docs-wrap">
      <header class="docs-header">
        <router-link to="/home" class="lux-brand" aria-label="返回幻想家首页">
          <img :src="siteLogo || '/logo.svg'" :alt="siteName" />
          <span><span class="lux-brand-name">{{ siteName }}</span><small>AI API GATEWAY</small></span>
        </router-link>
        <nav class="docs-header-actions" aria-label="教程导航">
          <router-link to="/home" class="docs-home-link">首页</router-link>
          <button
            type="button"
            class="btn-ghost btn-icon"
            :aria-label="isDark ? '切换为浅色主题' : '切换为深色主题'"
            @click="toggleTheme"
          >
            <Icon :name="isDark ? 'sun' : 'moon'" size="md" />
          </button>
          <router-link :to="dashboardPath" class="btn btn-secondary docs-console-link">
            {{ isAuthenticated ? '进入控制台' : '登录 / 注册' }}
            <Icon name="arrowRight" size="sm" />
          </router-link>
        </nav>
      </header>

      <section class="docs-hero" aria-labelledby="docs-title">
        <div>
          <p class="lux-eyebrow">GETTING STARTED / CODEX</p>
          <h1 id="docs-title">从第一把密钥，<br />到第一次对话。</h1>
          <p class="docs-lead">幻想家完整接入教程。选择设备系统，按顺序完成安装、配置与验证，让 Codex 使用你的中转站额度。</p>
          <div class="docs-hero-actions">
            <a href="#before-you-start" class="btn btn-primary">开始接入 <Icon name="arrowRight" size="sm" /></a>
            <a href="#troubleshooting" class="docs-text-link">遇到问题？查看排查说明</a>
          </div>
        </div>
        <div class="docs-hero-note">
          <span class="docs-note-mark" aria-hidden="true">01 / 03</span>
          <p>同一个请求地址，<br />适配你的开发环境。</p>
          <div class="docs-platform-labels"><span>Windows</span><span>macOS</span><span>Linux / WSL</span></div>
          <small>配置命令由你复制后执行。教程不会读取密钥，也不会自动发起模型请求。</small>
        </div>
      </section>

      <div class="docs-layout">
        <aside class="docs-sidebar" aria-label="教程目录">
          <details class="docs-toc" :open="tocOpen" @toggle="onTocToggle">
            <summary>本页目录 <span aria-hidden="true">⌄</span></summary>
            <nav>
              <a v-for="(item, index) in tableOfContents" :key="item.id" :href="`#${item.id}`">
                <span>{{ String(index + 1).padStart(2, '0') }}</span>{{ item.label }}
              </a>
            </nav>
          </details>
          <div class="docs-sidebar-footer">配置资料<br /><strong>{{ docsUpdatedAt.split('-').join('.') }}</strong><br />最后核对日期</div>
        </aside>

        <main id="docs-main" class="docs-content">
          <section id="before-you-start" class="docs-section" aria-labelledby="before-title">
            <div class="docs-section-heading"><span class="lux-eyebrow">01 / PREPARE</span><h2 id="before-title">准备账号与 API 密钥</h2></div>
            <p>网站登录账号、API 密钥和 OpenAI / ChatGPT 登录是不同的凭据。接入幻想家时，使用在本站创建的 API 密钥。</p>
            <ol class="docs-steps">
              <li><span class="docs-step-number">1</span><div><h3>登录并准备额度</h3><p>在<router-link to="/login">登录页面</router-link>进入账号；新用户按页面提示注册。在<router-link to="/purchase">购买额度</router-link>页面充值，随后到控制台核对可用余额。</p></div></li>
              <li><span class="docs-step-number">2</span><div><h3>创建适用于 Codex 的密钥</h3><p>打开<router-link to="/keys">API 密钥</router-link>，创建密钥并选择支持 OpenAI / Codex 的可用分组。Claude 分组的密钥和模型不能直接套用本页的 Codex 配置。</p></div></li>
              <li><span class="docs-step-number">3</span><div><h3>保存密钥，确认可用模型</h3><p>复制并妥善保存完整密钥；可以设置到期时间和额度限制。在密钥旁点击“使用密钥”查看当前分组的配置说明。后续先查询模型列表，再填写准确的模型 ID。</p></div></li>
            </ol>
            <div class="docs-callout"><strong>密钥只放在你自己的终端或密钥管理器中。</strong><p>本页没有密钥输入框。不要把密钥提交到 Git、公开截图、聊天或工单；发现泄露后应到 API 密钥页面停用并重新创建。</p></div>
          </section>

          <section id="endpoints" class="docs-section" aria-labelledby="endpoints-title">
            <div class="docs-section-heading"><span class="lux-eyebrow">02 / ENDPOINTS</span><h2 id="endpoints-title">请求地址怎么填</h2></div>
            <p>Codex 的 <code>base_url</code> 填写 OpenAI 兼容地址，保留末尾的 <code>/v1</code>。密钥通过 <code>Authorization: Bearer …</code> 请求头发送。</p>
            <div class="docs-endpoints">
              <article v-for="endpoint in endpointRows" :key="endpoint.url" class="docs-endpoint">
                <div><h3>{{ endpoint.label }}</h3><p>{{ endpoint.description }}</p></div>
                <div class="docs-endpoint-value"><code>{{ endpoint.url }}</code><button type="button" :aria-label="`复制${endpoint.label}`" @click="copyToClipboard(endpoint.url)"><Icon name="copy" size="sm" /></button></div>
              </article>
            </div>
            <p class="docs-caption">不要把网站登录地址 <code>/login</code>、控制台路径或完整的 <code>/v1/responses</code> 写成 Codex 的 base_url，也不要重复添加 <code>/v1/v1</code>。</p>
          </section>

          <section id="install" class="docs-section" aria-labelledby="install-title">
            <div class="docs-section-heading"><span class="lux-eyebrow">03 / YOUR DEVICE</span><h2 id="install-title">选择系统，安装 Codex</h2></div>
            <p>下面的安装、文件路径和验证命令随系统一起切换。已有 Codex 的用户先确认版本，再继续备份配置。</p>
            <div class="docs-platform-tabs" role="tablist" aria-label="操作系统">
              <button
                v-for="option in platformOptions"
                :id="`platform-${option.id}`"
                :key="option.id"
                type="button"
                role="tab"
                :aria-selected="platform === option.id"
                aria-controls="platform-guide"
                :tabindex="platform === option.id ? 0 : -1"
                :class="{ 'is-selected': platform === option.id }"
                @click="platform = option.id"
                @keydown="onPlatformKeydown($event, option.id)"
              >{{ option.label }}</button>
            </div>
            <div id="platform-guide" role="tabpanel" :aria-labelledby="`platform-${platform}`" tabindex="0">
              <template v-if="platform === 'wsl'">
                <h3 class="docs-subheading">先在 Windows 安装并进入 WSL2</h3>
                <p>以下这一块仅在 Windows 的管理员 PowerShell 中执行。安装后如提示重启，先重启 Windows 并完成 Linux 发行版的用户名、密码初始化；已有 WSL2 可以跳过安装。</p>
                <CodeBlock label="Windows 管理员 PowerShell · 安装 / 启动 WSL2" :code="guide.wslSetup" />
                <p>进入 Linux 发行版后，下面所有命令都在该 Linux 终端运行，不要继续粘贴到 PowerShell。</p>
              </template>
              <div class="docs-terminal-label"><span class="docs-terminal-dot" aria-hidden="true"></span>{{ selectedPlatform?.terminal }}</div>
              <p v-if="platform === 'wsl'">先在 Windows 安装并打开 WSL 的 Linux 发行版，再在该 Linux 终端内运行下列命令。WSL 的 <code>~/.codex</code> 与 Windows 的配置目录相互独立。</p>
              <p v-else-if="platform === 'windows'">使用 Windows PowerShell。先安装 Node.js 的当前 LTS 版本并重新打开终端；如果 npm 脚本被执行策略拦截，参照下方 Windows 故障排查。</p>
              <p v-else>先安装 Node.js 的当前 LTS 版本。使用自己的用户运行 npm 与 Codex，不要用 sudo 启动 Codex。</p>
              <CodeBlock label="安装与版本检查" :code="guide.install" />
              <p class="docs-caption">Node.js 安装入口：<a href="https://nodejs.org/en/download" target="_blank" rel="noopener noreferrer">Node.js 官方下载</a>。其他安装方式见页尾 Codex 官方来源；本页不会代你执行安装。</p>
            </div>
          </section>

          <section id="configuration" class="docs-section" aria-labelledby="configuration-title">
            <div class="docs-section-heading"><span class="lux-eyebrow">04 / CONFIGURATION</span><h2 id="configuration-title">备份并修改配置文件</h2></div>
            <p class="docs-selected-platform">当前系统：<strong>{{ selectedPlatform?.label }}</strong><a href="#install">切换系统 ↑</a></p>
            <h3 class="docs-subheading">1. 先确认路径并备份</h3>
            <p>默认目录为 macOS / Linux / WSL 的 <code>~/.codex</code>、Windows 的 <code>%USERPROFILE%\.codex</code>。设置了 <code>CODEX_HOME</code> 时以它为准；下面命令会输出实际使用的目录并备份已有的 <code>config.toml</code>。</p>
            <CodeBlock label="确认目录并备份 config.toml" :code="guide.backup" />
            <p class="docs-caption">保留命令输出的备份路径，恢复时需要它。原有 <code>auth.json</code> 保持原样；不要把本站密钥覆盖进原有 ChatGPT 登录文件。</p>

            <h3 class="docs-subheading">2. 在当前终端设置会话密钥</h3>
            <p>运行命令后，在终端提示中输入自己的幻想家 API 密钥。输入不会出现在本页。环境变量只在当前会话及从它启动的子进程中生效。</p>
            <CodeBlock label="设置当前会话的 FANTASY_API_KEY" :code="guide.keySession" />
            <div class="docs-callout docs-callout-subtle"><strong>需要下次打开终端继续使用？</strong><p>{{ guide.persistHint }}</p><p>从桌面图标打开的 Codex 通常不会继承这个终端会话，详见下方“CLI 与桌面客户端”。</p></div>

            <h3 class="docs-subheading">3. 查询并填写完整模型 ID</h3>
            <p>先执行只读的模型列表请求，从返回的 <code>data[].id</code> 中复制一个当前密钥可用的完整 ID。模型可用性取决于密钥分组；不要只填“GPT”或“Codex”这类系列名称。</p>
            <CodeBlock label="查询可用模型（不生成内容）" :code="guide.verifyModels" />
            <div class="docs-model-field">
              <label for="docs-model-id">可用模型 ID <span>这不是密钥</span></label>
              <input id="docs-model-id" v-model="modelInput" type="text" maxlength="128" autocomplete="off" autocapitalize="none" spellcheck="false" placeholder="粘贴模型列表中 data[].id 的值" :aria-invalid="!!modelInput.trim() && !modelValid" aria-describedby="docs-model-hint" />
              <p id="docs-model-hint" :class="{ 'docs-field-error': !!modelInput.trim() && !modelValid }">{{ modelInput.trim() && !modelValid ? '请填写完整模型 ID，仅使用字母、数字、点、冒号、横线、下划线或斜线，不要粘贴密钥、空格或 JSON。' : '只用于生成下面的本地配置与验证命令，不会发送或保存这个输入。' }}</p>
            </div>

            <h3 class="docs-subheading">4. 合并 Codex 配置</h3>
            <CodeBlock label="打开实际配置文件" :code="guide.edit" />
            <p>新文件可以使用下方配置。已有配置时按同名键逐项合并，保留原来的项目、MCP、沙箱与其他 provider 配置，避免重复定义顶层键或同名 table。</p>
            <CodeBlock v-if="modelValid" label="config.toml · 幻想家 Responses 接入" :code="codexConfig" />
            <div v-else class="docs-code-placeholder"><Icon name="terminal" size="md" /><div><strong>填写可用模型 ID 后生成配置</strong><p>不会猜测你的密钥能访问哪些模型。</p></div></div>
            <p class="docs-caption">本页采用 <code>env_key = "FANTASY_API_KEY"</code>。它与现有 ChatGPT 登录文件分开；命令行启动时需要让进程读取到该环境变量。</p>

            <h3 class="docs-subheading">5. 核对模型目录的绝对路径</h3>
            <p><code>/v1/models</code> 是 API 模型列表，不能直接当成 Codex 的模型目录文件。需要自定义目录时，到<router-link to="/keys">API 密钥</router-link> → “使用密钥” → “Codex CLI” → “获取目录” → “下载目录”，将 <code>codex-models.json</code> 保存到实际 Codex 配置目录。</p>
            <CodeBlock label="打印模型目录的目标绝对路径" :code="guide.catalog" />
            <p>{{ guide.catalogPath }} 其中的用户名需要替换，在编辑器中确认文件确实存在。</p>
            <div class="docs-callout"><strong>已有 model_catalog_json 时先核对，再合并。</strong><p>它是顶层键，必须写在第一个 <code>[table]</code> 之前。填写真实的绝对路径；Windows 可用 TOML 单引号，例如 <code>model_catalog_json = 'C:\Users\你的用户名\.codex\codex-models.json'</code>。不要把 <code>%USERPROFILE%</code>、<code>$HOME</code> 或未展开变量直接写进配置。旧目录可能让新 provider 显示旧模型，请同时检查文件内容与路径。</p></div>
          </section>

          <section id="verification" class="docs-section" aria-labelledby="verification-title">
            <div class="docs-section-heading"><span class="lux-eyebrow">05 / VERIFY</span><h2 id="verification-title">分三层验证接入</h2></div>
            <p>命令执行成功、API 返回成功和 Codex 真正使用幻想家，是三个不同的检查点。按顺序确认，便于定位问题。</p>
            <ol class="docs-steps docs-verification">
              <li><span class="docs-step-number">1</span><div><h3>地址与密钥：模型列表</h3><p>上方 <code>/v1/models</code> 请求应返回成功，并列出模型 ID。401 优先检查密钥；404 优先检查地址；网络或 TLS 错误先解决连接问题。</p><span class="docs-check-tag">只读查询 · 不生成内容</span></div></li>
              <li><span class="docs-step-number">2</span><div><h3>模型与余额：一次最小 Responses 请求</h3><p>确认所填模型可用、账户有余额后，再手动运行下面的请求。它会生成少量内容，可能扣除额度；检查返回的响应 ID 与 status。完成时应能在 output 数组的 message / output_text 中找到 fantasy-ok，不能只寻找顶层 output_text。</p><span class="docs-check-tag docs-check-tag-paid">真实模型请求 · 可能计费</span></div></li>
            </ol>
            <CodeBlock v-if="modelValid" label="手动验证 Responses（可能计费）" :code="guide.verifyResponses(modelId)" />
            <div v-else class="docs-code-placeholder"><Icon name="terminal" size="md" /><div><strong>先填写模型 ID</strong><p>配置区与这里共用同一个模型选择。</p></div></div>
            <ol start="3" class="docs-steps docs-verification">
              <li><span class="docs-step-number">3</span><div><h3>客户端：确认 provider、模型和后台用量</h3><p>彻底退出旧 Codex 会话，在设置过密钥的同一个终端、自己的项目目录中重新启动。进入后用 <code>/status</code> 核对 provider 和模型，发送一句简短测试消息，再到<router-link to="/usage">使用记录</router-link>查看对应密钥、模型、时间与扣费。</p></div></li>
            </ol>
            <CodeBlock label="重新启动 Codex 并检查状态" :code="guide.start" />
            <p class="docs-caption"><code>codex login status</code> 反映的是已有登录状态，不能单独证明已连到幻想家。本站用量记录也可能稍有延迟，最终以对应请求与记录核对为准。</p>
          </section>

          <section id="desktop" class="docs-section" aria-labelledby="desktop-title">
            <div class="docs-section-heading"><span class="lux-eyebrow">06 / CLI &amp; DESKTOP</span><h2 id="desktop-title">终端与桌面客户端的区别</h2></div>
            <div class="docs-comparison">
              <article><p class="lux-eyebrow">CODEX CLI</p><h3>从设置密钥的终端启动</h3><p>CLI 会继承当前 shell 的环境变量，并读取当前用户的配置目录。更换终端、新开窗口、切换用户或设置 <code>CODEX_HOME</code> 后，需要重新核对配置和密钥环境。</p></article>
              <article><p class="lux-eyebrow">CODEX DESKTOP</p><h3>确认客户端实际读取的环境</h3><p>桌面图标启动的进程通常不会继承终端的临时变量。完全退出并重新启动，确认客户端支持自定义 provider，按其官方方式设置所需环境和配置；不能仅凭 CLI 测试成功就认定桌面客户端已切换。</p></article>
            </div>
            <p>Windows 原生与 WSL 是两套用户环境；macOS 的图形界面应用与 shell 环境也可能不同。不要在一个系统里改配置、在另一个系统里排查。未核实支持的桌面版本，先使用本教程的 CLI 路径验证。</p>
            <h3 class="docs-subheading">macOS：把变量传入当前图形登录会话</h3>
            <p>已在终端设置过密钥、并确认当前桌面版本支持自定义 provider 时，可以运行下列命令。随后完全退出 Codex 桌面应用，再从“应用程序”重新打开，核对实际 provider、模型与本站用量。</p>
            <CodeBlock label="macOS 图形会话环境（不修改 auth.json）" :code="macosDesktopEnvironment" />
            <p class="docs-caption">仅对当前用户的图形登录会话有效，重启或重新登录后需要重新设置；它不把密钥写进 config.toml。停止使用时可在终端执行 <code>launchctl unsetenv FANTASY_API_KEY</code>，然后重新启动相关客户端。Windows 请使用前述“当前用户环境变量”步骤并彻底重启客户端；Linux / WSL 先使用 CLI。</p>
          </section>

          <section id="troubleshooting" class="docs-section" aria-labelledby="troubleshooting-title">
            <div class="docs-section-heading"><span class="lux-eyebrow">07 / TROUBLESHOOTING</span><h2 id="troubleshooting-title">常见错误，逐项定位</h2></div>
            <div class="docs-faq">
              <details v-for="item in troubleshooting" :key="item.title">
                <summary>{{ item.title }}<span aria-hidden="true">+</span></summary>
                <div><p class="docs-faq-symptom">{{ item.symptom }}</p><p>{{ item.solution }}</p></div>
              </details>
            </div>
            <p class="docs-caption">联系站点支持时提供时间、脱敏后的错误码、模型 ID、系统与 Codex 版本即可。不要发送完整 API 密钥、auth.json 或包含敏感内容的终端日志。</p>
          </section>

          <section id="restore" class="docs-section" aria-labelledby="restore-title">
            <div class="docs-section-heading"><span class="lux-eyebrow">08 / RESTORE</span><h2 id="restore-title">需要退回原来的配置？</h2></div>
            <p>先退出 Codex，找到前面保存的备份路径。下面命令由你输入选定的备份文件，仅还原 <code>config.toml</code>，不会改动 <code>auth.json</code>。还原前也建议另存当前配置，以保留这次接入的内容。</p>
            <CodeBlock label="从选定备份还原 config.toml" :code="guide.restore" />
            <p>还原后重新打开客户端，用 <code>/status</code> 核对实际 provider。如果配置之前不存在，可以先将新建的配置改名留存，再按原先的官方登录方式启动；无需删除整个 <code>.codex</code> 目录。</p>
          </section>

          <section id="sources" class="docs-section docs-sources-section" aria-labelledby="sources-title">
            <div class="docs-section-heading"><span class="lux-eyebrow">09 / REFERENCES</span><h2 id="sources-title">官方资料与更新说明</h2></div>
            <p>本文配置资料最后核对于 <time :datetime="docsUpdatedAt">{{ docsUpdatedAt }}</time>。Codex 的安装方式、配置项和客户端支持范围可能随版本变化；遇到差异时，先核对本机 <code>codex --version</code> 与官方说明。</p>
            <div class="docs-source-links"><a v-for="source in officialSources" :key="source.url" :href="source.url" target="_blank" rel="noopener noreferrer">{{ source.label }}<Icon name="externalLink" size="sm" /></a></div>
          </section>
        </main>
      </div>
      <footer class="docs-footer"><p>© {{ new Date().getFullYear() }} {{ siteName }} · 接入指南</p><router-link to="/keys">准备好了吗？前往 API 密钥 <Icon name="arrowRight" size="sm" /></router-link></footer>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, ref } from 'vue'
import { useAppStore, useAuthStore } from '@/stores'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { sanitizeUrl } from '@/utils/url'
import {
  buildCodexConfig,
  docsUpdatedAt,
  endpointRows,
  getPlatformGuide,
  isValidModelId,
  officialSources,
  platformOptions,
  troubleshooting,
  type DocsPlatform,
} from '@/utils/usageDocs'

const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()
const platform = ref<DocsPlatform>('macos')
const tocOpen = ref(window.matchMedia?.('(min-width: 761px)').matches ?? true)
const modelInput = ref('')
const modelId = computed(() => modelInput.value.trim())
const modelValid = computed(() => isValidModelId(modelId.value))
const codexConfig = computed(() => modelValid.value ? buildCodexConfig(modelId.value) : '')
const guide = computed(() => getPlatformGuide(platform.value))
const selectedPlatform = computed(() => platformOptions.find(option => option.id === platform.value))
const isDark = ref(document.documentElement.classList.contains('dark'))
const isAuthenticated = computed(() => authStore.isAuthenticated)
const dashboardPath = computed(() => isAuthenticated.value ? (authStore.isAdmin ? '/admin/dashboard' : '/dashboard') : '/login')
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || '幻想家')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const macosDesktopEnvironment = [
  '# 在已经设置 FANTASY_API_KEY 的 macOS 终端中运行',
  'if [ -n "$FANTASY_API_KEY" ]; then',
  '  launchctl setenv FANTASY_API_KEY "$FANTASY_API_KEY"',
  'else',
  '  printf "请先完成上方的会话密钥设置。\\n"',
  'fi',
  '# 完全退出 Codex 桌面应用，再从“应用程序”重新打开',
].join('\n')

const tableOfContents = [
  { id: 'before-you-start', label: '账号与 API 密钥' },
  { id: 'endpoints', label: '请求地址' },
  { id: 'install', label: '系统与安装' },
  { id: 'configuration', label: '备份与配置' },
  { id: 'verification', label: '接入验证' },
  { id: 'desktop', label: 'CLI 与桌面客户端' },
  { id: 'troubleshooting', label: '常见错误排查' },
  { id: 'restore', label: '恢复原有配置' },
  { id: 'sources', label: '官方资料' },
]

const CodeBlock = defineComponent({
  name: 'DocsCodeBlock',
  props: {
    label: { type: String, required: true },
    code: { type: String, required: true },
  },
  setup(props) {
    const wasCopied = ref(false)
    async function copy() {
      if (await copyToClipboard(props.code)) {
        wasCopied.value = true
        window.setTimeout(() => { wasCopied.value = false }, 2000)
      }
    }
    return () => h('div', { class: 'docs-code-block' }, [
      h('div', { class: 'docs-code-header' }, [
        h('span', props.label),
        h('button', { type: 'button', onClick: copy, 'aria-label': `复制${props.label}` }, [
          h(Icon, { name: wasCopied.value ? 'check' : 'copy', size: 'sm' }),
          h('span', wasCopied.value ? '已复制' : '复制'),
        ]),
      ]),
      h('pre', { tabindex: '0', 'aria-label': props.label }, [h('code', props.code)]),
    ])
  },
})

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function onTocToggle(event: Event) {
  tocOpen.value = (event.target as HTMLDetailsElement).open
}

function onPlatformKeydown(event: KeyboardEvent, current: DocsPlatform) {
  const index = platformOptions.findIndex(option => option.id === current)
  let next = index
  if (event.key === 'ArrowRight') next = (index + 1) % platformOptions.length
  else if (event.key === 'ArrowLeft') next = (index - 1 + platformOptions.length) % platformOptions.length
  else if (event.key === 'Home') next = 0
  else if (event.key === 'End') next = platformOptions.length - 1
  else return
  event.preventDefault()
  const option = platformOptions[next]
  if (option) {
    platform.value = option.id
    document.getElementById(`platform-${option.id}`)?.focus()
  }
}

onMounted(() => {
  if (!appStore.publicSettingsLoaded) appStore.fetchPublicSettings()
})
</script>

<style scoped>
.usage-docs { position: relative; min-height: 100vh; color: var(--lux-text); }
.docs-wrap { position: relative; max-width: 1280px; margin: 0 auto; padding: 0 48px; }
.docs-header { min-height: 96px; display: flex; align-items: center; justify-content: space-between; gap: 24px; border-bottom: 1px solid var(--lux-border); }
.docs-header-actions { display: flex; align-items: center; gap: 18px; }
.docs-home-link { color: var(--lux-muted); font-size: 13px; }
.docs-home-link:hover { color: var(--lux-gold); }
.docs-console-link { display: inline-flex; align-items: center; gap: 12px; padding: 10px 16px; font-size: 12px; white-space: nowrap; }
.docs-skip { position: fixed; top: 12px; left: 12px; z-index: 100; padding: 12px; background: var(--lux-panel); color: var(--lux-text); border: 1px solid var(--lux-gold); transform: translateY(-160%); }
.docs-skip:focus { transform: none; }
.docs-hero { display: grid; grid-template-columns: minmax(0, 1.4fr) minmax(240px, .65fr); align-items: center; gap: 76px; padding: 68px 0 64px; border-bottom: 1px solid var(--lux-border); }
.docs-hero h1 { font-size: clamp(34px, 4.4vw, 54px); font-weight: 400; line-height: 1.45; letter-spacing: -.025em; margin: 17px 0 22px; }
.docs-lead { color: var(--lux-muted); font-size: 15px; max-width: 520px; line-height: 1.95; }
.docs-hero-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 24px; margin-top: 28px; font-size: 12px; }
.docs-hero-actions .btn { display: inline-flex; align-items: center; gap: 18px; padding: 12px 20px; }
.docs-text-link { color: var(--lux-muted); }
.docs-text-link:hover { color: var(--lux-gold); }
.docs-hero-note { padding: 24px 0 24px 30px; border-left: 1px solid var(--lux-border); }
.docs-note-mark { font-family: var(--lux-display); font-size: 38px; color: var(--lux-gold); }
.docs-hero-note > p { font-size: 18px; line-height: 1.9; margin: 17px 0 24px; }
.docs-platform-labels { display: flex; gap: 14px; flex-wrap: wrap; color: var(--lux-gold); font-size: 11px; }
.docs-hero-note small { display: block; margin-top: 20px; color: var(--lux-muted); line-height: 1.8; font-size: 11px; max-width: 245px; }
.docs-layout { display: grid; grid-template-columns: 190px minmax(0, 1fr); align-items: start; gap: 64px; padding-top: 48px; }
.docs-sidebar { position: sticky; top: 30px; min-width: 0; }
.docs-toc summary { display: flex; justify-content: space-between; cursor: pointer; padding-bottom: 18px; color: var(--lux-text); font-size: 12px; letter-spacing: .04em; list-style: none; }
.docs-toc summary::-webkit-details-marker { display: none; }
.docs-toc summary > span { display: none; }
.docs-toc nav { display: flex; flex-direction: column; gap: 1px; }
.docs-toc nav a { display: flex; gap: 10px; align-items: baseline; border-left: 1px solid var(--lux-border); padding: 9px 0 9px 12px; font-size: 12px; line-height: 1.7; color: var(--lux-muted); transition: color 160ms, border-color 160ms; }
.docs-toc nav a span { font-family: var(--lux-display); color: var(--lux-gold); font-size: 14px; }
.docs-toc nav a:hover { color: var(--lux-gold); border-left-color: var(--lux-gold); }
.docs-sidebar-footer { padding-left: 12px; margin-top: 36px; color: var(--lux-muted); font-size: 10px; line-height: 1.9; }
.docs-sidebar-footer strong { font-family: var(--lux-display); font-size: 22px; font-weight: 400; color: var(--lux-gold); letter-spacing: .04em; }
.docs-content { min-width: 0; padding-bottom: 38px; }
.docs-section { min-width: 0; border-bottom: 1px solid var(--lux-border); padding-bottom: 44px; margin-bottom: 42px; scroll-margin-top: 36px; }
.docs-section-heading { margin-bottom: 22px; }
.docs-section-heading h2 { font-weight: 400; font-size: 26px; line-height: 1.5; margin-top: 8px; letter-spacing: -.02em; }
.docs-content p { font-size: 13px; line-height: 1.95; color: var(--lux-muted); margin: 12px 0; overflow-wrap: anywhere; }
.docs-content p code { font-size: .93em; padding: 2px 4px; border-radius: 3px; background: var(--lux-gold-soft); color: var(--lux-text); }
.docs-content p a, .docs-selected-platform a { color: var(--lux-gold); text-underline-offset: 3px; text-decoration: underline; text-decoration-color: color-mix(in srgb, var(--lux-gold) 35%, transparent); }
.docs-content p a:hover { text-decoration-color: var(--lux-gold); }
.docs-steps { list-style: none; padding: 0; margin: 23px 0; display: flex; flex-direction: column; gap: 22px; }
.docs-steps > li { display: flex; gap: 17px; align-items: flex-start; min-width: 0; }
.docs-step-number { flex: 0 0 28px; border: 1px solid var(--lux-border); border-radius: 50%; height: 28px; display: inline-flex; align-items: center; justify-content: center; color: var(--lux-gold); font-family: var(--lux-display); font-size: 19px; }
.docs-steps h3 { font-size: 14px; font-weight: 500; line-height: 1.9; margin: 0; }
.docs-steps p { margin: 5px 0 0; }
.docs-callout { padding: 18px 22px; border: 1px solid var(--lux-border); border-left: 3px solid var(--lux-gold); border-radius: 0 10px 10px 0; margin-top: 24px; background: var(--lux-gold-soft); }
.docs-callout strong { color: var(--lux-text); font-size: 13px; font-weight: 500; }
.docs-callout p { margin: 6px 0 0; font-size: 12px; }
.docs-callout-subtle { border-left-width: 1px; border-radius: 10px; background: transparent; }
.docs-endpoints { border: 1px solid var(--lux-border); border-radius: 12px; overflow: hidden; margin-top: 22px; background: var(--lux-panel); }
.docs-endpoint { padding: 20px; min-width: 0; }
.docs-endpoint + .docs-endpoint { border-top: 1px solid var(--lux-border); }
.docs-endpoint h3 { font-size: 13px; font-weight: 500; }
.docs-endpoint p { font-size: 12px; margin: 4px 0 10px; }
.docs-endpoint-value { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; }
.docs-endpoint-value > code { color: var(--lux-gold); font-size: 13px; min-width: 0; overflow-wrap: anywhere; }
.docs-endpoint-value > button { padding: 2px; flex-shrink: 0; color: var(--lux-muted); }
.docs-endpoint-value > button:hover { color: var(--lux-gold); }
.docs-content .docs-caption { font-size: 11px; line-height: 1.9; }
.docs-platform-tabs { display: flex; gap: 7px; margin: 22px 0 20px; overflow-x: auto; padding: 3px 3px 8px; max-width: 100%; }
.docs-platform-tabs button { flex-shrink: 0; border: 1px solid var(--lux-border); border-radius: 8px; padding: 10px 17px; min-height: 42px; font-size: 13px; color: var(--lux-muted); background: var(--lux-panel); transition: color 160ms, border-color 160ms, background-color 160ms; }
.docs-platform-tabs button:hover, .docs-platform-tabs button.is-selected { border-color: var(--lux-gold); color: var(--lux-gold); background: var(--lux-gold-soft); }
.docs-terminal-label { display: flex; align-items: center; gap: 8px; color: var(--lux-muted); font-size: 11px; margin-bottom: 9px; }
.docs-terminal-dot { height: 5px; width: 5px; border-radius: 50%; background: var(--lux-gold); }
.docs-selected-platform { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; }
.docs-selected-platform strong { font-weight: 500; color: var(--lux-text); }
.docs-selected-platform a { margin-left: 10px; font-size: 11px; }
.docs-subheading { font-size: 16px; font-weight: 500; line-height: 1.7; margin: 30px 0 10px; }
.docs-model-field { border: 1px solid var(--lux-border); border-radius: 10px; padding: 20px; background: var(--lux-panel); margin: 20px 0; min-width: 0; }
.docs-model-field label { display: flex; flex-wrap: wrap; gap: 12px; align-items: baseline; font-size: 13px; font-weight: 500; margin-bottom: 10px; }
.docs-model-field label span { font-size: 10px; color: var(--lux-gold); font-weight: 400; }
.docs-model-field input { width: 100%; min-width: 0; border: 1px solid var(--lux-border); border-radius: 7px; background: var(--lux-bg); padding: 11px 12px; min-height: 44px; color: var(--lux-text); font-family: 'SFMono-Regular', Consolas, monospace; font-size: 12px; }
.docs-model-field input::placeholder { color: var(--lux-muted); font-family: inherit; opacity: .7; }
.docs-model-field input[aria-invalid="true"] { border-color: #b65043; }
.docs-model-field > p { font-size: 11px; margin: 9px 0 0; }
.docs-model-field > p.docs-field-error { color: #b65043; }
.docs-code-placeholder { display: flex; align-items: center; gap: 14px; padding: 20px; border: 1px dashed var(--lux-border); border-radius: 10px; margin: 16px 0; color: var(--lux-muted); }
.docs-code-placeholder strong { font-size: 12px; font-weight: 500; }
.docs-code-placeholder p { margin: 2px 0 0; font-size: 11px; }
.docs-content :deep(.docs-code-block) { max-width: 100%; min-width: 0; border: 1px solid var(--lux-border); background: var(--lux-panel); border-radius: 10px; margin: 16px 0; overflow: hidden; }
.docs-content :deep(.docs-code-header) { display: flex; justify-content: space-between; align-items: center; gap: 12px; padding: 10px 15px; border-bottom: 1px solid var(--lux-border); background: var(--lux-gold-soft); color: var(--lux-muted); font-size: 10px; }
.docs-content :deep(.docs-code-header > span) { overflow-wrap: anywhere; min-width: 0; }
.docs-content :deep(.docs-code-header button) { display: inline-flex; align-items: center; gap: 6px; padding: 4px; flex-shrink: 0; color: var(--lux-gold); white-space: nowrap; font-size: 11px; }
.docs-content :deep(.docs-code-block pre) { display: block; max-width: 100%; overflow-x: auto; margin: 0; padding: 18px; color: var(--lux-text); font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', monospace; font-size: 11.5px; line-height: 1.9; tab-size: 2; white-space: pre; letter-spacing: 0; overscroll-behavior-x: contain; }
.docs-content :deep(.docs-code-block pre code) { font-family: inherit; }
.docs-check-tag { display: inline-block; font-size: 10px; margin-top: 9px; padding: 3px 8px; border-radius: 4px; background: var(--lux-gold-soft); color: var(--lux-gold); }
.docs-check-tag-paid { border: 1px solid var(--lux-border); background: var(--lux-panel); }
.docs-comparison { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 20px; margin: 24px 0; }
.docs-comparison article { border-top: 1px solid var(--lux-gold); padding: 16px 0 4px; }
.docs-comparison article > p.lux-eyebrow { font-size: 10px; color: var(--lux-gold); letter-spacing: .15em; margin: 0 0 13px; }
.docs-comparison h3 { font-size: 14px; font-weight: 500; }
.docs-comparison article > p:last-child { font-size: 12px; }
.docs-faq { border-top: 1px solid var(--lux-border); margin-top: 24px; }
.docs-faq details { border-bottom: 1px solid var(--lux-border); }
.docs-faq summary { display: flex; align-items: center; justify-content: space-between; gap: 15px; font-size: 13px; line-height: 1.7; font-weight: 500; padding: 18px 0; cursor: pointer; list-style: none; }
.docs-faq summary::-webkit-details-marker { display: none; }
.docs-faq summary > span { color: var(--lux-gold); font-family: var(--lux-display); font-size: 22px; font-weight: 400; }
.docs-faq details[open] summary > span { transform: rotate(45deg); }
.docs-faq details > div { padding: 0 20px 17px 0; }
.docs-faq details p { font-size: 12px; margin: 0 0 7px; white-space: pre-line; }
.docs-faq details .docs-faq-symptom { color: var(--lux-text); }
.docs-source-links { display: flex; flex-direction: column; margin-top: 20px; gap: 12px; }
.docs-source-links a { display: flex; justify-content: space-between; align-items: center; gap: 20px; color: var(--lux-gold); padding: 13px 15px; background: var(--lux-gold-soft); border-radius: 6px; font-size: 12px; }
.docs-source-links a:hover { background: var(--lux-raised); }
.docs-sources-section { border: 0; margin-bottom: 0; padding-bottom: 0; }
.docs-footer { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 18px; border-top: 1px solid var(--lux-border); padding: 26px 0; font-size: 11px; color: var(--lux-muted); }
.docs-footer > a { display: inline-flex; align-items: center; gap: 12px; color: var(--lux-gold); }
@media (max-width: 1050px) { .docs-wrap { padding: 0 28px; } .docs-layout { grid-template-columns: 160px minmax(0, 1fr); gap: 36px; } .docs-hero { gap: 40px; } }
@media (max-width: 760px) {
  .docs-wrap { padding: 0 22px; }
  .docs-header { min-height: 80px; gap: 10px; }
  .docs-header-actions { gap: 4px; }
  .docs-home-link { display: none; }
  .docs-console-link { padding: 9px 11px; font-size: 11px; gap: 6px; }
  .docs-console-link svg { display: none; }
  .docs-hero { grid-template-columns: minmax(0, 1fr); gap: 30px; padding: 38px 0 32px; }
  .docs-hero h1 { font-size: 35px; margin: 15px 0 18px; }
  .docs-lead { font-size: 13px; }
  .docs-hero-actions { gap: 20px; font-size: 11px; margin-top: 22px; }
  .docs-hero-note { padding: 17px 0 0; border-left: 0; border-top: 1px solid var(--lux-border); }
  .docs-hero-note > p, .docs-note-mark { display: none; }
  .docs-hero-note small { max-width: none; margin-top: 9px; font-size: 10px; }
  .docs-layout { display: block; padding-top: 22px; }
  .docs-sidebar { position: static; }
  .docs-toc { border: 1px solid var(--lux-border); border-radius: 10px; padding: 0 15px; background: var(--lux-panel); margin-bottom: 30px; }
  .docs-toc summary { padding: 13px 0; }
  .docs-toc summary > span { display: inline; }
  .docs-toc nav { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 2px 10px; padding: 0 0 14px; }
  .docs-toc nav a { padding: 6px 0 6px 10px; font-size: 11px; gap: 7px; }
  .docs-sidebar-footer { display: none; }
  .docs-section { margin-bottom: 30px; padding-bottom: 30px; scroll-margin-top: 18px; }
  .docs-section-heading { margin-bottom: 16px; }
  .docs-section-heading h2 { font-size: 23px; }
  .docs-content p { font-size: 12px; }
  .docs-steps { gap: 20px; }
  .docs-steps > li { gap: 12px; }
  .docs-step-number { flex-basis: 26px; height: 26px; }
  .docs-steps h3 { font-size: 13px; }
  .docs-callout { padding: 15px; }
  .docs-content .docs-callout p { font-size: 11px; }
  .docs-endpoint { padding: 16px; }
  .docs-endpoint-value > code { font-size: 11.5px; }
  .docs-platform-tabs { margin-top: 18px; gap: 6px; }
  .docs-platform-tabs button { padding: 10px 13px; font-size: 12px; }
  .docs-subheading { font-size: 15px; margin-top: 25px; }
  .docs-model-field { padding: 15px; }
  .docs-comparison { grid-template-columns: minmax(0, 1fr); gap: 16px; }
  .docs-content :deep(.docs-code-header) { padding: 8px 12px; font-size: 9px; }
  .docs-content :deep(.docs-code-block pre) { padding: 14px; font-size: 11px; }
  .docs-footer { align-items: flex-start; flex-direction: column; gap: 13px; }
}
</style>
