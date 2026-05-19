// Package doubao 豆包平台适配器
// 实现PlatformClient接口，将豆包AI平台的功能适配到统一平台架构
package doubao

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"aithink/internal/browser"
	"aithink/internal/models"
	"aithink/internal/platform"
)

// 豆包网站URL常量
const (
	doubaoLoginURL = "https://www.doubao.com/"
	doubaoChatURL  = "https://www.doubao.com/chat/"
)

// DoubaoClient 豆包平台客户端
// 嵌入BrowserSession以复用浏览器会话管理能力
type DoubaoClient struct {
	session *browser.BrowserSession
}

// NewDoubaoClient 创建豆包客户端
func NewDoubaoClient(session *browser.BrowserSession) *DoubaoClient {
	return &DoubaoClient{session: session}
}

// init 注册豆包平台到全局注册器
func init() {
	platform.GetRegistry().Register(
		models.PlatformDoubao,
		func(session *browser.BrowserSession) platform.PlatformClient {
			return NewDoubaoClient(session)
		},
		&platform.PlatformConfig{
			Platform: models.PlatformDoubao,
			LoginURL: "https://www.doubao.com/",
			ChatURL:  "https://www.doubao.com/chat/",
			Selectors: map[string]string{
				"input_box":       "textarea, div[contenteditable='true']",
				"send_button":     "button[class*='send'], [class*='submit']",
				"response_area":   "[class*='assistant'], [class*='message-content']",
				"new_chat_button": "[class*='new-chat'], [class*='new-conversation']",
			},
		},
	)
}

// ==================== PlatformClient 接口实现 ====================

// GetPlatformName 获取平台名称
func (d *DoubaoClient) GetPlatformName() string {
	return "doubao"
}

// GetLoginURL 获取平台登录页面URL
func (d *DoubaoClient) GetLoginURL() string {
	return doubaoLoginURL
}

// GetChatURL 获取平台聊天页面URL
func (d *DoubaoClient) GetChatURL() string {
	return doubaoChatURL
}

// NavigateToHome 导航到豆包首页（用于加载cookies后使cookies生效）
func (d *DoubaoClient) NavigateToHome() error {
	ctx := d.session.Ctx

	log.Printf("正在导航到豆包首页: %s", doubaoChatURL)

	// 导航到主页
	if err := chromedp.Run(ctx,
		chromedp.Navigate(doubaoChatURL),
	); err != nil {
		log.Printf("导航失败: %v", err)
		return fmt.Errorf("导航失败: %v", err)
	}

	// 等待页面加载
	chromedp.Run(ctx, chromedp.Sleep(3*time.Second))

	// 注入反检测脚本
	log.Println("注入反检测脚本...")
	if err := d.session.InjectAntiDetection(); err != nil {
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
// 豆包登录后通常有用户头像或特定元素，URL包含/chat/也认为已登录
func (d *DoubaoClient) CheckLoggedIn() bool {
	ctx := d.session.Ctx
	log.Println("CheckLoggedIn: 开始检查登录状态")

	// 获取页面文本
	var pageText string
	log.Println("CheckLoggedIn: 准备获取页面文本")
	err := chromedp.Run(ctx,
		chromedp.Text("body", &pageText, chromedp.ByQuery),
	)
	log.Printf("CheckLoggedIn: 获取页面文本完成, err=%v, text长度=%d", err, len(pageText))
	if err != nil || len(pageText) == 0 {
		log.Printf("获取页面文本失败或为空: %v", err)
		return false
	}

	// 检查是否是访客状态（未登录）
	// 豆包未登录时通常显示"登录"和"注册"按钮
	if strings.Contains(pageText, "登录") && strings.Contains(pageText, "注册") {
		// 进一步确认：如果同时存在聊天输入框，可能是已登录状态
		var hasInput bool
		chromedp.Run(ctx, chromedp.Evaluate(`
			(function() {
				return !!document.querySelector('textarea, div[contenteditable="true"]');
			})()
		`, &hasInput))
		if !hasInput {
			log.Println("检测到未登录状态（存在登录/注册按钮且无输入框）")
			return false
		}
	}

	// 使用JavaScript更精确地检查登录状态
	var isLoggedIn bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				// 检查是否有用户头像（豆包登录后通常有头像元素）
				var avatarSelectors = [
					'.user-avatar', '.avatar',
					'[class*="user-info"]', '[class*="login"]',
					'[class*="avatar"]', '[class*="profile"]',
					'[class*="user"]', '[data-testid*="user"]'
				];
				for (var i = 0; i < avatarSelectors.length; i++) {
					var el = document.querySelector(avatarSelectors[i]);
					if (el && el.offsetParent !== null) return true;
				}

				// 检查是否有聊天输入框（已登录特征）
				var input = document.querySelector('textarea, [contenteditable="true"]');
				if (input) return true;

				// 检查页面是否包含已登录才有的元素
				var bodyText = document.body.innerText || '';
				if (bodyText.includes('退出登录') || bodyText.includes('个人中心') ||
					bodyText.includes('我的') || bodyText.includes('设置')) return true;

				return false;
			})();
		`, &isLoggedIn),
	)

	if err == nil && isLoggedIn {
		log.Println("检测到已登录状态（JavaScript检查通过）")
		return true
	}

	// 检查URL是否包含/chat/（说明已经在聊天页面，通常已登录）
	var currentURL string
	chromedp.Run(ctx, chromedp.Location(&currentURL))
	if strings.Contains(currentURL, "/chat/") {
		log.Printf("检测到已登录状态（URL包含/chat/: %s）", currentURL)
		return true
	}

	log.Println("未检测到登录状态")
	return false
}

// OpenLoginPage 打开登录页面（供用户手动登录）
// 如果有保存的cookies，会自动加载以保持登录状态
func (d *DoubaoClient) OpenLoginPage() error {
	ctx := d.session.Ctx

	log.Println("正在打开豆包网站...")
	log.Printf("Session Context: %v", ctx)

	// 直接使用 session 的 ctx，不使用额外的超时包装
	log.Printf("开始导航到: %s", doubaoLoginURL)
	if err := chromedp.Run(ctx,
		chromedp.Navigate(doubaoLoginURL),
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
	if err := d.session.InjectAntiDetection(); err != nil {
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
	d.session.InjectAntiDetection()

	// 等待页面稳定
	chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	// 检查是否已经登录（cookie有效）
	if d.CheckLoggedIn() {
		log.Println("检测到已登录状态（cookie有效），无需重新登录")
		return nil
	}

	log.Println("页面已打开，请手动完成登录...")
	log.Println("========================================")
	log.Println("请在浏览器中手动完成以下步骤：")
	log.Println("1. 点击【登录】按钮")
	log.Println("2. 选择登录方式（手机号/扫码等）")
	log.Println("3. 完成登录验证")
	log.Println("========================================")

	return nil
}

// Ask 向豆包提问（智能获取AI回复）
// 创建新对话并发送问题，返回完整答案
func (d *DoubaoClient) Ask(question string) (*platform.AskResult, error) {
	ctx, cancel := context.WithTimeout(d.session.Ctx, 90*time.Second)
	defer cancel()

	// 用于存储流式内容的通道
	streamChan := make(chan string, 100)

	// 结果
	result := &platform.AskResult{
		StreamChan: streamChan,
	}

	// 0. 注入反检测脚本
	log.Println("注入反检测脚本...")
	d.session.InjectAntiDetection()
	chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	// 1. 导航到聊天页面
	log.Println("导航到聊天页面...")
	chromedp.Run(ctx,
		chromedp.Navigate(doubaoChatURL),
		chromedp.Sleep(3*time.Second),
	)
	log.Println("导航完成")

	// 注入反检测
	d.session.InjectAntiDetection()
	chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	// 检查当前URL
	var currentURL string
	chromedp.Run(ctx, chromedp.Location(&currentURL))
	log.Printf("当前URL: %s", currentURL)

	// 2. 等待输入框出现
	log.Println("等待输入框...")
	inputSelectors := []string{
		`textarea`,
		`div[contenteditable="true"]`,
		`.chat-input textarea`,
		`[class*="chat-input"] textarea`,
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
		return nil, fmt.Errorf("未能找到输入框")
	}

	// 2.5 新建对话
	log.Println("尝试新建对话...")
	var newChatResult string
	chromedp.Run(ctx, chromedp.Evaluate(`
		(function() {
			// 尝试通过选择器点击新建对话按钮
			var selectors = [
				'[class*="new-chat"]', '[class*="new-conversation"]',
				'[class*="newChat"]', '[class*="newConversation"]',
				'[data-testid*="new-chat"]'
			];
			for (var i = 0; i < selectors.length; i++) {
				var btn = document.querySelector(selectors[i]);
				if (btn && btn.offsetParent !== null) {
					btn.click();
					return 'clicked: ' + selectors[i];
				}
			}

			// 尝试通过文本内容查找按钮
			var btns = document.querySelectorAll('a, button, div[role="button"]');
			for (var i = 0; i < btns.length; i++) {
				var text = btns[i].innerText || btns[i].textContent || '';
				if (text.includes('新建') || text.includes('新对话') ||
					text.includes('New Chat') || text.includes('New chat') ||
					text.includes('开聊') || text.includes('发消息')) {
					btns[i].click();
					return 'clicked: ' + text.trim();
				}
			}
			return 'no button';
		})()
	`, &newChatResult))
	log.Printf("新建对话按钮结果: %s", newChatResult)

	// 如果没有找到新建对话按钮，直接导航到新对话URL
	if newChatResult == "no button" || newChatResult == "" {
		log.Println("未找到新建对话按钮，导航到新对话URL...")
		chromedp.Run(ctx, chromedp.Navigate(doubaoChatURL))
		chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
	} else {
		chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
	}

	// 重新等待输入框
	chromedp.Run(ctx, chromedp.WaitVisible("textarea, div[contenteditable='true']", chromedp.ByQuery))

	log.Println("开始输入问题...")

	// 3. 发送问题
	if err := d.sendQuestion(ctx, question); err != nil {
		return nil, fmt.Errorf("发送问题失败: %v", err)
	}

	log.Println("发送操作完成，等待AI响应...")
	chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	// 4. 检查页面异常提示（输入超限、人机验证等）
	log.Println("检查页面异常提示...")
	anomalyType, keyword, anomalyContext := d.detectPageAnomaly(ctx)
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
		passed, errMsg := d.handleCaptcha(ctx)
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
		chromedp.Run(ctx, chromedp.WaitVisible("textarea, div[contenteditable='true']", chromedp.ByQuery))
		chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
		if err := d.sendQuestion(ctx, question); err != nil {
			return nil, fmt.Errorf("重新发送问题失败: %v", err)
		}
		chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
	}

	// 5. 等待AI回复完成
	log.Println("等待AI回复...")
	if err := d.waitForAIResponse(ctx); err != nil {
		log.Printf("等待AI回复失败: %v", err)
	}

	// 6. 提取AI回复
	log.Println("获取AI回复...")
	answer, thinking, err := d.extractAIReply(ctx)
	if err != nil {
		log.Printf("提取AI回复失败: %v", err)
		close(streamChan)
		return nil, fmt.Errorf("提取AI回复失败: %v", err)
	}

	log.Printf("获取到答案长度: %d, 思考过程长度: %d", len(answer), len(thinking))

	// 检查是否被检测为机器人
	botDetectKeywords := []string{
		"人机验证", "机器识别", "自动检测", "异常请求",
		"请完成验证", "captcha", "robot", "automation",
		"检测到", "非正常", "自动化工具",
	}

	detectedAsBot := false
	detectInfo := ""
	for _, keyword := range botDetectKeywords {
		if strings.Contains(strings.ToLower(answer), strings.ToLower(keyword)) {
			detectedAsBot = true
			detectInfo = fmt.Sprintf("检测到关键词: %s", keyword)
			log.Printf("警告：可能被检测为机器人！关键词: %s", keyword)
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
func (d *DoubaoClient) AskInConversation(question string) (*platform.AskResult, error) {
	ctx, cancel := context.WithTimeout(d.session.Ctx, 90*time.Second)
	defer cancel()

	// 用于存储流式内容的通道
	streamChan := make(chan string, 100)

	// 结果
	result := &platform.AskResult{
		StreamChan: streamChan,
	}

	// 注入反检测脚本
	log.Println("AskInConversation: 注入反检测脚本...")
	d.session.InjectAntiDetection()
	chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	// 等待输入框出现（不导航，直接在当前页面操作）
	log.Println("AskInConversation: 等待输入框...")
	err := chromedp.Run(ctx,
		chromedp.WaitVisible("textarea, div[contenteditable='true']", chromedp.ByQuery),
	)
	if err != nil {
		return nil, fmt.Errorf("当前页面未找到输入框: %v", err)
	}

	// 输入并发送问题
	log.Println("AskInConversation: 开始输入问题...")
	if err := d.sendQuestion(ctx, question); err != nil {
		return nil, fmt.Errorf("发送问题失败: %v", err)
	}

	log.Println("AskInConversation: 等待AI响应...")
	chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	// 检查页面异常提示
	anomalyType, _, anomalyContext := d.detectPageAnomaly(ctx)
	if anomalyType == "input_error" {
		close(streamChan)
		result.IsBot = false
		result.DetectInfo = fmt.Sprintf("输入超限: %s", anomalyContext)
		return result, fmt.Errorf("输入内容超限: %s", anomalyContext)
	}
	if anomalyType == "captcha" {
		passed, errMsg := d.handleCaptcha(ctx)
		if !passed {
			close(streamChan)
			result.IsBot = true
			result.DetectInfo = fmt.Sprintf("人机验证未通过: %s", errMsg)
			return result, fmt.Errorf("触发人机验证且未能自动通过: %s", errMsg)
		}
		// 验证通过后重新输入并发送
		chromedp.Run(ctx, chromedp.WaitVisible("textarea, div[contenteditable='true']", chromedp.ByQuery))
		chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
		if err := d.sendQuestion(ctx, question); err != nil {
			return nil, fmt.Errorf("重新发送问题失败: %v", err)
		}
		chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
	}

	// 等待AI回复完成
	if err := d.waitForAIResponse(ctx); err != nil {
		log.Printf("等待AI回复失败: %v", err)
	}

	// 提取回复内容
	answer, thinking, err := d.extractAIReply(ctx)
	if err != nil {
		log.Printf("提取AI回复失败: %v", err)
		close(streamChan)
		return nil, fmt.Errorf("提取AI回复失败: %v", err)
	}

	// 检查是否被检测为机器人
	botDetectKeywords := []string{
		"人机验证", "机器识别", "自动检测", "异常请求",
		"请完成验证", "captcha", "robot", "automation",
		"检测到", "非正常", "自动化工具",
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
// 导航到聊天页面，点击"新建对话"按钮，输入并发送初始消息
func (d *DoubaoClient) StartNewConversation(initialMessage string) (*platform.AskResult, error) {
	ctx, cancel := context.WithTimeout(d.session.Ctx, 90*time.Second)
	defer cancel()

	// 用于存储流式内容的通道
	streamChan := make(chan string, 100)

	// 结果
	result := &platform.AskResult{
		StreamChan: streamChan,
	}

	// 注入反检测脚本
	log.Println("StartNewConversation: 注入反检测脚本...")
	d.session.InjectAntiDetection()
	chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	// 导航到聊天页面
	log.Println("StartNewConversation: 导航到聊天页面...")
	chromedp.Run(ctx,
		chromedp.Navigate(doubaoChatURL),
		chromedp.Sleep(3*time.Second),
	)

	// 注入反检测
	d.session.InjectAntiDetection()
	chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	// 点击"新建对话"按钮
	log.Println("StartNewConversation: 点击新建对话按钮...")
	var newChatResult string
	chromedp.Run(ctx, chromedp.Evaluate(`
		(function() {
			// 尝试通过选择器点击新建对话按钮
			var selectors = [
				'[class*="new-chat"]', '[class*="new-conversation"]',
				'[class*="newChat"]', '[class*="newConversation"]',
				'[data-testid*="new-chat"]'
			];
			for (var i = 0; i < selectors.length; i++) {
				var btn = document.querySelector(selectors[i]);
				if (btn && btn.offsetParent !== null) {
					btn.click();
					return 'clicked: ' + selectors[i];
				}
			}

			// 尝试通过文本内容查找按钮
			var btns = document.querySelectorAll('a, button, div[role="button"]');
			for (var i = 0; i < btns.length; i++) {
				var text = btns[i].innerText || btns[i].textContent || '';
				if (text.includes('新建') || text.includes('新对话') ||
					text.includes('New Chat') || text.includes('New chat') ||
					text.includes('开聊') || text.includes('发消息')) {
					btns[i].click();
					return 'clicked: ' + text.trim();
				}
			}
			return 'no button';
		})()
	`, &newChatResult))
	log.Printf("新建对话按钮结果: %s", newChatResult)

	// 如果没有找到新建对话按钮，直接导航到新对话URL
	if newChatResult == "no button" || newChatResult == "" {
		log.Println("未找到新建对话按钮，导航到新对话URL...")
		chromedp.Run(ctx, chromedp.Navigate(doubaoChatURL))
		chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
	} else {
		chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
	}

	// 等待新对话页面加载完成
	log.Println("StartNewConversation: 等待新对话页面加载...")
	err := chromedp.Run(ctx,
		chromedp.WaitVisible("textarea, div[contenteditable='true']", chromedp.ByQuery),
	)
	if err != nil {
		return nil, fmt.Errorf("新对话页面加载失败，未找到输入框: %v", err)
	}
	chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	// 输入并发送初始消息
	log.Println("StartNewConversation: 输入初始消息...")
	if err := d.sendQuestion(ctx, initialMessage); err != nil {
		return nil, fmt.Errorf("发送初始消息失败: %v", err)
	}

	log.Println("StartNewConversation: 等待AI响应...")
	chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	// 检查页面异常提示
	anomalyType, _, anomalyContext := d.detectPageAnomaly(ctx)
	if anomalyType == "input_error" {
		close(streamChan)
		result.IsBot = false
		result.DetectInfo = fmt.Sprintf("输入超限: %s", anomalyContext)
		return result, fmt.Errorf("输入内容超限: %s", anomalyContext)
	}
	if anomalyType == "captcha" {
		passed, errMsg := d.handleCaptcha(ctx)
		if !passed {
			close(streamChan)
			result.IsBot = true
			result.DetectInfo = fmt.Sprintf("人机验证未通过: %s", errMsg)
			return result, fmt.Errorf("触发人机验证且未能自动通过: %s", errMsg)
		}
		// 验证通过后重新输入并发送
		chromedp.Run(ctx, chromedp.WaitVisible("textarea, div[contenteditable='true']", chromedp.ByQuery))
		chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
		if err := d.sendQuestion(ctx, initialMessage); err != nil {
			return nil, fmt.Errorf("重新发送初始消息失败: %v", err)
		}
		chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
	}

	// 等待AI回复完成
	if err := d.waitForAIResponse(ctx); err != nil {
		log.Printf("等待AI回复失败: %v", err)
	}

	// 提取回复内容
	answer, thinking, err := d.extractAIReply(ctx)
	if err != nil {
		log.Printf("提取AI回复失败: %v", err)
		close(streamChan)
		return nil, fmt.Errorf("提取AI回复失败: %v", err)
	}

	// 检查是否被检测为机器人
	botDetectKeywords := []string{
		"人机验证", "机器识别", "自动检测", "异常请求",
		"请完成验证", "captcha", "robot", "automation",
		"检测到", "非正常", "自动化工具",
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

// sendQuestion 输入并发送问题
// 使用insertText方式输入内容，支持多种发送方式（按钮点击、Enter键、JS触发）
func (d *DoubaoClient) sendQuestion(ctx context.Context, question string) error {
	// 查找输入框选择器
	inputSelectors := []string{
		`textarea`,
		`div[contenteditable="true"]`,
		`.chat-input textarea`,
		`[class*="chat-input"] textarea`,
	}

	var inputSelector string
	for _, selector := range inputSelectors {
		var found bool
		err := chromedp.Run(ctx, chromedp.Evaluate(`
			(function() {
				var el = document.querySelector('`+selector+`');
				return el && el.offsetParent !== null;
			})()
		`, &found))
		if err == nil && found {
			inputSelector = selector
			log.Printf("找到输入框: %s", selector)
			break
		}
	}

	if inputSelector == "" {
		return fmt.Errorf("未找到可用的输入框")
	}

	// 点击输入框获取焦点
	log.Println("点击输入框...")
	err := chromedp.Run(ctx,
		chromedp.Click(inputSelector, chromedp.ByQuery),
	)
	if err != nil {
		log.Printf("点击输入框失败: %v", err)
	}
	chromedp.Run(ctx, chromedp.Sleep(300*time.Millisecond))

	// 使用document.execCommand('insertText')设置输入框内容
	// 这是最可靠的方式：触发React/Vue能识别的所有事件，且正确处理中文编码
	log.Println("使用insertText设置输入框内容...")
	questionJSON, err := json.Marshal(question)
	if err != nil {
		log.Printf("JSON编码问题失败: %v", err)
		questionJSON = []byte(fmt.Sprintf(`"%s"`, question))
	}

	var jsInputResult int
	err = chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
		(function() {
			var text = %s;
			// 尝试textarea
			var textarea = document.querySelector('textarea');
			if (textarea) {
				textarea.focus();
				textarea.select();
				document.execCommand('insertText', false, text);
				return textarea.value.length;
			}
			// 尝试contenteditable
			var editable = document.querySelector('div[contenteditable="true"]');
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
			chromedp.SendKeys(inputSelector, question, chromedp.ByQuery),
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
				var textarea = document.querySelector('textarea');
				if (textarea) return textarea.value;
				var editable = document.querySelector('div[contenteditable="true"]');
				if (editable) return editable.textContent;
				return '';
			})()
		`, &verifyValue))
		if len(verifyValue) > 0 {
			log.Printf("输入框验证成功，当前值长度: %d", len(verifyValue))
		} else {
			log.Printf("输入框验证失败，回退到SendKeys...")
			err = chromedp.Run(ctx,
				chromedp.SendKeys(inputSelector, question, chromedp.ByQuery),
			)
			if err != nil {
				return fmt.Errorf("输入内容失败: %v", err)
			}
		}
	}

	log.Println("问题输入完成")

	// 等待片刻后发送
	chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))

	// 尝试点击发送按钮
	sendSelectors := []string{
		"button[class*='send']",
		"button[class*='Send']",
		"button[class*='submit']",
		"button[class*='Submit']",
		"[class*='send-btn']",
		"[class*='send-button']",
		"[aria-label='发送']",
		"[aria-label='Send']",
		"button[type='submit']",
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
			log.Printf("点击按钮失败: %v", err)
		}
	}

	if !sendClicked {
		log.Println("未找到发送按钮，尝试使用JS点击...")
		var jsClicked bool
		err := chromedp.Run(ctx, chromedp.Evaluate(`
			(function() {
				var selectors = [
					'button[class*="send"]', 'button[class*="Send"]',
					'.send-btn', '.send-button',
					'[data-testid="send"]',
					'form button[type="submit"]',
					'.chat-input button', '.input-area button'
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

		if err == nil && jsClicked {
			sendClicked = true
			log.Println("通过JavaScript成功点击发送按钮")
		} else {
			log.Printf("JS点击失败或未找到按钮: %v", err)
		}
	}

	if !sendClicked {
		log.Println("使用 Enter 键发送...")
		err = chromedp.Run(ctx, chromedp.Focus(inputSelector, chromedp.ByQuery))
		if err != nil {
			log.Printf("聚焦输入框失败: %v", err)
		}

		chromedp.Run(ctx, chromedp.Sleep(200*time.Millisecond))

		err = chromedp.Run(ctx,
			chromedp.SendKeys(inputSelector, "\r", chromedp.ByQuery),
		)
		if err != nil {
			log.Printf("Enter键发送失败: %v，尝试JavaScript触发...", err)
			chromedp.Run(ctx, chromedp.Evaluate(`
				(function() {
					var textarea = document.querySelector('textarea');
					if (textarea) {
						var event = new KeyboardEvent('keydown', {key: 'Enter', code: 'Enter', keyCode: 13, bubbles: true});
						textarea.dispatchEvent(event);
					}
					var editable = document.querySelector('div[contenteditable="true"]');
					if (editable) {
						var event = new KeyboardEvent('keydown', {key: 'Enter', code: 'Enter', keyCode: 13, bubbles: true});
						editable.dispatchEvent(event);
					}
				})()
			`, nil))
		} else {
			log.Println("通过Enter键发送成功")
		}
	}

	return nil
}

// waitForAIResponse 等待AI回复完成
// 通过检测回复区域内容稳定性判断回复是否完成
func (d *DoubaoClient) waitForAIResponse(ctx context.Context) error {
	maxWaitTime := 90 * time.Second
	checkInterval := 2 * time.Second
	startTime := time.Now()

	var lastContentLength int
	stableCount := 0
	var resultStr string

	for time.Since(startTime) < maxWaitTime {
		chromedp.Run(ctx, chromedp.Sleep(checkInterval))

		// 每轮循环先检查页面异常状态
		loopAnomalyType, _, _ := d.detectPageAnomaly(ctx)
		if loopAnomalyType == "input_error" {
			log.Printf("等待过程中检测到输入超限提示")
			return fmt.Errorf("输入内容超限")
		}
		if loopAnomalyType == "captcha" {
			log.Printf("等待过程中检测到人机验证")
			passed, _ := d.handleCaptcha(ctx)
			if passed {
				// 验证通过后重置等待状态
				lastContentLength = 0
				stableCount = 0
				startTime = time.Now()
				continue
			}
			return fmt.Errorf("人机验证未通过")
		}

		var currentContentLength int
		var hasNonThinkingReply bool
		var thinkingLength int

		// 豆包回复区域检测：检查停止按钮、回复内容等
		chromedp.Run(ctx, chromedp.Evaluate(`
			(function() {
				// 豆包回复区域选择器
				var replySelectors = [
					'[class*="assistant"]',
					'[class*="bot-message"]',
					'[class*="message-content"]',
					'[class*="markdown-body"]',
					'[class*="ai-reply"]',
					'.chat-message:last-child',
					'.message-item:last-child',
					'[class*="markdown"]:last-child',
					'.message:last-child'
				];

				// 思考过程特征
				var thinkingPatterns = [
					/思考过程/, /深度思考/, /分析中/, /思考结束/, /思考中/, /跳过思考/,
					/让我想想/, /让我分析/, /我来分析/
				];

				function isThinkingOnly(text) {
					if (!text || text.length < 20) return false;
					var nonThinkingText = text;
					for (var i = 0; i < thinkingPatterns.length; i++) {
						nonThinkingText = nonThinkingText.replace(thinkingPatterns[i], '');
					}
					return nonThinkingText.trim().length < text.length * 0.3;
				}

				// 检查是否还在生成中（停止按钮是否可见）
				var stopSelectors = [
					'button[class*="stop"]', '[class*="stop-generate"]',
					'[class*="stop-btn"]', '[aria-label="停止"]',
					'[aria-label="Stop"]', '[class*="generating"]'
				];
				for (var i = 0; i < stopSelectors.length; i++) {
					var stopBtn = document.querySelector(stopSelectors[i]);
					if (stopBtn && stopBtn.offsetParent !== null) {
						// 还在生成中，返回当前内容但标记为未完成
						var chatArea = document.querySelector('[class*="chat"], [class*="conversation"], main');
						var len = chatArea ? chatArea.innerText.length : 0;
						return JSON.stringify({length: len, hasNonThinkingReply: false, thinkingLength: 0, generating: true});
					}
				}

				for (var i = 0; i < replySelectors.length; i++) {
					var elem = document.querySelector(replySelectors[i]);
					if (elem && elem.innerText && elem.innerText.length > 20) {
						var text = elem.innerText;
						if (isThinkingOnly(text)) {
							return JSON.stringify({length: text.length, hasNonThinkingReply: false, thinkingLength: text.length, generating: false});
						}
						return JSON.stringify({length: text.length, hasNonThinkingReply: true, thinkingLength: 0, generating: false});
					}
				}

				var chatArea = document.querySelector('[class*="chat"], [class*="conversation"], main');
				if (chatArea) {
					var chatText = chatArea.innerText;
					var isThinking = isThinkingOnly(chatText);
					return JSON.stringify({length: chatText.length, hasNonThinkingReply: !isThinking && chatText.length > 100, thinkingLength: isThinking ? chatText.length : 0, generating: false});
				}
				return JSON.stringify({length: 0, hasNonThinkingReply: false, thinkingLength: 0, generating: false});
			})()
		`, &resultStr))

		var evalResult struct {
			Length              int  `json:"length"`
			HasNonThinkingReply bool `json:"hasNonThinkingReply"`
			ThinkingLength      int  `json:"thinkingLength"`
			Generating          bool `json:"generating"`
		}
		if err := json.Unmarshal([]byte(resultStr), &evalResult); err == nil {
			currentContentLength = evalResult.Length
			hasNonThinkingReply = evalResult.HasNonThinkingReply
			thinkingLength = evalResult.ThinkingLength

			// 如果还在生成中，继续等待
			if evalResult.Generating {
				log.Printf("AI仍在生成中...（当前内容长度: %d）", currentContentLength)
				lastContentLength = currentContentLength
				continue
			}
		}

		if thinkingLength > 0 {
			log.Printf("检测到思考过程...（思考长度: %d，总长度: %d）", thinkingLength, currentContentLength)
		}

		// 如果只有思考过程，继续等待正式回复
		if !hasNonThinkingReply {
			if time.Since(startTime) > 60*time.Second {
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

	return nil
}

// extractAIReply 从页面提取AI回复，分离思考过程和正式回复
// 返回 (answer, thinking, error)
func (d *DoubaoClient) extractAIReply(ctx context.Context) (string, string, error) {
	var jsResult string
	chromedp.Run(ctx, chromedp.Evaluate(`
		(function() {
			// 豆包页面需要排除的无关文本
			var excludeKeywords = ['登录', '注册', '扫码', '下载', '意见反馈',
				'用户协议', '隐私政策', '关于我们', '退出登录',
				'ICP备', '网信算备', '京公网', '京ICP',
				'豆包', '抖音', '字节跳动', '今日头条',
				'新对话', '历史记录', '设置', '帮助',
				'如何可以帮你', '有什么可以帮你', '你可以问我'];

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
					/思考过程/, /深度思考/, /分析中/, /思考结束/, /思考中/, /跳过思考/,
					/让我想想/, /让我分析/, /我来分析/,
					/Formulate the Strategy/, /Identify the Core Task/, /Gather Information/,
					/Evaluate Alternatives/, /Structure the Response/,
					/Analyze the Input/, /Analyze the Question/, /Analyze the Request/
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

			// 策略1：从AI回复元素中分别提取思考过程和正式回复
			var replySelectors = [
				'[class*="assistant"]',
				'[class*="bot-message"]',
				'[class*="message-content"]',
				'[class*="markdown-body"]',
				'[class*="ai-reply"]',
				'.chat-message:last-child',
				'.message-item:last-child',
				'[class*="thinking"]',
				'[class*="think"]',
				'[class*="reasoning"]',
				'[class*="markdown"]',
				'.message:last-child'
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

			// 去重（按文本内容）
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

// detectPageAnomaly 检测页面异常状态
// 返回: anomalyType("input_error"/"captcha"/"none"), keyword, context
func (d *DoubaoClient) detectPageAnomaly(ctx context.Context) (string, string, string) {
	var resultStr string
	chromedp.Run(ctx, chromedp.Evaluate(`
		(function() {
			var errorKeywords = [
				'最多可以输入', '字数超', '超出限制', '内容过长', '输入超限',
				'字数限制', '超过最大', '超出字数', '最多输入', '字数已达上限',
				'limit', 'too long', 'exceeds', 'maximum',
				'频率限制', '请求过于频繁', '请稍后再试'
			];
			var captchaKeywords = [
				'人机验证', '机器识别', '自动检测', '异常请求',
				'请完成验证', 'captcha', 'robot', 'automation',
				'检测到', '非正常', '自动化工具', '安全验证',
				'滑块验证', '图形验证', '请拖动滑块', '请点击验证',
				'访问验证', '别离开', '请进行验证', '通过后即可继续访问',
				'verify', 'slider', 'puzzle', 'TraceID', '请求时间'
			];
			var allText = document.body.innerText || '';
			for (var i = 0; i < errorKeywords.length; i++) {
				var idx = allText.indexOf(errorKeywords[i]);
				if (idx !== -1) {
					var start = Math.max(0, idx - 20);
					var end = Math.min(allText.length, idx + 50);
					return JSON.stringify({type: 'input_error', keyword: errorKeywords[i], context: allText.substring(start, end)});
				}
			}
			for (var i = 0; i < captchaKeywords.length; i++) {
				var idx = allText.indexOf(captchaKeywords[i]);
				if (idx !== -1) {
					var start = Math.max(0, idx - 20);
					var end = Math.min(allText.length, idx + 50);
					return JSON.stringify({type: 'captcha', keyword: captchaKeywords[i], context: allText.substring(start, end)});
				}
			}
			var captchaSelectors = [
				'iframe[src*="captcha"]', '[class*="captcha"]', '[class*="verify"]',
				'[class*="slider"]', '[class*="puzzle"]', '[id*="captcha"]',
				'[id*="verify"]', '.geetest_holder', '.nc_wrapper', '#aliyunCaptcha',
				'[class*="challenge"]', '[id*="challenge"]', '[class*="check"]',
				'input[type="checkbox"][class*="check"]'
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

// sliderInfo 滑块验证码元素信息
type sliderInfo struct {
	Found      bool    `json:"found"`
	SliderX    float64 `json:"sliderX"`
	SliderY    float64 `json:"sliderY"`
	TrackWidth float64 `json:"trackWidth"`
	Selector   string  `json:"selector"`
}

// handleSliderCaptcha 自动处理滑块验证码
// 使用chromedp底层鼠标事件模拟人类拖拽行为
func (d *DoubaoClient) handleSliderCaptcha(ctx context.Context) bool {
	log.Println("尝试自动处理滑块验证码...")

	// 1. 查找滑块元素和轨道信息
	var infoStr string
	chromedp.Run(ctx, chromedp.Evaluate(`
		(function() {
			var sliderSelectors = [
				'[class*="slider"] [class*="btn"]',
				'[class*="slider"] [class*="handler"]',
				'[class*="slider"] [class*="drag"]',
				'[class*="slider"] [class*="thumb"]',
				'[class*="slider"] [class*="block"]',
				'[class*="slider"] [class*="button"]',
				'[class*="captcha"] [class*="slider"]',
				'[class*="captcha"] [class*="btn"]',
				'[class*="captcha"] [class*="drag"]',
				'[class*="verify"] [class*="slider"]',
				'[class*="verify"] [class*="btn"]',
				'[class*="verify"] [class*="drag"]',
				'[class*="nc"] [class*="btn"]',
				'[class*="nc"] [class*="slider"]',
				'[class*="nc"] [class*="handler"]',
				'.nc_iconfont.btn_slide',
				'.nc-lang-cnt',
				'.btn_slide',
				'[class*="slide"]',
				'[class*="drag"]',
				'[role="slider"]',
				'div[class*="handler"]',
				'span[class*="handler"]',
				'div[class*="btn"]'
			];
			var trackSelectors = [
				'[class*="slider"] [class*="track"]',
				'[class*="slider"] [class*="bar"]',
				'[class*="slider"] [class*="bg"]',
				'[class*="captcha"] [class*="track"]',
				'[class*="captcha"] [class*="bar"]',
				'[class*="verify"] [class*="track"]',
				'[class*="verify"] [class*="bar"]',
				'[class*="nc"] [class*="track"]',
				'[class*="nc"] [class*="bg"]',
				'.nc_iconfont',
				'.nc-lang-cnt',
				'[class*="slide-track"]',
				'[class*="drag-track"]'
			];

			for (var i = 0; i < sliderSelectors.length; i++) {
				var slider = document.querySelector(sliderSelectors[i]);
				if (!slider || slider.offsetParent === null) continue;

				var rect = slider.getBoundingClientRect();
				if (rect.width < 10 || rect.height < 10) continue;

				var trackWidth = 0;
				for (var j = 0; j < trackSelectors.length; j++) {
					var track = document.querySelector(trackSelectors[j]);
					if (track && track.offsetParent !== null) {
						var trackRect = track.getBoundingClientRect();
						if (trackRect.width > trackWidth) {
							trackWidth = trackRect.width;
						}
					}
				}

				if (trackWidth === 0) {
					var parent = slider.parentElement;
					if (parent) {
						var parentRect = parent.getBoundingClientRect();
						trackWidth = parentRect.width - rect.width;
					}
				}

				if (trackWidth < 50) trackWidth = 260;

				return JSON.stringify({
					found: true,
					sliderX: rect.left + rect.width / 2,
					sliderY: rect.top + rect.height / 2,
					trackWidth: trackWidth,
					selector: sliderSelectors[i]
				});
			}
			return JSON.stringify({found: false, sliderX: 0, sliderY: 0, trackWidth: 0, selector: ''});
		})()
	`, &infoStr))

	var info sliderInfo
	if err := json.Unmarshal([]byte(infoStr), &info); err != nil || !info.Found {
		log.Printf("未找到滑块元素: err=%v, info=%s", err, infoStr)
		return false
	}

	log.Printf("找到滑块元素: selector=%s, 位置=(%.0f, %.0f), 轨道宽度=%.0f",
		info.Selector, info.SliderX, info.SliderY, info.TrackWidth)

	// 2. 生成人类拖拽轨迹
	trajectory := d.generateHumanTrajectory(info.TrackWidth)

	// 3. 使用chromedp底层鼠标事件执行拖拽
	success := d.executeSliderDrag(ctx, info.SliderX, info.SliderY, trajectory)

	if success {
		log.Println("滑块拖拽完成，等待验证结果...")
		chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

		// 检查验证是否通过
		anomalyType, _, _ := d.detectPageAnomaly(ctx)
		if anomalyType == "none" {
			log.Println("滑块验证通过！")
			return true
		}
		log.Println("滑块验证未通过，可能需要重试...")
	}

	return false
}

// generateHumanTrajectory 生成模拟人类的拖拽轨迹
// 返回一系列相对偏移量 [{dx, dy, delay}, ...]
func (d *DoubaoClient) generateHumanTrajectory(totalDistance float64) []struct {
	dx    float64
	dy    float64
	delay time.Duration
} {
	var trajectory []struct {
		dx    float64
		dy    float64
		delay time.Duration
	}

	// 模拟人类拖拽：先快后慢，有微小抖动
	remaining := totalDistance
	step := 0

	for remaining > 0 {
		// 速度曲线：开始快，中间稳定，结尾慢
		var moveRatio float64
		progress := 1.0 - remaining/totalDistance
		if progress < 0.3 {
			moveRatio = 0.08 + rand.Float64()*0.04
		} else if progress < 0.7 {
			moveRatio = 0.05 + rand.Float64()*0.03
		} else if progress < 0.9 {
			moveRatio = 0.03 + rand.Float64()*0.02
		} else {
			moveRatio = 0.01 + rand.Float64()*0.015
		}

		dx := totalDistance * moveRatio
		if dx > remaining {
			dx = remaining
		}

		// 微小的Y轴抖动（模拟手抖）
		dy := (rand.Float64() - 0.5) * 2.0

		// 随机延迟（模拟人类反应时间）
		var delay time.Duration
		if progress < 0.1 {
			delay = time.Duration(30+rand.Intn(40)) * time.Millisecond
		} else if progress < 0.8 {
			delay = time.Duration(10+rand.Intn(25)) * time.Millisecond
		} else {
			delay = time.Duration(30+rand.Intn(60)) * time.Millisecond
		}

		// 偶尔停顿（模拟人类犹豫）
		if step > 3 && rand.Float64() < 0.1 {
			delay += time.Duration(50+rand.Intn(100)) * time.Millisecond
		}

		trajectory = append(trajectory, struct {
			dx    float64
			dy    float64
			delay time.Duration
		}{dx: dx, dy: dy, delay: delay})

		remaining -= dx
		step++
	}

	// 结尾微调（模拟人类松手前的微调）
	for i := 0; i < 3; i++ {
		trajectory = append(trajectory, struct {
			dx    float64
			dy    float64
			delay time.Duration
		}{
			dx:    (rand.Float64() - 0.5) * 3,
			dy:    (rand.Float64() - 0.5) * 2,
			delay: time.Duration(30+rand.Intn(50)) * time.Millisecond,
		})
	}

	log.Printf("生成拖拽轨迹: %d步, 总距离=%.0f", len(trajectory), totalDistance)
	return trajectory
}

// executeSliderDrag 使用chromedp鼠标事件执行滑块拖拽
func (d *DoubaoClient) executeSliderDrag(ctx context.Context, startX, startY float64, trajectory []struct {
	dx    float64
	dy    float64
	delay time.Duration
}) bool {
	// 移动鼠标到滑块起始位置
	err := chromedp.Run(ctx,
		chromedp.MouseEvent(input.MouseMoved, startX, startY, chromedp.Button("left")),
	)
	if err != nil {
		log.Printf("移动鼠标到滑块失败: %v", err)
		return false
	}
	chromedp.Run(ctx, chromedp.Sleep(200*time.Millisecond))

	// 按下鼠标左键
	err = chromedp.Run(ctx,
		chromedp.MouseEvent(input.MousePressed, startX, startY, chromedp.Button("left"), chromedp.ClickCount(1)),
	)
	if err != nil {
		log.Printf("按下鼠标失败: %v", err)
		return false
	}
	chromedp.Run(ctx, chromedp.Sleep(100*time.Millisecond))

	// 沿轨迹拖拽
	currentX := startX
	currentY := startY

	for i, step := range trajectory {
		currentX += step.dx
		currentY += step.dy

		err = chromedp.Run(ctx,
			chromedp.MouseEvent(input.MouseMoved, currentX, currentY, chromedp.Button("left")),
		)
		if err != nil {
			log.Printf("拖拽步骤%d失败: %v", i, err)
			break
		}

		chromedp.Run(ctx, chromedp.Sleep(step.delay))
	}

	// 松开鼠标
	chromedp.Run(ctx, chromedp.Sleep(100*time.Millisecond))
	err = chromedp.Run(ctx,
		chromedp.MouseEvent(input.MouseReleased, currentX, currentY, chromedp.Button("left"), chromedp.ClickCount(1)),
	)
	if err != nil {
		log.Printf("松开鼠标失败: %v", err)
		return false
	}

	log.Printf("滑块拖拽完成: 起点(%.0f,%.0f) -> 终点(%.0f,%.0f)", startX, startY, currentX, currentY)
	return true
}

// handleCaptcha 尝试自动处理人机验证页面
// 策略：1.尝试滑块验证码 2.尝试点击验证按钮/复选框 3.等待用户手动完成
// 返回: 是否成功通过验证, 错误信息
func (d *DoubaoClient) handleCaptcha(ctx context.Context) (bool, string) {
	log.Println("检测到人机验证页面，尝试自动处理...")

	// 策略0：尝试自动处理滑块验证码（最常见的验证类型）
	for attempt := 0; attempt < 3; attempt++ {
		sliderOK := d.handleSliderCaptcha(ctx)
		if sliderOK {
			return true, ""
		}
		if attempt < 2 {
			log.Printf("滑块验证第%d次尝试失败，重试...", attempt+1)
			chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
		}
	}

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
		`.geetest_holder`,
		`.nc_wrapper`,
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
		anomalyType, _, _ := d.detectPageAnomaly(ctx)
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

		anomalyType, _, _ := d.detectPageAnomaly(ctx)
		if anomalyType == "none" {
			log.Println("✅ 人机验证已通过！")
			// 验证通过后不刷新页面，直接在当前页面继续操作
			chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
			var currentURL string
			chromedp.Run(ctx, chromedp.Location(&currentURL))
			if !strings.Contains(currentURL, "doubao.com") {
				log.Println("验证后不在聊天页面，导航回来...")
				chromedp.Run(ctx,
					chromedp.Navigate(doubaoChatURL),
					chromedp.Sleep(3*time.Second),
				)
				d.session.InjectAntiDetection()
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

// GetCookies 获取当前页面的cookies（使用CDP原生API，避免JavaScript解析导致的unmarshal错误）
func (d *DoubaoClient) GetCookies() (string, error) {
	var cookieParts []string
	err := chromedp.Run(d.session.Ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			networkCookies, err := network.GetCookies().Do(ctx)
			if err != nil {
				// 检查是否是unmarshal解析错误（cookie值包含特殊字符导致）
				if strings.Contains(err.Error(), "could not unmarshal event") ||
					strings.Contains(err.Error(), "parse error") {
					log.Printf("Cookie解析错误（特殊字符导致），降级到document.cookie方案")
					return fmt.Errorf("cookie解析错误: %v", err)
				}
				return err
			}
			for _, c := range networkCookies {
				// 清洗cookie值，过滤可能导致JSON解析失败的控制字符
				cleanValue := sanitizeCookieValue(c.Value)
				cookieParts = append(cookieParts, fmt.Sprintf("%s=%s", c.Name, cleanValue))
			}
			return nil
		}),
	)
	if err != nil {
		// 如果CDP API失败，降级到document.cookie方案
		log.Printf("CDP GetCookies失败: %v，使用备用方案", err)
		return d.getCookiesFromDocument()
	}
	cookieStr := strings.Join(cookieParts, "; ")
	log.Printf("获取到Cookies（%d个）: %s", len(cookieParts), cookieStr)
	return cookieStr, nil
}

// getCookiesFromDocument 使用JavaScript获取cookies（备用方案）
func (d *DoubaoClient) getCookiesFromDocument() (string, error) {
	var cookieStr string
	err := chromedp.Run(d.session.Ctx,
		chromedp.Evaluate(`document.cookie`, &cookieStr),
	)
	if err != nil {
		log.Printf("获取document.cookie失败: %v", err)
		return "", err
	}

	if cookieStr == "" {
		log.Println("document.cookie为空")
		return "", nil
	}

	log.Printf("通过document.cookie获取到: %s", cookieStr)
	return cookieStr, nil
}

// sanitizeCookieValue 清洗cookie值，过滤可能导致JSON解析失败的控制字符
func sanitizeCookieValue(value string) string {
	// 过滤掉JSON控制字符和其他可能导致解析问题的字符
	var cleaned []rune
	for _, r := range value {
		// 跳过控制字符（除了常见的制表符等）
		if r < 0x20 && r != '\t' {
			continue
		}
		// 跳过DEL字符
		if r == 0x7F {
			continue
		}
		cleaned = append(cleaned, r)
	}
	return string(cleaned)
}

// absInt 返回整数的绝对值
func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
