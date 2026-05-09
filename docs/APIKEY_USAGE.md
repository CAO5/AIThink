# AIThink API Key 使用指南

## 概述

AIThink 提供 API Key 认证方式，允许用户通过 API Key 调用提问功能，而无需关心底层浏览器会话管理。系统会自动将 API Key 关联到对应的浏览器会话。

## 使用流程

### 步骤 1：登录并创建浏览器会话

首先需要通过传统方式登录，获取 `session_id`：

```bash
curl -X POST http://localhost:8081/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{
    "platform": "zhipu",
    "username": "your_username",
    "password": "your_password"
  }'
```

返回：
```json
{
  "code": 0,
  "message": "浏览器已打开，请手动完成登录",
  "data": {
    "session_id": "zhipu_1234567890",
    "message": "登录成功"
  }
}
```

### 步骤 2：创建 API Key

使用 `session_id` 创建 API Key：

```bash
curl -X POST http://localhost:8081/api/v1/apikey/create \
  -H "Content-Type: application/json" \
  -d '{
    "platform": "zhipu",
    "name": "我的智谱密钥",
    "session_id": "zhipu_1234567890"
  }'
```

返回：
```json
{
  "code": 0,
  "message": "API密钥创建成功，请妥善保存（仅显示一次）",
  "data": {
    "api_key": "a1b2c3d4e5f6...",
    "platform": "zhipu",
    "name": "我的智谱密钥",
    "session_id": "zhipu_1234567890",
    "status": "active",
    "created_at": "2024-01-01T00:00:00Z",
    "request_count": 0,
    "full_api_key": "a1b2c3d4e5f6..."
  }
}
```

**⚠️ 重要：`full_api_key` 仅在创建时返回一次，请妥善保存！**

### 步骤 3：使用 API Key 提问

现在可以使用 API Key 进行提问，无需提供 `session_id`：

#### 方式 1：通过请求头传递 API Key（推荐）

```bash
curl -X POST http://localhost:8081/api/v1/apikey/ask \
  -H "Content-Type: application/json" \
  -H "X-API-Key: a1b2c3d4e5f6..." \
  -d '{
    "question": "你好，请介绍一下你自己"
  }'
```

#### 方式 2：通过查询参数传递 API Key

```bash
curl -X POST "http://localhost:8081/api/v1/apikey/ask?api_key=a1b2c3d4e5f6..." \
  -H "Content-Type: application/json" \
  -d '{
    "question": "你好，请介绍一下你自己"
  }'
```

返回：
```json
{
  "code": 0,
  "message": "提问成功",
  "data": {
    "answer": "我是智谱清言...",
    "session_id": "zhipu_1234567890"
  }
}
```

## API Key 管理接口

### 1. 创建 API Key

**POST** `/api/v1/apikey/create`

请求体：
```json
{
  "platform": "zhipu",
  "name": "密钥名称",
  "session_id": "zhipu_1234567890",
  "expires_at": "2025-01-01T00:00:00Z"
}
```

参数说明：
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| platform | string | 是 | 平台类型：zhipu, chatgpt, claude |
| name | string | 是 | 密钥名称 |
| session_id | string | 是 | 已登录的会话ID |
| expires_at | string | 否 | 过期时间（ISO 8601格式） |

### 2. 列出所有 API Key

**GET** `/api/v1/apikey/list`

返回：
```json
{
  "code": 0,
  "message": "查询成功",
  "data": {
    "total": 2,
    "items": [
      {
        "api_key": "a1b2c3d4...",
        "platform": "zhipu",
        "name": "我的智谱密钥",
        "status": "active",
        "created_at": "2024-01-01T00:00:00Z",
        "request_count": 10
      }
    ]
  }
}
```

### 3. 更新 API Key

**POST** `/api/v1/apikey/update/{apikey}`

请求体：
```json
{
  "name": "新名称",
  "status": "inactive"
}
```

### 4. 删除 API Key

**POST** `/api/v1/apikey/delete/{apikey}`

## 使用 API Key 提问

### 提问接口

**POST** `/api/v1/apikey/ask`

请求头：
- `X-API-Key`: 您的API密钥（推荐）
- 或查询参数 `?api_key=xxx`

请求体：
```json
{
  "question": "你的问题"
}
```

### 流式提问接口

**POST** `/api/v1/apikey/ask/stream`

与普通提问接口相同，用于后续扩展流式返回功能。

## Python 使用示例

```python
import requests

# 1. 登录
login_resp = requests.post("http://localhost:8081/api/v1/login", json={
    "platform": "zhipu",
    "username": "your_username",
    "password": "your_password"
})
session_id = login_resp.json()["data"]["session_id"]

# 2. 创建 API Key
apikey_resp = requests.post("http://localhost:8081/api/v1/apikey/create", json={
    "platform": "zhipu",
    "name": "测试密钥",
    "session_id": session_id
})
api_key = apikey_resp.json()["data"]["full_api_key"]
print(f"API Key: {api_key}")

# 3. 使用 API Key 提问
answer_resp = requests.post(
    "http://localhost:8081/api/v1/apikey/ask",
    headers={"X-API-Key": api_key},
    json={"question": "你好"}
)
answer = answer_resp.json()["data"]["answer"]
print(f"AI回答: {answer}")
```

## 注意事项

1. **API Key 安全**：API Key 等同于密码，请妥善保存，不要泄露
2. **过期管理**：可以为 API Key 设置过期时间，到期后自动失效
3. **使用统计**：系统会记录每个 API Key 的请求次数和最后使用时间
4. **状态管理**：可以停用/启用 API Key，无需删除
5. **会话依赖**：API Key 关联的浏览器会话关闭后，API Key 将无法使用
