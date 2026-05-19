// Package deepseek DeepSeek平台适配器
// 实现PlatformClient接口，将DeepSeek网页版的功能适配到统一平台架构
package deepseek

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

// DeepSeek网站URL常量
const (
	deepseekLoginURL = "https://chat.deepseek.com/"
	deepseekChatURL  = "https://chat.deepseek.com/"
)

// DeepSeekClient DeepSeek平台客户端
// 嵌入BrowserSession以复用浏览器会话管理能力
type DeepSeekClient struct {
	session *browser.BrowserSession
}

// NewDeepSeekClient 创建DeepSeek客户端
func NewDeepSeekClient(session *browser.BrowserSession) *DeepSeekClient {
	return &DeepSeekClient{session: session}
}

// init 注册DeepSeek平台到全局注册器
func init() {
	platform.GetRegistry().Register(
		models.PlatformDeepSeek,
		func(session *browser.BrowserSession) platform.PlatformClient {
			return NewDeepSeekClient(session)
		},
		&platform.PlatformConfig{
			Platform: models.PlatformDeepSeek,
			LoginURL: "https://chat.deepseek.com/",
			ChatURL:  "https://chat.deepseek.com/",
			Selectors: map[string]string{
				"input_box":       "textarea, #chat-input",
				"send_button":     "button[class*='send']",
				"response_area":   "[class*='assistant'], .markdown-body",
				"new_chat_button": "[class*='new-chat'], a[href='/']",
			},
		},
	)
}

// ==================== PlatformClient 接口实现 ====================

// GetPlatformName 获取平台名称
func (d *DeepSeekClient) GetPlatformName() string {
	return "deepseek"
}

// GetLoginURL 获取平台登录页面URL
func (d *DeepSeekClient) GetLoginURL() string {
	return deepseekLoginURL
}

// GetChatURL 获取平台聊天页面URL
func (d *DeepSeekClient) GetChatURL() string {
	return deepseekChatURL
}

// NavigateToHome 导航到DeepSeek首页（用于加载cookies后使cookies生效）
func (d *DeepSeekClient) NavigateToHome() error {
	ctx := d.session.Ctx

	log.Printf("正在导航到DeepSeek首页: %s", deepseekChatURL)

	// 导航到主页
	if err := chromedp.Run(ctx,
		chromedp.Navigate(deepseekChatURL),
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

// CheckLoggedIn 检查是否已登录（通过页面内容判断）
// DeepSeek登录后通常有用户头像或菜单
func (d *DeepSeekClient) CheckLoggedIn() bool {
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

	// 检查是否是未登录状态（包含登录/注册按钮）
	if strings.Contains(pageText, "登录") && strings.Contains(pageText, "注册") {
		log.Println("检测到未登录状态（页面包含登录/注册按钮）")
		return false
	}

	// 使用JavaScript更精确地检查登录状态
	var isLoggedIn bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				// 检查是否有用户头像（已登录特征）
				var avatar = document.querySelector('[class*="avatar"]');
				if (avatar) return true;

				// 检查是否有用户菜单（已登录特征）
				var userMenu = document.querySelector('[class*="user-menu"]');
				if (userMenu) return true;

				// 检查是否有个人资料相关元素（已登录特征）
				var profile = document.querySelector('[class*="profile"]');
				if (profile) return true;

				// 检查是否有聊天输入框（已登录才能看到）
				var input = document.querySelector('textarea, [contenteditable="true"]');
				if (input) return true;

				// 检查页面是否包含已登录才有的元素
				if (document.body.innerText.includes('退出登录') || document.body.innerText.includes('个人中心')) return true;

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
func (d *DeepSeekClient) OpenLoginPage() error {
	ctx := d.session.Ctx

	log.Println("正在打开DeepSeek网站...")
	log.Printf("Session Context: %v", ctx)

	// 导航到DeepSeek首页
	log.Printf("开始导航到: %s", deepseekLoginURL)
	if err := chromedp.Run(ctx,
		chromedp.Navigate(deepseekLoginURL),
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
	log.Println("2. 输入手机号/邮箱")
	log.Println("3. 输入密码或获取验证码")
	log.Println("4. 完成登录验证")
	log.Println("========================================")

	return nil
}

// Ask 向DeepSeek提问（智能获取AI回复）
// 创建新对话并发送问题，返回完整答案
func (d *DeepSeekClient) Ask(question string) (*platform.AskResult, error) {
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
		chromedp.Navigate(deepseekChatURL),
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
		`#chat-input`,
		`[class*="input"] textarea`,
		`[contenteditable="true"]`,
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

	// 2.5 新建对话（通过点击新建对话按钮或导航到首页）
	log.Println("尝试新建对话...")
	var newChatResult string
	chromedp.Run(ctx, chromedp.Evaluate(`
		(function() {
			// 尝试点击新建对话按钮
			var btns = document.querySelectorAll('a, button, div[role="button"]');
			for (var i = 0; i < btns.length; i++) {
				var text = btns[i].innerText || btns[i].textContent || '';
				if (text.includes('新建') || text.includes('新对话') || text.includes('New Chat') || text.includes('New chat')) {
					btns[i].click();
					return 'clicked: ' + text.trim();
				}
			}
			// 尝试通过选择器查找新建对话按钮
			var newChatBtn = document.querySelector('[class*="new-chat"], a[href="/"]');
			if (newChatBtn) {
				newChatBtn.click();
				return 'clicked: selector';
			}
			return 'no button';
		})()
	`, &newChatResult))
	log.Printf("新建对话按钮结果: %s", newChatResult)

	// 如果没有找到新建对话按钮，直接导航到首页
	if newChatResult == "no button" || newChatResult == "" {
		log.Println("未找到新建对话按钮，导航到首页...")
		chromedp.Run(ctx, chromedp.Navigate(deepseekChatURL))
		chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
	} else {
		chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
	}

	// 重新等待输入框
	chromedp.Run(ctx, chromedp.WaitVisible("textarea", chromedp.ByQuery))

	log.Println("开始输入问题...")

	// 3. 模拟真人输入
	runes := []rune(question)
	log.Printf("输入问题: %s...", string(runes[:min(50, len(runes))]))

	// 点击输入框
	log.Println("点击输入框...")
	err := chromedp.Run(ctx,
		chromedp.Click(inputSelector, chromedp.ByQuery),
	)
	if err != nil {
		log.Printf("点击输入框失败: %v", err)
	}
	chromedp.Run(ctx, chromedp.Sleep(300*time.Millisecond))

	// 使用document.execCommand('insertText')设置textarea值
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
			var textarea = document.querySelector('textarea, #chat-input');
			if (textarea) {
				textarea.focus();
				textarea.select();
				document.execCommand('insertText', false, text);
				return textarea.value.length;
			}
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
			chromedp.SendKeys(inputSelector, question, chromedp.ByQuery),
		)
		if err != nil {
			return nil, fmt.Errorf("输入内容失败: %v", err)
		}
	} else {
		log.Printf("insertText设置输入框成功，内容长度: %d", jsInputResult)
		chromedp.Run(ctx, chromedp.Sleep(300*time.Millisecond))
		var verifyValue string
		chromedp.Run(ctx, chromedp.Evaluate(`
			(function() {
				var textarea = document.querySelector('textarea, #chat-input');
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
				chromedp.SendKeys(inputSelector, question, chromedp.ByQuery),
			)
			if err != nil {
				return nil, fmt.Errorf("输入内容失败: %v", err)
			}
		}
	}

	log.Println("问题输入完成")

	// 4. 发送问题
	log.Println("准备发送问题...")
	chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))

	// 首先确保输入框有内容
	var jsValue string
	err = chromedp.Run(ctx, chromedp.Evaluate(`
		(function() {
			var textarea = document.querySelector('textarea, #chat-input');
			if (textarea && textarea.value) return textarea.value;
			var editable = document.querySelector('[contenteditable="true"]');
			if (editable) return editable.innerText;
			return '';
		})()
	`, &jsValue))
	if err == nil && jsValue != "" {
		log.Printf("输入框当前值: %s (长度:%d)", jsValue[:min(30, len(jsValue))], len(jsValue))
	} else {
		log.Printf("获取输入框值失败或为空: %v", err)
	}

	// 尝试点击发送按钮
	sendSelectors := []string{
		"button[class*='send']",
		"button[class*='Send']",
		"[aria-label='发送']",
		"[aria-label='Send']",
		"button[type='submit']",
		"[data-testid='send-button']",
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
			} else {
				log.Printf("点击按钮失败: %v", err)
			}
		}
	}

	if !sendClicked {
		log.Println("未找到发送按钮，尝试使用JS点击...")
		var jsClicked bool
		err := chromedp.Run(ctx, chromedp.Evaluate(`
			(function() {
				var selectors = [
					'button[class*="send"]',
					'button[class*="Send"]',
					'.send-btn',
					'.send-button',
					'[data-testid="send-button"]',
					'form button[type="submit"]',
					'.chat-input button',
					'.input-area button'
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
					var textarea = document.querySelector('textarea, #chat-input');
					if (textarea) {
						var event = new KeyboardEvent('keydown', {key: 'Enter', code: 'Enter', keyCode: 13, bubbles: true});
						textarea.dispatchEvent(event);
					}
				})()
			`, &sendClicked))
		} else {
			sendClicked = true
			log.Println("通过Enter键发送成功")
		}
	}

	log.Println("发送操作完成，等待AI响应...")
	chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	// 4.5 检查页面异常提示（输入超限、人机验证等）
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
		chromedp.Run(ctx, chromedp.WaitVisible("textarea", chromedp.ByQuery))
		chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
		// 重新输入问题
		questionJSON, _ := json.Marshal(question)
		var reInputResult int
		chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
			(function() {
				var text = %s;
				var textarea = document.querySelector('textarea, #chat-input');
				if (textarea) {
					textarea.focus();
					textarea.select();
					document.execCommand('insertText', false, text);
					return textarea.value.length;
				}
				return 0;
			})()
		`, string(questionJSON)), &reInputResult))
		log.Printf("重新输入问题完成，内容长度: %d", reInputResult)
		chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))
		// 重新发送
		chromedp.Run(ctx, chromedp.SendKeys("textarea", "\r", chromedp.ByQuery))
		chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
	}

	// 5. 等待AI回复完成（区分思考过程和正式回复）
	log.Println("等待AI回复...")

	botDetectKeywords := []string{
		"人机验证", "机器识别", "自动检测", "异常请求",
		"请完成验证", "captcha", "robot", "automation",
		"检测到", "非正常", "自动化工具",
	}

	detectedAsBot := false
	detectInfo := ""

	maxWaitTime := 90 * time.Second
	checkInterval := 2 * time.Second
	startTime := time.Now()

	var lastContentLength int
	stableCount := 0
	var resultStr string

	for time.Since(startTime) < maxWaitTime {
		chromedp.Run(ctx, chromedp.Sleep(checkInterval))

		// 每轮循环先检查页面异常状态（人机验证、输入超限等）
		loopAnomalyType, _, _ := d.detectPageAnomaly(ctx)
		if loopAnomalyType == "input_error" {
			log.Printf("等待过程中检测到输入超限提示")
			close(streamChan)
			result.IsBot = false
			result.DetectInfo = "等待过程中检测到输入超限"
			result.Answer = ""
			result.Thinking = ""
			return result, fmt.Errorf("输入内容超限，页面提示字数限制")
		}
		if loopAnomalyType == "captcha" {
			log.Printf("等待过程中检测到人机验证")
			// 尝试自动处理验证
			passed, errMsg := d.handleCaptcha(ctx)
			if !passed {
				close(streamChan)
				result.IsBot = true
				result.DetectInfo = fmt.Sprintf("等待过程中人机验证未通过: %s", errMsg)
				result.Answer = ""
				result.Thinking = ""
				return result, fmt.Errorf("触发人机验证且未能自动通过: %s", errMsg)
			}
			// 验证通过后，重新输入问题并发送
			log.Println("等待过程中人机验证已通过，重新输入问题...")
			chromedp.Run(ctx, chromedp.WaitVisible("textarea", chromedp.ByQuery))
			chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
			questionJSON, _ := json.Marshal(question)
			var reInputResult int
			chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
				(function() {
					var text = %s;
					var textarea = document.querySelector('textarea, #chat-input');
					if (textarea) {
						textarea.focus();
						textarea.select();
						document.execCommand('insertText', false, text);
						return textarea.value.length;
					}
					return 0;
				})()
			`, string(questionJSON)), &reInputResult))
			log.Printf("重新输入问题完成，内容长度: %d", reInputResult)
			chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))
			chromedp.Run(ctx, chromedp.SendKeys("textarea", "\r", chromedp.ByQuery))
			chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
			// 重置等待状态
			lastContentLength = 0
			stableCount = 0
			startTime = time.Now()
			continue
		}

		var currentContentLength int
		var hasNonThinkingReply bool
		var thinkingLength int

		// DeepSeek特有的思考过程检测
		chromedp.Run(ctx, chromedp.Evaluate(`
			(function() {
				var replySelectors = [
					'.chat-message:last-child',
					'.message-item:last-child',
					'[class*="message-content"]:last-child',
					'[class*="assistant"]:last-child',
					'[class*="markdown"]:last-child',
					'.message:last-child',
					'[class*="ds-markdown"]:last-child'
				];

				// DeepSeek特有的思考过程模式
				var thinkingPatterns = [
					/思考过程/, /深度思考/, /分析中/, /思考结束/, /思考中/, /跳过思考/,
					/Formulate the Strategy/, /Identify the Core Task/, /Gather Information/,
					/Evaluate Alternatives/, /Structure the Response/,
					/Analyze the Input/, /Analyze the Question/, /Analyze the Request/,
					/好的，/, /让我/, /我来/, /首先/, /我需要/
				];

				function isThinkingOnly(text) {
					if (!text || text.length < 20) return false;
					var nonThinkingText = text;
					for (var i = 0; i < thinkingPatterns.length; i++) {
						nonThinkingText = nonThinkingText.replace(thinkingPatterns[i], '');
					}
					return nonThinkingText.trim().length < text.length * 0.3;
				}

				for (var i = 0; i < replySelectors.length; i++) {
					var elem = document.querySelector(replySelectors[i]);
					if (elem && elem.innerText && elem.innerText.length > 20) {
						var text = elem.innerText;
						if (isThinkingOnly(text)) {
							return JSON.stringify({length: text.length, hasNonThinkingReply: false, thinkingLength: text.length});
						}
						return JSON.stringify({length: text.length, hasNonThinkingReply: true, thinkingLength: 0});
					}
				}

				var chatArea = document.querySelector('.chat-container, .chat-main, [class*="chat"], [class*="conversation"], main');
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

		// 核心策略：只有当非思考内容出现并稳定时，才判定回复完成
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

	// 6. 获取AI回复（提取思考过程和正式回复）
	log.Println("获取AI回复...")

	answer, thinking := d.extractAIReply(ctx)

	log.Printf("获取到答案长度: %d, 思考过程长度: %d", len(answer), len(thinking))

	// 如果答案仍然包含思考过程，将思考内容移到thinking字段
	if strings.Contains(answer, "Formulate the Strategy") || strings.Contains(answer, "跳过思考") ||
		strings.Contains(answer, "Refining the") || strings.Contains(answer, "Identify the Core") ||
		strings.Contains(answer, "Gather Information") || strings.Contains(answer, "Evaluate Alternatives") ||
		strings.Contains(answer, "Structure the Response") {
		log.Println("检测到答案包含思考过程，分离思考内容...")
		markers := []string{"Answer:", "回答：", "正式回复："}
		for _, marker := range markers {
			if idx := strings.Index(answer, marker); idx > 0 {
				if thinking == "" {
					thinking = answer[:idx]
				}
				answer = answer[idx+len(marker):]
				break
			}
		}
		log.Printf("分离后答案长度: %d, 思考过程长度: %d", len(answer), len(thinking))
	}

	// 检查是否被检测为机器人
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
func (d *DeepSeekClient) AskInConversation(question string) (*platform.AskResult, error) {
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
		chromedp.WaitVisible("textarea", chromedp.ByQuery),
	)
	if err != nil {
		return nil, fmt.Errorf("当前页面未找到输入框: %v", err)
	}

	// 输入问题
	log.Println("AskInConversation: 开始输入问题...")
	questionJSON, err := json.Marshal(question)
	if err != nil {
		log.Printf("JSON编码问题失败: %v", err)
		questionJSON = []byte(fmt.Sprintf(`"%s"`, question))
	}

	var jsInputResult int
	err = chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
		(function() {
			var text = %s;
			var textarea = document.querySelector('textarea, #chat-input');
			if (textarea) {
				textarea.focus();
				textarea.select();
				document.execCommand('insertText', false, text);
				return textarea.value.length;
			}
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

	if err != nil || jsInputResult == 0 {
		log.Printf("insertText设置输入框失败: %v，回退到SendKeys...", err)
		err = chromedp.Run(ctx,
			chromedp.SendKeys("textarea", question, chromedp.ByQuery),
		)
		if err != nil {
			return nil, fmt.Errorf("输入内容失败: %v", err)
		}
	}

	log.Printf("AskInConversation: 问题输入完成，内容长度: %d", jsInputResult)
	chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))

	// 发送问题
	log.Println("AskInConversation: 发送问题...")
	d.sendQuestion(ctx)

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
		chromedp.Run(ctx, chromedp.WaitVisible("textarea", chromedp.ByQuery))
		chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
		questionJSON, _ := json.Marshal(question)
		var reInputResult int
		chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
			(function() {
				var text = %s;
				var textarea = document.querySelector('textarea, #chat-input');
				if (textarea) {
					textarea.focus();
					textarea.select();
					document.execCommand('insertText', false, text);
					return textarea.value.length;
				}
				return 0;
			})()
		`, string(questionJSON)), &reInputResult))
		chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))
		d.sendQuestion(ctx)
		chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
	}

	// 等待AI回复完成
	answer, thinking := d.waitForAIResponse(ctx)

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
func (d *DeepSeekClient) StartNewConversation(initialMessage string) (*platform.AskResult, error) {
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
		chromedp.Navigate(deepseekChatURL),
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
			var btns = document.querySelectorAll('a, button, div[role="button"]');
			for (var i = 0; i < btns.length; i++) {
				var text = btns[i].innerText || btns[i].textContent || '';
				if (text.includes('新建') || text.includes('新对话') || text.includes('New Chat') || text.includes('New chat')) {
					btns[i].click();
					return 'clicked: ' + text.trim();
				}
			}
			var newChatBtn = document.querySelector('[class*="new-chat"], a[href="/"]');
			if (newChatBtn) {
				newChatBtn.click();
				return 'clicked: selector';
			}
			return 'no button';
		})()
	`, &newChatResult))
	log.Printf("新建对话按钮结果: %s", newChatResult)

	// 如果没有找到新建对话按钮，直接导航到首页
	if newChatResult == "no button" || newChatResult == "" {
		log.Println("未找到新建对话按钮，导航到首页...")
		chromedp.Run(ctx, chromedp.Navigate(deepseekChatURL))
		chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
	} else {
		chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
	}

	// 等待新对话页面加载完成
	log.Println("StartNewConversation: 等待新对话页面加载...")
	err := chromedp.Run(ctx,
		chromedp.WaitVisible("textarea", chromedp.ByQuery),
	)
	if err != nil {
		return nil, fmt.Errorf("新对话页面加载失败，未找到输入框: %v", err)
	}
	chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	// 输入初始消息
	log.Println("StartNewConversation: 输入初始消息...")
	messageJSON, err := json.Marshal(initialMessage)
	if err != nil {
		log.Printf("JSON编码消息失败: %v", err)
		messageJSON = []byte(fmt.Sprintf(`"%s"`, initialMessage))
	}

	var jsInputResult int
	err = chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
		(function() {
			var text = %s;
			var textarea = document.querySelector('textarea, #chat-input');
			if (textarea) {
				textarea.focus();
				textarea.select();
				document.execCommand('insertText', false, text);
				return textarea.value.length;
			}
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
	`, string(messageJSON)), &jsInputResult))

	if err != nil || jsInputResult == 0 {
		log.Printf("insertText设置输入框失败: %v，回退到SendKeys...", err)
		err = chromedp.Run(ctx,
			chromedp.SendKeys("textarea", initialMessage, chromedp.ByQuery),
		)
		if err != nil {
			return nil, fmt.Errorf("输入初始消息失败: %v", err)
		}
	}

	log.Printf("StartNewConversation: 初始消息输入完成，内容长度: %d", jsInputResult)
	chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))

	// 发送消息
	log.Println("StartNewConversation: 发送初始消息...")
	d.sendQuestion(ctx)

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
		chromedp.Run(ctx, chromedp.WaitVisible("textarea", chromedp.ByQuery))
		chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
		messageJSON, _ := json.Marshal(initialMessage)
		var reInputResult int
		chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
			(function() {
				var text = %s;
				var textarea = document.querySelector('textarea, #chat-input');
				if (textarea) {
					textarea.focus();
					textarea.select();
					document.execCommand('insertText', false, text);
					return textarea.value.length;
				}
				return 0;
			})()
		`, string(messageJSON)), &reInputResult))
		chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))
		d.sendQuestion(ctx)
		chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
	}

	// 等待AI回复完成
	answer, thinking := d.waitForAIResponse(ctx)

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

// sendQuestion 发送当前输入框中的问题
// 尝试多种方式点击发送按钮，最终回退到Enter键发送
func (d *DeepSeekClient) sendQuestion(ctx context.Context) {
	// 尝试点击发送按钮
	sendSelectors := []string{
		"button[class*='send']",
		"button[class*='Send']",
		"[aria-label='发送']",
		"[aria-label='Send']",
		"button[type='submit']",
		"[data-testid='send-button']",
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
		}
	}

	if !sendClicked {
		log.Println("未找到发送按钮，尝试使用JS点击...")
		var jsClicked bool
		chromedp.Run(ctx, chromedp.Evaluate(`
			(function() {
				var selectors = [
					'button[class*="send"]',
					'button[class*="Send"]',
					'.send-btn',
					'.send-button',
					'[data-testid="send-button"]',
					'form button[type="submit"]',
					'.chat-input button',
					'.input-area button'
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

	if !sendClicked {
		log.Println("使用 Enter 键发送...")
		chromedp.Run(ctx, chromedp.Focus("textarea", chromedp.ByQuery))
		chromedp.Run(ctx, chromedp.Sleep(200*time.Millisecond))
		err := chromedp.Run(ctx,
			chromedp.SendKeys("textarea", "\r", chromedp.ByQuery),
		)
		if err != nil {
			log.Printf("Enter键发送失败: %v，尝试JavaScript触发...", err)
			chromedp.Run(ctx, chromedp.Evaluate(`
				(function() {
					var textarea = document.querySelector('textarea, #chat-input');
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

// waitForAIResponse 等待AI回复完成
// 区分思考过程和正式回复，当非思考内容稳定后返回
func (d *DeepSeekClient) waitForAIResponse(ctx context.Context) (string, string) {
	maxWaitTime := 90 * time.Second
	checkInterval := 2 * time.Second
	startTime := time.Now()

	var lastContentLength int
	stableCount := 0
	var resultStr string

	for time.Since(startTime) < maxWaitTime {
		chromedp.Run(ctx, chromedp.Sleep(checkInterval))

		// 检查页面异常状态
		loopAnomalyType, _, _ := d.detectPageAnomaly(ctx)
		if loopAnomalyType == "input_error" {
			log.Println("等待过程中检测到输入超限提示")
			return "", ""
		}
		if loopAnomalyType == "captcha" {
			log.Println("等待过程中检测到人机验证")
			passed, _ := d.handleCaptcha(ctx)
			if passed {
				// 验证通过后重置等待状态
				lastContentLength = 0
				stableCount = 0
				startTime = time.Now()
				continue
			}
			return "", ""
		}

		var currentContentLength int
		var hasNonThinkingReply bool
		var thinkingLength int

		chromedp.Run(ctx, chromedp.Evaluate(`
			(function() {
				var replySelectors = [
					'.chat-message:last-child',
					'.message-item:last-child',
					'[class*="message-content"]:last-child',
					'[class*="assistant"]:last-child',
					'[class*="markdown"]:last-child',
					'.message:last-child',
					'[class*="ds-markdown"]:last-child'
				];

				var thinkingPatterns = [
					/思考过程/, /深度思考/, /分析中/, /思考结束/, /思考中/, /跳过思考/,
					/Formulate the Strategy/, /Identify the Core Task/, /Gather Information/,
					/Evaluate Alternatives/, /Structure the Response/,
					/Analyze the Input/, /Analyze the Question/, /Analyze the Request/,
					/好的，/, /让我/, /我来/, /首先/, /我需要/
				];

				function isThinkingOnly(text) {
					if (!text || text.length < 20) return false;
					var nonThinkingText = text;
					for (var i = 0; i < thinkingPatterns.length; i++) {
						nonThinkingText = nonThinkingText.replace(thinkingPatterns[i], '');
					}
					return nonThinkingText.trim().length < text.length * 0.3;
				}

				for (var i = 0; i < replySelectors.length; i++) {
					var elem = document.querySelector(replySelectors[i]);
					if (elem && elem.innerText && elem.innerText.length > 20) {
						var text = elem.innerText;
						if (isThinkingOnly(text)) {
							return JSON.stringify({length: text.length, hasNonThinkingReply: false, thinkingLength: text.length});
						}
						return JSON.stringify({length: text.length, hasNonThinkingReply: true, thinkingLength: 0});
					}
				}

				var chatArea = document.querySelector('.chat-container, .chat-main, [class*="chat"], [class*="conversation"], main');
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

	// 提取回复内容
	return d.extractAIReply(ctx)
}

// extractAIReply 从页面提取AI回复，分离思考过程和正式回复
func (d *DeepSeekClient) extractAIReply(ctx context.Context) (string, string) {
	var jsResult string
	var thinking string
	var answer string
	chromedp.Run(ctx, chromedp.Evaluate(`
		(function() {
			// DeepSeek页面需要排除的无关文本关键词
			var excludeKeywords = ['主题模式', '学习模式', '深度思考', '联网搜索',
			                       '帮助与反馈', '关于我们', '退出登录',
			                       'DeepSeek', 'Beijing',
			                       '新对话', '登录', '注册', '手机号', '验证码', '人机验证', 'captcha',
			                       'How can I help you', 'Feel free to type', 'I can help with',
			                       '用户协议', '隐私政策'];

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
					/Formulate the Strategy/, /Identify the Core Task/, /Gather Information/,
					/Evaluate Alternatives/, /Structure the Response/, /Refining/,
					/Analyze the Input/, /Analyze the Question/, /Analyze the Request/,
					/Option [0-9]+ in/, /1\.\s*Identify/, /2\.\s*Gather/, /3\.\s*Formulate/,
					/4\.\s*Answer/, /5\.\s*Answer/,
					/Formal Output/, /正式输出/, /Final Output/,
					/Okay, /, /Alright, /, /Let me/, /Let's/,
					/I'll /, /I should/, /I need to/, /I will/,
					/好的，/, /让我/, /我来/, /首先/, /我需要/
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
				'.assistant-message',
				'[class*="assistant"]',
				'.ai-reply',
				'[class*="ai-reply"]',
				'.message-content',
				'[class*="message-content"]',
				'.chat-message:last-child',
				'.message-item:last-child',
				'[class*="thinking"]',
				'[class*="think"]',
				'[class*="reasoning"]',
				'[class*="markdown"]',
				'[class*="ds-markdown"]',
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

			// 策略3：从完整文本中分割思考过程和正式回复
			if (!answerText || answerText.length < 50) {
				var bodyText = document.body.innerText || '';
				var lines = bodyText.split('\n');
				var thinkingLines = [];
				var answerLines = [];
				var inThinking = false;
				var inAnswer = false;

				for (var k = 0; k < lines.length; k++) {
					var line = lines[k].trim();
					if (line.length < 5 || shouldExclude(line)) continue;

					if (isThinkingContent(line)) {
						inThinking = true;
						inAnswer = false;
						thinkingLines.push(line);
						continue;
					}

					var transitionMarkers = ['Answer:', '回答：', '正式回复：', '正式输出：', 'Final Output:', 'Formal Output:'];
					var isTransition = false;
					for (var m = 0; m < transitionMarkers.length; m++) {
						if (line.includes(transitionMarkers[m])) {
							isTransition = true;
							inThinking = false;
							inAnswer = true;
							var afterMarker = line.substring(line.indexOf(transitionMarkers[m]) + transitionMarkers[m].length).trim();
							if (afterMarker.length > 5) answerLines.push(afterMarker);
							break;
						}
					}
					if (isTransition) continue;

					if (inThinking) {
						thinkingLines.push(line);
					} else if (inAnswer || (!inThinking && !isThinkingContent(line) && line.length > 10)) {
						inAnswer = true;
						answerLines.push(line);
					}
				}

				if (!thinkingText && thinkingLines.length > 0) thinkingText = thinkingLines.join('\n');
				if (!answerText && answerLines.length > 0) answerText = answerLines.join('\n');
			}

			// 策略4：如果答案主要是思考过程，将整个答案移到thinking字段
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
	if err := json.Unmarshal([]byte(jsResult), &parsedResult); err == nil {
		thinking = parsedResult.Thinking
		answer = parsedResult.Answer
	}

	return answer, thinking
}

// detectPageAnomaly 检测页面异常状态
// 返回: anomalyType("input_error"/"captcha"/"none"), keyword, context
func (d *DeepSeekClient) detectPageAnomaly(ctx context.Context) (string, string, string) {
	var resultStr string
	chromedp.Run(ctx, chromedp.Evaluate(`
		(function() {
			var errorKeywords = [
				'最多可以输入', '字数超', '超出限制', '内容过长', '输入超限',
				'字数限制', '超过最大', '超出字数', '最多输入', '字数已达上限',
				'limit', 'too long', 'exceeds', 'maximum'
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
func (d *DeepSeekClient) handleSliderCaptcha(ctx context.Context) bool {
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
func (d *DeepSeekClient) generateHumanTrajectory(totalDistance float64) []struct {
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
func (d *DeepSeekClient) executeSliderDrag(ctx context.Context, startX, startY float64, trajectory []struct {
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

	log.Printf("滑块拖拽完成: 起点(%.0f,%.0f) → 终点(%.0f,%.0f)", startX, startY, currentX, currentY)
	return true
}

// handleCaptcha 尝试自动处理人机验证页面
// 策略：1.尝试点击验证按钮/复选框 2.等待用户手动完成 3.验证通过后刷新页面继续
// 返回: 是否成功通过验证, 错误信息
func (d *DeepSeekClient) handleCaptcha(ctx context.Context) (bool, string) {
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
			if !strings.Contains(currentURL, "deepseek.com") {
				log.Println("验证后不在聊天页面，导航回来...")
				chromedp.Run(ctx,
					chromedp.Navigate(deepseekChatURL),
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
func (d *DeepSeekClient) GetCookies() (string, error) {
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
func (d *DeepSeekClient) getCookiesFromDocument() (string, error) {
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
