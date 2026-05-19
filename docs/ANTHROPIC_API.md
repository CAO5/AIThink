# Anthropic原生API接口文档

## 概述

AIThink提供完整的Anthropic Messages API兼容接口，可以让任何支持Anthropic API的客户端直接使用。

## 基本信息

| 项目 | 说明 |
|------|------|
| 基础URL | `http://localhost:8081` |
| Messages接口 | `POST /v1/messages` |
| Models接口 | `GET /v1/models` |
| 认证方式 | Header: `x-api-key` 或 `Authorization: Bearer <api_key>` |
| API版本 | Header: `anthropic-version: 2023-06-01`（可选） |

## cc-Switch 兼容性

本服务完全兼容 cc-Switch 工具的测试和配置功能：

- ✅ 支持 cc-Switch 的"测试速度"功能
- ✅ 支持 Claude Code、Codex、Gemini CLI 等工具的 API 配置
- ✅ 响应格式符合 Anthropic API 规范
- ✅ 包含所有必要的响应头（Content-Type、X-Request-Id）
- ✅ 支持 `x-api-key` 和 `Authorization: Bearer` 两种认证方式
- ✅ 支持 CORS 跨域请求（cc-Switch Web界面）
- ✅ 自动处理URL路径重复问题（兼容各种base URL配置）
- ✅ 支持 `anthropic-version` 和 `anthropic-beta` 请求头
- ✅ 支持 `?beta=true` 查询参数
- ✅ 支持 `system` 字段的字符串和数组两种格式
- ✅ 支持 `tool_result` 类型的消息内容解析
- ✅ 支持 OpenAI Responses API 格式（`/v1/responses` 端点）
- ✅ 支持 OpenAI Chat Completions 格式（`/v1/chat/completions` 端点）
- ✅ 支持 Extended Thinking（扩展思考）功能，返回 `thinking` 内容块

### cc-Switch 配置说明

在 cc-Switch 中配置 AIThink 时，**请求地址**应填写：

```
http://localhost:8081
```

**重要**：不要在请求地址中包含 `/v1/messages` 路径！cc-Switch/Claude Code SDK 会自动追加 API 路径。

| 配置项 | 正确值 | 错误值 |
|--------|--------|--------|
| 请求地址 | `http://localhost:8081` | `http://localhost:8081/v1/messages` |
| 请求地址 | `http://localhost:8081` | `http://localhost:8081/v1` |

如果误将请求地址配置为包含路径的URL，AIThink 会自动兼容处理（通过路径重写），但建议使用正确的配置以获得最佳性能。

### 兼容的URL路径映射

AIThink 自动处理以下URL路径重复情况：

| 请求路径 | 实际处理 | 触发条件 |
|----------|----------|----------|
| `/v1/messages` | `/v1/messages` | 标准配置（推荐） |
| `/v1/messages/v1/messages` | `/v1/messages` | base URL包含 `/v1/messages` |
| `/v1/v1/messages` | `/v1/messages` | base URL包含 `/v1` |
| `/messages` | `/v1/messages` | base URL为根路径 |

## 认证

所有请求需要在Header中携带API Key（任选一种方式）：

```
方式1: x-api-key: your-api-key
方式2: Authorization: Bearer your-api-key
```

可选的 API 版本头（为了兼容 Anthropic SDK 客户端）：
```
anthropic-version: 2023-06-01
```

## 支持的接口

### 1. 发送消息 (POST /v1/messages)

#### 请求格式

**Headers:**
```
Content-Type: application/json
x-api-key: <your-api-key>
anthropic-version: 2023-06-01
```

**Body参数:**
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| model | string | 是 | 模型名称 |
| messages | array | 是 | 消息数组 |
| max_tokens | int | 是 | 最大生成token数 |
| system | string | 否 | 系统提示词 |
| temperature | float | 否 | 温度参数(0-1) |
| top_p | float | 否 | Top-p采样参数 |
| top_k | int | 否 | Top-k采样参数 |
| stream | bool | 否 | 是否使用流式输出 |
| stop_sequences | array | 否 | 停止序列列表 |
| tools | array | 否 | 工具定义列表 |
| tool_choice | object | 否 | 工具选择策略 |

**Message格式:**
```json
{
  "role": "user",
  "content": "你好"
}
```

或支持多模态内容：
```json
{
  "role": "user",
  "content": [
    {"type": "text", "text": "这是什么图片?"},
    {
      "type": "image",
      "source": {
        "type": "base64",
        "media_type": "image/jpeg",
        "data": "base64编码的图片数据"
      }
    }
  ]
}
```

#### 非流式响应格式

**成功响应 (200):**
```json
{
  "id": "msg_1778651664802724300",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "AI回复内容..."
    }
  ],
  "model": "claude-sonnet-4-5-20250929",
  "stop_reason": "end_turn",
  "stop_sequence": null,
  "usage": {
    "input_tokens": 3,
    "output_tokens": 125
  }
}
```

**错误响应 (4XX/5XX):**
```json
{
  "type": "error",
  "error": {
    "type": "authentication_error",
    "message": "错误信息"
  }
}
```

#### 流式响应格式 (SSE)

```
event: message_start
data: {"type":"message_start","message":{"id":"msg_xxx","type":"message","role":"assistant","model":"claude-sonnet-4-5-20250929","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":3,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"AI回复内容"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":125}}

event: message_stop
data: {"type":"message_stop"}
```

### 2. 获取模型列表 (GET /v1/models)

**请求:**
```
GET /v1/models
x-api-key: <your-api-key>
```

**响应:**
```json
{
  "object": "list",
  "data": [
    {
      "id": "zhipu-glm-5",
      "object": "model",
      "created": 1778651722,
      "owned_by": "aithink"
    },
    {
      "id": "claude-sonnet",
      "object": "model",
      "created": 1778651722,
      "owned_by": "aithink"
    },
    {
      "id": "chatgpt-gpt-4",
      "object": "model",
      "created": 1778651722,
      "owned_by": "aithink"
    }
  ]
}
```

## 使用示例

### cURL示例

**非流式请求:**
```bash
curl -X POST http://localhost:8081/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: 3dd4b3c25c0989cf88a4103332d75fb34877ef12d770aaed37a20556f635ba4e" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4-5-20250929",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "你好，请用一句话介绍你自己"}
    ]
  }'
```

**流式请求:**
```bash
curl -X POST http://localhost:8081/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: 3dd4b3c25c0989cf88a4103332d75fb34877ef12d770aaed37a20556f635ba4e" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4-5-20250929",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "你好"}
    ],
    "stream": true
  }'
```

### Python示例

```python
import requests
import json

# 非流式请求
response = requests.post(
    "http://localhost:8081/v1/messages",
    headers={
        "Content-Type": "application/json",
        "x-api-key": "3dd4b3c25c0989cf88a4103332d75fb34877ef12d770aaed37a20556f635ba4e",
        "anthropic-version": "2023-06-01"
    },
    json={
        "model": "claude-sonnet-4-5-20250929",
        "max_tokens": 1024,
        "messages": [
            {"role": "user", "content": "你好，请用一句话介绍你自己"}
        ]
    }
)

result = response.json()
print(f"AI回复: {result['content'][0]['text']}")
print(f"输入Token: {result['usage']['input_tokens']}")
print(f"输出Token: {result['usage']['output_tokens']}")
```

### 流式Python示例

```python
import requests

response = requests.post(
    "http://localhost:8081/v1/messages",
    headers={
        "Content-Type": "application/json",
        "x-api-key": "3dd4b3c25c0989cf88a4103332d75fb34877ef12d770aaed37a20556f635ba4e",
        "anthropic-version": "2023-06-01"
    },
    json={
        "model": "claude-sonnet-4-5-20250929",
        "max_tokens": 1024,
        "messages": [
            {"role": "user", "content": "你好"}
        ],
        "stream": True
    },
    stream=True
)

for line in response.iter_lines():
    if line:
        line = line.decode('utf-8')
        if line.startswith('data:'):
            print(line[6:])
```

## 错误码说明

| HTTP状态码 | 错误类型 | 说明 |
|-----------|----------|------|
| 400 | invalid_request_error | 请求格式错误或缺少必填字段 |
| 401 | authentication_error | API Key无效或缺失 |
| 500 | api_error | 服务器内部错误 |

## Extended Thinking（扩展思考）

AIThink支持Anthropic的Extended Thinking功能。当AI平台（如智谱清言）返回思考过程时，AIThink会将其转换为Anthropic格式的 `thinking` 内容块。

### 请求参数

在请求中添加 `thinking` 参数即可启用扩展思考：

```json
{
    "model": "claude-sonnet-4-5-20250929",
    "max_tokens": 16000,
    "thinking": {"type": "enabled", "budget_tokens": 10000},
    "messages": [{"role": "user", "content": "请解释量子纠缠"}]
}
```

AIThink会自动检测AI平台的思考过程并返回，无需客户端显式启用 `thinking` 参数。即使请求中没有 `thinking` 参数，如果AI平台返回了思考过程，AIThink也会将其包含在响应中。

### 非流式响应格式

```json
{
    "id": "msg_xxx",
    "type": "message",
    "role": "assistant",
    "content": [
        {
            "type": "thinking",
            "thinking": "让我分析这个问题...",
            "signature": "ErUBk..."
        },
        {
            "type": "text",
            "text": "量子纠缠是..."
        }
    ],
    "model": "claude-sonnet-4-5-20250929",
    "stop_reason": "end_turn",
    "usage": {"input_tokens": 10, "output_tokens": 200}
}
```

### 流式响应格式

```
event: message_start
data: {"type":"message_start","message":{...}}

event: ping
data: {}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","id":"blk_xxx"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"让我分析"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":"","id":"blk_yyy"}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"量子纠缠是"}}

event: content_block_stop
data: {"type":"content_block_stop","index":1}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":200}}

event: message_stop
data: {"type":"message_stop"}
```

### 思考过程提取

AIThink通过以下策略从AI平台提取思考过程：

1. **元素级提取**：从AI回复的DOM元素中分别识别思考过程和正式回复
2. **段落级提取**：从文本段落中根据关键词模式识别思考内容
3. **文本分割**：从完整文本中通过转换标记（如"Answer:"、"回答："）分割思考过程和正式回复

识别的思考过程关键词包括：`思考过程`、`深度思考`、`Formulate the Strategy`、`Identify the Core Task`、`Gather Information`、`Evaluate Alternatives`、`Structure the Response`、`Analyze the Input`、`Analyze the Question`等。

### 中文输入编码

AIThink使用JavaScript的`nativeInputValueSetter`方式设置textarea值，避免`chromedp.SendKeys`对中文字符的编码问题。同时触发多种DOM事件（`input`、`change`、`InputEvent`、`KeyboardEvent`）确保React/Vue框架能正确检测到输入变化。

### Chrome进程管理

AIThink使用独立的用户数据目录（`sessions/zhipu_data`），启动前会：
1. 查找并终止使用该目录的旧Chrome进程（不影响用户正常使用的Chrome）
2. 清理singleton lock文件
3. 启动新的Chrome实例

## 验证记录

### 2026-05-13 验证结果（cc-Switch兼容性更新）

| 测试项 | 状态 | 说明 |
|--------|------|------|
| 非流式消息 | ✅ 通过 | 响应格式正确，包含所有必要字段 |
| 流式消息 | ✅ 通过 | SSE格式正确，含message_start、ping、content_block_start/delta/stop、message_delta/stop |
| Models列表 | ✅ 通过 | 返回5个可用模型（含Claude系列模型名） |
| 错误处理 | ✅ 通过 | 401/400/500错误响应格式正确 |
| cc-Switch URL兼容 | ✅ 通过 | 自动处理 `/v1/messages/v1/messages` 等重复路径 |
| CORS跨域 | ✅ 通过 | 支持cc-Switch Web界面的跨域请求 |
| x-api-key认证 | ✅ 通过 | 支持Anthropic标准认证头 |
| Authorization: Bearer认证 | ✅ 通过 | 支持cc-Switch代理模式认证 |
| anthropic-version头 | ✅ 通过 | 正确接收并忽略版本头 |
| anthropic-beta头 | ✅ 通过 | 正确接收并忽略beta头 |
| ?beta=true参数 | ✅ 通过 | 正确接收并忽略beta查询参数 |
| system数组格式 | ✅ 通过 | 支持system字段的字符串和数组格式 |

## cc-Switch 连接测试问题排查

### 常见问题：Connection failed

如果cc-Switch提示 `Connection failed: error sending request for url (http://localhost:8081/v1/messages?beta=true)`，请按以下步骤排查：

#### 1. 检查cc-Switch全局代理设置（最常见原因）

cc-Switch的所有HTTP请求（包括连接测试）都会通过其配置的全局代理发送。如果全局代理不可用，连接测试会失败。

**排查方法**：
- 打开cc-Switch → 设置 → 代理
- 检查"全局代理"是否配置了代理URL（如 `http://127.0.0.1:10809`）
- 如果代理服务未运行，请**清除全局代理URL**或**启动代理服务**

**cc-Switch日志中的关键信息**：
```
[GlobalProxy] Initialized: http://127.0.0.1:10809  ← 代理已启用
```
如果看到这行日志，说明cc-Switch会通过该代理发送所有请求。如果代理不可用，连接会超时失败（通常约2秒）。

#### 2. 检查AIThink服务是否运行

```bash
curl http://localhost:8081/v1/messages
```

如果返回JSON响应，说明AIThink服务正常运行。

#### 3. 检查API Key配置

确保cc-Switch中配置的API Key与AIThink的 `api_keys.json` 文件中的密钥一致。

#### 4. 检查apiFormat设置

cc-Switch支持多种API格式，AIThink推荐使用 `anthropic` 格式（默认）。如果设置了 `openai_responses` 或 `openai_chat`，AIThink也支持这些格式的端点。

| apiFormat | 端点 | 说明 |
|-----------|------|------|
| anthropic（默认） | POST /v1/messages | Anthropic Messages API格式 |
| openai_responses | POST /v1/responses | OpenAI Responses API格式 |
| openai_chat | POST /v1/chat/completions | OpenAI Chat Completions格式 |

## 记忆管理

AIThink在Anthropic网关中集成了记忆管理流程，实现跨请求的对话上下文保持。

### 工作流程

```
请求到达 → 解析消息(ParseMessages) → 对话决策(ConversationManager.Decide) →
根据决策组装消息 → 发送Ask(带ConversationMode) → 处理后更新记忆(HandlePostAsk) → 返回响应
```

### 核心组件

| 组件 | 说明 |
|------|------|
| MemoryStore | 记忆持久化存储，以JSON文件形式保存到 `data/memory` 目录 |
| MemoryManager | 记忆管理器，管理对话状态和记忆条目，支持并发安全访问 |
| ConversationManager | 对话生命周期管理器，负责跟踪对话状态、处理超时、决定何时重建对话 |
| MessageParser | 消息解析器，将请求消息拆分为固定提示词/记忆/新需求 |

### 对话决策逻辑

| 条件 | 决策 | 对话模式 | 说明 |
|------|------|----------|------|
| 活跃对话且提示词未变化 | ActionSendOnly | existing | 仅发送当前请求 |
| 活跃对话但提示词变化 | ActionRebuildAndSend | new | 重建对话，发送完整消息 |
| 对话过期/丢失 | ActionRebuildAndSend | new | 重建对话，发送完整消息 |
| 无对话记录 | ActionCreateAndSend | new | 新建对话，发送完整消息 |

### 配置参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| MaxEntries | 20 | 每个API Key最多保留的记忆条目数 |
| RepeatThreshold | 3 | 重复出现次数超过阈值触发精简 |
| ConversationTimeout | 30分钟 | 对话超时时间，超时后标记为expired |

### 记忆更新

每次AI回复后，系统会自动执行以下操作：
1. 将AI回复添加到记忆（相同内容增加重复计数）
2. 更新对话活跃时间
3. 精简记忆（防止记忆过多）

## 注意事项

1. 当前版本不支持图片输入（多模态）
2. 当前版本不支持工具调用（tools）
3. `input_tokens`和`output_tokens`为估算值（字符数/4）
4. 服务默认监听在8081端口
5. 首次提问可能需要较长时间来启动浏览器和加载页面
6. **cc-Switch全局代理不可用会导致连接测试失败**，请确保代理服务运行或清除代理设置
