// Package platform 平台抽象层，定义AI平台客户端的统一接口和注册机制
// 通过接口抽象，支持多种AI平台（智谱清言、ChatGPT、Claude等）的统一管理
package platform

import (
	"time"

	"aithink/internal/browser"
	"aithink/internal/models"
)

// AskResult 提问结果，包含答案和检测信息
// 所有平台的提问操作均返回此结构体
type AskResult struct {
	Answer     string        // 完整答案
	Thinking   string        // 思考过程（Extended Thinking）
	IsBot      bool          // 是否被检测为机器人
	DetectInfo string        // 检测信息
	StreamChan <-chan string // 流式返回通道（可选）
}

// PlatformClient AI平台客户端接口
// 所有平台实现必须满足此接口，以便注册器统一管理
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

// PlatformConfig 平台配置，包含URL、CSS选择器和超时设置
type PlatformConfig struct {
	Platform models.Platform            // 平台类型
	LoginURL string                     // 登录页面URL
	ChatURL  string                     // 聊天页面URL
	Selectors map[string]string         // CSS选择器配置（如输入框、发送按钮等）
	Timeouts  map[string]time.Duration  // 超时配置（如页面加载、回复等待等）
}

// Factory 平台客户端工厂函数类型
// 根据浏览器会话创建对应的平台客户端实例
type Factory func(*browser.BrowserSession) PlatformClient
