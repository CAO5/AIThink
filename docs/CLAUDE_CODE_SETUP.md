# Claude Code 使用 AIThink 配置指南

## 概述

本指南介绍如何配置 Claude Code 使用 AIThink 服务，通过浏览器模拟的方式调用智谱清言、ChatGPT、Claude 等 AI 平台。

## 架构原理

```
Claude Code → AIThink Anthropic兼容网关 → 浏览器会话 → AI平台(智谱/Claude/ChatGPT)
```

AIThink 提供兼容 Anthropic API 和 OpenAI API 的接口，Claude Code 可以直接使用。

### cc-Switch 集成架构

```
Claude Code → cc-Switch代理 → AIThink Anthropic兼容网关 → 浏览器会话 → AI平台
```

cc-Switch 是 Claude Code 的配置管理工具，可以一键切换不同的 API 提供商。

## 配置步骤

### 步骤1：确保 AIThink 服务运行

```bash
# 编译
cd d:\wan_workspase\AIThink
go build -o bin/aithink.exe cmd/server/main.go

# 启动服务
bin\aithink.exe
```

服务默认运行在 `http://localhost:8081`

### 步骤2：登录并创建 API Key

```bash
# 1. 登录智谱清言
curl -X POST http://localhost:8081/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"platform": "zhipu", "username": "test", "password": "test"}'

# 2. 等待登录成功后，创建 API Key
curl -X POST http://localhost:8081/api/v1/apikey/create \
  -H "Content-Type: application/json" \
  -d '{
    "platform": "zhipu",
    "name": "Claude Code密钥",
    "session_id": "zhipu_你的session_id"
  }'
```

保存返回的 `full_api_key` 值。

### 步骤3：配置 Claude Code

#### 方法1：通过环境变量（推荐）

在启动 Claude Code 之前设置环境变量：

**Windows PowerShell:**
```powershell
$env:ANTHROPIC_BASE_URL = "http://localhost:8081"
$env:ANTHROPIC_AUTH_TOKEN = "你的api_key"
claude
```

**Windows CMD:**
```cmd
set ANTHROPIC_BASE_URL=http://localhost:8081
set ANTHROPIC_AUTH_TOKEN=你的api_key
claude
```

**Linux/Mac:**
```bash
export ANTHROPIC_BASE_URL=http://localhost:8081
export ANTHROPIC_AUTH_TOKEN=你的api_key
claude
```

#### 方法2：通过 Claude Code 配置文件

编辑 `~/.claude/settings.json`：

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://localhost:8081",
    "ANTHROPIC_AUTH_TOKEN": "你的api_key"
  },
  "model": "zhipu-glm-5"
}
```

#### 方法3：使用启动脚本

创建 `claude-aithink.bat`（Windows）：

```batch
@echo off
set ANTHROPIC_BASE_URL=http://localhost:8081
set ANTHROPIC_AUTH_TOKEN=你的api_key
claude %*
```

创建 `claude-aithink.sh`（Linux/Mac）：

```bash
#!/bin/bash
export ANTHROPIC_BASE_URL=http://localhost:8081
export ANTHROPIC_AUTH_TOKEN=你的api_key
claude "$@"
```

### 步骤4：验证配置

```bash
# 使用 curl 测试 Anthropic 兼容接口
curl -X POST http://localhost:8081/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: 你的api_key" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4-5-20250929",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "你好，请介绍一下你自己"}
    ]
  }'
```

### 步骤5：使用 cc-Switch 配置（可选）

如果使用 cc-Switch 管理 Claude Code 的 API 配置：

1. 打开 cc-Switch Web 界面：`cc-switch web`
2. 添加新供应商，填写以下信息：
   - 供应商名称：`AIThink`
   - API Key：你的 AIThink API Key
   - 请求地址：`http://localhost:8081`（**注意：不要包含 `/v1/messages` 路径**）
3. 切换到 AIThink 配置：`cc-switch use AIThink`
4. 测试连接：`cc-switch test`

**cc-Switch 配置注意事项：**
- 请求地址必须只填写到端口号，不要追加任何路径
- AIThink 已内置兼容处理，即使误配了路径也能自动纠正
- 支持通过 `ANTHROPIC_API_KEY` 环境变量使用 `x-api-key` 认证头

## 可用模型

AIThink 提供以下虚拟模型：

| 模型 ID | 实际平台 |
|---------|----------|
| `zhipu-glm-5` | 智谱清言 |
| `claude-sonnet` | Claude |
| `chatgpt-gpt-4` | ChatGPT |

注意：实际使用的平台由 API Key 绑定时决定。

## 流式支持

AIThink 网关支持 OpenAI 兼容的流式响应：

```bash
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer 你的api_key" \
  -d '{
    "model": "zhipu-glm-5",
    "messages": [
      {"role": "user", "content": "你好"}
    ],
    "stream": true
  }'
```

## 常见问题

### 1. 认证错误

如果 Claude Code 提示认证错误，请检查：
- API Key 是否正确
- API Key 是否处于活跃状态
- API Key 是否已过期

### 2. cc-Switch 测试失败：URL路径重复

**错误示例：** `Connection failed: error sending request for url (http://localhost:8081/v1/messages/v1/messages?beta=true)`

**原因：** cc-Switch 的"请求地址"配置包含了 API 路径（如 `http://localhost:8081/v1/messages`），导致 SDK 追加路径后出现重复。

**解决方案：**
- 将 cc-Switch 的"请求地址"改为 `http://localhost:8081`（不含任何路径）
- AIThink 已内置兼容处理，即使路径重复也会自动纠正，但建议使用正确配置

### 3. cc-Switch 认证失败：Missing API key

**原因：** cc-Switch 代理模式默认使用 `Authorization: Bearer` 头发送 API Key，但某些上游服务要求 `x-api-key` 头。

**解决方案：**
- AIThink 已同时支持 `x-api-key` 和 `Authorization: Bearer` 两种认证方式
- 确保 cc-Switch 中配置的 API Key 与 AIThink 创建的 API Key 一致

### 4. 浏览器会话失效

如果浏览器会话关闭或过期：
1. 重新登录：`POST /api/v1/login`
2. 获取新 session_id
3. 创建新的 API Key

### 5. 响应包含思考过程

智谱清言的"深度思考"功能可能会在回复前显示思考过程。AIThink 已自动尝试过滤这些内容，但某些情况下可能仍会显示。

## 安全建议

1. **不要将 API Key 提交到代码仓库**
2. **使用环境变量存储 API Key**
3. **定期轮换 API Key**
4. **为不同用途创建不同的 API Key**
5. **设置 API Key 过期时间**

## 高级配置

### 配置超时

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://localhost:8081",
    "ANTHROPIC_AUTH_TOKEN": "你的api_key",
    "ANTHROPIC_TIMEOUT_MS": "120000"
  }
}
```

### 使用自定义端口

如果修改了 AIThink 服务端口：

```bash
# 启动时指定端口
bin\aithink.exe --port 8080

# Claude Code 配置
export ANTHROPIC_BASE_URL=http://localhost:8080
```
