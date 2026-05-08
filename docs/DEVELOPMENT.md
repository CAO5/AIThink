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
│   │   └── router.go  # 路由配置
│   ├── browser/       # 浏览器自动化层
│   │   ├── browser.go # 浏览器管理器
│   │   └── zhipu.go   # 智谱清言实现
│   ├── models/        # 数据模型层
│   │   └── types.go   # 数据结构定义
│   └── service/       # 业务逻辑层
│       └── aithink.go # AI 服务实现
├── configs/           # 配置文件
│   └── config.yaml    # YAML 配置
├── docs/              # 文档
│   ├── API.md         # API 接口文档
│   └── DEVELOPMENT.md # 本开发文档
├── go.mod             # Go 模块定义
├── Makefile           # 构建脚本
└── README.md          # 项目说明
```

### 分层架构说明

1. **API 层** (`internal/api`): 处理 HTTP 请求和响应
2. **Service 层** (`internal/service`): 业务逻辑处理
3. **Browser 层** (`internal/browser`): 浏览器自动化操作
4. **Models 层** (`internal/models`): 数据结构和类型定义

## 核心代码说明

### 1. 浏览器管理器 (`browser/browser.go`)

使用单例模式管理浏览器会话，支持:
- 创建/关闭浏览器会话
- 会话复用
- 基本的浏览器操作（导航、点击、输入等）

### 2. 智谱清言实现 (`browser/zhipu.go`)

实现智谱清言网站的自动化操作:
- 登录流程（手机号+验证码）
- 提问和获取回复
- 会话管理

### 3. API 接口 (`api/handler.go`)

提供 RESTful API:
- `POST /api/v1/login` - 登录
- `POST /api/v1/ask` - 提问
- `POST /api/v1/logout` - 登出
- `GET /health` - 健康检查

## 开发指南

### 添加新平台支持

1. 在 `internal/models/types.go` 中添加平台常量
2. 在 `internal/browser/` 下创建新平台的实现文件
3. 在 `internal/service/aithink.go` 中添加平台处理逻辑

示例（添加 ChatGPT 支持）:

```go
// internal/browser/chatgpt.go
package browser

type ChatGPTClient struct {
    session *BrowserSession
}

func NewChatGPTClient(session *BrowserSession) *ChatGPTClient {
    return &ChatGPTClient{session: session}
}

func (c *ChatGPTClient) Login(username, password string) error {
    // 实现 ChatGPT 登录逻辑
    return nil
}

func (c *ChatGPTClient) Ask(question string) (string, error) {
    // 实现提问逻辑
    return "", nil
}
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
