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

	"github.com/chromedp/cdproto/network"
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
// 使用CDP原生network.GetCookies API获取cookies，避免JavaScript解析导致的unmarshal错误
func (cs *CookieStore) SaveCookies(ctx interface{}, platform string) error {
	c := ctx.(context.Context)

	var cookies []*Cookie
	err := chromedp.Run(c,
		chromedp.ActionFunc(func(ctx context.Context) error {
			// 使用CDP原生API获取cookies，避免JavaScript evaluate导致的parse error
			networkCookies, err := network.GetCookies().Do(ctx)
			if err != nil {
				// 检查是否是unmarshal解析错误（cookie值包含特殊字符导致）
				if strings.Contains(err.Error(), "could not unmarshal event") ||
					strings.Contains(err.Error(), "parse error") {
					log.Printf("Cookie解析错误（特殊字符导致），降级到document.cookie方案")
					return fmt.Errorf("cookie解析错误: %v", err)
				}
				log.Printf("CDP获取cookies失败: %v，回退到document.cookie方案", err)
				return err
			}

			for _, nc := range networkCookies {
				// 清洗cookie值，过滤可能导致JSON解析失败的控制字符
				cleanValue := sanitizeCookieValue(nc.Value)
				
				cookie := &Cookie{
					Name:     nc.Name,
					Value:    cleanValue,
					Domain:   nc.Domain,
					Path:     nc.Path,
					Expires:  time.Unix(0, int64(nc.Expires)*int64(time.Second)),
					Secure:   nc.Secure,
					HTTPOnly: nc.HTTPOnly,
				}
				switch nc.SameSite {
				case network.CookieSameSiteStrict:
					cookie.SameSite = "strict"
				case network.CookieSameSiteLax:
					cookie.SameSite = "lax"
				case network.CookieSameSiteNone:
					cookie.SameSite = "none"
				default:
					cookie.SameSite = "none"
				}
				cookies = append(cookies, cookie)
			}
			return nil
		}),
	)

	if err != nil {
		log.Printf("CDP方式获取cookies失败: %v，使用备用方案", err)
		return cs.saveCookiesFromDocument(ctx, platform)
	}

	if len(cookies) == 0 {
		log.Println("未获取到任何cookies，尝试备用方案")
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

	log.Printf("Cookies已保存到: %s（共%d个）", filePath, len(cookies))
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

// ApplyCookies 将cookie应用到浏览器上下文（使用CDP原生API设置）
func (cs *CookieStore) ApplyCookies(ctx interface{}, platform string) error {
	cookies, err := cs.LoadCookies(platform)
	if err != nil {
		return err
	}

	if len(cookies) == 0 {
		log.Println("没有可加载的cookies")
		return nil
	}

	// 使用 CDP 原生API设置cookies，支持httpOnly
	c := ctx.(context.Context)
	for _, cookie := range cookies {
		if cookie.Name == "raw_cookies" {
			// 解析 raw cookie 字符串并逐个设置
			rawCookies := cookie.Value
			cookiePairs := strings.Split(rawCookies, "; ")
			for _, pair := range cookiePairs {
				if pair == "" {
					continue
				}
				parts := strings.SplitN(pair, "=", 2)
				if len(parts) != 2 {
					continue
				}
				cName := strings.TrimSpace(parts[0])
				cValue := strings.TrimSpace(parts[1])
				
				setCookieAction := network.SetCookie(cName, cValue).
					WithDomain(cookie.Domain).
					WithPath("/").
					WithHTTPOnly(false)
				err := chromedp.Run(c, setCookieAction)
				if err != nil {
					log.Printf("设置cookie %s 失败: %v", cName, err)
				}
			}
			continue
		}
		
		// 使用CDP原生API设置cookie
		setCookieAction := network.SetCookie(cookie.Name, cookie.Value).
			WithDomain(cookie.Domain).
			WithPath(cookie.Path).
			WithSecure(cookie.Secure).
			WithHTTPOnly(cookie.HTTPOnly)
		
		sameSite := network.CookieSameSiteNone
		switch strings.ToLower(cookie.SameSite) {
		case "strict":
			sameSite = network.CookieSameSiteStrict
		case "lax":
			sameSite = network.CookieSameSiteLax
		case "none":
			sameSite = network.CookieSameSiteNone
		default:
			sameSite = network.CookieSameSiteNone
		}
		
		setCookieAction = setCookieAction.WithSameSite(sameSite)
		
		err := chromedp.Run(c, setCookieAction)
		if err != nil {
			log.Printf("设置cookie %s 失败: %v", cookie.Name, err)
		}
	}

	log.Printf("已尝试应用 %d 个cookies", len(cookies))
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