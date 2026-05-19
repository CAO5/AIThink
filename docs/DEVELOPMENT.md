# AIThink 开发文档

## 环境准备

### 1. 安装 Go 语言环境

**Windows:**
1. 下载 Go 安装包: https://go.dev/dl/
2. 运行安装程序，按照提示安装
3. 验证安装: `go version`

**Linux/macOS:**
```bash
# 使用包管理器或下载二进制文件
wget https://go.dev/dl/go1.21.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

### 2. 安装 Chrome/Chromium 浏览器

chromedp 需要 Chrome 或 Chromium 浏览器才能运行。

**Windows:** 安装 Google Chrome
**Linux:** `sudo apt install chromium-browser`
**macOS:** `brew install chromium`

### 3. 安装项目依赖

```bash
cd AIThink
go mod download
go mod tidy
```

## 项目架构

### 目录结构说明

```
AIThink/
├── cmd/server/          # 主程序入口
│   └── main.go         # 程序启动文件
├── internal/           # 内部代码（不对外暴露）
│   ├── api/           # API 层
│   │   ├── handler.go # HTTP 请求处理器
│   │   ├── router.go  # 路由配置
│   │   ├── anthropic_gateway.go # Anthropic兼容网关（集成记忆管理）
│   │   └── gateway.go # OpenAI兼容网关
│   ├── browser/       # 浏览器自动化层
│   │   ├── browser.go # 浏览器管理器
│   │   ├── cookie_store.go # Cookie持久化存储
│   │   ├── image_analyzer.go # 图片分析器
│   │   ├── page_analyzer.go  # 页面分析器
│   │   └── zhipu.go   # 智谱清言原始实现
│   ├── config/        # 配置管理
│   │   └── config.go  # 配置加载与解析
│   ├── memory/        # 消息解析与记忆管理
│   │   ├── parser.go  # 消息解析器
│   │   ├── manager.go # 记忆管理器
│   │   ├── conversation.go # 对话生命周期管理器
│   │   └── store.go   # JSON持久化存储
│   ├── models/        # 数据模型层
│   │   ├── types.go   # 数据结构定义
│   │   └── api_key.go # API Key模型定义
│   ├── platform/      # 平台抽象层
│   │   ├── interface.go # 平台客户端接口与通用类型定义
│   │   ├── registry.go  # 平台注册器（工厂模式+单例）
│   │   ├── deepseek/    # DeepSeek平台适配器
│   │   │   └── deepseek.go # DeepSeek PlatformClient实现
│   │   ├── doubao/       # 豆包平台适配器
│   │   │   └── doubao.go # 豆包PlatformClient实现
│   │   ├── gpt/         # ChatGPT平台适配器
│   │   │   └── gpt.go   # ChatGPT PlatformClient实现
│   │   ├── qwen/        # 千问（通义千问）平台适配器
│   │   │   └── qwen.go  # 千问PlatformClient实现
│   │   └── zhipu/       # 智谱清言平台适配器
│   │       └── zhipu.go # 智谱清言PlatformClient实现
│   ├── service/       # 业务逻辑层
│   │   ├── aithink.go # AI 服务实现（通过PlatformRegistry获取平台客户端）
│   │   ├── api_key_manager.go # API密钥管理
│   │   └── session_manager.go # 会话健康监控与恢复
│   └── tls/           # TLS证书管理
│       └── cert.go    # 自签名证书生成
├── configs/           # 配置文件
│   └── config.yaml    # YAML 配置
├── docs/              # 文档
│   ├── API.md         # API 接口文档
│   ├── DEVELOPMENT.md # 本开发文档
│   ├── MEMORY_MANAGEMENT.md # 记忆管理机制文档
│   └── PLATFORM_GUIDE.md    # 平台接入指南
├── go.mod             # Go 模块定义
├── Makefile           # 构建脚本
└── README.md          # 项目说明
```

### 分层架构说明

1. **API 层** (`internal/api`): 处理 HTTP 请求和响应，包含 Anthropic 兼容网关和 OpenAI 兼容网关
2. **Service 层** (`internal/service`): 业务逻辑处理，包括 AI 服务、API Key 管理和会话管理
3. **Platform 层** (`internal/platform`): 平台抽象层，定义统一接口和注册机制（详见 [平台接入指南](PLATFORM_GUIDE.md)）
4. **Browser 层** (`internal/browser`): 浏览器自动化操作，包括 Cookie 存储和图片分析
5. **Memory 层** (`internal/memory`): 消息解析、记忆管理与对话生命周期管理（详见 [记忆管理机制](MEMORY_MANAGEMENT.md)）
6. **Models 层** (`internal/models`): 数据结构和类型定义
7. **Config 层** (`internal/config`): 配置加载与解析
8. **TLS 层** (`internal/tls`): 自签名 TLS 证书生成

## 核心代码说明

### 1. 浏览器管理器 (`browser/browser.go`)

使用单例模式管理浏览器会话，支持:
- 创建/关闭浏览器会话
- 会话复用
- 基本的浏览器操作（导航、点击、输入等）

**BrowserSession 结构体：**
- `Ctx` - 浏览器上下文
- `Cancel` - 上下文取消函数
- `SessionID` - 会话标识
- `Platform` - 平台类型（如 zhipu、chatgpt、claude 等），在创建会话时指定
- `CreatedAt` - 创建时间
- `LastActive` - 最后活跃时间

**CreateSession 方法签名：**
```go
func (bm *BrowserManager) CreateSession(sessionID string, userDataDir string, platform string) error
```
- `platform` 参数用于标识会话所属平台，在关闭会话保存 cookies 时使用
- 调用方需传入 `string(platform)` 形式的平台标识

**CloseSession 方法：**
- 关闭会话时通过 `session.Platform` 获取平台类型（而非旧的 sessionID 前缀判断方式）
- 若 Platform 为空则默认为 "unknown"

### 2. 智谱清言实现

**原始实现** (`browser/zhipu.go`)：
实现智谱清言网站的自动化操作:
- 登录流程（手机号+验证码）
- 提问和获取回复
- 会话管理

**平台适配器** (`platform/zhipu/zhipu.go`)：
将原始 `browser.ZhipuClient` 重构为适配 `PlatformClient` 接口的实现:
- 实现 `PlatformClient` 接口的所有方法（`GetPlatformName`、`GetLoginURL`、`GetChatURL`、`NavigateToHome`、`CheckLoggedIn`、`OpenLoginPage`、`Ask`、`AskInConversation`、`StartNewConversation`）
- 通过 `init()` 函数自动注册到平台注册器
- `AskInConversation`：在已有对话中继续提问（不新建对话，直接在当前输入框输入并发送）
- `StartNewConversation`：新建对话并发送初始消息
- 保留所有辅助方法（`detectPageAnomaly`、`handleSliderCaptcha`、`generateHumanTrajectory`、`executeSliderDrag`、`getCookiesFromDocument`、`sanitizeCookieValue`、`extractAIReply`、`waitForAIResponse`、`sendQuestion`）
- 返回类型统一使用 `platform.AskResult`

**ChatGPT平台适配器** (`platform/gpt/gpt.go`)：
将ChatGPT网站适配到 `PlatformClient` 接口:
- 实现 `PlatformClient` 接口的所有方法（`GetPlatformName`、`GetLoginURL`、`GetChatURL`、`NavigateToHome`、`CheckLoggedIn`、`OpenLoginPage`、`Ask`、`AskInConversation`、`StartNewConversation`）
- 通过 `init()` 函数自动注册到平台注册器（注册为 `models.PlatformChatGPT`）
- ChatGPT特有选择器：
  - 输入框：`#prompt-textarea`（优先）、`textarea[placeholder*="Message"]`、`textarea`
  - 发送按钮：`button[data-testid='send-button']`、`button[aria-label='Send prompt']`
  - 回复区域：`[data-message-author-role='assistant']`
  - 新建对话：`a[href='/']`
- 登录检测：检查 `[data-testid="profile-button"]`、`[class*="user"]`、`nav` 等元素
- 辅助方法：
  - `sendQuestion(ctx, question)` - 输入问题到ChatGPT输入框（使用insertText方式，兼容React）
  - `clickSendButton(ctx)` - 点击发送按钮（多选择器尝试，回退到Enter键）
  - `waitForAIResponse(ctx)` - 等待AI回复完成（区分思考过程和正式回复）
  - `extractAIReply(ctx)` - 从页面提取AI回复，分离思考过程和正式回复
  - `detectPageAnomaly(ctx)` - 检测页面异常状态（输入超限、人机验证、Cloudflare等）
- 超时设置：120秒（ChatGPT回复可能较长）
- 返回类型统一使用 `platform.AskResult`

**豆包平台适配器** (`platform/doubao/doubao.go`)：
将豆包（Doubao）网站适配到 `PlatformClient` 接口:
- 实现 `PlatformClient` 接口的所有方法（`GetPlatformName`、`GetLoginURL`、`GetChatURL`、`NavigateToHome`、`CheckLoggedIn`、`OpenLoginPage`、`Ask`、`AskInConversation`、`StartNewConversation`）
- 通过 `init()` 函数自动注册到平台注册器（注册为 `models.PlatformDoubao`）
- 豆包特有选择器：
  - 输入框：`textarea`（优先）、`div[contenteditable="true"]`、`.chat-input textarea`
  - 发送按钮：`button[class*='send']`、`[class*='submit']`、`[aria-label='发送']`
  - 回复区域：`[class*='assistant']`、`[class*='message-content']`、`[class*='markdown-body']`
  - 新建对话：`[class*='new-chat']`、`[class*='new-conversation']`
- 登录检测：检查 `.user-avatar`、`.avatar`、`[class*="user-info"]`、`[class*="login"]` 等元素，URL包含 `/chat/` 也认为已登录
- 辅助方法：
  - `sendQuestion(ctx, question)` - 输入问题到豆包输入框（使用insertText方式，兼容React/Vue）
  - `waitForAIResponse(ctx)` - 等待AI回复完成（检测停止按钮消失、回复内容稳定）
  - `extractAIReply(ctx)` - 从页面提取AI回复，分离思考过程和正式回复
  - `detectPageAnomaly(ctx)` - 检测页面异常状态（输入超限、人机验证等）
  - `handleSliderCaptcha(ctx)` - 自动处理滑块验证码
  - `generateHumanTrajectory(totalDistance)` - 生成模拟人类的拖拽轨迹
  - `executeSliderDrag(ctx, startX, startY, trajectory)` - 使用chromedp鼠标事件执行滑块拖拽
  - `handleCaptcha(ctx)` - 尝试自动处理人机验证页面
  - `GetCookies()` / `getCookiesFromDocument()` - 获取当前页面cookies
  - `sanitizeCookieValue(value)` - 清洗cookie值
- 超时设置：90秒
- 返回类型统一使用 `platform.AskResult`

**DeepSeek平台适配器** (`platform/deepseek/deepseek.go`)：
将DeepSeek网页版适配到 `PlatformClient` 接口:
- 实现 `PlatformClient` 接口的所有方法（`GetPlatformName`、`GetLoginURL`、`GetChatURL`、`NavigateToHome`、`CheckLoggedIn`、`OpenLoginPage`、`Ask`、`AskInConversation`、`StartNewConversation`）
- 通过 `init()` 函数自动注册到平台注册器（注册为 `models.PlatformDeepSeek`）
- DeepSeek特有选择器：
  - 输入框：`textarea`（优先）、`#chat-input`、`[class*="input"] textarea`
  - 发送按钮：`button[class*='send']`、`[aria-label='Send']`、`[data-testid='send-button']`
  - 回复区域：`[class*='assistant']`、`.markdown-body`、`[class*='ds-markdown']`
  - 新建对话：`[class*='new-chat']`、`a[href='/']`
- 登录检测：检查 `[class*="avatar"]`、`[class*="user-menu"]`、`[class*="profile"]`、`textarea` 等元素
- 辅助方法：
  - `sendQuestion(ctx)` - 发送当前输入框中的问题（多选择器尝试，回退到Enter键）
  - `waitForAIResponse(ctx)` - 等待AI回复完成（区分思考过程和正式回复）
  - `extractAIReply(ctx)` - 从页面提取AI回复，分离思考过程和正式回复
  - `detectPageAnomaly(ctx)` - 检测页面异常状态（输入超限、人机验证等）
  - `handleSliderCaptcha(ctx)` - 自动处理滑块验证码
  - `generateHumanTrajectory(totalDistance)` - 生成模拟人类的拖拽轨迹
  - `executeSliderDrag(ctx, startX, startY, trajectory)` - 使用chromedp鼠标事件执行滑块拖拽
  - `handleCaptcha(ctx)` - 尝试自动处理人机验证页面
  - `GetCookies()` - 获取当前页面cookies（CDP原生API，降级到document.cookie）
- 超时设置：90秒
- 返回类型统一使用 `platform.AskResult`

### 3. 平台抽象层 (`platform/`)

通过接口抽象和工厂模式，统一管理多种AI平台客户端：

**interface.go - 接口与类型定义：**
- `PlatformClient` 接口：定义平台客户端的统一行为（登录、提问、新建对话等）
- `AskResult` 结构体：提问结果，包含答案、思考过程、机器人检测信息、流式通道
- `PlatformConfig` 结构体：平台配置，包含URL、CSS选择器、超时设置
- `Factory` 类型：平台客户端工厂函数签名

**registry.go - 平台注册器：**
- `PlatformRegistry` 结构体：管理平台工厂函数和配置的映射，读写锁保证并发安全
- `GetRegistry()` 单例获取：首次调用初始化，后续返回同一实例
- `Register()` 注册平台：注册工厂函数和配置
- `GetClient()` 创建客户端：根据平台类型和浏览器会话创建客户端实例
- `GetConfig()` 获取配置：获取指定平台的配置
- `ListPlatforms()` 列出平台：返回所有已注册平台列表
- `IsRegistered()` 检查注册：判断指定平台是否已注册

### 4. API 接口 (`api/handler.go`)

提供 RESTful API:
- `POST /api/v1/login` - 登录
- `POST /api/v1/ask` - 提问（支持 `conversation_mode` 参数）
- `POST /api/v1/logout` - 登出
- `GET /health` - 健康检查
- `GET /api/v1/memory/:api_key` - 查看记忆状态
- `DELETE /api/v1/memory/:api_key` - 清除记忆
- `GET /api/v1/conversation/:api_key` - 查看对话状态
- `POST /api/v1/conversation/:api_key/reset` - 重置对话
- `GET /api/v1/platforms` - 列出已注册平台
- `GET /api/v1/platforms/:name/config` - 获取平台配置

### 5. AI 服务层 (`service/aithink.go`)

AI服务核心实现，通过 `PlatformRegistry` 获取平台客户端，实现平台无关的业务逻辑：

**核心结构体：**
- `AIService` - AI服务单例，管理浏览器、Cookie存储、会话和登录状态
- `LoginState` - 登录状态跟踪，包含 `Platform` 字段关联平台类型

**平台客户端获取方式：**
所有平台客户端均通过 `platform.GetRegistry().GetClient(platformType, session)` 获取，
不再硬编码 `browser.NewZhipuClient`。通过空白导入确保平台注册：
- `_ "aithink/internal/platform/deepseek"` - 注册DeepSeek平台
- `_ "aithink/internal/platform/doubao"` - 注册豆包平台
- `_ "aithink/internal/platform/gpt"` - 注册ChatGPT平台
- `_ "aithink/internal/platform/qwen"` - 注册千问平台
- `_ "aithink/internal/platform/zhipu"` - 注册智谱平台

> 注意：空白导入分布在多个文件中。`aithink.go` 导入全部5个平台，`anthropic_gateway.go` 和 `session_manager.go` 导入部分平台。由于 `sync.Once` 机制，多次导入不会重复注册。

**核心方法：**
- `Login(req)` - 启动登录流程，根据 `req.Platform` 通过注册器获取对应平台客户端
- `GetLoginStatus(sessionID)` - 查询登录状态，从 `LoginState.Platform` 获取平台信息
- `Ask(req)` - 向AI平台提问，支持 `ConversationMode` 参数（new/existing）
- `AskWithConversation(req, conversationMode)` - 根据对话模式提问：
  - `ConversationModeNew`：调用 `client.StartNewConversation()`
  - `ConversationModeExisting`：调用 `client.AskInConversation()`
  - 默认：调用 `client.Ask()`
- `CheckAntiDetection(sessionID)` - 反检测检查（直接使用BrowserSession，不依赖平台客户端）
- `autoCreateSession(sessionID, platformType)` - 自动创建会话，通过注册器获取客户端

**会话管理器 (`service/session_manager.go`)：**
- `createPlatformClient(sessionID, platformType)` - 通过注册器创建平台客户端
- 健康检查和会话恢复均使用注册器获取客户端
- 空白导入：`_ "aithink/internal/platform/qwen"`、`_ "aithink/internal/platform/zhipu"`

### 5.1 Anthropic网关 (`api/anthropic_gateway.go`)

提供Anthropic Messages API兼容接口，集成记忆管理流程，支持cc-Switch/Claude Code等客户端。

**核心结构体：**
- `AnthropicGateway` - Anthropic兼容网关，包含以下字段：
  - `apiKeyManager` - API Key管理器
  - `aiService` - AI服务
  - `memoryManager` - 记忆管理器
  - `conversationManager` - 对话生命周期管理器
  - `messageParser` - 消息解析器

**记忆管理集成流程：**
1. `NewAnthropicGateway()` 构造函数中初始化记忆存储、记忆管理器、对话管理器和消息解析器
2. `Messages()` 方法中，提取用户消息后执行记忆管理决策：
   - 调用 `messageParser.ParseMessages()` 解析请求消息
   - 调用 `conversationManager.Decide()` 进行对话决策
   - 根据决策结果设置 `ConversationMode`（new/existing）和发送内容
3. `handleNonStreamRequest()` / `handleStreamRequest()` 中：
   - 将 `ConversationMode` 传递给 `AskRequest`
   - AI回复后调用 `conversationManager.HandlePostAsk()` 更新记忆

**systemPrompt拼接策略：**
- 新建/重建对话时（ConversationModeNew）：question已包含固定提示词和记忆，不再额外拼接systemPrompt
- 继续现有对话时（ConversationModeExisting）：将systemPrompt拼接到question前面

**空白导入：**
- `_ "aithink/internal/platform/qwen"` - 确保千问平台注册
- `_ "aithink/internal/platform/zhipu"` - 确保智谱平台注册

> 记忆管理机制的详细说明请参考 [记忆管理机制文档](MEMORY_MANAGEMENT.md)

### 6. 消息解析器 (`memory/parser.go`)

负责从Anthropic API请求中解析消息，提取关键信息供记忆管理使用：

**核心类型：**
- `InputMessage` - 输入消息格式，与Anthropic API兼容，独立于api包定义
- `ParsedRequest` - 解析后的请求结构，包含固定提示词、提示词指纹、记忆内容和当前请求
- `MessageParser` - 消息解析器

**核心方法：**
- `NewMessageParser()` - 创建解析器实例
- `ParseMessages(messages, system)` - 解析消息列表，返回ParsedRequest
- `extractSystemPrompt(system)` - 提取系统提示词，支持string/[]interface{}/nil等多种格式
- `extractTextFromContent(content)` - 提取消息文本，支持text/tool_result等内容类型
- `computeFingerprint(content)` - 计算内容SHA256指纹（前8位），用于判断提示词是否变化

**设计原则：**
- memory包不依赖api包，通过独立的InputMessage类型解耦
- 支持Anthropic API的多种消息格式（string、数组、嵌套结构）
- 指纹计算对内容做标准化处理（TrimSpace、统一换行符），确保语义相同的内容产生相同指纹

### 7. 记忆管理器 (`memory/manager.go`)

管理所有API Key对应的对话状态和记忆条目，支持并发安全访问，提供记忆的增删改查、精简和消息组装功能：

**核心类型：**
- `MemoryEntry` - 记忆条目，包含ID、内容、指纹、重复计数、精简标记和时间信息
- `ConvStatus` - 对话状态枚举：`active`（活跃）、`expired`（过期）、`lost`（丢失）
- `ConversationState` - 对话状态，记录API Key关联的完整对话上下文（平台、会话、提示词、记忆列表等）
- `MemoryConfig` - 记忆管理配置（条目上限、重复阈值、对话超时时间）
- `MemoryManager` - 记忆管理器，管理对话状态和记忆条目

**核心方法：**
- `NewMemoryManager(store, config)` - 创建管理器，自动从持久化存储加载已有数据
- `GetOrCreateConversation(apiKey, platform)` - 获取或创建对话状态（active直接返回，expired/lost重建，不存在则新建）
- `GetActiveConversation(apiKey)` - 获取活跃对话，非active返回nil
- `AddMemory(apiKey, content)` - 添加记忆（相同指纹增加重复计数，否则新增条目）
- `CompactMemories(apiKey)` - 精简记忆（超阈值精简为首行摘要，超上限LRU淘汰）
- `MarkConversationExpired(apiKey)` / `MarkConversationLost(apiKey)` - 标记对话状态
- `UpdateConversationActivity(apiKey)` - 更新最后活跃时间
- `SetFixedPrompt(apiKey, prompt, promptHash)` - 设置固定提示词
- `BuildMessage(apiKey, parsedReq)` - 组装发送内容（活跃对话仅当前请求，新建/重建对话含提示词+记忆+请求）
- `CheckConversationTimeout(apiKey)` - 检查对话是否超时
- `GetConversationState(apiKey)` - 获取对话状态（只读）
- `ClearMemory(apiKey)` - 清除所有记忆
- `UpdateConversationIDs(apiKey, sessionID, conversationID)` - 更新对话的SessionID和ConversationID（空字符串字段不更新）
- `ClearConversationIDs(apiKey)` - 清除对话的SessionID和ConversationID
- `StartCleanupLoop()` - 启动定期清理协程（每分钟检查超时，每5分钟持久化）
- `Stop()` - 停止清理协程并保存所有数据

**设计原则：**
- 读写锁保证并发安全
- 记忆指纹（SHA256前8位）用于去重判断
- LRU策略淘汰最久未使用的记忆
- 对话超时自动标记为expired
- 启动时从持久化存储恢复，定期自动保存

### 9. 对话生命周期管理器 (`memory/conversation.go`)

管理对话的生命周期，与MemoryManager配合工作，负责跟踪对话状态、处理超时、决定何时重建对话。仅依赖MemoryManager，不依赖browser和service包，避免循环依赖。

**核心类型：**
- `ConversationAction` - 对话操作决策枚举：`send_only`（仅发送）、`create_and_send`（新建并发送）、`rebuild_and_send`（重建并发送）
- `ConversationDecision` - 对话决策结果，包含操作决策、待发送消息、是否需要新建浏览器会话、是否需要重建对话
- `ConversationManager` - 对话生命周期管理器

**核心方法：**
- `NewConversationManager(memoryManager)` - 创建管理器实例
- `Decide(apiKey, platform, parsedReq)` - 核心决策方法，根据对话状态决定操作：
  - 活跃对话且PromptHash一致 → `ActionSendOnly`，仅发送CurrentRequest
  - 活跃对话但PromptHash变化 → `ActionRebuildAndSend`，重建对话（提示词变了）
  - 对话expired/lost → `ActionRebuildAndSend`，重建对话
  - 无对话 → `ActionCreateAndSend`，新建对话
- `HandlePostAsk(apiKey, answer, sessionID, conversationID)` - 请求完成后的处理：添加AI回复到记忆、更新活跃时间、更新对话ID、精简记忆
- `ResetConversation(apiKey)` - 重置对话：标记为expired，清除对话ID，下次请求自动重建
- `GetConversationInfo(apiKey)` - 获取对话信息（只读深拷贝）

**决策流程：**
1. 检查对话是否超时，超时则标记为expired
2. 获取活跃对话，比较PromptHash决定是否仅发送或重建
3. 非活跃对话（expired/lost）通过GetOrCreateConversation重置为活跃状态，保留记忆
4. 新建对话时设置固定提示词并组装完整消息（提示词+记忆+当前请求）

**设计原则：**
- 仅依赖MemoryManager，浏览器会话管理由AIService层负责
- 通过ConversationDecision向AIService层传递决策结果，不直接操作浏览器
- Decide方法内部调用GetOrCreateConversation和SetFixedPrompt确保对话状态一致
- HandlePostAsk在请求完成后统一更新对话状态，包括记忆、活跃时间和对话ID

### 8. JSON持久化存储 (`memory/store.go`)

将对话状态以JSON文件形式持久化到磁盘，支持原子写入和批量操作：

**核心类型：**
- `MemoryStore` - 持久化存储，管理JSON文件的读写

**核心方法：**
- `NewMemoryStore(dataDir)` - 创建存储实例，自动创建目录
- `SaveConversation(state)` - 保存单个对话（原子写入：先写临时文件再重命名）
- `LoadConversation(apiKey)` - 加载单个对话（文件不存在返回nil）
- `DeleteConversation(apiKey)` - 删除对话文件
- `ListConversations()` - 列出所有已存储的API Key哈希
- `SaveAll(conversations)` - 批量保存所有对话
- `LoadAll()` - 批量加载所有对话

**设计原则：**
- API Key通过SHA256哈希前16位作为文件名，避免直接暴露
- 原子写入机制（临时文件+重命名）防止写入中断导致数据损坏
- 日志中遮蔽API Key中间部分，仅显示前4位和后4位
- 读写锁保证并发安全

## 开发指南

### 添加新平台支持

> 完整的平台接入指南请参考 [平台接入指南](PLATFORM_GUIDE.md)

1. 在 `internal/models/types.go` 中添加平台常量（现有平台: `zhipu`, `chatgpt`, `claude`, `doubao`, `qwen`, `deepseek`）
2. 在 `internal/platform/` 下创建新平台的子目录和实现文件，实现 `PlatformClient` 接口
3. 在实现文件的 `init()` 函数中通过 `platform.GetRegistry().Register()` 注册新平台
4. 在 `internal/service/aithink.go` 中添加空白导入 `_ "aithink/internal/platform/{platform_name}"`
5. 在 `internal/api/anthropic_gateway.go` 中添加空白导入（如需 Anthropic 网关支持）
6. 在 `internal/service/session_manager.go` 中添加空白导入（如需会话管理器支持）

示例（添加 ChatGPT 支持）:

```go
// 步骤1: internal/models/types.go 添加常量
const PlatformChatGPT Platform = "chatgpt"

// 步骤2: internal/platform/gpt/gpt.go 实现接口
package gpt

import (
    "aithink/internal/browser"
    "aithink/internal/models"
    "aithink/internal/platform"
)

type GPTClient struct {
    session *browser.BrowserSession
}

func NewGPTClient(session *browser.BrowserSession) *GPTClient {
    return &GPTClient{session: session}
}

// init 注册平台到全局注册器
func init() {
    platform.GetRegistry().Register(
        models.PlatformChatGPT,
        func(session *browser.BrowserSession) platform.PlatformClient {
            return NewGPTClient(session)
        },
        &platform.PlatformConfig{
            Platform: models.PlatformChatGPT,
            LoginURL: "https://chatgpt.com/",
            ChatURL:  "https://chatgpt.com/",
            Selectors: map[string]string{
                "input_box":  "#prompt-textarea, textarea",
                "send_button": "button[data-testid='send-button']",
            },
        },
    )
}

func (g *GPTClient) GetPlatformName() string { return "chatgpt" }
func (g *GPTClient) GetLoginURL() string { return "https://chatgpt.com/" }
func (g *GPTClient) GetChatURL() string { return "https://chatgpt.com/" }
func (g *GPTClient) NavigateToHome() error { /* 实现 */ return nil }
func (g *GPTClient) CheckLoggedIn() bool { /* 实现 */ return false }
func (g *GPTClient) OpenLoginPage() error { /* 实现 */ return nil }
func (g *GPTClient) Ask(question string) (*platform.AskResult, error) { /* 实现 */ return nil, nil }
func (g *GPTClient) AskInConversation(question string) (*platform.AskResult, error) { /* 实现 */ return nil, nil }
func (g *GPTClient) StartNewConversation(initialMessage string) (*platform.AskResult, error) { /* 实现 */ return nil, nil }

// 步骤3: 通过注册器获取客户端
client, err := platform.GetRegistry().GetClient(models.PlatformChatGPT, session)
```

### 调试技巧

1. **查看日志**: 程序运行时会输出详细日志
2. **非无头模式**: 设置 `headless: false` 可看到浏览器操作过程
3. **增加等待时间**: 如果遇到超时，适当增加 `Sleep` 时间

## 测试

### 运行测试
```bash
go test ./...
```

### 手动测试 API

```bash
# 健康检查
curl http://localhost:8080/health

# 登录（需要先启动服务）
curl -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"platform":"zhipu","username":"your_phone","password":"your_code"}'

# 提问
curl -X POST http://localhost:8080/api/v1/ask \
  -H "Content-Type: application/json" \
  -d '{"platform":"zhipu","session_id":"YOUR_SESSION_ID","question":"你好"}'
```

## 常见问题

### 1. chromedp 找不到浏览器

**错误**: `exec: "chrome": executable file not found in %PATH%`

**解决**: 确保 Chrome 已安装并在 PATH 中，或在代码中指定浏览器路径:
```go
opts := append(chromedp.DefaultExecAllocatorOptions[:],
    chromedp.ExecPath("C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe"),
)
```

### 2. 登录失败

- 检查网络连接
- 确认网站结构是否变化（可能需要更新选择器）
- 查看浏览器窗口中的实际操作过程

### 3. 并发问题

chromedp 的浏览器实例是独立的，每个会话使用独立的浏览器上下文，支持并发访问。

## 性能优化建议

1. **会话池**: 实现会话池复用已登录的会话
2. **限流**: 添加 API 限流，防止过于频繁的请求
3. **超时控制**: 为所有操作设置合理的超时时间
4. **资源清理**: 定期清理过期的浏览器会话

## 部署

### 编译二进制
```bash
make build
# 或
go build -o bin/aithink cmd/server/main.go
```

### 运行
```bash
./bin/aithink
```

### Docker 部署（待实现）
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o aithink cmd/server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates chromium
COPY --from=builder /app/aithink /usr/local/bin/
CMD ["aithink"]
```
