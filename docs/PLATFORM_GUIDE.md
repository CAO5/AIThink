# 平台接入指南

## 概述

AIThink 采用平台注册器架构，新增 AI 平台只需实现 `PlatformClient` 接口并注册即可，无需修改核心代码。注册器使用工厂模式 + 单例模式，支持运行时动态注册新平台。

## 已支持平台

| 平台 | Platform 常量 | 登录URL | 聊天URL | 代码位置 |
|------|--------------|---------|---------|---------|
| 智谱清言 | `PlatformZhipu` | https://chatglm.cn/ | https://chatglm.cn/main/ | `internal/platform/zhipu/` |
| 豆包 | `PlatformDoubao` | https://www.doubao.com/ | https://www.doubao.com/chat/ | `internal/platform/doubao/` |
| 千问 | `PlatformQwen` | https://tongyi.aliyun.com/ | https://tongyi.aliyun.com/qianwen/ | `internal/platform/qwen/` |
| DeepSeek | `PlatformDeepSeek` | https://chat.deepseek.com/ | https://chat.deepseek.com/ | `internal/platform/deepseek/` |
| ChatGPT | `PlatformChatGPT` | https://chatgpt.com/ | https://chatgpt.com/ | `internal/platform/gpt/` |

## PlatformClient 接口

`PlatformClient` 是所有平台客户端必须实现的统一接口，定义在 `internal/platform/interface.go`：

```go
// PlatformClient AI平台客户端接口
type PlatformClient interface {
    // GetPlatformName 获取平台名称
    GetPlatformName() string

    // GetLoginURL 获取平台登录页面URL
    GetLoginURL() string

    // GetChatURL 获取平台聊天页面URL
    GetChatURL() string

    // NavigateToHome 导航到平台首页（用于加载cookies后使cookies生效）
    NavigateToHome() error

    // CheckLoggedIn 检查当前是否已登录
    CheckLoggedIn() bool

    // OpenLoginPage 打开登录页面（供用户手动登录）
    OpenLoginPage() error

    // Ask 向平台提问，返回完整答案
    Ask(question string) (*AskResult, error)

    // AskInConversation 在已有对话中继续提问
    AskInConversation(question string) (*AskResult, error)

    // StartNewConversation 新建对话并发送初始消息
    StartNewConversation(initialMessage string) (*AskResult, error)
}
```

### AskResult 结构体

所有提问操作均返回 `AskResult`：

```go
type AskResult struct {
    Answer     string        // 完整答案
    Thinking   string        // 思考过程（Extended Thinking）
    IsBot      bool          // 是否被检测为机器人
    DetectInfo string        // 检测信息
    StreamChan <-chan string // 流式返回通道（可选）
}
```

### PlatformConfig 结构体

平台配置包含 URL、CSS 选择器和超时设置：

```go
type PlatformConfig struct {
    Platform models.Platform            // 平台类型
    LoginURL string                     // 登录页面URL
    ChatURL  string                     // 聊天页面URL
    Selectors map[string]string         // CSS选择器配置
    Timeouts  map[string]time.Duration  // 超时配置
}
```

## 新增平台步骤

### 步骤1：创建平台目录

在 `internal/platform/` 下创建以平台名命名的子目录：

```
internal/platform/{platform_name}/
```

例如新增 Kimi 平台：

```
internal/platform/kimi/
```

### 步骤2：实现 PlatformClient 接口

创建实现文件 `internal/platform/kimi/kimi.go`，以下为代码模板：

```go
// Package kimi Kimi平台适配器
// 实现PlatformClient接口，将Kimi平台的功能适配到统一平台架构
package kimi

import (
    "context"
    "log"
    "time"

    "github.com/chromedp/chromedp"

    "aithink/internal/browser"
    "aithink/internal/models"
    "aithink/internal/platform"
)

// Kimi网站URL常量
const (
    kimiLoginURL = "https://kimi.moonshot.cn/"
    kimiChatURL  = "https://kimi.moonshot.cn/chat/"
)

// KimiClient Kimi平台客户端
type KimiClient struct {
    session *browser.BrowserSession
}

// NewKimiClient 创建Kimi客户端
func NewKimiClient(session *browser.BrowserSession) *KimiClient {
    return &KimiClient{session: session}
}

// init 注册Kimi平台到全局注册器
func init() {
    platform.GetRegistry().Register(
        models.PlatformKimi,
        func(session *browser.BrowserSession) platform.PlatformClient {
            return NewKimiClient(session)
        },
        &platform.PlatformConfig{
            Platform: models.PlatformKimi,
            LoginURL: "https://kimi.moonshot.cn/",
            ChatURL:  "https://kimi.moonshot.cn/chat/",
            Selectors: map[string]string{
                "input_box":       "textarea",
                "send_button":     "button[class*='send']",
                "response_area":   "[class*='assistant']",
                "new_chat_button": "[class*='new-chat']",
            },
        },
    )
}

// ==================== PlatformClient 接口实现 ====================

// GetPlatformName 获取平台名称
func (k *KimiClient) GetPlatformName() string {
    return "kimi"
}

// GetLoginURL 获取平台登录页面URL
func (k *KimiClient) GetLoginURL() string {
    return kimiLoginURL
}

// GetChatURL 获取平台聊天页面URL
func (k *KimiClient) GetChatURL() string {
    return kimiChatURL
}

// NavigateToHome 导航到Kimi首页（用于加载cookies后使cookies生效）
func (k *KimiClient) NavigateToHome() error {
    ctx := k.session.Ctx
    // 实现导航逻辑
    return chromedp.Run(ctx, chromedp.Navigate(kimiChatURL))
}

// CheckLoggedIn 检查当前是否已登录
func (k *KimiClient) CheckLoggedIn() bool {
    ctx := k.session.Ctx
    var exists bool
    // 实现登录检测逻辑：检查页面中是否存在登录后才有的元素
    err := chromedp.Run(ctx,
        chromedp.WaitReady("body", chromedp.ByQuery),
        // 替换为实际的登录状态检测选择器
        chromedp.Evaluate(`document.querySelector('.user-avatar') !== null`, &exists),
    )
    return err == nil && exists
}

// OpenLoginPage 打开登录页面（供用户手动登录）
func (k *KimiClient) OpenLoginPage() error {
    ctx := k.session.Ctx
    return chromedp.Run(ctx, chromedp.Navigate(kimiLoginURL))
}

// Ask 向平台提问，返回完整答案
func (k *KimiClient) Ask(question string) (*platform.AskResult, error) {
    // 实现提问逻辑：输入问题 → 点击发送 → 等待回复 → 提取答案
    return nil, nil
}

// AskInConversation 在已有对话中继续提问
func (k *KimiClient) AskInConversation(question string) (*platform.AskResult, error) {
    // 实现继续对话逻辑：直接在当前输入框输入并发送
    return nil, nil
}

// StartNewConversation 新建对话并发送初始消息
func (k *KimiClient) StartNewConversation(initialMessage string) (*platform.AskResult, error) {
    // 实现新建对话逻辑：点击新建对话按钮 → 输入消息 → 发送
    return nil, nil
}
```

### 步骤3：注册平台

平台通过 `init()` 函数自动注册到全局注册器，无需手动调用注册方法。`init()` 函数在包被导入时自动执行：

```go
func init() {
    platform.GetRegistry().Register(
        models.PlatformKimi,       // 平台常量
        func(session *browser.BrowserSession) platform.PlatformClient {
            return NewKimiClient(session)  // 工厂函数
        },
        &platform.PlatformConfig{   // 平台配置
            Platform: models.PlatformKimi,
            LoginURL: "https://kimi.moonshot.cn/",
            ChatURL:  "https://kimi.moonshot.cn/chat/",
            Selectors: map[string]string{...},
        },
    )
}
```

### 步骤4：更新模型定义

在 `internal/models/types.go` 中添加平台常量：

```go
const (
    // ... 已有平台常量 ...
    PlatformKimi Platform = "kimi"  // Kimi
)
```

### 步骤5：添加空白导入

在以下文件中添加空白导入，确保 `init()` 函数被执行：

**1. `internal/service/aithink.go`**（主要导入位置）：

```go
import (
    _ "aithink/internal/platform/kimi"  // 通过 init() 注册Kimi平台
)
```

**2. `internal/api/anthropic_gateway.go`**（如果 Anthropic 网关需要使用该平台）：

```go
import (
    _ "aithink/internal/platform/kimi"  // 确保Kimi平台注册（init）
)
```

**3. `internal/service/session_manager.go`**（如果会话管理器需要使用该平台）：

```go
import (
    _ "aithink/internal/platform/kimi"  // 通过 init() 注册Kimi平台
)
```

> **注意**：空白导入只需在真正使用该平台的包中添加。由于 Go 的 `sync.Once` 机制，`init()` 中的 `Register()` 只会执行一次，多次导入不会重复注册。

## 选择器配置

### 各平台关键 CSS 选择器

| 选择器键 | 说明 | 智谱清言 | ChatGPT | 豆包 | 千问 | DeepSeek |
|---------|------|---------|---------|------|------|----------|
| input_box | 输入框 | `textarea` | `#prompt-textarea, textarea` | `textarea, div[contenteditable='true']` | `textarea, div[contenteditable='true']` | `textarea, #chat-input` |
| send_button | 发送按钮 | `button[type='submit']` | `button[data-testid='send-button']` | `button[class*='send'], [class*='submit']` | `button[class*='send']` | `button[class*='send']` |
| response_area | 回复区域 | `.message-content, .markdown-body` | `[data-message-author-role='assistant']` | `[class*='assistant'], [class*='message-content']` | `[class*='assistant'], [class*='message-content']` | `[class*='assistant'], .markdown-body` |
| new_chat_button | 新建对话 | `新建对话` | `a[href='/']` | `[class*='new-chat'], [class*='new-conversation']` | `[class*='new-chat']` | `[class*='new-chat'], a[href='/']` |

### 选择器配置方式

选择器在 `init()` 函数中通过 `PlatformConfig.Selectors` 注册：

```go
Selectors: map[string]string{
    "input_box":       "textarea",                    // 输入框
    "send_button":     "button[type='submit']",       // 发送按钮
    "response_area":   ".message-content",            // 回复区域
    "new_chat_button": "新建对话",                     // 新建对话按钮
},
```

> **提示**：选择器支持 CSS 选择器语法，多个选择器用逗号分隔表示优先级（从左到右依次尝试）。部分平台使用文本匹配（如智谱的"新建对话"）而非 CSS 选择器。

### 选择器获取方法

1. 打开目标平台的网页
2. 按 F12 打开开发者工具
3. 使用元素选择器（Ctrl+Shift+C）点击目标元素
4. 右键 → 复制 → Copy selector
5. 在代码中使用获取到的选择器

## 平台注册器 API

`PlatformRegistry` 提供以下方法：

| 方法 | 说明 |
|------|------|
| `GetRegistry()` | 获取注册器单例（首次调用初始化） |
| `Register(platform, factory, config)` | 注册平台（工厂函数 + 配置） |
| `GetClient(platform, session)` | 根据平台类型创建客户端实例 |
| `GetConfig(platform)` | 获取指定平台的配置 |
| `ListPlatforms()` | 列出所有已注册平台（按名称排序） |
| `IsRegistered(platform)` | 检查指定平台是否已注册 |

## 调试技巧

### 1. 非无头模式调试

设置环境变量或修改配置，将浏览器设为非无头模式，可以看到实际操作过程：

```go
// 在 browser.go 中设置 headless: false
opts := append(chromedp.DefaultExecAllocatorOptions[:],
    chromedp.Flag("headless", false),
)
```

### 2. 选择器验证

在浏览器开发者工具的 Console 中验证选择器是否正确：

```javascript
// 验证输入框选择器
document.querySelector('textarea')

// 验证发送按钮选择器
document.querySelector("button[type='submit']")

// 验证回复区域选择器
document.querySelectorAll("[class*='assistant']")
```

### 3. 日志调试

各平台适配器中已内置详细日志，运行时观察日志输出：

```
[DeepSeek] 正在导航到首页: https://chat.deepseek.com/
[DeepSeek] 检查登录状态...
[DeepSeek] 已登录
[DeepSeek] 发送问题: 你好
[DeepSeek] 等待AI回复...
[DeepSeek] AI回复完成，长度=128
```

### 4. 页面异常检测

各平台实现了 `detectPageAnomaly()` 方法，可检测以下异常：

- 输入超限（字数限制）
- 人机验证（滑块验证码）
- Cloudflare 防护
- 页面加载失败

### 5. 滑块验证码处理

豆包和 DeepSeek 等平台实现了自动滑块验证码处理：

```go
// generateHumanTrajectory - 生成模拟人类的拖拽轨迹
// executeSliderDrag - 使用 chromedp 鼠标事件执行滑块拖拽
// handleSliderCaptcha - 自动处理滑块验证码
```

如果自动处理失败，可以在非无头模式下手动完成验证。

### 6. 常见问题排查

| 问题 | 可能原因 | 解决方案 |
|------|---------|---------|
| 选择器找不到元素 | 网站结构更新 | 用开发者工具重新获取选择器 |
| 登录检测失败 | 登录状态元素变化 | 更新 `CheckLoggedIn()` 中的选择器 |
| 回复提取为空 | 回复区域选择器不匹配 | 更新 `response_area` 选择器 |
| 超时 | 网络慢或回复过长 | 增加超时时间 |
| 人机验证 | 被检测为机器人 | 使用非无头模式手动验证 |

## 相关代码文件

| 文件 | 说明 |
|------|------|
| [interface.go](../internal/platform/interface.go) | 平台客户端接口与通用类型定义 |
| [registry.go](../internal/platform/registry.go) | 平台注册器（工厂模式+单例） |
| [types.go](../internal/models/types.go) | 平台常量定义 |
| [zhipu.go](../internal/platform/zhipu/zhipu.go) | 智谱清言适配器 |
| [gpt.go](../internal/platform/gpt/gpt.go) | ChatGPT 适配器 |
| [doubao.go](../internal/platform/doubao/doubao.go) | 豆包适配器 |
| [qwen.go](../internal/platform/qwen/qwen.go) | 千问适配器 |
| [deepseek.go](../internal/platform/deepseek/deepseek.go) | DeepSeek 适配器 |
