package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// Cookie 存储cookie信息
type Cookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires"`
	Secure   bool      `json:"secure"`
	HTTPOnly bool      `json:"httpOnly"`
	SameSite string    `json:"sameSite"`
}

// CookieStore cookie存储管理器
type CookieStore struct {
	storageDir string // cookie存储目录
}

// NewCookieStore 创建cookie存储管理器
func NewCookieStore(storageDir string) *CookieStore {
	// 确保存储目录存在
	os.MkdirAll(storageDir, 0755)
	return &CookieStore{storageDir: storageDir}
}

// getCookieFilePath 获取cookie文件路径
func (cs *CookieStore) getCookieFilePath(platform string) string {
	return filepath.Join(cs.storageDir, fmt.Sprintf("%s_cookies.json", platform))
}

// SaveCookies 保存cookies到文件
func (cs *CookieStore) SaveCookies(ctx interface{}, platform string) error {
	// 使用chromedp获取所有cookie
	var cookies []*Cookie
	err := chromedp.Run(ctx.(context.Context),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// 获取所有cookie
			var networkCookies []struct {
				Name     string  `json:"name"`
				Value    string  `json:"value"`
				Domain   string  `json:"domain"`
				Path     string  `json:"path"`
				Expires  float64 `json:"expires"`
				Secure   bool    `json:"secure"`
				HTTPOnly bool    `json:"httpOnly"`
				SameSite string  `json:"sameSite"`
			}

			// 使用CDP命令获取cookie
			return chromedp.Evaluate(`
				(async () => {
					// 尝试使用CDP的Network.getCookies
					if (typeof chrome !== 'undefined' && chrome.cookies) {
						const cookies = await chrome.cookies.getAll({});
						return cookies;
					}
					// 备用方案：从document.cookie解析
					return document.cookie.split(';').map(c => {
						const parts = c.trim().split('=');
						return {
							name: parts[0],
							value: parts.slice(1).join('='),
							domain: location.hostname,
							path: '/'
						};
					});
				})()
			`, &networkCookies).Do(ctx)
		}),
	)

	if err != nil {
		log.Printf("获取cookies失败: %v", err)
		// 尝试备用方案：直接获取document.cookie
		return cs.saveCookiesFromDocument(ctx, platform)
	}

	// 保存到文件
	filePath := cs.getCookieFilePath(platform)
	data, err := json.MarshalIndent(cookies, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化cookies失败: %v", err)
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("保存cookies文件失败: %v", err)
	}

	log.Printf("Cookies已保存到: %s", filePath)
	return nil
}

// saveCookiesFromDocument 从document.cookie保存（备用方案）
func (cs *CookieStore) saveCookiesFromDocument(ctx interface{}, platform string) error {
	var cookieStr string
	err := chromedp.Run(ctx.(context.Context),
		chromedp.Evaluate(`document.cookie`, &cookieStr),
	)
	if err != nil {
		return fmt.Errorf("获取document.cookie失败: %v", err)
	}

	if cookieStr == "" {
		log.Println("document.cookie为空，无需保存")
		return nil
	}

	// 解析cookie字符串
	var cookies []*Cookie
	// 简单解析，实际可能需要更复杂的逻辑
	cookies = append(cookies, &Cookie{
		Name:   "raw_cookies",
		Value:  cookieStr,
		Domain: "chatglm.cn",
		Path:   "/",
	})

	filePath := cs.getCookieFilePath(platform)
	data, err := json.MarshalIndent(cookies, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化cookies失败: %v", err)
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("保存cookies文件失败: %v", err)
	}

	log.Printf("Cookies（从document.cookie）已保存到: %s", filePath)
	return nil
}

// LoadCookies 从文件加载cookies
func (cs *CookieStore) LoadCookies(platform string) ([]*Cookie, error) {
	filePath := cs.getCookieFilePath(platform)

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("Cookie文件不存在: %s", filePath)
		return nil, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取cookie文件失败: %v", err)
	}

	var cookies []*Cookie
	err = json.Unmarshal(data, &cookies)
	if err != nil {
		return nil, fmt.Errorf("解析cookie文件失败: %v", err)
	}

	log.Printf("从文件加载了 %d 个cookies", len(cookies))
	return cookies, nil
}

// ApplyCookies 将cookie应用到浏览器上下文（在页面导航后调用）
func (cs *CookieStore) ApplyCookies(ctx interface{}, platform string) error {
	cookies, err := cs.LoadCookies(platform)
	if err != nil {
		return err
	}

	if len(cookies) == 0 {
		log.Println("没有可加载的cookies")
		return nil
	}

	// 构建JavaScript代码来设置cookie
	jsCode := ""
	for _, cookie := range cookies {
		if cookie.Name == "raw_cookies" {
			// 直接设置raw cookie - 需要解析并逐个设置
			rawCookies := cookie.Value
			// 将raw cookie字符串分割成单个cookie
			cookiePairs := strings.Split(rawCookies, "; ")
			for _, pair := range cookiePairs {
				if pair != "" {
					jsCode += fmt.Sprintf(`document.cookie = "%s; domain=%s; path=/"; `, pair, cookie.Domain)
				}
			}
		} else {
			// 构建单个cookie字符串
			cookieStr := fmt.Sprintf("%s=%s", cookie.Name, cookie.Value)
			if cookie.Domain != "" {
				cookieStr += fmt.Sprintf("; domain=%s", cookie.Domain)
			}
			if cookie.Path != "" {
				cookieStr += fmt.Sprintf("; path=%s", cookie.Path)
			}
			if cookie.Expires.After(time.Now()) {
				cookieStr += fmt.Sprintf("; expires=%s", cookie.Expires.UTC().Format(time.RFC1123))
			}
			if cookie.Secure {
				cookieStr += "; secure"
			}
			if cookie.HTTPOnly {
				cookieStr += "; httponly"
			}
			jsCode += fmt.Sprintf(`document.cookie = "%s"; `, cookieStr)
		}
	}

	if jsCode != "" {
		// 先等待页面加载完成
		err := chromedp.Run(ctx.(context.Context),
			chromedp.Sleep(2*time.Second), // 等待页面加载
			chromedp.Evaluate(jsCode, nil),
		)
		if err != nil {
			return fmt.Errorf("应用cookies失败: %v", err)
		}
		log.Printf("已应用 %d 个cookies到浏览器", len(cookies))
	}

	return nil
}

// ClearCookies 清除指定平台的cookie
func (cs *CookieStore) ClearCookies(platform string) error {
	filePath := cs.getCookieFilePath(platform)
	err := os.Remove(filePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除cookie文件失败: %v", err)
	}
	log.Printf("已清除 %s 平台的cookies", platform)
	return nil
}

// IsCookiesValid 检查cookie是否有效（通过检查过期时间）
func (cs *CookieStore) IsCookiesValid(platform string) bool {
	cookies, err := cs.LoadCookies(platform)
	if err != nil || len(cookies) == 0 {
		return false
	}

	// 检查是否有未过期的cookie
	now := time.Now()
	for _, cookie := range cookies {
		if cookie.Expires.IsZero() || cookie.Expires.After(now) {
			return true
		}
	}

	log.Printf("所有cookies都已过期")
	return false
}

// GetCookieStorageDir 获取cookie存储目录
func (cs *CookieStore) GetCookieStorageDir() string {
	return cs.storageDir
}