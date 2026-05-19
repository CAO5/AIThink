// Package gpt ChatGPT平台适配器
// 实现PlatformClient接口，将ChatGPT网站的功能适配到统一平台架构
package gpt

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/chromedp/chromedp"

	"aithink/internal/browser"
	"aithink/internal/models"
	"aithink/internal/platform"
)

// ChatGPT网站URL常量
const (
	gptLoginURL = "https://chatgpt.com/"
	gptChatURL  = "https://chatgpt.com/"
)

// GPTClient ChatGPT平台客户端
// 持有BrowserSession以复用浏览器会话管理能力
type GPTClient struct {
	session *browser.BrowserSession
}

// NewGPTClient 创建ChatGPT客户端
func NewGPTClient(session *browser.BrowserSession) *GPTClient {
	return &GPTClient{session: session}
}

// init 注册ChatGPT平台到全局注册器
func init() {
	platform.GetRegistry().Register(
		models.PlatformChatGPT,
		func(session *browser.BrowserSession) platform.PlatformClient {
			return NewGPTClient(session)
		},
		&platform.PlatformConfig{
			Platform: models.PlatformChatGPT,
			LoginURL: "https://chatgpt.com/",
			ChatURL:  "https://chatgpt.com/",
			Selectors: map[string]string{
				"input_box":       "#prompt-textarea, textarea",
				"send_button":     "button[data-testid='send-button']",
				"response_area":   "[data-message-author-role='assistant']",
				"new_chat_button": "a[href='/']",
			},
		},
	)
}

// ==================== PlatformClient 接口实现 ====================

// GetPlatformName 获取平台名称
func (g *GPTClient) GetPlatformName() string {
	return "gpt"
}

// GetLoginURL 获取平台登录页面URL
func (g *GPTClient) GetLoginURL() string {
	return gptLoginURL
}

// GetChatURL 获取平台聊天页面URL
func (g *GPTClient) GetChatURL() string {
	return gptChatURL
}

// NavigateToHome 导航到ChatGPT首页（用于加载cookies后使cookies生效）
func (g *GPTClient) NavigateToHome() error {
	ctx := g.session.Ctx

	log.Printf("正在导航到ChatGPT首页: %s", gptChatURL)

	// 导航到主页
	if err := chromedp.Run(ctx,
		chromedp.Navigate(gptChatURL),
	); err != nil {
		log.Printf("导航失败: %v", err)
		return fmt.Errorf("导航失败: %v", err)
	}

	// 等待页面加载
	chromedp.Run(ctx, chromedp.Sleep(3*time.Second))

	// 注入反检测脚本
	log.Println("注入反检测脚本...")
	if err := g.session.InjectAntiDetection(); err != nil {
		log.Printf("注入反检测脚本失败: %v", err)
	}

	// 再次等待确保页面稳定
	chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	// 检查导航后的URL
	var currentURL string
	chromedp.Run(ctx, chromedp.Location(&currentURL))
	log.Printf("导航完成，当前URL: %s", currentURL)

	return nil
}

// CheckLoggedIn 检查是否已登录
// ChatGPT登录后会出现用户菜单按钮和导航栏
func (g *GPTClient) CheckLoggedIn() bool {
	ctx := g.session.Ctx
	log.Println("CheckLoggedIn: 开始检查ChatGPT登录状态")

	// 使用JavaScript检查登录状态
	var isLoggedIn bool
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				// 检查是否有用户菜单按钮（ChatGPT登录后的特征元素）
				var profileButton = document.querySelector('[data-testid="profile-button"]');
				if (profileButton) return true;

				// 检查是否有包含user相关类名的元素
				var userElements = document.querySelectorAll('[class*="user"], [class*="User"]');
				for (var i = 0; i < userElements.length; i++) {
					if (userElements[i].offsetParent !== null) return true;
				}

				// 检查是否有导航栏（登录后才有完整导航）
				var nav = document.querySelector('nav');
				if (nav) return true;

				// 检查是否有聊天输入框（登录后才能看到）
				var textarea = document.querySelector('#prompt-textarea, textarea[placeholder*="Message"]');
				if (textarea) return true;

				// 检查是否有"New chat"链接（登录后才有）
				var newChatLink = document.querySelector('a[href="/"]');
				if (newChatLink) return true;

				return false;
			})();
		`, &isLoggedIn),
	)

	if err == nil && isLoggedIn {
		log.Println("检测到已登录状态（JavaScript检查通过）")
		return true
	}

	log.Println("未检测到登录状态")
	return false
}

// OpenLoginPage 打开登录页面（供用户手动登录）
// 如果有保存的cookies，会自动加载以保持登录状态
func (g *GPTClient) OpenLoginPage() error {
	ctx := g.session.Ctx

	log.Println("正在打开ChatGPT网站...")
	log.Printf("Session Context: %v", ctx)

	// 导航到ChatGPT首页
	log.Printf("开始导航到: %s", gptLoginURL)
	if err := chromedp.Run(ctx,
		chromedp.Navigate(gptLoginURL),
	); err != nil {
		log.Printf("导航失败: %v", err)
		return fmt.Errorf("导航失败: %v", err)
	}
	log.Printf("导航命令已发送，等待5秒...")
	// 等待足够时间让页面完全加载（包括SPA路由）
	chromedp.Run(ctx, chromedp.Sleep(5*time.Second))
	log.Printf("等待完成")

	// 注入反检测脚本
	log.Println("注入反检测脚本...")
	if err := g.session.InjectAntiDetection(); err != nil {
		log.Printf("注入反检测脚本失败: %v", err)
		// 不返回错误，继续执行
	}

	// 再次等待确保反检测脚本生效
	chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	// 检查导航后的URL
	var currentURL string
	chromedp.Run(ctx, chromedp.Location(&currentURL))
	log.Printf("导航完成，当前URL: %s", currentURL)

	// 再次注入反检测（应对导航后的新页面）
	g.session.InjectAntiDetection()

	// 等待页面稳定
	chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	// 检查是否已经登录（cookie有效）
	if g.CheckLoggedIn() {
		log.Println("检测到已登录状态（cookie有效），无需重新登录")
		return nil
	}

	log.Println("页面已打开，请手动完成登录...")
	log.Println("========================================")
	log.Println("请在浏览器中手动完成以下步骤：")
	log.Println("1. 点击【Log in】或【登录】按钮")
	log.Println("2. 输入邮箱地址")
	log.Println("3. 点击【Continue】继续")
	log.Println("4. 输入密码并点击【Continue】")
	log.Println("5. 如需验证，完成邮箱/手机验证")
	log.Println("========================================")

	return nil
}

// Ask 向ChatGPT提问（智能获取AI回复）
// 创建新对话并发送问题，返回完整答案
func (g *GPTClient) Ask(question string) (*platform.AskResult, error) {
	ctx, cancel := context.WithTimeout(g.session.Ctx, 120*time.Second)
	defer cancel()

	// 用于存储流式内容的通道
	streamChan := make(chan string, 100)

	// 结果
	result := &platform.AskResult{
		StreamChan: streamChan,
	}

	// 0. 注入反检测脚本
	log.Println("注入反检测脚本...")
	g.session.InjectAntiDetection()
	chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	// 1. 导航到聊天页面
	log.Println("导航到ChatGPT聊天页面...")
	chromedp.Run(ctx,
		chromedp.Navigate(gptChatURL),
		chromedp.Sleep(3*time.Second),
	)
	log.Println("导航完成")

	// 注入反检测
	g.session.InjectAntiDetection()
	chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	// 检查当前URL
	var currentURL string
	chromedp.Run(ctx, chromedp.Location(&currentURL))
	log.Printf("当前URL: %s", currentURL)

	// 2. 等待输入框出现
	log.Println("等待输入框...")
	inputSelectors := []string{
		`#prompt-textarea`,
		`textarea[placeholder*="Message"]`,
		`textarea`,
	}

	var inputSelector string
	for _, selector := range inputSelectors {
		log.Printf("尝试查找输入框: %s", selector)
		err := chromedp.Run(ctx,
			chromedp.WaitVisible(selector, chromedp.ByQuery),
		)
		if err == nil {
			inputSelector = selector
			log.Printf("找到输入框: %s", selector)
			break
		}
		log.Printf("未找到输入框 %s: %v", selector, err)
	}

	if inputSelector == "" {
		return nil, fmt.Errorf("未能找到ChatGPT输入框")
	}

	// 2.5 新建对话（点击侧边栏的新对话链接）
	log.Println("尝试新建对话...")
	var newChatResult string
	chromedp.Run(ctx, chromedp.Evaluate(`
		(function() {
			// 优先点击侧边栏的"New chat"链接
			var newChatLink = document.querySelector('a[href="/"]');
			if (newChatLink) {
				newChatLink.click();
				return 'clicked: new chat link';
			}
			// 备用：查找包含"New chat"文本的按钮
			var btns = document.querySelectorAll('a, button, div[role="button"]');
			for (var i = 0; i < btns.length; i++) {
				var text = btns[i].innerText || btns[i].textContent || '';
				if (text.includes('New chat') || text.includes('新对话') || text.includes('New Chat')) {
					btns[i].click();
					return 'clicked: ' + text.trim();
				}
			}
			return 'no button';
		})()
	`, &newChatResult))
	log.Printf("新建对话按钮结果: %s", newChatResult)

	// 如果没有找到新建对话按钮，导航到首页
	if newChatResult == "no button" || newChatResult == "" {
		log.Println("未找到新建对话按钮，导航到ChatGPT首页...")
		chromedp.Run(ctx, chromedp.Navigate(gptChatURL))
		chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
	} else {
		chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
	}

	// 重新等待输入框
	chromedp.Run(ctx, chromedp.WaitVisible("#prompt-textarea, textarea", chromedp.ByQuery))

	log.Println("开始输入问题...")

	// 3. 输入问题到ChatGPT输入框
	if err := g.sendQuestion(ctx, question); err != nil {
		return nil, fmt.Errorf("输入问题失败: %v", err)
	}

	log.Println("问题输入完成")

	// 4. 发送问题（点击发送按钮或按Enter）
	log.Println("准备发送问题...")
	chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))

	// 尝试点击发送按钮
	g.clickSendButton(ctx)

	log.Println("发送操作完成，等待AI响应...")
	chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	// 4.5 检查页面异常提示
	log.Println("检查页面异常提示...")
	anomalyType, keyword, anomalyContext := g.detectPageAnomaly(ctx)
	if anomalyType == "input_error" {
		log.Printf("检测到输入超限提示: 关键词=%s, 上下文=%s", keyword, anomalyContext)
		close(streamChan)
		result.IsBot = false
		result.DetectInfo = fmt.Sprintf("输入超限: %s", anomalyContext)
		result.Answer = ""
		result.Thinking = ""
		return result, fmt.Errorf("输入内容超限: %s", anomalyContext)
	}
	if anomalyType == "captcha" {
		log.Printf("检测到人机验证: 关键词=%s, 上下文=%s", keyword, anomalyContext)
		// 尝试自动处理验证
		passed, errMsg := g.handleCaptcha(ctx)
		if !passed {
			close(streamChan)
			result.IsBot = true
			result.DetectInfo = fmt.Sprintf("人机验证未通过: %s", errMsg)
			result.Answer = ""
			result.Thinking = ""
			return result, fmt.Errorf("触发人机验证且未能自动通过: %s", errMsg)
		}
		// 验证通过后，需要重新输入问题并发送
		log.Println("人机验证已通过，重新输入问题...")
		chromedp.Run(ctx, chromedp.WaitVisible("#prompt-textarea, textarea", chromedp.ByQuery))
		chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
		if err := g.sendQuestion(ctx, question); err != nil {
			return nil, fmt.Errorf("重新发送问题失败: %v", err)
		}
		g.clickSendButton(ctx)
		chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
	}

	// 5. 等待AI回复完成
	log.Println("等待AI回复...")
	answer, thinking, err := g.waitForAIResponse(ctx)
	if err != nil {
		close(streamChan)
		return nil, fmt.Errorf("等待AI回复失败: %v", err)
	}

	log.Printf("获取到答案长度: %d, 思考过程长度: %d", len(answer), len(thinking))

	// 检查是否被检测为机器人
	botDetectKeywords := []string{
		"人机验证", "机器识别", "自动检测", "异常请求",
		"请完成验证", "captcha", "robot", "automation",
		"检测到", "非正常", "自动化工具",
		"unusual activity", "verify you are human",
		"cloudflare", "access denied",
	}

	detectedAsBot := false
	detectInfo := ""
	for _, kw := range botDetectKeywords {
		if strings.Contains(strings.ToLower(answer), strings.ToLower(kw)) {
			detectedAsBot = true
			detectInfo = fmt.Sprintf("检测到关键词: %s", kw)
			log.Printf("警告：可能被检测为机器人！关键词: %s", kw)
			break
		}
	}

	close(streamChan)
	result.Answer = answer
	result.Thinking = thinking
	result.IsBot = detectedAsBot
	result.DetectInfo = detectInfo

	return result, nil
}

// AskInConversation 在已有对话中继续提问
// 不新建对话，不导航到新页面，直接在当前输入框输入并发送
func (g *GPTClient) AskInConversation(question string) (*platform.AskResult, error) {
	ctx, cancel := context.WithTimeout(g.session.Ctx, 120*time.Second)
	defer cancel()

	// 用于存储流式内容的通道
	streamChan := make(chan string, 100)

	// 结果
	result := &platform.AskResult{
		StreamChan: streamChan,
	}

	// 注入反检测脚本
	log.Println("AskInConversation: 注入反检测脚本...")
	g.session.InjectAntiDetection()
	chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	// 等待输入框出现（不导航，直接在当前页面操作）
	log.Println("AskInConversation: 等待输入框...")
	err := chromedp.Run(ctx,
		chromedp.WaitVisible("#prompt-textarea, textarea", chromedp.ByQuery),
	)
	if err != nil {
		return nil, fmt.Errorf("当前页面未找到ChatGPT输入框: %v", err)
	}

	// 输入问题
	log.Println("AskInConversation: 开始输入问题...")
	if err := g.sendQuestion(ctx, question); err != nil {
		return nil, fmt.Errorf("输入问题失败: %v", err)
	}

	log.Printf("AskInConversation: 问题输入完成")
	chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))

	// 发送问题
	log.Println("AskInConversation: 发送问题...")
	g.clickSendButton(ctx)

	log.Println("AskInConversation: 等待AI响应...")
	chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	// 检查页面异常提示
	anomalyType, _, anomalyContext := g.detectPageAnomaly(ctx)
	if anomalyType == "input_error" {
		close(streamChan)
		result.IsBot = false
		result.DetectInfo = fmt.Sprintf("输入超限: %s", anomalyContext)
		return result, fmt.Errorf("输入内容超限: %s", anomalyContext)
	}
	if anomalyType == "captcha" {
		passed, errMsg := g.handleCaptcha(ctx)
		if !passed {
			close(streamChan)
			result.IsBot = true
			result.DetectInfo = fmt.Sprintf("人机验证未通过: %s", errMsg)
			return result, fmt.Errorf("触发人机验证且未能自动通过: %s", errMsg)
		}
		// 验证通过后重新输入并发送
		chromedp.Run(ctx, chromedp.WaitVisible("#prompt-textarea, textarea", chromedp.ByQuery))
		chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
		if err := g.sendQuestion(ctx, question); err != nil {
			return nil, fmt.Errorf("重新发送问题失败: %v", err)
		}
		g.clickSendButton(ctx)
		chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
	}

	// 等待AI回复完成
	answer, thinking, err := g.waitForAIResponse(ctx)
	if err != nil {
		close(streamChan)
		return nil, fmt.Errorf("等待AI回复失败: %v", err)
	}

	// 检查是否被检测为机器人
	botDetectKeywords := []string{
		"人机验证", "机器识别", "自动检测", "异常请求",
		"请完成验证", "captcha", "robot", "automation",
		"检测到", "非正常", "自动化工具",
		"unusual activity", "verify you are human",
		"cloudflare", "access denied",
	}
	detectedAsBot := false
	detectInfo := ""
	for _, kw := range botDetectKeywords {
		if strings.Contains(strings.ToLower(answer), strings.ToLower(kw)) {
			detectedAsBot = true
			detectInfo = fmt.Sprintf("检测到关键词: %s", kw)
			break
		}
	}

	close(streamChan)
	result.Answer = answer
	result.Thinking = thinking
	result.IsBot = detectedAsBot
	result.DetectInfo = detectInfo

	return result, nil
}

// StartNewConversation 新建对话并发送初始消息
// 导航到聊天页面，点击"New chat"按钮，输入并发送初始消息
func (g *GPTClient) StartNewConversation(initialMessage string) (*platform.AskResult, error) {
	ctx, cancel := context.WithTimeout(g.session.Ctx, 120*time.Second)
	defer cancel()

	// 用于存储流式内容的通道
	streamChan := make(chan string, 100)

	// 结果
	result := &platform.AskResult{
		StreamChan: streamChan,
	}

	// 注入反检测脚本
	log.Println("StartNewConversation: 注入反检测脚本...")
	g.session.InjectAntiDetection()
	chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	// 导航到聊天页面
	log.Println("StartNewConversation: 导航到ChatGPT聊天页面...")
	chromedp.Run(ctx,
		chromedp.Navigate(gptChatURL),
		chromedp.Sleep(3*time.Second),
	)

	// 注入反检测
	g.session.InjectAntiDetection()
	chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	// 点击"New chat"按钮
	log.Println("StartNewConversation: 点击新建对话按钮...")
	var newChatResult string
	chromedp.Run(ctx, chromedp.Evaluate(`
		(function() {
			// 优先点击侧边栏的"New chat"链接
			var newChatLink = document.querySelector('a[href="/"]');
			if (newChatLink) {
				newChatLink.click();
				return 'clicked: new chat link';
			}
			// 备用：查找包含"New chat"文本的按钮
			var btns = document.querySelectorAll('a, button, div[role="button"]');
			for (var i = 0; i < btns.length; i++) {
				var text = btns[i].innerText || btns[i].textContent || '';
				if (text.includes('New chat') || text.includes('新对话') || text.includes('New Chat')) {
					btns[i].click();
					return 'clicked: ' + text.trim();
				}
			}
			return 'no button';
		})()
	`, &newChatResult))
	log.Printf("新建对话按钮结果: %s", newChatResult)

	// 如果没有找到新建对话按钮，导航到首页
	if newChatResult == "no button" || newChatResult == "" {
		log.Println("未找到新建对话按钮，导航到ChatGPT首页...")
		chromedp.Run(ctx, chromedp.Navigate(gptChatURL))
		chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
	} else {
		chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
	}

	// 等待新对话页面加载完成
	log.Println("StartNewConversation: 等待新对话页面加载...")
	err := chromedp.Run(ctx,
		chromedp.WaitVisible("#prompt-textarea, textarea", chromedp.ByQuery),
	)
	if err != nil {
		return nil, fmt.Errorf("新对话页面加载失败，未找到输入框: %v", err)
	}
	chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	// 输入初始消息
	log.Println("StartNewConversation: 输入初始消息...")
	if err := g.sendQuestion(ctx, initialMessage); err != nil {
		return nil, fmt.Errorf("输入初始消息失败: %v", err)
	}

	log.Printf("StartNewConversation: 初始消息输入完成")
	chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))

	// 发送消息
	log.Println("StartNewConversation: 发送初始消息...")
	g.clickSendButton(ctx)

	log.Println("StartNewConversation: 等待AI响应...")
	chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	// 检查页面异常提示
	anomalyType, _, anomalyContext := g.detectPageAnomaly(ctx)
	if anomalyType == "input_error" {
		close(streamChan)
		result.IsBot = false
		result.DetectInfo = fmt.Sprintf("输入超限: %s", anomalyContext)
		return result, fmt.Errorf("输入内容超限: %s", anomalyContext)
	}
	if anomalyType == "captcha" {
		passed, errMsg := g.handleCaptcha(ctx)
		if !passed {
			close(streamChan)
			result.IsBot = true
			result.DetectInfo = fmt.Sprintf("人机验证未通过: %s", errMsg)
			return result, fmt.Errorf("触发人机验证且未能自动通过: %s", errMsg)
		}
		// 验证通过后重新输入并发送
		chromedp.Run(ctx, chromedp.WaitVisible("#prompt-textarea, textarea", chromedp.ByQuery))
		chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
		if err := g.sendQuestion(ctx, initialMessage); err != nil {
			return nil, fmt.Errorf("重新发送初始消息失败: %v", err)
		}
		g.clickSendButton(ctx)
		chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
	}

	// 等待AI回复完成
	answer, thinking, err := g.waitForAIResponse(ctx)
	if err != nil {
		close(streamChan)
		return nil, fmt.Errorf("等待AI回复失败: %v", err)
	}

	// 检查是否被检测为机器人
	botDetectKeywords := []string{
		"人机验证", "机器识别", "自动检测", "异常请求",
		"请完成验证", "captcha", "robot", "automation",
		"检测到", "非正常", "自动化工具",
		"unusual activity", "verify you are human",
		"cloudflare", "access denied",
	}
	detectedAsBot := false
	detectInfo := ""
	for _, kw := range botDetectKeywords {
		if strings.Contains(strings.ToLower(answer), strings.ToLower(kw)) {
			detectedAsBot = true
			detectInfo = fmt.Sprintf("检测到关键词: %s", kw)
			break
		}
	}

	close(streamChan)
	result.Answer = answer
	result.Thinking = thinking
	result.IsBot = detectedAsBot
	result.DetectInfo = detectInfo

	return result, nil
}

// ==================== 辅助方法 ====================

// sendQuestion 将问题输入到ChatGPT输入框
// 使用insertText方式设置输入框内容，兼容React框架的事件处理
func (g *GPTClient) sendQuestion(ctx context.Context, question string) error {
	// 使用JSON编码确保中文等特殊字符安全
	questionJSON, err := json.Marshal(question)
	if err != nil {
		log.Printf("JSON编码问题失败: %v", err)
		questionJSON = []byte(fmt.Sprintf(`"%s"`, question))
	}

	// 使用document.execCommand('insertText')设置textarea值
	// 这是最可靠的方式：触发React能识别的所有事件，且正确处理中文编码
	log.Println("使用insertText设置输入框内容...")
	var jsInputResult int
	err = chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
		(function() {
			var text = %s;
			// ChatGPT优先使用#prompt-textarea
			var textarea = document.querySelector('#prompt-textarea') || document.querySelector('textarea');
			if (textarea) {
				textarea.focus();
				// 清空现有内容
				var nativeInputValueSetter = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value').set;
				nativeInputValueSetter.call(textarea, '');
				textarea.dispatchEvent(new Event('input', { bubbles: true }));
				// 使用insertText输入内容
				document.execCommand('insertText', false, text);
				return textarea.value.length;
			}
			// 备用：contenteditable元素
			var editable = document.querySelector('[contenteditable="true"]');
			if (editable) {
				editable.focus();
				var range = document.createRange();
				range.selectNodeContents(editable);
				var sel = window.getSelection();
				sel.removeAllRanges();
				sel.addRange(range);
				document.execCommand('insertText', false, text);
				return editable.textContent.length;
			}
			return 0;
		})()
	`, string(questionJSON)), &jsInputResult))

	if err != nil {
		log.Printf("insertText设置输入框失败: %v，回退到SendKeys...", err)
		err = chromedp.Run(ctx,
			chromedp.SendKeys("#prompt-textarea, textarea", question, chromedp.ByQuery),
		)
		if err != nil {
			return fmt.Errorf("输入内容失败: %v", err)
		}
	} else {
		log.Printf("insertText设置输入框成功，内容长度: %d", jsInputResult)
		chromedp.Run(ctx, chromedp.Sleep(300*time.Millisecond))
		// 验证输入框内容
		var verifyValue string
		chromedp.Run(ctx, chromedp.Evaluate(`
			(function() {
				var textarea = document.querySelector('#prompt-textarea') || document.querySelector('textarea');
				if (textarea) return textarea.value;
				var editable = document.querySelector('[contenteditable="true"]');
				if (editable) return editable.textContent;
				return '';
			})()
		`, &verifyValue))
		if len(verifyValue) > 0 {
			log.Printf("输入框验证成功，当前值长度: %d", len(verifyValue))
		} else {
			log.Printf("输入框验证失败，回退到SendKeys...")
			err = chromedp.Run(ctx,
				chromedp.SendKeys("#prompt-textarea, textarea", question, chromedp.ByQuery),
			)
			if err != nil {
				return fmt.Errorf("输入内容失败: %v", err)
			}
		}
	}

	return nil
}

// clickSendButton 点击ChatGPT发送按钮
// 尝试多种选择器，最终回退到Enter键发送
func (g *GPTClient) clickSendButton(ctx context.Context) {
	// ChatGPT发送按钮选择器（按优先级排列）
	sendSelectors := []string{
		"button[data-testid='send-button']",
		"button[aria-label='Send prompt']",
		"button[aria-label='发送提示']",
		"button[data-testid='fruitjuice-send-button']",
	}

	sendClicked := false
	for _, selector := range sendSelectors {
		var found bool
		err := chromedp.Run(ctx, chromedp.Evaluate(`
			(function() {
				var el = document.querySelector('`+selector+`');
				return el && !el.disabled && el.offsetParent !== null;
			})()
		`, &found))

		if err == nil && found {
			log.Printf("找到发送按钮: %s", selector)
			err = chromedp.Run(ctx, chromedp.Click(selector, chromedp.ByQuery))
			if err == nil {
				sendClicked = true
				log.Printf("成功点击发送按钮: %s", selector)
				break
			}
			log.Printf("点击发送按钮失败: %v", err)
		}
	}

	// 备用：通过JavaScript查找并点击发送按钮
	if !sendClicked {
		log.Println("未找到发送按钮，尝试使用JS点击...")
		var jsClicked bool
		chromedp.Run(ctx, chromedp.Evaluate(`
			(function() {
				var selectors = [
					"button[data-testid='send-button']",
					"button[aria-label='Send prompt']",
					"button[aria-label='发送提示']",
					"button[data-testid='fruitjuice-send-button']",
					'form button[type="submit"]',
					'svg[class*="send"]'
				];
				for (var i = 0; i < selectors.length; i++) {
					var btn = document.querySelector(selectors[i]);
					if (btn && !btn.disabled && btn.offsetParent !== null) {
						btn.click();
						return true;
					}
				}
				return false;
			})()
		`, &jsClicked))

		if jsClicked {
			sendClicked = true
			log.Println("通过JavaScript成功点击发送按钮")
		}
	}

	// 最终回退：使用Enter键发送
	if !sendClicked {
		log.Println("使用 Enter 键发送...")
		chromedp.Run(ctx, chromedp.Focus("#prompt-textarea, textarea", chromedp.ByQuery))
		chromedp.Run(ctx, chromedp.Sleep(200*time.Millisecond))

		err := chromedp.Run(ctx,
			chromedp.SendKeys("#prompt-textarea, textarea", "\r", chromedp.ByQuery),
		)
		if err != nil {
			log.Printf("Enter键发送失败: %v，尝试JavaScript触发...", err)
			chromedp.Run(ctx, chromedp.Evaluate(`
				(function() {
					var textarea = document.querySelector('#prompt-textarea') || document.querySelector('textarea');
					if (textarea) {
						var event = new KeyboardEvent('keydown', {key: 'Enter', code: 'Enter', keyCode: 13, bubbles: true});
						textarea.dispatchEvent(event);
					}
				})()
			`, nil))
		} else {
			log.Println("通过Enter键发送成功")
		}
	}
}

// waitForAIResponse 等待ChatGPT的AI回复完成
// 区分思考过程和正式回复，当非思考内容稳定后返回
func (g *GPTClient) waitForAIResponse(ctx context.Context) (string, string, error) {
	maxWaitTime := 120 * time.Second
	checkInterval := 2 * time.Second
	startTime := time.Now()

	var lastContentLength int
	stableCount := 0
	var resultStr string

	for time.Since(startTime) < maxWaitTime {
		chromedp.Run(ctx, chromedp.Sleep(checkInterval))

		// 每轮循环检查页面异常状态
		loopAnomalyType, _, _ := g.detectPageAnomaly(ctx)
		if loopAnomalyType == "input_error" {
			log.Println("等待过程中检测到输入超限提示")
			return "", "", fmt.Errorf("输入内容超限")
		}
		if loopAnomalyType == "captcha" {
			log.Println("等待过程中检测到人机验证")
			passed, _ := g.handleCaptcha(ctx)
			if passed {
				// 验证通过后重置等待状态
				lastContentLength = 0
				stableCount = 0
				startTime = time.Now()
				continue
			}
			return "", "", fmt.Errorf("人机验证未通过")
		}

		var currentContentLength int
		var hasNonThinkingReply bool
		var thinkingLength int

		// 使用ChatGPT特定的选择器检测回复状态
		chromedp.Run(ctx, chromedp.Evaluate(`
			(function() {
				// ChatGPT回复区域选择器
				var replySelectors = [
					'[data-message-author-role="assistant"]',
					'.markdown',
					'[class*="message-content"]',
					'[class*="assistant"]',
					'.turn-content'
				];

				// 思考过程特征模式
				var thinkingPatterns = [
					/Thinking/, /Let me think/, /Let's think/,
					/思考/, /分析中/, /思考过程/,
					/Step \d+/, /First,/, /Next,/, /Finally,/,
					/I need to/, /I should/, /Let me/
				];

				function isThinkingOnly(text) {
					if (!text || text.length < 20) return false;
					var nonThinkingText = text;
					for (var i = 0; i < thinkingPatterns.length; i++) {
						nonThinkingText = nonThinkingText.replace(thinkingPatterns[i], '');
					}
					return nonThinkingText.trim().length < text.length * 0.3;
				}

				// 查找最后一条AI回复
				for (var i = 0; i < replySelectors.length; i++) {
					var elems = document.querySelectorAll(replySelectors[i]);
					if (elems.length > 0) {
						var lastElem = elems[elems.length - 1];
						var text = lastElem.innerText || '';
						if (text.length > 20) {
							if (isThinkingOnly(text)) {
								return JSON.stringify({length: text.length, hasNonThinkingReply: false, thinkingLength: text.length});
							}
							return JSON.stringify({length: text.length, hasNonThinkingReply: true, thinkingLength: 0});
						}
					}
				}

				// 备用：检查页面整体内容
				var chatArea = document.querySelector('main, [class*="conversation"], [class*="chat"]');
				if (chatArea) {
					var chatText = chatArea.innerText;
					var isThinking = isThinkingOnly(chatText);
					return JSON.stringify({length: chatText.length, hasNonThinkingReply: !isThinking && chatText.length > 100, thinkingLength: isThinking ? chatText.length : 0});
				}
				return JSON.stringify({length: 0, hasNonThinkingReply: false, thinkingLength: 0});
			})()
		`, &resultStr))

		var evalResult struct {
			Length              int  `json:"length"`
			HasNonThinkingReply bool `json:"hasNonThinkingReply"`
			ThinkingLength      int  `json:"thinkingLength"`
		}
		if err := json.Unmarshal([]byte(resultStr), &evalResult); err == nil {
			currentContentLength = evalResult.Length
			hasNonThinkingReply = evalResult.HasNonThinkingReply
			thinkingLength = evalResult.ThinkingLength
		}

		if thinkingLength > 0 {
			log.Printf("检测到思考过程...（思考长度: %d，总长度: %d）", thinkingLength, currentContentLength)
		}

		// 如果只有思考过程，继续等待正式回复
		if !hasNonThinkingReply {
			if time.Since(startTime) > 90*time.Second {
				log.Printf("等待正式回复超时（已等待: %v），使用当前内容", time.Since(startTime))
				break
			}
			log.Printf("等待正式回复...（当前内容长度: %d）", currentContentLength)
			lastContentLength = currentContentLength
			continue
		}

		// 非思考内容已出现，检测稳定性
		if lastContentLength > 0 {
			change := absInt(currentContentLength - lastContentLength)
			if float64(change)/float64(lastContentLength) < 0.05 {
				stableCount++
				if stableCount >= 3 {
					log.Printf("内容已稳定，回复完成（稳定次数: %d，长度: %d）", stableCount, currentContentLength)
					break
				}
			} else {
				stableCount = 0
				log.Printf("内容变化中...（长度: %d，变化: %d）", currentContentLength, change)
			}
		}
		lastContentLength = currentContentLength
	}

	// 提取回复内容
	return g.extractAIReply(ctx)
}

// extractAIReply 从页面提取ChatGPT的AI回复，分离思考过程和正式回复
func (g *GPTClient) extractAIReply(ctx context.Context) (string, string, error) {
	var jsResult string
	chromedp.Run(ctx, chromedp.Evaluate(`
		(function() {
			// ChatGPT页面中需要排除的无关文本关键词
			var excludeKeywords = ['ChatGPT', 'Log in', 'Sign up', 'New chat',
			                         'Upgrade', 'Share', 'Settings', 'Help',
			                         'Terms', 'Privacy', 'Clear chat',
			                         'How can I help', 'Message ChatGPT',
			                         '4o', '4o mini', 'GPT-4', 'Search',
			                         'Sora', 'Canvas'];

			function shouldExclude(text) {
				if (!text) return true;
				var lowerText = text.toLowerCase();
				for (var k = 0; k < excludeKeywords.length; k++) {
					if (lowerText.includes(excludeKeywords[k].toLowerCase())) {
						return true;
					}
				}
				return false;
			}

			function isThinkingContent(text) {
				var patterns = [
					/Thinking/, /Let me think/, /Let's think/,
					/思考/, /分析中/, /思考过程/,
					/Step \d+/, /First,/, /Next,/, /Finally,/,
					/I need to/, /I should/, /Let me/,
					/Okay, /, /Alright, /, /I'll /, /I will/
				];
				for (var i = 0; i < patterns.length; i++) {
					if (patterns[i].test(text)) return true;
				}
				return false;
			}

			// 判断文本是否主要是思考过程（超过50%的内容是思考）
			function isMostlyThinking(text) {
				if (!text || text.length < 20) return false;
				var lines = text.split('\n');
				var thinkingLines = 0;
				var totalLines = 0;
				for (var i = 0; i < lines.length; i++) {
					var line = lines[i].trim();
					if (line.length < 5) continue;
					totalLines++;
					if (isThinkingContent(line)) thinkingLines++;
				}
				return totalLines > 0 && thinkingLines / totalLines > 0.5;
			}

			var thinkingText = '';
			var answerText = '';

			// 策略1：从ChatGPT回复元素中提取
			var replySelectors = [
				'[data-message-author-role="assistant"]',
				'.markdown',
				'[class*="message-content"]',
				'[class*="assistant"]',
				'.turn-content',
				'[class*="thinking"]',
				'[class*="think"]',
				'[class*="reasoning"]'
			];

			// 收集所有回复元素
			var allTexts = [];
			for (var i = 0; i < replySelectors.length; i++) {
				var elems = document.querySelectorAll(replySelectors[i]);
				for (var j = 0; j < elems.length; j++) {
					var text = elems[j].innerText.trim();
					if (text.length < 20 || shouldExclude(text)) continue;
					allTexts.push({text: text, isThinking: isThinkingContent(text), len: text.length});
				}
			}

			// 去重（按文本内容前100字符）
			var seen = {};
			var uniqueTexts = [];
			for (var i = 0; i < allTexts.length; i++) {
				var key = allTexts[i].text.substring(0, 100);
				if (!seen[key]) {
					seen[key] = true;
					uniqueTexts.push(allTexts[i]);
				}
			}

			// 按长度排序，优先选择较长的文本
			uniqueTexts.sort(function(a, b) { return b.len - a.len; });

			// 先找最长的非思考文本作为答案
			for (var i = 0; i < uniqueTexts.length; i++) {
				if (!uniqueTexts[i].isThinking && !answerText) {
					answerText = uniqueTexts[i].text;
				}
			}

			// 再找最长的思考文本
			for (var i = 0; i < uniqueTexts.length; i++) {
				if (uniqueTexts[i].isThinking && !thinkingText) {
					thinkingText = uniqueTexts[i].text;
				}
			}

			// 策略2：从段落中提取
			if (!thinkingText || !answerText) {
				var paragraphs = document.querySelectorAll('p, div, article, section');
				var paraTexts = [];
				for (var j = 0; j < paragraphs.length; j++) {
					var p = paragraphs[j];
					if (p.offsetParent === null) continue;
					var text = p.innerText.trim();
					if (text.length < 50 || text.length > 10000 || shouldExclude(text)) continue;
					paraTexts.push({text: text, isThinking: isThinkingContent(text), len: text.length});
				}

				paraTexts.sort(function(a, b) { return b.len - a.len; });

				for (var i = 0; i < paraTexts.length; i++) {
					if (paraTexts[i].isThinking && !thinkingText) {
						thinkingText = paraTexts[i].text;
					} else if (!paraTexts[i].isThinking && !answerText) {
						answerText = paraTexts[i].text;
					}
					if (thinkingText && answerText) break;
				}
			}

			// 策略3：如果答案主要是思考过程，将整个答案移到thinking字段
			if (answerText && !thinkingText && isMostlyThinking(answerText)) {
				thinkingText = answerText;
				answerText = '';
			}

			return JSON.stringify({thinking: thinkingText, answer: answerText});
		})()
	`, &jsResult))

	var parsedResult struct {
		Thinking string `json:"thinking"`
		Answer   string `json:"answer"`
	}
	if err := json.Unmarshal([]byte(jsResult), &parsedResult); err != nil {
		return "", "", fmt.Errorf("解析AI回复失败: %v", err)
	}

	return parsedResult.Answer, parsedResult.Thinking, nil
}

// detectPageAnomaly 检测ChatGPT页面异常状态
// 返回: anomalyType("input_error"/"captcha"/"none"), keyword, context
func (g *GPTClient) detectPageAnomaly(ctx context.Context) (string, string, string) {
	var resultStr string
	chromedp.Run(ctx, chromedp.Evaluate(`
		(function() {
			// 输入超限相关关键词
			var errorKeywords = [
				'maximum', 'too long', 'exceeds', 'limit',
				'字数超', '超出限制', '内容过长', '输入超限',
				'字数限制', '超过最大', '超出字数'
			];
			// 人机验证相关关键词
			var captchaKeywords = [
				'verify you are human', 'unusual activity', 'captcha',
				'robot', 'automation', 'cloudflare',
				'access denied', 'blocked', 'security check',
				'人机验证', '机器识别', '自动检测', '异常请求',
				'请完成验证', '安全验证', '滑块验证',
				'slider', 'puzzle', 'challenge'
			];
			var allText = document.body.innerText || '';
			for (var i = 0; i < errorKeywords.length; i++) {
				var idx = allText.toLowerCase().indexOf(errorKeywords[i].toLowerCase());
				if (idx !== -1) {
					var start = Math.max(0, idx - 20);
					var end = Math.min(allText.length, idx + 50);
					return JSON.stringify({type: 'input_error', keyword: errorKeywords[i], context: allText.substring(start, end)});
				}
			}
			for (var i = 0; i < captchaKeywords.length; i++) {
				var idx = allText.toLowerCase().indexOf(captchaKeywords[i].toLowerCase());
				if (idx !== -1) {
					var start = Math.max(0, idx - 20);
					var end = Math.min(allText.length, idx + 50);
					return JSON.stringify({type: 'captcha', keyword: captchaKeywords[i], context: allText.substring(start, end)});
				}
			}
			// 检查Cloudflare验证页面元素
			var captchaSelectors = [
				'iframe[src*="captcha"]', '[class*="captcha"]', '[class*="verify"]',
				'[class*="challenge"]', '[id*="captcha"]', '[id*="verify"]',
				'#challenge-running', '#challenge-stage',
				'.cf-turnstile', '[class*="turnstile"]'
			];
			for (var i = 0; i < captchaSelectors.length; i++) {
				if (document.querySelector(captchaSelectors[i])) {
					return JSON.stringify({type: 'captcha', keyword: 'captcha_element', context: captchaSelectors[i]});
				}
			}
			return JSON.stringify({type: 'none', keyword: '', context: ''});
		})()
	`, &resultStr))

	var result struct {
		Type    string `json:"type"`
		Keyword string `json:"keyword"`
		Context string `json:"context"`
	}
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		return "none", "", ""
	}
	return result.Type, result.Keyword, result.Context
}

// handleCaptcha 尝试自动处理人机验证页面
// 策略：1.尝试点击验证按钮/复选框 2.等待用户手动完成 3.验证通过后智能判断是否需要刷新页面
// 返回: 是否成功通过验证, 错误信息
func (g *GPTClient) handleCaptcha(ctx context.Context) (bool, string) {
	log.Println("检测到人机验证页面，尝试自动处理...")

	// 策略1：尝试点击验证按钮/复选框
	clickSelectors := []string{
		`input[type="checkbox"]`,
		`[class*="check"]`,
		`[class*="verify"] button`,
		`[class*="verify"] a`,
		`[class*="challenge"] button`,
		`[class*="challenge"] a`,
		`button[class*="submit"]`,
		`a[class*="submit"]`,
		`[class*="captcha"] button`,
		`[class*="captcha"] a`,
		`[id*="verify"] button`,
		`[id*="verify"] a`,
		`.cf-turnstile`,
		`[class*="turnstile"]`,
		`iframe[src*="captcha"]`,
	}

	clicked := false
	for _, selector := range clickSelectors {
		var found bool
		err := chromedp.Run(ctx, chromedp.Evaluate(`
			(function() {
				var el = document.querySelector('`+selector+`');
				return el && el.offsetParent !== null;
			})()
		`, &found))
		if err == nil && found {
			log.Printf("找到验证元素: %s，尝试点击...", selector)
			err = chromedp.Run(ctx, chromedp.Click(selector, chromedp.ByQuery))
			if err == nil {
				clicked = true
				log.Printf("成功点击验证元素: %s", selector)
				break
			}
			log.Printf("点击验证元素失败: %v", err)
		}
	}

	// 策略2：尝试通过JavaScript点击所有可能的验证按钮
	if !clicked {
		log.Println("尝试JavaScript查找并点击验证按钮...")
		var jsClicked bool
		chromedp.Run(ctx, chromedp.Evaluate(`
			(function() {
				var selectors = [
					'input[type="checkbox"]', 'button', 'a[href]',
					'[role="button"]', '[role="checkbox"]',
					'[class*="check"]', '[class*="verify"]',
					'[class*="submit"]', '[class*="confirm"]',
					'[class*="pass"]', '[class*="continue"]'
				];
				var pageText = document.body.innerText || '';
				var clickTexts = ['验证', '确认', '通过', '继续', '提交', 'verify', 'confirm', 'submit', 'continue', 'pass'];
				for (var i = 0; i < selectors.length; i++) {
					var elems = document.querySelectorAll(selectors[i]);
					for (var j = 0; j < elems.length; j++) {
						var el = elems[j];
						if (el.offsetParent === null) continue;
						var elText = (el.innerText || el.textContent || el.value || '').toLowerCase();
						for (var k = 0; k < clickTexts.length; k++) {
							if (elText.indexOf(clickTexts[k]) !== -1) {
								el.click();
								return true;
							}
						}
					}
				}
				var checkbox = document.querySelector('input[type="checkbox"]');
				if (checkbox) { checkbox.click(); return true; }
				return false;
			})()
		`, &jsClicked))
		if jsClicked {
			clicked = true
			log.Println("通过JavaScript成功点击验证按钮")
		}
	}

	if clicked {
		log.Println("已点击验证按钮，等待验证结果...")
		chromedp.Run(ctx, chromedp.Sleep(3*time.Second))

		// 检查验证是否通过
		anomalyType, _, _ := g.detectPageAnomaly(ctx)
		if anomalyType == "none" {
			log.Println("验证已通过！")
			return true, ""
		}
		log.Println("验证尚未通过，可能需要进一步操作...")
	}

	// 策略3：等待用户手动完成验证（最多等待5分钟）
	log.Println("========================================")
	log.Println("⚠️ 需要手动完成人机验证！")
	log.Println("请在浏览器中完成验证操作...")
	log.Println("等待验证通过（最多5分钟）...")
	log.Println("========================================")

	maxCaptchaWait := 5 * time.Minute
	captchaCheckInterval := 3 * time.Second
	captchaStart := time.Now()

	for time.Since(captchaStart) < maxCaptchaWait {
		chromedp.Run(ctx, chromedp.Sleep(captchaCheckInterval))

		anomalyType, _, _ := g.detectPageAnomaly(ctx)
		if anomalyType == "none" {
			log.Println("✅ 人机验证已通过！")
			// 验证通过后不刷新页面，直接在当前页面继续操作
			chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
			var currentURL string
			chromedp.Run(ctx, chromedp.Location(&currentURL))
			if !strings.Contains(currentURL, "chatgpt.com") {
				log.Println("验证后不在聊天页面，导航回来...")
				chromedp.Run(ctx,
					chromedp.Navigate(gptChatURL),
					chromedp.Sleep(3*time.Second),
				)
				g.session.InjectAntiDetection()
				chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
			}
			return true, ""
		}
		elapsed := time.Since(captchaStart)
		if int(elapsed.Seconds())%30 == 0 && int(elapsed.Seconds()) > 0 {
			log.Printf("⏳ 仍在等待人机验证通过...（已等待: %.0f秒）", elapsed.Seconds())
		}
	}

	log.Println("❌ 人机验证等待超时（5分钟）")
	return false, "人机验证等待超时（5分钟），请在验证通过后重试"
}

// absInt 返回整数的绝对值
func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
