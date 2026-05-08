package config

import (
	"encoding/json"
	"log"
	"os"
	"sync"
)

// Config 应用配置
type Config struct {
	// 图片识别AI配置
	ImageAI ImageAIConfig `json:"image_ai"`
	
	// 其他配置可以在这里扩展
}

// ImageAIConfig 图片识别AI配置
type ImageAIConfig struct {
	// 启用的AI服务类型：openai, baidu, tencent, custom
	Provider string `json:"provider"`
	
	// OpenAI GPT-4V配置
	OpenAI OpenAIConfig `json:"openai"`
	
	// 百度OCR配置
	Baidu BaiduOCRConfig `json:"baidu"`
	
	// 腾讯云OCR配置
	Tencent TencentOCRConfig `json:"tencent"`
	
	// 自定义API配置
	Custom CustomAPIConfig `json:"custom"`
}

// OpenAIConfig OpenAI配置
type OpenAIConfig struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
}

// BaiduOCRConfig 百度OCR配置
type BaiduOCRConfig struct {
	APIKey    string `json:"api_key"`
	SecretKey string `json:"secret_key"`
}

// TencentOCRConfig 腾讯云OCR配置
type TencentOCRConfig struct {
	SecretID  string `json:"secret_id"`
	SecretKey string `json:"secret_key"`
}

// CustomAPIConfig 自定义API配置
type CustomAPIConfig struct {
	URL     string `json:"url"`
	APIKey  string `json:"api_key"`
	Headers map[string]string `json:"headers"`
}

// ConfigManager 配置管理器
type ConfigManager struct {
	config     *Config
	configPath string
	mu         sync.RWMutex
}

var (
	instance *ConfigManager
	once     sync.Once
)

// GetConfigManager 获取配置管理器单例
func GetConfigManager() *ConfigManager {
	once.Do(func() {
		// 优先使用环境变量 CONFIG_PATH，否则使用默认值 config.json
		configPath := os.Getenv("CONFIG_PATH")
		if configPath == "" {
			configPath = "config.json"
		}
		instance = &ConfigManager{
			config:     &Config{},
			configPath: configPath,
		}
		instance.load()
	})
	return instance
}

// load 从文件加载配置
func (cm *ConfigManager) load() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		log.Printf("配置文件不存在或读取失败，将使用默认配置: %v", err)
		cm.config = &Config{
			ImageAI: ImageAIConfig{
				Provider: "openai",
				OpenAI: OpenAIConfig{
					BaseURL: "https://api.openai.com/v1",
					Model:   "gpt-4-vision-preview",
				},
			},
		}
		return
	}
	
	if err := json.Unmarshal(data, cm.config); err != nil {
		log.Printf("解析配置文件失败: %v", err)
		return
	}
	
	log.Println("✅ 配置已加载")
}

// Save 保存配置到文件
func (cm *ConfigManager) Save() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	data, err := json.MarshalIndent(cm.config, "", "  ")
	if err != nil {
		return err
	}
	
	if err := os.WriteFile(cm.configPath, data, 0644); err != nil {
		return err
	}
	
	log.Println("✅ 配置已保存")
	return nil
}

// GetConfig 获取配置
func (cm *ConfigManager) GetConfig() *Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.config
}

// UpdateImageAIConfig 更新图片识别AI配置
func (cm *ConfigManager) UpdateImageAIConfig(imageAI ImageAIConfig) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.config.ImageAI = imageAI
}

// SetOpenAIKey 设置OpenAI API Key
func (cm *ConfigManager) SetOpenAIKey(apiKey string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.config.ImageAI.OpenAI.APIKey = apiKey
}

// SetBaiduOCRKeys 设置百度OCR密钥
func (cm *ConfigManager) SetBaiduOCRKeys(apiKey, secretKey string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.config.ImageAI.Baidu.APIKey = apiKey
	cm.config.ImageAI.Baidu.SecretKey = secretKey
}

// SetTencentOCRKeys 设置腾讯云OCR密钥
func (cm *ConfigManager) SetTencentOCRKeys(secretID, secretKey string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.config.ImageAI.Tencent.SecretID = secretID
	cm.config.ImageAI.Tencent.SecretKey = secretKey
}
