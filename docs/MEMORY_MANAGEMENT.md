# 记忆管理机制

## 概述

AIThink 的记忆管理模块负责将 Claude Code 发送的 Anthropic API 请求拆分为**固定提示词**、**上下文记忆**和**当前需求**三部分，并在多次调用时智能管理上下文，避免重复发送不变内容，从而节省 Token 消耗并提升响应速度。

## 核心概念

### 三级消息分类

记忆管理模块将 Anthropic API 请求中的消息拆分为三个层级：

1. **固定提示词（Fixed Prompt）**：从 Anthropic API 的 `system` 字段提取，包含 CLAUDE.md、工具定义等不变内容。在同一对话中，固定提示词不会重复发送。
2. **上下文记忆（Context Memory）**：从历史 `assistant` 消息中提取的交互结果。每次 AI 回复后自动添加到记忆列表，支持去重和精简。
3. **当前需求（Current Request）**：最后一条 `user` 消息，每次请求都单独发送。

### 对话状态管理

每个 API Key 关联一个对话状态（`ConversationState`），包含以下信息：

| 字段 | 说明 |
|------|------|
| APIKey | 关联的 API Key |
| Platform | 平台类型（zhipu/chatgpt/doubao/qwen/deepseek） |
| SessionID | 浏览器会话 ID |
| ConversationID | 浏览器内对话 ID |
| FixedPrompt | 已发送的固定提示词内容 |
| PromptHash | 提示词指纹（SHA256 前8位） |
| Memories | 记忆条目列表 |
| Status | 对话状态（active/expired/lost） |
| LastActiveAt | 最后活跃时间 |
| CreatedAt | 创建时间 |

对话状态有三种：

- **active**：活跃对话，仅发送新需求，不重复发送固定提示词和记忆
- **expired**：超时过期（默认30分钟无活动），需要重建对话
- **lost**：浏览器崩溃等导致丢失，需要重建对话

### 智能组装策略

| 对话状态 | 发送内容 | 对话模式 |
|---------|---------|---------|
| 已有活跃对话（PromptHash 一致） | 仅发送当前需求 | `existing` |
| 活跃对话但 PromptHash 变化 | 固定提示词 + 记忆 + 当前需求 | `new`（重建） |
| 对话超时（expired） | 固定提示词 + 精简记忆 + 当前需求 | `new`（重建） |
| 对话丢失（lost） | 固定提示词 + 精简记忆 + 当前需求 | `new`（重建） |
| 无对话（首次） | 固定提示词 + 当前需求 | `new`（新建） |

## 记忆去重与精简

### 指纹计算

使用 SHA256 哈希的前8位作为内容指纹，计算过程：

1. 对内容做 `TrimSpace` 处理（去除首尾空白）
2. 统一换行符（`\r\n` 和 `\r` 统一替换为 `\n`）
3. 计算 SHA256 哈希
4. 取哈希结果的前8位作为指纹

```go
// 代码位置：internal/memory/parser.go - computeFingerprint()
normalized := strings.TrimSpace(content)
normalized = strings.ReplaceAll(normalized, "\r\n", "\n")
normalized = strings.ReplaceAll(normalized, "\r", "\n")
hash := sha256.Sum256([]byte(normalized))
return hex.EncodeToString(hash[:])[:8]
```

### 去重规则

- 添加记忆时自动检查指纹
- 相同指纹的记忆 `RepeatCount` 递增，更新 `LastUsedAt`
- 不同指纹的记忆新增条目

```go
// 代码位置：internal/memory/manager.go - AddMemory()
for i := range conv.Memories {
    if conv.Memories[i].Fingerprint == fingerprint {
        conv.Memories[i].RepeatCount++
        conv.Memories[i].LastUsedAt = now
        return conv.Memories[i]
    }
}
```

### 精简策略

精简分两轮进行：

**第一轮：重复超阈值精简**
- `RepeatCount <= 阈值`（默认3）：保留完整内容
- `RepeatCount > 阈值`：精简为首行摘要（最长200字符），标记 `IsCompacted=true`

**第二轮：条目超上限淘汰**
- 总条数超过上限（默认20条）：按 LRU 策略淘汰最久未使用的（按 `LastUsedAt` 升序排序，移除最旧的）

```go
// 代码位置：internal/memory/manager.go - CompactMemories()
// 第一轮：精简重复次数超过阈值的记忆
for i := range conv.Memories {
    if conv.Memories[i].RepeatCount > m.config.RepeatThreshold && !conv.Memories[i].IsCompacted {
        conv.Memories[i].Content = extractFirstLine(content)
        conv.Memories[i].IsCompacted = true
    }
}
// 第二轮：如果总条数超过上限，按LRU策略淘汰
if len(conv.Memories) > m.config.MaxEntries {
    sort.Slice(conv.Memories, func(i, j int) bool {
        return conv.Memories[i].LastUsedAt.Before(conv.Memories[j].LastUsedAt)
    })
    removeCount := len(conv.Memories) - m.config.MaxEntries
    conv.Memories = conv.Memories[removeCount:]
}
```

## 配置参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| max_entries | 20 | 记忆条目上限，超过后按 LRU 策略淘汰 |
| repeat_threshold | 3 | 重复阈值，超过后记忆精简为首行摘要 |
| conversation_timeout | 30m | 对话超时时间，超时后标记为 expired |

配置定义位于 `internal/memory/manager.go` 的 `MemoryConfig` 结构体，通过 `GetGlobalMemoryManager()` 初始化时传入默认配置。

## API 接口

### 记忆管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/memory/:api_key` | 查看指定 API Key 的记忆状态 |
| DELETE | `/api/v1/memory/:api_key` | 清除指定 API Key 的所有记忆 |

### 对话管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/conversation/:api_key` | 查看指定 API Key 的对话状态 |
| POST | `/api/v1/conversation/:api_key/reset` | 重置指定 API Key 的对话（标记为 expired，清除对话 ID） |

路由定义位于 `internal/api/router.go`。

## 数据存储

### 存储目录

```
data/memory/
```

### 文件格式

- 文件名：`{api_key_sha256前16位}.json`
- API Key 通过 SHA256 哈希前16位作为文件名，避免直接暴露
- 日志中遮蔽 API Key 中间部分，仅显示前4位和后4位

### 持久化机制

| 操作 | 说明 |
|------|------|
| 自动保存间隔 | 5分钟（通过清理协程定时保存） |
| 启动时加载 | `NewMemoryManager()` 创建时自动从存储加载已有数据 |
| 原子写入 | 先写临时文件（`.tmp`），再重命名为目标文件，防止写入中断导致数据损坏 |
| 停止时保存 | 调用 `Stop()` 时保存所有数据 |

```go
// 代码位置：internal/memory/store.go - SaveConversation()
tmpPath := filePath + ".tmp"
os.WriteFile(tmpPath, data, 0644)
os.Rename(tmpPath, filePath) // 原子操作
```

## 工作流程

### 整体流程图

```
Claude Code 发送请求
        |
        v
AnthropicGateway.Messages()
        |
        v
MessageParser.ParseMessages()
  |-- 提取 system 字段 → FixedPrompt + PromptHash
  |-- 遍历 assistant 消息 → MemoryEntries
  |-- 最后一条 user 消息 → CurrentRequest
        |
        v
ConversationManager.Decide()
  |-- 检查对话是否超时 → 超时则标记 expired
  |-- 获取活跃对话
  |     |-- 活跃 + PromptHash 一致 → ActionSendOnly
  |     |-- 活跃 + PromptHash 变化 → ActionRebuildAndSend
  |-- 非活跃对话（expired/lost） → ActionRebuildAndSend
  |-- 无对话 → ActionCreateAndSend
        |
        v
根据决策设置 ConversationMode 和发送内容
  |-- ActionSendOnly → existing 模式，仅发送 CurrentRequest
  |-- ActionCreateAndSend → new 模式，发送完整消息
  |-- ActionRebuildAndSend → new 模式，发送完整消息
        |
        v
AIService.Ask() → 平台客户端执行提问
        |
        v
ConversationManager.HandlePostAsk()
  |-- 将 AI 回复添加到记忆（AddMemory）
  |-- 更新对话活跃时间
  |-- 更新 SessionID/ConversationID
  |-- 精简记忆（CompactMemories）
```

### 对话决策详细流程

```
Decide(apiKey, platform, parsedReq)
        |
        v
  检查对话超时?
   /        \
  是         否
  |          |
  标记expired  继续
        |
        v
  获取活跃对话?
   /        \
  有         无
  |          |
  比较PromptHash  检查非活跃对话?
  /    \        /        \
 一致   变化   有         无
 |      |      |          |
SendOnly Rebuild Rebuild   Create
```

### 记忆组装格式

新建/重建对话时，消息按以下格式组装：

```
{固定提示词}
[记忆1] 第一条记忆内容
[记忆2] 第二条记忆内容
...
{当前需求}
```

## 模块架构

```
internal/memory/
├── parser.go          # 消息解析器 - 解析 Anthropic API 请求
├── manager.go         # 记忆管理器 - 管理对话状态和记忆条目
├── conversation.go    # 对话生命周期管理器 - 决策和状态管理
└── store.go           # JSON 持久化存储 - 文件读写和原子写入
```

### 依赖关系

```
AnthropicGateway
    ├── MessageParser          （无外部依赖）
    ├── ConversationManager    → MemoryManager
    └── MemoryManager          → MemoryStore
```

### 设计原则

1. **解耦**：memory 包不依赖 api 包，通过独立的 `InputMessage` 类型解耦
2. **并发安全**：`MemoryManager` 使用 `sync.RWMutex` 读写锁保护并发访问
3. **单一职责**：`ConversationManager` 仅依赖 `MemoryManager`，不依赖 browser 和 service 包，浏览器会话管理由 AIService 层负责
4. **全局单例**：通过 `GetGlobalMemoryManager()` 确保 Handler 和 Gateway 共享同一个 MemoryManager 实例
5. **防御性编程**：`ConversationManager` 在 MemoryManager 为 nil 时提供降级处理

## 相关代码文件

| 文件 | 说明 |
|------|------|
| [parser.go](../internal/memory/parser.go) | 消息解析器实现 |
| [manager.go](../internal/memory/manager.go) | 记忆管理器实现 |
| [conversation.go](../internal/memory/conversation.go) | 对话生命周期管理器实现 |
| [store.go](../internal/memory/store.go) | JSON 持久化存储实现 |
| [anthropic_gateway.go](../internal/api/anthropic_gateway.go) | Anthropic 网关中的记忆管理集成 |
