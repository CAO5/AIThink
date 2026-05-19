package platform

import (
	"fmt"
	"sort"
	"sync"

	"aithink/internal/browser"
	"aithink/internal/models"
)

// PlatformRegistry 平台注册器，管理所有AI平台的工厂函数和配置
// 使用读写锁保证并发安全，支持运行时动态注册新平台
type PlatformRegistry struct {
	mu        sync.RWMutex                                         // 读写锁，保护并发访问
	factories map[models.Platform]func(*browser.BrowserSession) PlatformClient // 平台工厂函数映射
	configs   map[models.Platform]*PlatformConfig                 // 平台配置映射
}

// 全局注册器实例和单例控制
var (
	registry *PlatformRegistry
	once     sync.Once
)

// GetRegistry 获取平台注册器单例
// 首次调用时初始化注册器，后续调用返回同一实例
func GetRegistry() *PlatformRegistry {
	once.Do(func() {
		registry = &PlatformRegistry{
			factories: make(map[models.Platform]func(*browser.BrowserSession) PlatformClient),
			configs:   make(map[models.Platform]*PlatformConfig),
		}
	})
	return registry
}

// Register 注册平台客户端工厂函数和配置
// 如果平台已注册，将覆盖原有的工厂函数和配置
func (r *PlatformRegistry) Register(platform models.Platform, factory func(*browser.BrowserSession) PlatformClient, config *PlatformConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.factories[platform] = factory
	r.configs[platform] = config
}

// GetClient 根据平台类型和浏览器会话创建客户端实例
// 如果平台未注册，返回错误
func (r *PlatformRegistry) GetClient(platform models.Platform, session *browser.BrowserSession) (PlatformClient, error) {
	r.mu.RLock()
	factory, exists := r.factories[platform]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("平台未注册: %s", platform)
	}

	return factory(session), nil
}

// GetConfig 获取指定平台的配置
// 如果平台未注册，返回错误
func (r *PlatformRegistry) GetConfig(platform models.Platform) (*PlatformConfig, error) {
	r.mu.RLock()
	config, exists := r.configs[platform]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("平台配置不存在: %s", platform)
	}

	return config, nil
}

// ListPlatforms 获取所有已注册的平台列表
// 返回按平台名称排序的列表
func (r *PlatformRegistry) ListPlatforms() []models.Platform {
	r.mu.RLock()
	defer r.mu.RUnlock()

	platforms := make([]models.Platform, 0, len(r.factories))
	for p := range r.factories {
		platforms = append(platforms, p)
	}

	// 按名称排序，保证返回顺序稳定
	sort.Slice(platforms, func(i, j int) bool {
		return string(platforms[i]) < string(platforms[j])
	})

	return platforms
}

// IsRegistered 检查指定平台是否已注册
func (r *PlatformRegistry) IsRegistered(platform models.Platform) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.factories[platform]
	return exists
}
