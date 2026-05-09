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
	defaultSessionTimeout = 10 * time.Minute
	cleanupInterval       = 30 * time.Second
)

func findChromePath() string {
	paths := []string{
		"C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe",
		"C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe",
		os.Getenv("LOCALAPPDATA") + "\\Google\\Chrome\\Application\\chrome.exe",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			log.Printf("找到Chrome路径: %s", path)
			return path
		}
	}
	log.Println("未找到Chrome路径，将使用系统默认路径")
	return ""
}

type BrowserManager struct {
	browsers       map[string]*BrowserSession
	mu             sync.RWMutex
	sessionTimeout time.Duration
	cleanupOnce    sync.Once
	cookieStore    *CookieStore
}

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

func GetBrowserManager() *BrowserManager {
	once.Do(func() {
		instance = &BrowserManager{
			browsers:       make(map[string]*BrowserSession),
			sessionTimeout: defaultSessionTimeout,
			cookieStore:    NewCookieStore("sessions/cookies"),
		}
	})
	instance.cleanupOnce.Do(func() {
		go instance.startCleanupLoop()
	})
	return instance
}

func (bm *BrowserManager) SetSessionTimeout(timeout time.Duration) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.sessionTimeout = timeout
}

func (bm *BrowserManager) CreateSession(sessionID string, userDataDir string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if _, exists := bm.browsers[sessionID]; exists {
		return fmt.Errorf("会话已存在: %s", sessionID)
	}

	log.Println("正在启动浏览器...")

	opts := []chromedp.ExecAllocatorOption{
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.Flag("start-maximized", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("exclude-switches", "enable-automation"),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-automation-extension", true),
		chromedp.Flag("disable-save-password-bubble", true),
		chromedp.Flag("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.Flag("disable-popup-blocking", true),
		chromedp.Flag("disable-sync", true),
	}

	if userDataDir != "" {
		if !filepath.IsAbs(userDataDir) {
			if absPath, err := filepath.Abs(userDataDir); err == nil {
				userDataDir = absPath
			}
		}
		os.MkdirAll(userDataDir, 0755)
		opts = append(opts, chromedp.UserDataDir(userDataDir))
		log.Printf("使用用户数据目录(绝对路径): %s", userDataDir)
	}

	chromePath := findChromePath()
	if chromePath != "" {
		opts = append(opts, chromedp.ExecPath(chromePath))
		log.Printf("使用Chrome路径: %s", chromePath)
	}

	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel := chromedp.NewContext(allocCtx)
	log.Println("浏览器上下文已创建")

	log.Println("注入反检测脚本...")
	err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(`
				(function() {
					'use strict';
					try { delete navigator.__proto__.webdriver; } catch(e) {}
					try {
						Object.defineProperty(navigator, 'webdriver', {
							get: () => undefined,
							set: undefined,
							configurable: true,
							enumerable: true
						});
					} catch(e) {}
					try { delete window.cdc_adoQpoasnfa76pfcZLmcfl_; } catch(e) {}
					try { delete window.cdc_adoQpoasnfa76pfcZLmcfl; } catch(e) {}
					const cdpProps = ['__driver_evaluate', '__webdriver_evaluate', '__selenium_evaluate', '__webdriver_script_function', '__webdriver_script_func', '__webdriver_script', '__webdriver_unwrapped', '__webdriver_unwrapped_func', '__webdriver_unwrapped_script'];
					cdpProps.forEach(prop => { try { delete window[prop]; } catch(e) {} });
					if (!window.chrome) {
						window.chrome = {
							runtime: {},
							loadTimes: function() { return {commitLoadTime: Date.now(), startLoadTime: Date.now()}; },
							cil: function() { return []; },
							supported: function() { return true; },
							app: { isInstalled: false }
						};
					}
					try {
						Object.defineProperty(navigator, 'plugins', {
							get: () => {
								const plugins = [
									{name: 'Chrome PDF Plugin', filename: 'internal-pdf-viewer', description: 'Portable Document Format'},
									{name: 'Chrome PDF Viewer', filename: 'mhjfbmdgcfjbbpaeojofohoefgiehjai', description: ''},
									{name: 'Native Client', filename: 'internal-nacl-plugin', description: ''}
								];
								Object.defineProperty(plugins, 'length', {value: plugins.length});
								return plugins;
							},
							configurable: true
						});
					} catch(e) {}
					try {
						Object.defineProperty(navigator, 'languages', {
							get: () => ['zh-CN', 'zh', 'en-US', 'en'],
							configurable: true
						});
					} catch(e) {}
					try {
						Object.defineProperty(navigator, 'platform', {
							get: () => 'Win32',
							configurable: true
						});
					} catch(e) {}
					try {
						Object.defineProperty(navigator, 'hardwareConcurrency', {
							get: () => 8,
							configurable: true
						});
					} catch(e) {}
					try {
						Object.defineProperty(navigator, 'deviceMemory', {
							get: () => 8,
							configurable: true
						});
					} catch(e) {}
					if (navigator.permissions && navigator.permissions.query) {
						const originalQuery = navigator.permissions.query;
						navigator.permissions.query = function(permissionDesc) {
							if (permissionDesc.name === 'notifications' || permissionDesc.name === 'clipboard-read' || permissionDesc.name === 'clipboard-write') {
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
	if err != nil {
		log.Printf("注入反检测脚本失败: %v", err)
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

func (bm *BrowserManager) GetSession(sessionID string) (*BrowserSession, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	session, exists := bm.browsers[sessionID]
	if !exists {
		return nil, fmt.Errorf("会话不存在: %s", sessionID)
	}

	session.LastActive = time.Now()
	return session, nil
}

func (bm *BrowserManager) CloseSession(sessionID string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	session, exists := bm.browsers[sessionID]
	if !exists {
		return fmt.Errorf("会话不存在: %s", sessionID)
	}

	// 保存cookies再关闭
	if bm.cookieStore != nil {
		platform := "unknown"
		if sessionID[:5] == "zhipu" {
			platform = "zhipu"
		} else if sessionID[:7] == "chatgpt" {
			platform = "chatgpt"
		} else if sessionID[:6] == "claude" {
			platform = "claude"
		}
		if err := bm.cookieStore.SaveCookies(session.Ctx, platform); err != nil {
			log.Printf("保存cookies失败: %v", err)
		}
	}

	log.Printf("正在关闭浏览器会话: %s", sessionID)
	session.Cancel()
	delete(bm.browsers, sessionID)
	log.Printf("浏览器会话已关闭: %s", sessionID)
	return nil
}

func (bm *BrowserManager) startCleanupLoop() {
	ticker := time.NewTicker(cleanupInterval)
	for range ticker.C {
		bm.cleanupExpiredSessions()
	}
}

func (bm *BrowserManager) cleanupExpiredSessions() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	now := time.Now()
	for id, session := range bm.browsers {
		if now.Sub(session.LastActive) > bm.sessionTimeout {
			log.Printf("清理过期会话: %s (最后活跃: %v)", id, session.LastActive)
			session.Cancel()
			delete(bm.browsers, id)
		}
	}
}

func (bm *BrowserManager) HasValidCookies(platform string) bool {
	if bm.cookieStore == nil {
		return false
	}
	return bm.cookieStore.IsCookiesValid(platform)
}

func (bm *BrowserManager) LoadCookies(sessionID string, platform string) error {
	if bm.cookieStore == nil {
		return fmt.Errorf("cookie存储未初始化")
	}

	session, err := bm.GetSession(sessionID)
	if err != nil {
		return err
	}

	return bm.cookieStore.ApplyCookies(session.Ctx, platform)
}

func (bm *BrowserManager) SaveCookies(sessionID string, platform string) error {
	if bm.cookieStore == nil {
		return fmt.Errorf("cookie存储未初始化")
	}

	session, err := bm.GetSession(sessionID)
	if err != nil {
		return err
	}

	return bm.cookieStore.SaveCookies(session.Ctx, platform)
}

func (bs *BrowserSession) InjectAntiDetection() error {
	return chromedp.Run(bs.Ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(`
				(function() {
					'use strict';
					try { delete navigator.__proto__.webdriver; } catch(e) {}
					try {
						Object.defineProperty(navigator, 'webdriver', {
							get: () => undefined,
							set: undefined,
							configurable: true,
							enumerable: true
						});
					} catch(e) {}
					try { delete window.cdc_adoQpoasnfa76pfcZLmcfl_; } catch(e) {}
					try { delete window.cdc_adoQpoasnfa76pfcZLmcfl; } catch(e) {}
					const cdpProps = ['__driver_evaluate', '__webdriver_evaluate', '__selenium_evaluate', '__webdriver_script_function', '__webdriver_script_func', '__webdriver_script', '__webdriver_unwrapped', '__webdriver_unwrapped_func', '__webdriver_unwrapped_script'];
					cdpProps.forEach(prop => { try { delete window[prop]; } catch(e) {} });
					if (!window.chrome) {
						window.chrome = {
							runtime: {},
							loadTimes: function() { return {commitLoadTime: Date.now(), startLoadTime: Date.now()}; },
							cil: function() { return []; },
							supported: function() { return true; },
							app: { isInstalled: false }
						};
					}
					try {
						Object.defineProperty(navigator, 'plugins', {
							get: () => {
								const plugins = [
									{name: 'Chrome PDF Plugin', filename: 'internal-pdf-viewer', description: 'Portable Document Format'},
									{name: 'Chrome PDF Viewer', filename: 'mhjfbmdgcfjbbpaeojofohoefgiehjai', description: ''},
									{name: 'Native Client', filename: 'internal-nacl-plugin', description: ''}
								];
								Object.defineProperty(plugins, 'length', {value: plugins.length});
								return plugins;
							},
							configurable: true
						});
					} catch(e) {}
					try {
						Object.defineProperty(navigator, 'languages', {
							get: () => ['zh-CN', 'zh', 'en-US', 'en'],
							configurable: true
						});
					} catch(e) {}
					try {
						Object.defineProperty(navigator, 'platform', {
							get: () => 'Win32',
							configurable: true
						});
					} catch(e) {}
					try {
						Object.defineProperty(navigator, 'hardwareConcurrency', {
							get: () => 8,
							configurable: true
						});
					} catch(e) {}
					try {
						Object.defineProperty(navigator, 'deviceMemory', {
							get: () => 8,
							configurable: true
						});
					} catch(e) {}
					console.log('反检测脚本已注入');
				})();
			`, nil).Do(ctx)
		}),
	)
}

func (bs *BrowserSession) CheckAntiDetection() (bool, map[string]interface{}, error) {
	details := make(map[string]interface{})
	allPassed := true

	tests := map[string]string{
		"webdriver检查": `
			(() => {
				try {
					return navigator.webdriver === undefined || navigator.webdriver === false;
				} catch(e) {
					return false;
				}
			})()
		`,
		"chrome对象检查": `
			(() => {
				return typeof window.chrome !== 'undefined';
			})()
		`,
		"plugins检查": `
			(() => {
				try {
					return navigator.plugins && navigator.plugins.length > 0;
				} catch(e) {
					return false;
				}
			})()
		`,
		"languages检查": `
			(() => {
				try {
					return navigator.languages && navigator.languages.length > 0;
				} catch(e) {
					return false;
				}
			})()
		`,
	}

	for testName, script := range tests {
		var result bool
		err := chromedp.Run(bs.Ctx, chromedp.Evaluate(script, &result))
		if err != nil {
			details[testName] = fmt.Sprintf("测试失败: %v", err)
			allPassed = false
		} else {
			details[testName] = result
			if !result {
				allPassed = false
			}
		}
	}

	return allPassed, details, nil
}