# AIThink - AI工具API服务

一个高性能、高并发的AI工具API服务，支持模拟登录多个AI模型在线网站。

## 功能特性

- 🚀 **高性能**: 基于Go语言和Gin框架，支持高并发低延迟访问
- 🌐 **浏览器自动化**: 使用chromedp进行真实的浏览器操作
- 🔐 **会话管理**: 支持多用户会话管理和复用
- 📡 **RESTful API**: 提供标准的REST API接口
- 🤖 **AI视觉识别**: 支持AI智能分析页面，解决网页动态变更问题（New！）
- ⚙️ **配置管理**: 提供API接口管理AI视觉识别服务的配置

## 目前支持的平台

- ✅ 智谱清言 (Zhipu ChatGLM)

## 项目结构

```
AIThink/
├── cmd/
│   └── server/          # 主程序入口
├── internal/
│   ├── api/            # API处理器和路由
│   ├── browser/        # 浏览器自动化逻辑（含AI视觉识别）
│   │   ├── page_analyzer.go      # 页面分析器
│   │   ├── image_analyzer.go     # 图片分析器（AI视觉）
│   │   └── zhipu.go             # 智谱清言实现
│   ├── config/         # 配置管理模块
│   ├── models/         # 数据模型定义
│   └── service/        # 业务逻辑层
├── configs/            # 配置文件
├── docs/              # 文档
│   └── AI_VISION_GUIDE.md  # AI视觉识别使用指南
└── go.mod             # Go模块文件
```

## 快速开始

### 前置要求

- Go 1.21+
- Chrome/Chromium浏览器（chromedp需要）

### 安装依赖

```bash
go mod download
go mod tidy
```

### 运行服务

```bash
# 开发模式
go run cmd/server/main.go

# 或编译后运行
make build
./bin/aithink
```

服务默认运行在 `http://localhost:8081`

## API接口

### 1. 健康检查

```http
GET /health
```

### 2. 登录AI平台

```http
POST /api/v1/login
Content-Type: application/json

{
    "platform": "zhipu",
    "username": "your_phone_number",
    "password": "your_password_or_code",
    "session_id": "optional_session_id"
}
```

响应:
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

### 3. 向AI提问

```http
POST /api/v1/ask
Content-Type: application/json

{
    "platform": "zhipu",
    "session_id": "zhipu_1234567890",
    "question": "你好，请介绍一下自己"
}
```

响应:
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

### 4. 登出

```http
POST /api/v1/logout?session_id=zhipu_1234567890
```

### 5. 配置管理（New！）

#### 获取配置

```http
GET /api/v1/config
```

响应:
```json
{
    "code": 0,
    "message": "获取配置成功",
    "data": {
        "image_ai": {
            "provider": "openai",
            "openai": {
                "base_url": "https://api.openai.com/v1",
                "model": "gpt-4-vision-preview",
                "has_key": true
            }
        }
    }
}
```

#### 更新配置

```http
POST /api/v1/config
Content-Type: application/json

{
    "provider": "openai",
    "openai": {
        "api_key": "sk-your-openai-key",
        "base_url": "https://api.openai.com/v1",
        "model": "gpt-4-vision-preview"
    }
}
```

## AI视觉识别功能

AIThink现在支持使用AI视觉识别技术来智能分析网页，解决网页动态变更导致选择器失效的问题。

### 工作原理

1. **常规方法优先**：首先尝试使用JavaScript和DOM查询查找元素
2. **AI后备方案**：如果常规方法失败，自动使用AI视觉识别
3. **截图分析**：对当前页面截图并发送给配置的AI服务
4. **智能操作**：使用AI返回的选择器进行操作

### 支持的AI服务

- **OpenAI GPT-4V**（推荐）：识别准确度高
- **百度OCR**：国内访问快
- **腾讯云OCR**：企业级服务
- **自定义API**：接入任何支持图片分析的API

### 配置示例

#### 使用OpenAI GPT-4V

```bash
curl -X POST http://localhost:8081/api/v1/config \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "openai",
    "openai": {
      "api_key": "sk-your-openai-key",
      "base_url": "https://api.openai.com/v1",
      "model": "gpt-4-vision-preview"
    }
  }'
```

#### 使用百度OCR

```bash
curl -X POST http://localhost:8081/api/v1/config \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "baidu",
    "baidu": {
      "api_key": "your-baidu-api-key",
      "secret_key": "your-baidu-secret-key"
    }
  }'
```

### 更多文档

详细使用指南请参考：[AI视觉识别使用指南](docs/AI_VISION_GUIDE.md)

## 注意事项

1. **浏览器要求**: 需要确保系统安装了Chrome或Chromium浏览器
2. **验证码处理**: 智谱清言登录需要验证码，当前版本需要手动处理
3. **会话管理**: 会话ID用于复用浏览器会话，避免重复登录
4. **并发限制**: 虽然支持高并发，但请注意目标网站的限制
5. **AI视觉识别**: 配置AI服务后，系统会自动在常规方法失败时启用AI视觉识别
   - OpenAI GPT-4V推荐使用，识别准确度高
   - 使用AI识别会产生API调用成本
   - 截图默认保存在 `screenshots/` 目录

## 开发计划

- [x] 完善智谱清言的登录流程（自动处理验证码）
- [x] 实现AI视觉识别功能（New！）
- [x] 添加配置管理API（New！）
- [ ] 添加ChatGPT支持
- [ ] 添加Claude支持
- [ ] 实现会话池管理
- [ ] 添加API限流和认证
- [ ] 支持Docker部署
- [ ] 实现识别结果缓存
- [ ] 添加更多AI视觉服务支持

## 许可证

MIT License
