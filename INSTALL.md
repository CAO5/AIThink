# AIThink 安装指南

## 系统要求

- **操作系统**: Windows 10+, Linux, macOS
- **Go**: 1.21 或更高版本
- **浏览器**: Chrome 或 Chromium（chromedp 需要）
- **内存**: 建议 4GB 以上（每个浏览器会话约占用 200-500MB）

## 安装步骤

### 1. 安装 Go 语言环境

#### Windows
1. 下载 Go 安装程序: https://go.dev/dl/go1.21.windows-amd64.msi
2. 运行安装程序，按照提示完成安装
3. 打开新的 PowerShell 窗口，验证安装:
   ```powershell
   go version
   ```
   应该输出类似: `go version go1.21 windows/amd64`

#### Linux
```bash
# 下载并解压
wget https://go.dev/dl/go1.21.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.linux-amd64.tar.gz

# 添加到 PATH（添加到 ~/.bashrc 或 ~/.zshrc）
export PATH=$PATH:/usr/local/go/bin

# 验证
go version
```

#### macOS
```bash
# 使用 Homebrew
brew install go

# 或下载安装包
# https://go.dev/dl/go1.21.darwin-amd64.pkg
```

### 2. 安装 Chrome/Chromium 浏览器

chromedp 需要 Chrome 或 Chromium 才能运行。

#### Windows
- 下载并安装 Google Chrome: https://www.google.com/chrome/

#### Linux (Ubuntu/Debian)
```bash
sudo apt update
sudo apt install chromium-browser
```

#### macOS
```bash
brew install --cask chromium
```

### 3. 获取 AIThink 代码

```bash
cd d:\wan_workspase\AIThink
# 或使用 git clone（如果从远程仓库）
```

### 4. 安装项目依赖

```bash
cd d:\wan_workspase\AIThink
go mod download
go mod tidy
```

这会下载以下依赖:
- gin: Web 框架
- chromedp: 浏览器自动化库
- 其他依赖项

### 5. 编译运行

#### 方式一: 直接运行（开发模式）
```bash
go run cmd/server/main.go
```

#### 方式二: 编译后运行（生产模式）
```bash
# 编译
go build -o bin/aithink.exe cmd/server/main.go

# 运行
./bin/aithink.exe
```

看到类似输出表示启动成功:
```
[INFO] 服务启动在 http://localhost:8080
[INFO] 健康检查: http://localhost:8080/health
[INFO] 登录接口: POST http://localhost:8080/api/v1/login
[INFO] 提问接口: POST http://localhost:8080/api/v1/ask
```

## 验证安装

### 1. 健康检查
打开浏览器或使用 curl:
```bash
curl http://localhost:8080/health
```

应该返回:
```json
{
  "code": 0,
  "message": "服务正常运行"
}
```

### 2. 测试登录（智谱清言）

```bash
curl -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{
    "platform": "zhipu",
    "username": "你的手机号",
    "password": "验证码"
  }'
```

**注意**: 
- 智谱清言登录需要验证码，首次运行会打开浏览器窗口
- 需要手动输入验证码完成登录
- 登录成功后会返回 session_id

### 3. 测试提问

```bash
curl -X POST http://localhost:8080/api/v1/ask \
  -H "Content-Type: application/json" \
  -d '{
    "platform": "zhipu",
    "session_id": "上一步返回的session_id",
    "question": "你好，请介绍一下自己"
  }'
```

## 常见问题

### 1. `go: command not found`
**原因**: Go 未安装或未添加到 PATH

**解决**: 
- 确认 Go 已安装
- 将 Go 的 bin 目录添加到 PATH 环境变量
- Windows: `C:\Program Files\Go\bin`
- Linux/macOS: `/usr/local/go/bin`

### 2. chromedp: exec: "chrome": executable file not found
**原因**: 未安装 Chrome 或 chromedp 找不到浏览器

**解决**:
- 安装 Chrome 浏览器
- 或在代码中指定浏览器路径:
  ```go
  opts := append(chromedp.DefaultExecAllocatorOptions[:],
      chromedp.ExecPath("C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe"),
  )
  ```

### 3. 端口 8080 被占用
**解决**: 修改 `cmd/server/main.go` 中的端口号，或设置环境变量:
```bash
set PORT=8081  # Windows
export PORT=8081  # Linux/macOS
```

### 4. 登录时验证码无法自动处理
**说明**: 当前版本需要手动处理验证码

**解决方法**:
- 运行时会打开浏览器窗口
- 在浏览器中手动输入验证码
- 等待程序自动继续

## 性能调优

### 1. 启用无头模式（生产环境）
修改 `internal/browser/browser.go`:
```go
opts := append(chromedp.DefaultExecAllocatorOptions[:],
    chromedp.Flag("headless", true),  // 改为 true
    chromedp.Flag("disable-gpu", true),
)
```

### 2. 调整并发数
Go 的 goroutine 天然支持高并发，但需要注意:
- 每个会话一个浏览器实例，占用资源较多
- 建议实现会话池，复用已登录的会话
- 根据服务器内存调整最大并发数

### 3. 使用编译优化
```bash
go build -ldflags="-s -w" -o bin/aithink cmd/server/main.go
```

## 下一步

- 阅读 [README.md](README.md) 了解项目功能
- 阅读 [docs/API.md](docs/API.md) 了解 API 接口
- 阅读 [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) 了解开发指南

## 获取帮助

如果遇到问题:
1. 查看日志输出
2. 检查浏览器是否正常打开
3. 查看 [开发文档](docs/DEVELOPMENT.md) 中的常见问题部分
