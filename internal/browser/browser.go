package browser

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

const (
	// 默认会话超时时间（10分钟无活动则自动清理，用于测试）
	defaultSessionTimeout = 10 * time.Minute
	// 清理检查间隔
	cleanupInterval = 30 * time.Second
)

// BrowserManager 管理浏览器实例和会话
type BrowserManager struct {
	browsers       map[string]*BrowserSession // sessionID -> BrowserSession
	mu             sync.RWMutex
	sessionTimeout time.Duration
	cleanupOnce    sync.Once
	cookieStore    *CookieStore // cookie持久化存储
}

// BrowserSession 保存浏览器上下文
type BrowserSession struct {
	Ctx        context.Context
	Cancel     context.CancelFunc
	SessionID  string
	CreatedAt  time.Time
	LastActive time.Time
}

var (
	instance *BrowserManager
	once     sync.Once
)

// GetBrowserManager 获取单例浏览器管理器
func GetBrowserManager() *BrowserManager {
	once.Do(func() {
		instance = &BrowserManager{
			browsers:       make(map[string]*BrowserSession),
			sessionTimeout: defaultSessionTimeout,
			cookieStore:    NewCookieStore("sessions/cookies"),
		}
	})
	// 确保清理协程启动（每个GetBrowserManager调用都可能触发首次清理）
	instance.cleanupOnce.Do(func() {
		go instance.startCleanupLoop()
	})
	return instance
}

// SetSessionTimeout 设置会话超时时间
func (bm *BrowserManager) SetSessionTimeout(timeout time.Duration) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.sessionTimeout = timeout
}

// CreateSession 创建新的浏览器会话
// userDataDir: Chrome用户数据目录，如果指定则保持登录状态
func (bm *BrowserManager) CreateSession(sessionID string, userDataDir string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// 检查会话是否已存在
	if _, exists := bm.browsers[sessionID]; exists {
		return fmt.Errorf("会话已存在: %s", sessionID)
	}

	// 用最简单的方式启动Chrome - 让chromedp自己处理
	log.Println("正在启动浏览器...")

	// 基本选项 - 增强反检测能力
	opts := []chromedp.ExecAllocatorOption{
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.Flag("start-maximized", true), // 确保窗口显示在前台
		
		// 反检测关键参数：禁用自动化控制特征
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		// 排除自动化开关，隐藏自动化特征
		chromedp.Flag("exclude-switches", "enable-automation"),
		// 禁用自动化扩展
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-automation-extension", true),
		// 禁用保存密码弹窗
		chromedp.Flag("disable-save-password-bubble", true),
		// 使用真实用户模式
		chromedp.Flag("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		// 禁用开发者工具提示栏
		chromedp.Flag("disable-infobars", true),
		// 忽略证书错误
		chromedp.Flag("ignore-certificate-errors", true),
		// 禁用弹出窗口阻止（有些网站会检测这个）
		chromedp.Flag("disable-popup-blocking", true),
	}

	// 设置用户数据目录（转换为绝对路径）
	if userDataDir != "" {
		// 如果是相对路径，转换为绝对路径
		if !filepath.IsAbs(userDataDir) {
			absPath, err := filepath.Abs(userDataDir)
			if err == nil {
				userDataDir = absPath
			}
		}
		// 确保目录存在
		os.MkdirAll(userDataDir, 0755)
		opts = append(opts, chromedp.UserDataDir(userDataDir))
		log.Printf("使用用户数据目录(绝对路径): %s", userDataDir)
	}

	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel := chromedp.NewContext(allocCtx)
	log.Println("浏览器上下文已创建")

	// 通过CDP协议注入反检测脚本，使其在每个新页面加载前执行
	log.Println("注入反检测脚本...")
	err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			// 使用chromedp.Evaluate来注入脚本
			// 注意：这是在当前页面注入，新页面需要重新注入
			return chromedp.Evaluate(`
				// 反检测脚本 - 增强版
				(function() {
					'use strict';

					// 1. 覆盖navigator.webdriver（最关键的反检测点）
					try {
						delete navigator.__proto__.webdriver;
					} catch(e) {}
					try {
						Object.defineProperty(navigator, 'webdriver', {
							get: () => undefined,
							set: undefined,
							configurable: true,
							enumerable: true
						});
					} catch(e) {}

					// 2. 删除Chrome自动化特征标记（cdc_adoQpoasnfa76pfcZLmcfl_是chromedp的典型特征）
					try { delete window.cdc_adoQpoasnfa76pfcZLmcfl_; } catch(e) {}
					try { delete window.cdc_adoQpoasnfa76pfcZLmcfl; } catch(e) {}
					
					// 3. 删除其他可能的CDP特征
					const cdpProps = ['__driver_evaluate', '__webdriver_evaluate', '__selenium_evaluate', '__webdriver_script_function', '__webdriver_script_func', '__webdriver_script', '__webdriver_unwrapped', '__webdriver_unwrapped_func', '__webdriver_unwrapped_script'];
					cdpProps.forEach(prop => {
						try { delete window[prop]; } catch(e) {}
					});

					// 4. 伪装chrome属性（完整的chrome对象）
					if (!window.chrome) {
						window.chrome = {
							runtime: {},
							loadTimes: function() { return {commitLoadTime: Date.now(), startLoadTime: Date.now()}; },
							cil: function() { return []; },
							supported: function() { return true; },
							app: {
								isInstalled: false,
							}
						};
					}

					// 5. 伪装plugins（更真实的插件列表）
					try {
						Object.defineProperty(navigator, 'plugins', {
							get: () => {
								const plugins = [
									{name: 'Chrome PDF Plugin', filename: 'internal-pdf-viewer', description: 'Portable Document Format'},
									{name: 'Chrome PDF Viewer', filename: 'mhjfbmdgcfjbbpaeojofohoefgiehjai', description: ''},
									{name: 'Native Client', filename: 'internal-nacl-plugin', description: ''}
								];
								// 添加length属性
								Object.defineProperty(plugins, 'length', {value: plugins.length});
								return plugins;
							},
							configurable: true
						});
					} catch(e) {}

					// 6. 伪装languages
					try {
						Object.defineProperty(navigator, 'languages', {
							get: () => ['zh-CN', 'zh', 'en-US', 'en'],
							configurable: true
						});
					} catch(e) {}

					// 7. 伪装platform
					try {
						Object.defineProperty(navigator, 'platform', {
							get: () => 'Win32',
							configurable: true
						});
					} catch(e) {}

					// 8. 伪装硬件并发数
					try {
						Object.defineProperty(navigator, 'hardwareConcurrency', {
							get: () => 8,
							configurable: true
						});
					} catch(e) {}

					// 9. 伪装内存大小
					try {
						Object.defineProperty(navigator, 'deviceMemory', {
							get: () => 8,
							configurable: true
						});
					} catch(e) {}

					// 10. 覆盖permissions查询（防止检测通知权限等）
					if (navigator.permissions && navigator.permissions.query) {
						const originalQuery = navigator.permissions.query;
						navigator.permissions.query = function(permissionDesc) {
							if (permissionDesc.name === 'notifications' || permissionDesc.name === 'clipboard-read' || permissionDesc.name === 'clipboard-write') {
								return Promise.resolve({state: Notification.permission});
							}
							return originalQuery.call(navigator.permissions, permissionDesc);
						};
					}

					// 11. 禁用iframe检测（如果页面有iframe）
					try {
						// 为iframe也注入反检测（通过监听新iframe创建）
						const observer = new MutationObserver(function(mutations) {
							mutations.forEach(function(mutation) {
								if (mutation.addedNodes) {
									mutation.addedNodes.forEach(function(node) {
										if (node.tagName === 'IFRAME') {
											try {
												node.contentWindow.eval('Object.defineProperty(navigator, "webdriver", {get: () => undefined})');
											} catch(e) {}
										}
									});
								}
							});
						});
						observer.observe(document, {childList: true, subtree: true});
					} catch(e) {}

					console.log('反检测脚本已注入（增强版）');
				})();
			`, nil).Do(ctx)
		}),
	)
	if err != nil {
		log.Printf("注入反检测脚本失败: %v", err)
		// 不返回错误，继续执行
	} else {
		log.Println("反检测脚本已注入")
	}

	now := time.Now()
	bm.browsers[sessionID] = &BrowserSession{
		Ctx:        ctx,
		Cancel:     cancel,
		SessionID:  sessionID,
		CreatedAt:  now,
		LastActive: now,
	}

	log.Printf("创建浏览器会话: %s", sessionID)
	return nil
}

// GetSession 获取浏览器会话
func (bm *BrowserManager) GetSession(sessionID string) (*BrowserSession, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	session, exists := bm.browsers[sessionID]
	if !exists {
		return nil, fmt.Errorf("会话不存在: %s", sessionID)
	}

	// 更新最后活跃时间
	session.LastActive = time.Now()
	return session, nil
}

// CloseSession 关闭浏览器会话
func (bm *BrowserManager) CloseSession(sessionID string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	session, exists := bm.browsers[sessionID]
	if !exists {
		return fmt.Errorf("会话不存在: %s", sessionID)
	}

	session.Cancel()
	delete(bm.browsers, sessionID)
	log.Printf("关闭浏览器会话: %s", sessionID)
	return nil
}

// startCleanupLoop 启动定期清理过期会话的协程
func (bm *BrowserManager) startCleanupLoop() {
	log.Printf("会话清理协程已启动，间隔: %v，超时: %v", cleanupInterval, bm.sessionTimeout)
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		bm.cleanupExpiredSessions()
	}
}

// cleanupExpiredSessions 清理过期会话
func (bm *BrowserManager) cleanupExpiredSessions() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	now := time.Now()
	expired := make([]string, 0)

	for id, session := range bm.browsers {
		if now.Sub(session.LastActive) > bm.sessionTimeout {
			expired = append(expired, id)
		}
	}

	for _, id := range expired {
		log.Printf("清理过期会话: %s (最后活跃: %v)", id, bm.browsers[id].LastActive)
		if session, exists := bm.browsers[id]; exists {
			session.Cancel()
			delete(bm.browsers, id)
		}
	}

	if len(expired) > 0 {
		log.Printf("已清理 %d 个过期会话，当前活跃会话: %d", len(expired), len(bm.browsers))
	}
}

// Navigate 导航到指定URL，并注入反检测脚本
func (s *BrowserSession) Navigate(url string) error {
	s.updateActive()
	err := chromedp.Run(s.Ctx,
		chromedp.Navigate(url),
	)
	if err != nil {
		return err
	}
	// 导航后注入反检测脚本
	return s.InjectAntiDetection()
}

// InjectAntiDetection 注入反检测脚本（增强版，与CreateSession中的脚本保持一致）
func (s *BrowserSession) InjectAntiDetection() error {
	return chromedp.Run(s.Ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(`
				(function() {
					'use strict';

					// 1. 覆盖navigator.webdriver（最关键的反检测点）
					try {
						delete navigator.__proto__.webdriver;
					} catch(e) {}
					try {
						Object.defineProperty(navigator, 'webdriver', {
							get: function() { return undefined; },
							set: undefined,
							configurable: true,
							enumerable: true
						});
					} catch(e) {}

					// 2. 删除Chrome自动化特征标记
					try { 
						delete window.cdc_adoQpoasnfa76pfcZLmcfl_; 
						delete window.cdc_adoQpoasnfa76pfcZLmcfl;
					} catch(e) {}

					// 3. 删除其他可能的CDP特征
					var cdpProps = ['__driver_evaluate', '__webdriver_evaluate', '__selenium_evaluate', 
					                '__webdriver_script_function', '__webdriver_script_func', 
					                '__webdriver_script', '__webdriver_unwrapped', 
					                '__webdriver_unwrapped_func', '__webdriver_unwrapped_script'];
					cdpProps.forEach(function(prop) {
						try { delete window[prop]; } catch(e) {}
					});

					// 4. 伪装chrome属性（完整的chrome对象）
					if (!window.chrome) {
						window.chrome = {
							runtime: {},
							loadTimes: function() { 
								return {
									commitLoadTime: Date.now() - Math.random() * 1000,
									connectionInfo: 'http/1.1',
									finishDocumentLoadTime: Date.now() - Math.random() * 500,
									finishLoadTime: Date.now(),
									firstPaintAfterLoadTime: 0,
									firstPaintTime: Date.now() - Math.random() * 200,
									navigationType: 'Other',
									npnNegotiatedProtocol: 'http/1.1',
									requestTime: Date.now() - Math.random() * 1500,
									startLoadTime: Date.now() - Math.random() * 2000,
									wasAlternateProtocolAvailable: false,
									wasFetchedViaSpdy: false,
									wasNpnNegotiated: true
								};
							},
							cil: function() { return []; },
							supported: function() { return true; },
							app: {
								isInstalled: false
							}
						};
					}

					// 5. 伪装plugins（更真实的插件列表）
					try {
						Object.defineProperty(navigator, 'plugins', {
							get: function() {
								var plugins = [
									{name: 'Chrome PDF Plugin', filename: 'internal-pdf-viewer', description: 'Portable Document Format'},
									{name: 'Chrome PDF Viewer', filename: 'mhjfbmdgcfjbbpaeojofohoefgiehjai', description: ''},
									{name: 'Native Client', filename: 'internal-nacl-plugin', description: ''}
								];
								// 添加length属性
								Object.defineProperty(plugins, 'length', {value: plugins.length});
								return plugins;
							},
							configurable: true
						});
					} catch(e) {}

					// 6. 伪装languages
					try {
						Object.defineProperty(navigator, 'languages', {
							get: function() { return ['zh-CN', 'zh', 'en-US', 'en']; },
							configurable: true
						});
					} catch(e) {}

					// 7. 伪装platform
					try {
						Object.defineProperty(navigator, 'platform', {
							get: function() { return 'Win32'; },
							configurable: true
						});
					} catch(e) {}

					// 8. 伪装硬件并发数
					try {
						Object.defineProperty(navigator, 'hardwareConcurrency', {
							get: function() { return 8; },
							configurable: true
						});
					} catch(e) {}

					// 9. 伪装内存大小
					try {
						Object.defineProperty(navigator, 'deviceMemory', {
							get: function() { return 8; },
							configurable: true
						});
					} catch(e) {}

					// 10. 覆盖permissions查询
					if (navigator.permissions && navigator.permissions.query) {
						var originalQuery = navigator.permissions.query;
						navigator.permissions.query = function(permissionDesc) {
							if (permissionDesc.name === 'notifications' || 
							    permissionDesc.name === 'clipboard-read' || 
							    permissionDesc.name === 'clipboard-write') {
								return Promise.resolve({state: Notification.permission});
							}
							return originalQuery.call(navigator.permissions, permissionDesc);
						};
					}

					console.log('反检测脚本已注入（增强版）');
				})();
			`, nil).Do(ctx)
		}),
	)
}

// WaitForSelector 等待元素出现
func (s *BrowserSession) WaitForSelector(selector string, timeout time.Duration) error {
	s.updateActive()
	// 操作前注入反检测脚本
	s.InjectAntiDetection()
	return chromedp.Run(s.Ctx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
	)
}

// Click 点击元素
func (s *BrowserSession) Click(selector string) error {
	s.updateActive()
	// 操作前注入反检测脚本
	s.InjectAntiDetection()
	return chromedp.Run(s.Ctx,
		chromedp.Click(selector, chromedp.ByQuery),
	)
}

// SendKeys 向元素发送文本
func (s *BrowserSession) SendKeys(selector, text string) error {
	s.updateActive()
	// 操作前注入反检测脚本
	s.InjectAntiDetection()
	return chromedp.Run(s.Ctx,
		chromedp.SendKeys(selector, text, chromedp.ByQuery),
	)
}

// GetText 获取元素文本
func (s *BrowserSession) GetText(selector string) (string, error) {
	s.updateActive()
	// 操作前注入反检测脚本
	s.InjectAntiDetection()
	var text string
	err := chromedp.Run(s.Ctx,
		chromedp.Text(selector, &text, chromedp.ByQuery),
	)
	return text, err
}

// SetInputValue 设置输入框的值
func (s *BrowserSession) SetInputValue(selector, value string) error {
	s.updateActive()
	// 操作前注入反检测脚本
	s.InjectAntiDetection()
	return chromedp.Run(s.Ctx,
		chromedp.SetValue(selector, value, chromedp.ByQuery),
	)
}

// Screenshot 截图
func (s *BrowserSession) Screenshot() ([]byte, error) {
	s.updateActive()
	var buf []byte
	err := chromedp.Run(s.Ctx,
		chromedp.CaptureScreenshot(&buf),
	)
	return buf, err
}

// CheckAntiDetection 检查反检测是否生效（自测功能）
// 返回检测结果和详细信息
func (s *BrowserSession) CheckAntiDetection() (bool, map[string]interface{}, error) {
	result := make(map[string]interface{})
	
	// 注入最新的反检测脚本
	s.InjectAntiDetection()
	
	// 检查1: navigator.webdriver 是否为 undefined（最关键）
	var webdriver interface{}
	err := chromedp.Run(s.Ctx,
		chromedp.Evaluate(`navigator.webdriver`, &webdriver),
	)
	if err != nil {
		return false, result, fmt.Errorf("检查navigator.webdriver失败: %v", err)
	}
	result["navigator.webdriver"] = webdriver
	result["webdriver_passed"] = (webdriver == nil)
	
	// 检查2: window.cdc_adoQpoasnfa76pfcZLmcfl_ 是否存在
	var cdcExists bool
	chromedp.Run(s.Ctx,
		chromedp.Evaluate(`typeof window.cdc_adoQpoasnfa76pfcZLmcfl_ !== 'undefined'`, &cdcExists),
	)
	result["cdc_flag_exists"] = cdcExists
	result["cdc_passed"] = !cdcExists
	
	// 检查3: chrome 对象是否存在
	var chromeExists bool
	chromedp.Run(s.Ctx,
		chromedp.Evaluate(`typeof window.chrome !== 'undefined'`, &chromeExists),
	)
	result["chrome_exists"] = chromeExists
	result["chrome_passed"] = chromeExists
	
	// 检查4: plugins 是否伪装
	var pluginsLen int
	chromedp.Run(s.Ctx,
		chromedp.Evaluate(`navigator.plugins ? navigator.plugins.length : 0`, &pluginsLen),
	)
	result["plugins_length"] = pluginsLen
	result["plugins_passed"] = (pluginsLen > 0)
	
	// 检查5: languages 是否设置
	var languages string
	chromedp.Run(s.Ctx,
		chromedp.Evaluate(`navigator.languages ? navigator.languages.join(',') : ''`, &languages),
	)
	result["languages"] = languages
	result["languages_passed"] = (languages != "")
	
	// 检查6: platform 是否伪装
	var platform string
	chromedp.Run(s.Ctx,
		chromedp.Evaluate(`navigator.platform`, &platform),
	)
	result["platform"] = platform
	result["platform_passed"] = (platform == "Win32")
	
	// 检查7: hardwareConcurrency 是否设置
	var hardwareConcurrency int
	chromedp.Run(s.Ctx,
		chromedp.Evaluate(`navigator.hardwareConcurrency || 0`, &hardwareConcurrency),
	)
	result["hardwareConcurrency"] = hardwareConcurrency
	result["hardware_passed"] = (hardwareConcurrency > 0)
	
	// 检查8: deviceMemory 是否设置
	var deviceMemory float64
	chromedp.Run(s.Ctx,
		chromedp.Evaluate(`navigator.deviceMemory || 0`, &deviceMemory),
	)
	result["deviceMemory"] = deviceMemory
	result["memory_passed"] = (deviceMemory > 0)
	
	// 综合判断
	allPassed := result["webdriver_passed"].(bool) && 
	              result["cdc_passed"].(bool) &&
	              result["chrome_passed"].(bool) &&
	              result["plugins_passed"].(bool) &&
	              result["languages_passed"].(bool)
	
	result["all_passed"] = allPassed
	
	if allPassed {
		log.Println("✅ 反检测检查通过！所有关键检测点都已正确伪装")
	} else {
		log.Println("❌ 反检测检查未完全通过，请检查以下项目：")
		if !result["webdriver_passed"].(bool) {
			log.Println("  - navigator.webdriver 未正确设置")
		}
		if !result["cdc_passed"].(bool) {
			log.Println("  - cdc_adoQpoasnfa76pfcZLmcfl_ 标记仍存在")
		}
		if !result["chrome_passed"].(bool) {
			log.Println("  - chrome 对象不存在")
		}
		if !result["plugins_passed"].(bool) {
			log.Println("  - plugins 未正确伪装")
		}
		if !result["languages_passed"].(bool) {
			log.Println("  - languages 未正确设置")
		}
	}
	
	return allPassed, result, nil
}

// updateActive 更新最后活跃时间
func (s *BrowserSession) updateActive() {
	s.LastActive = time.Now()
}

// SaveCookies 保存当前会话的cookies到文件
func (bm *BrowserManager) SaveCookies(sessionID string, platform string) error {
	session, err := bm.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("获取会话失败: %v", err)
	}

	if bm.cookieStore == nil {
		bm.cookieStore = NewCookieStore("sessions/cookies")
	}

	return bm.cookieStore.SaveCookies(session.Ctx, platform)
}

// LoadCookies 从文件加载cookies并应用到会话
func (bm *BrowserManager) LoadCookies(sessionID string, platform string) error {
	session, err := bm.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("获取会话失败: %v", err)
	}

	if bm.cookieStore == nil {
		bm.cookieStore = NewCookieStore("sessions/cookies")
	}

	return bm.cookieStore.ApplyCookies(session.Ctx, platform)
}

// HasValidCookies 检查是否有有效的cookies
func (bm *BrowserManager) HasValidCookies(platform string) bool {
	if bm.cookieStore == nil {
		bm.cookieStore = NewCookieStore("sessions/cookies")
	}
	return bm.cookieStore.IsCookiesValid(platform)
}

// ClearCookies 清除指定平台的cookies
func (bm *BrowserManager) ClearCookies(platform string) error {
	if bm.cookieStore == nil {
		bm.cookieStore = NewCookieStore("sessions/cookies")
	}
	return bm.cookieStore.ClearCookies(platform)
}

// GetCookieStore 获取cookie存储管理器
func (bm *BrowserManager) GetCookieStore() *CookieStore {
	if bm.cookieStore == nil {
		bm.cookieStore = NewCookieStore("sessions/cookies")
	}
	return bm.cookieStore
}
