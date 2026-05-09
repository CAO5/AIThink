package browser

import (
	"encoding/json"
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

// NavigateToHome 导航到智谱首页（用于加载cookies后使cookies生效）
func (z *ZhipuClient) NavigateToHome() error {
	ctx := z.session.Ctx

	log.Printf("正在导航到智谱首页: %s", zhipuChatURL)

	// 导航到主页
	if err := chromedp.Run(ctx,
		chromedp.Navigate(zhipuChatURL),
	); err != nil {
		log.Printf("导航失败: %v", err)
		return fmt.Errorf("导航失败: %v", err)
	}

	// 等待页面加载
	chromedp.Run(ctx, chromedp.Sleep(3*time.Second))

	// 注入反检测脚本
	log.Println("注入反检测脚本...")
	if err := z.session.InjectAntiDetection(); err != nil {
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
	log.Printf("Session Context: %v", ctx)

	// 直接使用 session 的 ctx，不使用额外的超时包装
	log.Printf("开始导航到: %s", zhipuLoginURL)
	if err := chromedp.Run(ctx,
		chromedp.Navigate(zhipuLoginURL),
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
	if err := z.session.InjectAntiDetection(); err != nil {
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
	z.session.InjectAntiDetection()

	// 等待页面稳定
	chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

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

	// 获取页面文本 - 直接使用 session ctx，让 chromedp 内部处理超时
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

	// 5. 等待AI回复完成（优化：区分思考过程和正式回复）
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
	maxWaitTime := 60 * time.Second  // 增加到60秒，给AI更多时间
	checkInterval := 2 * time.Second
	startTime := time.Now()
	
	var lastContentLength int
	stableCount := 0
	var resultStr string // 用于存储JS返回的JSON字符串
	
	for time.Since(startTime) < maxWaitTime {
		chromedp.Run(ctx, chromedp.Sleep(checkInterval))
		
		// 获取当前页面内容长度
		var currentContentLength int
		var hasReplyElement bool
		
		chromedp.Run(ctx, chromedp.Evaluate(`
			(function() {
				// 检查是否有正式回复元素（排除思考过程）
				var replySelectors = [
					'.chat-message:last-child',
					'.message-item:last-child',
					'[class*="message-content"]:last-child'
				];
				
				for (var i = 0; i < replySelectors.length; i++) {
					var elem = document.querySelector(replySelectors[i]);
					if (elem && elem.innerText && elem.innerText.length > 20) {
						// 检查是否包含思考过程标记
						var text = elem.innerText;
						if (text.includes('思考过程') || text.includes('Formulate') || 
						    text.includes('Analyze') || text.includes('思考中')) {
							return JSON.stringify({length: text.length, hasReply: false});
						}
						return JSON.stringify({length: text.length, hasReply: true});
					}
				}
				
				// 备用：获取整个聊天区域长度
				var chatArea = document.querySelector('.chat-container, .chat-main, [class*="chat"]');
				if (chatArea) {
					return JSON.stringify({length: chatArea.innerText.length, hasReply: false});
				}
				return JSON.stringify({length: 0, hasReply: false});
			})()
		`, &resultStr))
		
		// 解析JSON结果
		var result struct {
			Length   int  `json:"length"`
			HasReply bool `json:"hasReply"`
		}
		if err := json.Unmarshal([]byte(resultStr), &result); err == nil {
			currentContentLength = result.Length
			hasReplyElement = result.HasReply
		}
		
		// 检查是否检测到思考过程
		if !hasReplyElement && currentContentLength > 100 {
			log.Printf("检测到思考过程...（内容长度: %d）", currentContentLength)
		}
		
		// 如果内容长度稳定（变化小于10%），认为回复已完成
		if lastContentLength > 0 {
			change := absInt(currentContentLength-lastContentLength)
			if float64(change)/float64(lastContentLength) < 0.1 {
				stableCount++
				if stableCount >= 3 { // 需要3次稳定，避免过早结束
					log.Printf("内容已稳定，回复可能已完成（稳定次数: %d，长度: %d）", stableCount, currentContentLength)
					break
				}
			} else {
				stableCount = 0
				log.Printf("内容变化中...（长度: %d，变化: %d）", currentContentLength, change)
			}
		}
		lastContentLength = currentContentLength
	}

	// 6. 获取AI回复（优化：排除思考过程）
	log.Println("获取AI回复...")
	
	var answer string
	chromedp.Run(ctx, chromedp.Evaluate(`
		(function() {
			// 排除的关键词（包含思考过程标记）
			var excludeKeywords = ['主题模式', '学习模式', '云知识库', '实名认证', '我的订单', 
			                         '帮助与反馈', '注销账号', '关于我们', '退出登录', 
			                         '积分', '网信算备', 'ICP备', 'ChatGLM', 'Beijing',
			                         'GLM-5', '最新旗舰', '扫描二维码', '体验智能体', '保存名片', '分享智能体',
			                         '新对话', '登录', '注册', '手机号', '验证码', '人机验证', 'captcha',
			                         'How can I help you', 'Feel free to type', 'I can help with',
			                         '勾选即代表您阅读并同意', '用户协议', '隐私政策',
			                         'Formulate the Strategy', 'Analyze the Input', 'Identify the Missing',
			                         '跳过思考', '思考过程'];
			
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
				// 检查是否是思考过程
				var thinkMarkers = ['Formulate', 'Analyze', 'Identify', 'Strategy', 'Input', 'Missing Information', '跳过思考'];
				for (var i = 0; i < thinkMarkers.length; i++) {
					if (text.includes(thinkMarkers[i])) {
						return true;
					}
				}
				return false;
			}
			
			// 策略1：尝试找到AI回复的特定元素（优先找非思考内容）
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
			
			// 先找正式回复
			for (var i = 0; i < replySelectors.length; i++) {
				var elem = document.querySelector(replySelectors[i]);
				if (elem) {
					var text = elem.innerText.trim();
					if (text.length > 50 && !isThinkingContent(text) && !shouldExclude(text)) {
						return text;
					}
				}
			}
			
			// 策略2：如果找到的是思考过程，尝试获取其他消息
			for (var i = 0; i < replySelectors.length; i++) {
				var elems = document.querySelectorAll(replySelectors[i]);
				for (var j = 0; j < elems.length; j++) {
					var text = elems[j].innerText.trim();
					if (text.length > 50 && !isThinkingContent(text) && !shouldExclude(text)) {
						return text;
					}
				}
			}
			
			// 策略3：获取所有段落，找最长的有效文本（排除思考过程）
			var paragraphs = document.querySelectorAll('p, div, article');
			var bestText = '';
			
			for (var j = 0; j < paragraphs.length; j++) {
				var p = paragraphs[j];
				if (p.offsetParent === null) continue;
				
				var text = p.innerText.trim();
				if (text.length > bestText.length && text.length > 100 && text.length < 5000 && 
				    !isThinkingContent(text) && !shouldExclude(text)) {
					bestText = text;
				}
			}
			
			if (bestText) {
				return bestText;
			}
			
			// 策略4：返回页面主要内容（排除头部和尾部，排除思考过程）
			var bodyText = document.body.innerText || '';
			var lines = bodyText.split('\n');
			var contentLines = [];
			var skipMode = false;
			
			for (var k = 0; k < lines.length; k++) {
				var line = lines[k].trim();
				
				// 跳过思考过程
				if (isThinkingContent(line)) {
					skipMode = true;
					continue;
				}
				if (skipMode && (line.includes('4.') || line.includes('5.') || line.includes('Answer:'))) {
					skipMode = false;
					continue;
				}
				if (skipMode) continue;
				
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
	
	// 如果答案仍然包含思考过程，进行二次清理
	if strings.Contains(answer, "Formulate the Strategy") || strings.Contains(answer, "跳过思考") {
		log.Println("检测到答案包含思考过程，进行清理...")
		// 找到正式回复的开始位置
		markers := []string{"Answer:", "回答：", "正式回复："}
		for _, marker := range markers {
			if idx := strings.Index(answer, marker); idx > 0 {
				answer = answer[idx+len(marker):]
				break
			}
		}
		// 如果仍然很长，截取前500字符
		if len(answer) > 500 {
			answer = answer[:500] + "..."
		}
		log.Printf("清理后答案长度: %d", len(answer))
	}
	
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

// absInt 返回整数的绝对值
func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
