package browser

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// PageAnalyzer 智能页面分析器
type PageAnalyzer struct {
	ctx           context.Context
	debug         bool
	screen        string        // 截图保存目录
	imageAnalyzer *ImageAnalyzer // AI图片分析器
	sessionID     string        // 会话ID（用于截图命名）
}

// NewPageAnalyzer 创建页面分析器
func NewPageAnalyzer(ctx context.Context, debug bool, screenshotDir string) *PageAnalyzer {
	pa := &PageAnalyzer{
		ctx:       ctx,
		debug:     debug,
		screen:    screenshotDir,
		sessionID: "",
	}
	pa.imageAnalyzer = NewImageAnalyzer()

	// 设置页面查询回调函数，使百度OCR等能通过JS查询页面元素
	pa.imageAnalyzer.SetPageQueryFunc(func(script string) (interface{}, error) {
		var result interface{}
		err := chromedp.Run(ctx,
			chromedp.Evaluate(script, &result),
		)
		return result, err
	})

	return pa
}

// SetSessionID 设置会话ID（用于截图命名）
func (pa *PageAnalyzer) SetSessionID(sessionID string) {
	pa.sessionID = sessionID
}

// AnalyzePage 完整分析当前页面
func (pa *PageAnalyzer) AnalyzePage() map[string]interface{} {
	info := make(map[string]interface{})

	// 获取基本信息
	var url, title, bodyText, html string
	chromedp.Run(pa.ctx,
		chromedp.Location(&url),
		chromedp.Title(&title),
		chromedp.Evaluate(`document.body.innerText.substring(0, 1000)`, &bodyText),
		chromedp.Evaluate(`document.documentElement.outerHTML`, &html),
	)

	info["url"] = url
	info["title"] = title
	info["text"] = bodyText

	if pa.debug {
		log.Println("========== 页面分析 ==========")
		log.Printf("URL: %s", url)
		log.Printf("标题: %s", title)
		if len(bodyText) > 200 {
			log.Printf("页面文本（前200字符）: %s", bodyText[:200])
		} else {
			log.Printf("页面文本: %s", bodyText)
		}
	}

	// 分析页面状态
	pageStatus := pa.AnalyzePageStatus(bodyText)
	info["status"] = pageStatus

	if pa.debug {
		log.Printf("页面状态: %s", pageStatus)
		log.Println("===============================")
	}

	// 保存页面源码到文件（用于调试）
	if pa.debug && html != "" {
		// 确保截图目录存在
		if pa.screen == "" {
			pa.screen = "screenshots"
		}
		os.MkdirAll(pa.screen, 0755)

		// 保存HTML（带时间戳和sessionID）
		filename := pa.buildFilename("page", "html")
		htmlFile := filepath.Join(pa.screen, filename)
		if err := os.WriteFile(htmlFile, []byte(html), 0644); err != nil {
			log.Printf("保存页面源码失败: %v", err)
		} else {
			log.Printf("页面源码已保存: %s", htmlFile)
		}
	}

	return info
}

// AnalyzePageStatus 分析页面状态
func (pa *PageAnalyzer) AnalyzePageStatus(pageText string) string {
	text := strings.ToLower(pageText)

	// 检测是否在登录页面
	if strings.Contains(text, "登录") || strings.Contains(text, "登 录") ||
		strings.Contains(text, "login") || strings.Contains(text, "sign in") {
		return "login_page"
	}

	// 检测是否需要验证码
	if strings.Contains(text, "验证码") || strings.Contains(text, "获取验证码") ||
		strings.Contains(text, "captcha") {
		return "need_verification"
	}

	// 检测是否登录成功（通常有"退出"、"个人中心"等）
	if strings.Contains(text, "退出") || strings.Contains(text, "个人中心") ||
		strings.Contains(text, "对话") || strings.Contains(text, "新建对话") {
		return "logged_in"
	}

	// 检测是否正在加载
	if strings.Contains(text, "加载") || strings.Contains(text, "loading") {
		return "loading"
	}

	return "unknown"
}

// FindPhoneInput 查找手机号输入框（先尝试常规方法，失败后使用AI视觉识别）
func (pa *PageAnalyzer) FindPhoneInput() string {
	var selector string
	script := `
	() => {
		const inputs = document.querySelectorAll('input');
		for (let inp of inputs) {
			const type = inp.type || '';
			const placeholder = (inp.placeholder || '').toLowerCase();
			const name = (inp.name || '').toLowerCase();
			const ariaLabel = (inp.getAttribute('aria-label') || '').toLowerCase();

			// 检查是否是手机号输入框
			if (type === 'tel' ||
			    placeholder.includes('手机') ||
			    placeholder.includes('phone') ||
			    placeholder.includes('手机号') ||
			    name.includes('phone') ||
			    name.includes('mobile') ||
			    ariaLabel.includes('手机') ||
			    ariaLabel.includes('phone')) {
				// 返回最佳选择器
				if (inp.id) return '#' + inp.id;
				if (inp.name) return 'input[name="' + inp.name + '"]';
				if (inp.className) return 'input.' + inp.className.split(' ').join('.');
				return 'input[type="tel"]';
			}
		}
		return '';
	}`

	chromedp.Run(pa.ctx, chromedp.Evaluate(script, &selector))

	if pa.debug && selector != "" {
		log.Printf("通过常规方法找到手机号输入框: %s", selector)
	}

	// 如果常规方法失败，使用AI视觉识别
	if selector == "" && pa.imageAnalyzer != nil {
		log.Println("常规方法未找到手机号输入框，尝试使用AI视觉识别...")

		// 先截图
		screenshotPath := pa.TakeScreenshot("") // 使用自动生成文件名
		if screenshotPath != "" {
			aiSelector, _, _, err := pa.imageAnalyzer.FindPhoneInputByAI(screenshotPath)
			if err == nil && aiSelector != "" {
				log.Printf("AI找到手机号输入框: %s", aiSelector)
				return aiSelector
			} else {
				log.Printf("AI视觉识别失败: %v", err)
			}
		}
	}

	return selector
}

// FindInputByPlaceholder 根据placeholder查找输入框
func (pa *PageAnalyzer) FindInputByPlaceholder(keywords []string) string {
	var selector string

	jsKeywords := "["
	for i, kw := range keywords {
		if i > 0 {
			jsKeywords += ","
		}
		jsKeywords += `"` + kw + `"`
	}
	jsKeywords += "]"

	script := `
	() => {
		const keywords = ` + jsKeywords + `;
		const inputs = document.querySelectorAll('input');

		for (let inp of inputs) {
			const placeholder = (inp.placeholder || '').toLowerCase();
			const ariaLabel = (inp.getAttribute('aria-label') || '').toLowerCase();

			for (let kw of keywords) {
				if (placeholder.includes(kw.toLowerCase()) ||
				    ariaLabel.includes(kw.toLowerCase())) {
					if (inp.id) return '#' + inp.id;
					if (inp.name) return 'input[name="' + inp.name + '"]';
					return '';
				}
			}
		}
		return '';
	}`

	chromedp.Run(pa.ctx, chromedp.Evaluate(script, &selector))
	return selector
}

// FindButtonByText 根据文本查找按钮（先尝试常规方法，失败后使用AI视觉识别）
func (pa *PageAnalyzer) FindButtonByText(texts []string) string {
	var selector string

	jsTexts := "["
	for i, text := range texts {
		if i > 0 {
			jsTexts += ","
		}
		jsTexts += `"` + text + `"`
	}
	jsTexts += "]"

	script := `
	() => {
		const searchTexts = ` + jsTexts + `;
		const elements = document.querySelectorAll('button, a, [role="button"]');

		for (let el of elements) {
			const elText = (el.textContent || '').trim();
			const ariaLabel = (el.getAttribute('aria-label') || '').trim();

			for (let searchText of searchTexts) {
				if (elText.includes(searchText) || ariaLabel.includes(searchText)) {
					if (el.id) return '#' + el.id;
					if (el.className) {
						return '.' + el.className.split(' ').filter(c => c).join('.');
					}
					return '';
				}
			}
		}
		return '';
	}`

	chromedp.Run(pa.ctx, chromedp.Evaluate(script, &selector))

	if pa.debug && selector != "" {
		log.Printf("通过常规方法找到按钮: %s", selector)
	}

	// 如果常规方法失败，使用AI视觉识别
	if selector == "" && pa.imageAnalyzer != nil {
		log.Println("常规方法未找到按钮，尝试使用AI视觉识别...")

		// 先截图
		screenshotPath := pa.TakeScreenshot("") // 使用自动生成文件名
		if screenshotPath != "" {
			aiSelector, _, _, err := pa.imageAnalyzer.FindButtonByAI(screenshotPath, texts)
			if err == nil && aiSelector != "" {
				log.Printf("AI找到按钮: %s", aiSelector)
				return aiSelector
			} else {
				log.Printf("AI视觉识别失败: %v", err)
			}
		}
	}

	return selector
}

// SmartInput 智能输入
func (pa *PageAnalyzer) SmartInput(selector, value string) bool {
	if selector == "" {
		log.Println("未找到输入框")
		return false
	}

	log.Printf("输入到 %s: %s", selector, value)
	err := chromedp.Run(pa.ctx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.Clear(selector, chromedp.ByQuery),
		chromedp.SendKeys(selector, value, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	)

	if err != nil {
		log.Printf("输入失败: %v", err)
		return false
	}

	log.Printf("输入成功")
	return true
}

// SmartClick 智能点击
func (pa *PageAnalyzer) SmartClick(selector string) bool {
	if selector == "" {
		log.Println("未找到可点击元素")
		return false
	}

	log.Printf("点击: %s", selector)
	err := chromedp.Run(pa.ctx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.Click(selector, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	)

	if err != nil {
		log.Printf("点击失败: %v", err)
		return false
	}

	log.Printf("点击成功")
	return true
}

// ClickByCoordinate 按坐标点击（用于AI识别到的动态元素）
func (pa *PageAnalyzer) ClickByCoordinate(x, y int) bool {
	log.Printf("按坐标点击: (%d, %d)", x, y)

	// 将截图坐标转换为页面坐标（考虑设备像素比）
	var devicePixelRatio float64
	chromedp.Run(pa.ctx, chromedp.Evaluate(`window.devicePixelRatio`, &devicePixelRatio))
	if devicePixelRatio == 0 {
		devicePixelRatio = 1
	}
	// AI返回的是截图坐标，截图通常是实际页面的 devicePixelRatio 倍
	// 但如果截图和页面尺寸一致，则不需要转换
	// 这里直接使用AI返回的坐标（假设AI返回的是截图坐标，截图尺寸=页面渲染尺寸）

	err := chromedp.Run(pa.ctx,
		chromedp.MouseClickXY(float64(x), float64(y)),
		chromedp.Sleep(1*time.Second),
	)

	if err != nil {
		log.Printf("坐标点击失败: %v", err)
		return false
	}

	log.Printf("坐标点击成功")
	return true
}

// FindButtonByTextWithAI 使用AI查找按钮并返回坐标
func (pa *PageAnalyzer) FindButtonByTextWithAI(texts []string) (string, int, int, bool) {
	if pa.imageAnalyzer == nil {
		return "", 0, 0, false
	}

	screenshotPath := pa.TakeScreenshot("")
	if screenshotPath == "" {
		return "", 0, 0, false
	}

	selector, x, y, err := pa.imageAnalyzer.FindButtonByAI(screenshotPath, texts)
	if err != nil {
		log.Printf("AI查找按钮失败: %v", err)
		return "", 0, 0, false
	}

	log.Printf("AI找到按钮: selector=%s, x=%d, y=%d", selector, x, y)
	return selector, x, y, true
}

// FindPhoneInputWithAI 使用AI查找手机号输入框并返回坐标
func (pa *PageAnalyzer) FindPhoneInputWithAI() (string, int, int, bool) {
	if pa.imageAnalyzer == nil {
		return "", 0, 0, false
	}

	screenshotPath := pa.TakeScreenshot("")
	if screenshotPath == "" {
		return "", 0, 0, false
	}

	selector, x, y, err := pa.imageAnalyzer.FindPhoneInputByAI(screenshotPath)
	if err != nil {
		log.Printf("AI查找手机号输入框失败: %v", err)
		return "", 0, 0, false
	}

	log.Printf("AI找到手机号输入框: selector=%s, x=%d, y=%d", selector, x, y)
	return selector, x, y, true
}

// WaitForPageChange 等待页面变化
func (pa *PageAnalyzer) WaitForPageChange(timeout time.Duration) bool {
	var oldURL string
	chromedp.Run(pa.ctx, chromedp.Location(&oldURL))

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var currentURL string
		chromedp.Run(pa.ctx, chromedp.Location(&currentURL))

		if currentURL != oldURL {
			log.Printf("页面已变化: %s -> %s", oldURL, currentURL)
			return true
		}

		time.Sleep(500 * time.Millisecond)
	}

	log.Printf("等待页面变化超时")
	return false
}

// WaitForText 等待指定文本出现
func (pa *PageAnalyzer) WaitForText(texts []string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		var bodyText string
		chromedp.Run(pa.ctx, chromedp.Evaluate(`document.body.innerText`, &bodyText))

		for _, text := range texts {
			if strings.Contains(bodyText, text) {
				log.Printf("找到文本: %s", text)
				return true
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	log.Printf("等待文本超时: %v", texts)
	return false
}

// TakeScreenshot 截图并保存到文件
func (pa *PageAnalyzer) TakeScreenshot(filename string) string {
	var buf []byte
	if err := chromedp.Run(pa.ctx,
		chromedp.CaptureScreenshot(&buf),
	); err != nil {
		log.Printf("截图失败: %v", err)
		return ""
	}

	// 确保截图目录存在
	if pa.screen == "" {
		pa.screen = "screenshots"
	}
	os.MkdirAll(pa.screen, 0755)

	// 生成文件名（带时间戳和sessionID，避免冲突）
	if filename == "" {
		filename = pa.buildFilename("screenshot", "png")
	} else {
		// 为指定文件名也加上sessionID和时间戳前缀
		filename = pa.buildFilename(filename, "png")
	}

	filePath := filepath.Join(pa.screen, filename)
	if err := os.WriteFile(filePath, buf, 0644); err != nil {
		log.Printf("保存截图失败: %v", err)
		return ""
	}

	log.Printf("截图已保存: %s (%d bytes)", filePath, len(buf))
	return filePath
}

// TakeScreenshotBase64 截图并返回base64编码
func (pa *PageAnalyzer) TakeScreenshotBase64() string {
	var buf []byte
	if err := chromedp.Run(pa.ctx,
		chromedp.CaptureScreenshot(&buf),
	); err != nil {
		log.Printf("截图失败: %v", err)
		return ""
	}

	return base64.StdEncoding.EncodeToString(buf)
}

// buildFilename 构建唯一的文件名（带sessionID和时间戳）
func (pa *PageAnalyzer) buildFilename(base, ext string) string {
	ts := time.Now().Format("20060102_150405")
	if pa.sessionID != "" {
		return fmt.Sprintf("%s_%s_%s.%s", pa.sessionID, ts, base, ext)
	}
	return fmt.Sprintf("%s_%s.%s", ts, base, ext)
}

// GetAllInputs 获取页面所有输入框信息
func (pa *PageAnalyzer) GetAllInputs() []map[string]interface{} {
	var inputsInfo []map[string]interface{}
	script := `
	() => {
		const inputs = document.querySelectorAll('input');
		const result = [];
		for (let inp of inputs) {
			result.push({
				type: inp.type || '',
				placeholder: inp.placeholder || '',
				name: inp.name || '',
				id: inp.id || '',
				className: inp.className || '',
				value: inp.value || '',
				ariaLabel: inp.getAttribute('aria-label') || ''
			});
		}
		return result;
	}`

	chromedp.Run(pa.ctx, chromedp.Evaluate(script, &inputsInfo))
	return inputsInfo
}

// GetAllButtons 获取页面所有按钮信息
func (pa *PageAnalyzer) GetAllButtons() []map[string]interface{} {
	var buttonsInfo []map[string]interface{}
	script := `
	() => {
		const buttons = document.querySelectorAll('button, a, [role="button"]');
		const result = [];
		for (let btn of buttons) {
			result.push({
				tag: btn.tagName,
				text: (btn.textContent || '').trim(),
				id: btn.id || '',
				className: btn.className || '',
				href: btn.href || '',
				ariaLabel: btn.getAttribute('aria-label') || ''
			});
		}
		return result;
	}`

	chromedp.Run(pa.ctx, chromedp.Evaluate(script, &buttonsInfo))

	if pa.debug {
		log.Printf("页面共有 %d 个按钮", len(buttonsInfo))
		for i, btn := range buttonsInfo {
			log.Printf("  按钮%d: %v", i+1, btn)
		}
	}

	return buttonsInfo
}

// Go 1.21+ 内置了 min 函数，直接使用
