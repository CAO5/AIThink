package browser

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// 截图保存目录
const screenshotDir = "screenshots"

const (
	// 智谱清言网站URL
	zhipuLoginURL = "https://chatglm.cn/"
	zhipuChatURL  = "https://chatglm.cn/main/" // 修改：使用主页而不是alltoolsdetail
)

// ZhipuClient 智谱清言客户端
type ZhipuClient struct {
	session *BrowserSession
}

// NewZhipuClient 创建智谱清言客户端
func NewZhipuClient(session *BrowserSession) *ZhipuClient {
	return &ZhipuClient{session: session}
}

// GetCookies 获取当前页面的cookies（通过JavaScript）
func (z *ZhipuClient) GetCookies() (string, error) {
	var cookieStr string
	err := chromedp.Run(z.session.Ctx,
		chromedp.Evaluate(`document.cookie`, &cookieStr),
	)
	if err != nil {
		return "", err
	}
	log.Printf("获取到Cookies: %s", cookieStr)
	return cookieStr, nil
}

// OpenLoginPage 打开登录页面（供用户手动登录）
// 如果有保存的cookies，会自动加载以保持登录状态
func (z *ZhipuClient) OpenLoginPage() error {
	ctx := z.session.Ctx

	log.Println("正在打开智谱清言网站...")
	// 增加超时，避免卡住
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := chromedp.Run(timeoutCtx,
		chromedp.Navigate(zhipuLoginURL),
		chromedp.Sleep(3*time.Second),
	); err != nil {
		return fmt.Errorf("导航失败: %v", err)
	}

	// 注入反检测脚本
	log.Println("注入反检测脚本...")
	if err := z.session.InjectAntiDetection(); err != nil {
		log.Printf("注入反检测脚本失败: %v", err)
		// 不返回错误，继续执行
	}

	// 检查导航后的URL
	var currentURL string
	chromedp.Run(ctx, chromedp.Location(&currentURL))
	log.Printf("导航完成，当前URL: %s", currentURL)

	// 检查是否已经登录（cookie有效）
	if z.CheckLoggedIn() {
		log.Println("检测到已登录状态（cookie有效），无需重新登录")
		return nil
	}

	log.Println("页面已打开，请手动完成登录...")
	log.Println("========================================")
	log.Println("请在浏览器中手动完成以下步骤：")
	log.Println("1. 点击【登录】按钮")
	log.Println("2. 输入手机号")
	log.Println("3. 点击【获取验证码】")
	log.Println("4. 查看手机短信，输入验证码")
	log.Println("5. 点击【登录】按钮")
	log.Println("========================================")

	return nil
}

// CheckLoggedIn 检查是否已登录（通过页面内容判断 - 优化版）
func (z *ZhipuClient) CheckLoggedIn() bool {
	ctx := z.session.Ctx
	log.Println("CheckLoggedIn: 开始检查登录状态")

	// 获取页面文本（增加超时）
	var pageText string
	log.Println("CheckLoggedIn: 准备获取页面文本")
	err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			// 设置超时
			timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			return chromedp.Text("body", &pageText).Do(timeoutCtx)
		}),
	)
	log.Printf("CheckLoggedIn: 获取页面文本完成, err=%v, text长度=%d", err, len(pageText))
	if err != nil {
		log.Printf("获取页面文本失败: %v", err)
		return false
	}

	// 检查是否是访客状态（未登录）
	if strings.Contains(pageText, "访客") || strings.Contains(pageText, "登录") && strings.Contains(pageText, "注册") {
		log.Println("检测到访客状态或未登录状态")
		return false
	}

	// 使用JavaScript更精确地检查登录状态
	var isLoggedIn bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				// 检查是否有用户头像或用户菜单（已登录特征）
				var userMenu = document.querySelector('[class*="user"], [class*="avatar"], [class*="profile"]');
				if (userMenu) return true;
				
				// 检查是否有"新建对话"按钮（已登录特征）
				var newChatBtn = document.querySelector('[class*="new"], [class*="create"]');
				if (newChatBtn && newChatBtn.innerText && newChatBtn.innerText.includes('新建')) return true;
				
				// 检查是否有聊天输入框
				var input = document.querySelector('textarea, [contenteditable="true"]');
				if (input) return true;
				
				// 检查页面是否包含"退出登录"等已登录才有的元素
				if (document.body.innerText.includes('退出登录') || document.body.innerText.includes('个人中心')) return true;
				
				return false;
			})();
		`, &isLoggedIn),
	)
	
	if err == nil && isLoggedIn {
		log.Println("检测到已登录状态（JavaScript检查通过）")
		return true
	}

	// 检查URL是否包含对话ID（cid参数，说明已经在聊天页面）
	var currentURL string
	chromedp.Run(ctx, chromedp.Location(&currentURL))
	if strings.Contains(currentURL, "cid=") {
		log.Printf("检测到已登录状态（URL包含对话ID: %s）", currentURL)
		return true
	}

	log.Println("未检测到登录状态")
	return false
}

// AskResult 提问结果，包含答案和检测信息
type AskResult struct {
	Answer     string   // 完整答案
	IsBot      bool     // 是否被检测为机器人
	DetectInfo string   // 检测信息
	StreamChan <-chan string // 流式返回通道（可选）
}

// Ask 向智谱清言提问（智能获取AI回复）
// 返回完整答案、是否被检测为机器人、检测信息
func (z *ZhipuClient) Ask(question string) (*AskResult, error) {
	ctx := z.session.Ctx

	// 用于存储流式内容的通道
	streamChan := make(chan string, 100)
	
	// 结果
	result := &AskResult{
		StreamChan: streamChan,
	}

	// 0. 注入反检测脚本
	log.Println("注入反检测脚本...")
	z.session.InjectAntiDetection()
	chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	// 1. 导航到聊天页面
	log.Println("导航到聊天页面...")
	chromedp.Run(ctx,
		chromedp.Navigate(zhipuChatURL),
		chromedp.Sleep(3*time.Second),
	)
	
	// 注入反检测
	z.session.InjectAntiDetection()
	chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
	
	// 检查当前URL
	var currentURL string
	chromedp.Run(ctx, chromedp.Location(&currentURL))
	log.Printf("当前URL: %s", currentURL)
	
	// 2. 等待输入框出现
	log.Println("等待输入框...")
	inputSelectors := []string{
		`textarea`,
		`[contenteditable="true"]`,
		`.input-area textarea`,
		`form textarea`,
	}

	var inputSelector string
	for _, selector := range inputSelectors {
		err := chromedp.Run(ctx,
			chromedp.WaitVisible(selector, chromedp.ByQuery),
			chromedp.Sleep(500*time.Millisecond),
		)
		if err == nil {
			inputSelector = selector
			log.Printf("找到输入框: %s", selector)
			break
		}
	}

	if inputSelector == "" {
		return nil, fmt.Errorf("未能找到输入框")
	}

	// 3. 模拟真人输入
	log.Printf("输入问题: %s...", question[:min(50, len(question))])
	chromedp.Run(ctx,
		chromedp.Click(inputSelector, chromedp.ByQuery),
		chromedp.Sleep(300*time.Millisecond),
	)

	// 模拟真人逐字输入
	for i := 0; i < len(question); i++ {
		char := question[i : i+1]
		chromedp.Run(ctx,
			chromedp.SendKeys(inputSelector, char, chromedp.ByQuery),
		)
		delay := time.Duration(50+rand.Intn(100)) * time.Millisecond
		time.Sleep(delay)
	}
	log.Println("问题输入完成")

	// 4. 发送问题
	chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
	log.Println("发送问题（使用 Enter 键）...")
	chromedp.Run(ctx,
		chromedp.SendKeys(inputSelector, "\n", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	)

	// 5. 等待AI回复完成（使用智能检测）
	log.Println("等待AI回复...")
	
	// 人机检测关键词
	botDetectKeywords := []string{
		"人机验证", "机器识别", "自动检测", "异常请求",
		"请完成验证", "captcha", "robot", "automation",
		"检测到", "非正常", "自动化工具",
	}

	detectedAsBot := false
	detectInfo := ""
	
	// 等待回复完成，通过检测页面内容变化
	maxWaitTime := 30 * time.Second
	checkInterval := 2 * time.Second
	startTime := time.Now()
	
	var lastContent string
	stableCount := 0
	
	for time.Since(startTime) < maxWaitTime {
		chromedp.Run(ctx, chromedp.Sleep(checkInterval))
		
		// 获取当前页面内容
		var currentContent string
		chromedp.Run(ctx, chromedp.Evaluate(`
			(function() {
				// 尝试获取聊天内容区域
				var chatArea = document.querySelector('.chat-container, .chat-main, [class*="chat"], .conversation, [class*="conversation"]');
				if (chatArea) {
					return chatArea.innerText;
				}
				// 备用：获取body内容
				return document.body.innerText;
			})()
		`, &currentContent))
		
		// 如果内容没有变化，认为回复已完成
		if currentContent == lastContent {
			stableCount++
			if stableCount >= 2 {
				log.Printf("内容已稳定，回复可能已完成（稳定次数: %d）", stableCount)
				break
			}
		} else {
			stableCount = 0
			lastContent = currentContent
			log.Printf("内容变化中...（长度: %d）", len(currentContent))
		}
	}

	// 6. 获取AI回复（使用JavaScript智能提取）
	log.Println("获取AI回复...")
	
	var answer string
	chromedp.Run(ctx, chromedp.Evaluate(`
		(function() {
			// 排除的关键词
			var excludeKeywords = ['主题模式', '学习模式', '云知识库', '实名认证', '我的订单', 
			                         '帮助与反馈', '注销账号', '关于我们', '退出登录', 
			                         '积分', '网信算备', 'ICP备', 'ChatGLM', 'Beijing',
			                         'GLM-5', '最新旗舰', '扫描二维码', '体验智能体', '保存名片', '分享智能体',
			                         '新对话', '登录', '注册', '手机号', '验证码', '人机验证', 'captcha',
			                         'How can I help you', 'Feel free to type', 'I can help with',
			                         '勾选即代表您阅读并同意', '用户协议', '隐私政策'];
			
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
			
			// 策略1：尝试找到AI回复的特定元素
			var replySelectors = [
				'.assistant-message',
				'[class*="assistant"]',
				'.ai-reply',
				'[class*="ai-reply"]',
				'.message-content',
				'[class*="message-content"]',
				'.chat-message:last-child',
				'.message-item:last-child'
			];
			
			for (var i = 0; i < replySelectors.length; i++) {
				var elem = document.querySelector(replySelectors[i]);
				if (elem) {
					var text = elem.innerText.trim();
					if (text.length > 50 && !shouldExclude(text)) {
						return text;
					}
				}
			}
			
			// 策略2：获取所有段落，找最长的有效文本
			var paragraphs = document.querySelectorAll('p, div, span, article');
			var bestText = '';
			
			for (var j = 0; j < paragraphs.length; j++) {
				var p = paragraphs[j];
				if (p.offsetParent === null) continue; // 不可见元素
				
				var text = p.innerText.trim();
				if (text.length > bestText.length && text.length > 100 && text.length < 5000 && !shouldExclude(text)) {
					bestText = text;
				}
			}
			
			if (bestText) {
				return bestText;
			}
			
			// 策略3：返回页面主要内容（排除头部和尾部）
			var bodyText = document.body.innerText || '';
			var lines = bodyText.split('\n');
			var contentLines = [];
			
			for (var k = 0; k < lines.length; k++) {
				var line = lines[k].trim();
				if (line.length > 50 && !shouldExclude(line)) {
					contentLines.push(line);
				}
			}
			
			if (contentLines.length > 0) {
				return contentLines.join('\n');
			}
			
			return bodyText.substring(0, 2000);
		})()
	`, &answer))
	
	log.Printf("获取到答案长度: %d", len(answer))
	
	// 检查是否被检测为机器人
	for _, keyword := range botDetectKeywords {
		if strings.Contains(strings.ToLower(answer), strings.ToLower(keyword)) {
			detectedAsBot = true
			detectInfo = fmt.Sprintf("检测到关键词: %s", keyword)
			log.Printf("⚠️ 警告：可能被检测为机器人！关键词: %s", keyword)
			break
		}
	}
	
	close(streamChan)
	result.Answer = answer
	result.IsBot = detectedAsBot
	result.DetectInfo = detectInfo
	
	return result, nil
}

// min 返回两个整数中的较小值
// Go 1.21+ 内置了 min 函数，直接使用
