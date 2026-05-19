# AIThink 变更日志

## [2026-05-15] 升级人机验证等待机制

### 变更概述
统一升级4个平台适配器（doubao、qwen、deepseek、gpt）的 `handleCaptcha` 方法，将人机验证等待时间从60秒延长至5分钟，优化验证通过后的页面刷新策略，改善等待过程中的日志输出。

### 修改文件

#### 1. `internal/platform/doubao/doubao.go`
- **等待时间**：`maxCaptchaWait` 从 `60 * time.Second` 改为 `5 * time.Minute`
- **验证通过后不强制刷新页面**：改为智能判断当前URL是否在 `doubao.com` 域名内，仅在不在聊天页面时才导航回来
- **日志优化**：验证通过日志增加图标，等待日志改为每30秒输出一次，超时日志增加时间提示

#### 2. `internal/platform/qwen/qwen.go`
- **等待时间**：`maxCaptchaWait` 从 `60 * time.Second` 改为 `5 * time.Minute`
- **验证通过后不强制刷新页面**：改为智能判断当前URL是否在 `tongyi.aliyun.com` 域名内
- **日志优化**：同 doubao

#### 3. `internal/platform/deepseek/deepseek.go`
- **等待时间**：`maxCaptchaWait` 从 `60 * time.Second` 改为 `5 * time.Minute`
- **验证通过后不强制刷新页面**：改为智能判断当前URL是否在 `deepseek.com` 域名内
- **日志优化**：同 doubao

#### 4. `internal/platform/gpt/gpt.go`
- **新增 `handleCaptcha` 方法**：原 gpt 适配器没有人机验证自动处理方法，检测到验证时直接返回错误。现新增完整的 `handleCaptcha` 方法，与其他3个平台保持一致的处理策略
- **验证通过后智能判断**：使用 `chatgpt.com` 域名判断
- **更新 `Ask` 方法**：检测到人机验证时调用 `handleCaptcha`，验证通过后重新输入问题并发送
- **更新 `AskInConversation` 方法**：同上
- **更新 `StartNewConversation` 方法**：同上
- **更新 `waitForAIResponse` 方法**：等待过程中检测到人机验证时调用 `handleCaptcha`，验证通过后重置等待状态继续

#### 5. `docs/STABILITY.md`
- 更新人机验证等待时间描述从"最多60秒"改为"最多5分钟"
- 更新验证通过后的页面刷新策略描述

### 关键改进
| 改进项 | 旧行为 | 新行为 |
|--------|--------|--------|
| 等待时间 | 60秒 | 5分钟 |
| 验证通过后 | 强制刷新页面 | 智能判断是否需要刷新 |
| 等待日志 | 每次轮询都输出 | 每30秒输出一次 |
| 超时消息 | "人机验证等待超时（60秒）" | "人机验证等待超时（5分钟），请在验证通过后重试" |
| GPT平台 | 无自动处理 | 新增完整 handleCaptcha 方法 |

## [2026-05-15] 新增记忆管理和平台管理 API 端点

### 变更概述
为 AIThink 项目新增记忆管理和平台管理的 REST API 端点，支持外部查询和操作对话记忆状态、对话生命周期以及平台配置信息。

### 修改文件

#### 1. `internal/memory/manager.go`
- **新增全局单例机制**：添加 `GetGlobalMemoryManager()` 函数，使用 `sync.Once` 确保全局只创建一个 `MemoryManager` 实例
- **目的**：确保 `Handler`（API层）和 `AnthropicGateway`（网关层）操作同一个 `MemoryManager` 实例，避免记忆数据不一致

#### 2. `internal/api/anthropic_gateway.go`
- **修改 `NewAnthropicGateway` 构造函数**：从自建 `MemoryManager` 改为调用 `memory.GetGlobalMemoryManager()` 全局单例
- **移除**：不再在 Gateway 内部创建 `MemoryStore`、`MemoryManager` 和启动清理循环
- **目的**：与 Handler 共享同一个 MemoryManager 实例

#### 3. `internal/api/handler.go`
- **新增 import**：`aithink/internal/memory`、`aithink/internal/platform`
- **新增 Handler 方法**：
  - `GetMemoryStatus` - 查看 API Key 的记忆状态（GET）
  - `ClearMemory` - 清除 API Key 的所有记忆（DELETE）
  - `GetConversationStatus` - 查看 API Key 的对话状态（GET）
  - `ResetConversation` - 重置 API Key 的对话（POST）
  - `ListPlatforms` - 列出所有已注册平台（GET）
  - `GetPlatformConfig` - 获取指定平台配置（GET）

#### 4. `internal/api/router.go`
- **新增路由组**：
  - 记忆管理：`GET /api/v1/memory/:api_key`、`DELETE /api/v1/memory/:api_key`
  - 对话管理：`GET /api/v1/conversation/:api_key`、`POST /api/v1/conversation/:api_key/reset`
  - 平台管理：`GET /api/v1/platforms`、`GET /api/v1/platforms/:name/config`
- **更新首页端点列表**：在首页 JSON 响应中添加新增的 API 端点描述

### API 端点汇总

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/memory/:api_key` | 查看指定 API Key 的记忆状态 |
| DELETE | `/api/v1/memory/:api_key` | 清除指定 API Key 的所有记忆 |
| GET | `/api/v1/conversation/:api_key` | 查看指定 API Key 的对话状态 |
| POST | `/api/v1/conversation/:api_key/reset` | 重置指定 API Key 的对话 |
| GET | `/api/v1/platforms` | 列出所有已注册的 AI 平台 |
| GET | `/api/v1/platforms/:name/config` | 获取指定平台的配置信息 |

### 架构变更说明
- **全局单例模式**：`MemoryManager` 改为全局单例，通过 `memory.GetGlobalMemoryManager()` 获取
- **共享实例**：`AnthropicGateway` 和 `Handler` 均使用同一个 `MemoryManager` 实例
- **向后兼容**：所有现有 API 端点行为不变，仅新增端点

### 部署注意
- 如果是 Docker 镜像部署，需要重新构建镜像并重启容器
- 新增的 API 端点无需额外配置即可使用
