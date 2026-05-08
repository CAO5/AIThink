# AIThink API 文档

## 概述

AIThink 提供 RESTful API 接口，用于模拟登录和操作各种 AI 模型网站。

基础 URL: `http://localhost:8080`

## 通用响应格式

```json
{
    "code": 0,
    "message": "操作结果消息",
    "data": {}  // 可选，具体数据
}
```

状态码说明:
- `0`: 成功
- `400`: 请求参数错误
- `500`: 服务器内部错误

## 接口列表

### 1. 健康检查

检查服务是否正常运行。

**请求**
```http
GET /health
```

**响应**
```json
{
    "code": 0,
    "message": "服务正常运行"
}
```

### 2. 登录 AI 平台

登录到指定的 AI 平台，创建浏览器会话。

**请求**
```http
POST /api/v1/login
Content-Type: application/json

{
    "platform": "zhipu",
    "username": "13800138000",
    "password": "验证码或密码",
    "session_id": "optional_custom_session_id"
}
```

**请求参数说明**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| platform | string | 是 | 平台类型: `zhipu`(智谱清言), `chatgpt`, `claude` |
| username | string | 是 | 用户名/手机号 |
| password | string | 是 | 密码或验证码 |
| session_id | string | 否 | 自定义会话ID，不提供则自动生成 |

**响应**
```json
{
    "code": 0,
    "message": "登录成功",
    "data": {
        "session_id": "zhipu_1234567890",
        "message": "登录成功"
    }
}
```

### 3. 向 AI 提问

使用已登录的会话向 AI 平台提问。

**请求**
```http
POST /api/v1/ask
Content-Type: application/json

{
    "platform": "zhipu",
    "session_id": "zhipu_1234567890",
    "question": "你好，请介绍一下你自己"
}
```

**请求参数说明**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| platform | string | 是 | 平台类型，需与登录时一致 |
| session_id | string | 是 | 登录时返回的会话ID |
| question | string | 是 | 要提问的内容 |

**响应**
```json
{
    "code": 0,
    "message": "提问成功",
    "data": {
        "answer": "我是智谱清言，是由智谱AI公司开发的...",
        "session_id": "zhipu_1234567890"
    }
}
```

### 4. 登出

关闭指定的会话，释放浏览器资源。

**请求**
```http
POST /api/v1/logout?session_id=zhipu_1234567890
```

**查询参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| session_id | string | 是 | 要关闭的会话ID |

**响应**
```json
{
    "code": 0,
    "message": "登出成功"
}
```

## 使用示例

### cURL 示例

#### 登录智谱清言
```bash
curl -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{
    "platform": "zhipu",
    "username": "13800138000",
    "password": "123456"
  }'
```

#### 提问
```bash
curl -X POST http://localhost:8080/api/v1/ask \
  -H "Content-Type: application/json" \
  -d '{
    "platform": "zhipu",
    "session_id": "zhipu_1234567890",
    "question": "你好"
  }'
```

#### 登出
```bash
curl -X POST "http://localhost:8080/api/v1/logout?session_id=zhipu_1234567890"
```

### Python 示例

```python
import requests

# 登录
login_resp = requests.post("http://localhost:8080/api/v1/login", json={
    "platform": "zhipu",
    "username": "13800138000",
    "password": "123456"
})
session_id = login_resp.json()["data"]["session_id"]

# 提问
ask_resp = requests.post("http://localhost:8080/api/v1/ask", json={
    "platform": "zhipu",
    "session_id": session_id,
    "question": "你好"
})
answer = ask_resp.json()["data"]["answer"]
print(f"AI回答: {answer}")

# 登出
requests.post(f"http://localhost:8080/api/v1/logout?session_id={session_id}")
```

## 错误码

| 错误码 | 说明 |
|--------|------|
| 0 | 成功 |
| 400 | 请求参数错误 |
| 500 | 服务器内部错误 |

## 注意事项

1. **会话管理**: 登录后返回的 `session_id` 需要保存，用于后续提问和登出
2. **并发限制**: 虽然支持高并发，但请遵守目标网站的使用限制
3. **验证码**: 目前智谱清言登录需要手动处理验证码
4. **浏览器依赖**: 需要系统安装 Chrome/Chromium 浏览器
