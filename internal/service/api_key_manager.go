package service

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"aithink/internal/models"
)

// APIKeyManager API密钥管理器（带文件持久化）
type APIKeyManager struct {
	mu         sync.RWMutex
	apiKeys    map[string]*models.APIKeyInfo
	storePath  string
}

// NewAPIKeyManager 创建API密钥管理器
func NewAPIKeyManager() *APIKeyManager {
	storePath := filepath.Join("data", "api_keys.json")
	
	m := &APIKeyManager{
		apiKeys:   make(map[string]*models.APIKeyInfo),
		storePath: storePath,
	}
	
	// 加载已保存的API密钥
	if err := m.loadFromDisk(); err != nil {
		log.Printf("加载API密钥失败: %v", err)
	}
	
	return m
}

// loadFromDisk 从文件加载API密钥
func (m *APIKeyManager) loadFromDisk() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，创建目录
			if err := os.MkdirAll(filepath.Dir(m.storePath), 0755); err != nil {
				return fmt.Errorf("创建数据目录失败: %v", err)
			}
			return nil
		}
		return fmt.Errorf("读取文件失败: %v", err)
	}

	// 去除UTF-8 BOM
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}

	var keys []*models.APIKeyInfo
	if err := json.Unmarshal(data, &keys); err != nil {
		return fmt.Errorf("解析文件失败: %v", err)
	}

	for _, key := range keys {
		m.apiKeys[key.APIKey] = key
	}

	log.Printf("已加载 %d 个API密钥", len(keys))
	return nil
}

// saveToDiskLocked 保存API密钥到文件（调用方必须持有锁）
func (m *APIKeyManager) saveToDiskLocked() error {
	keys := make([]*models.APIKeyInfo, 0, len(m.apiKeys))
	for _, key := range m.apiKeys {
		if key.Status != models.APIKeyStatusDeleted {
			keys = append(keys, key)
		}
	}

	data, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化失败: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.storePath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	if err := os.WriteFile(m.storePath, data, 0600); err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	return nil
}

// saveToDisk 保存API密钥到文件（自动获取读锁）
func (m *APIKeyManager) saveToDisk() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.saveToDiskLocked()
}

// GenerateAPIKey 生成API密钥
func (m *APIKeyManager) GenerateAPIKey(req models.CreateAPIKeyRequest) (models.CreateAPIKeyResponse, error) {
	if req.Platform == "" {
		return models.CreateAPIKeyResponse{}, fmt.Errorf("平台不能为空")
	}
	if req.Name == "" {
		return models.CreateAPIKeyResponse{}, fmt.Errorf("密钥名称不能为空")
	}
	if req.SessionID == "" {
		return models.CreateAPIKeyResponse{}, fmt.Errorf("会话ID不能为空")
	}

	// 生成唯一API密钥
	apiKey, err := generateKey()
	if err != nil {
		return models.CreateAPIKeyResponse{}, fmt.Errorf("生成密钥失败: %v", err)
	}

	now := time.Now()
	keyInfo := &models.APIKeyInfo{
		APIKey:       apiKey,
		Platform:     req.Platform,
		Name:         req.Name,
		SessionID:    req.SessionID,
		Status:       models.APIKeyStatusActive,
		CreatedAt:    now,
		ExpiresAt:    req.ExpiresAt,
		RequestCount: 0,
	}

	m.mu.Lock()
	m.apiKeys[apiKey] = keyInfo
	m.mu.Unlock()

	// 持久化到文件（调用方已释放写锁）
	if err := m.saveToDisk(); err != nil {
		log.Printf("保存API密钥到文件失败: %v", err)
	}

	resp := models.CreateAPIKeyResponse{
		APIKeyInfo: *keyInfo,
		FullAPIKey: apiKey,
	}

	return resp, nil
}

// ValidateAPIKey 验证API密钥
func (m *APIKeyManager) ValidateAPIKey(apiKey string) (*models.APIKeyInfo, error) {
	m.mu.RLock()
	keyInfo, exists := m.apiKeys[apiKey]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("无效的API密钥")
	}

	// 检查状态
	switch keyInfo.Status {
	case models.APIKeyStatusInactive:
		return nil, fmt.Errorf("API密钥已停用")
	case models.APIKeyStatusDeleted:
		return nil, fmt.Errorf("API密钥已删除")
	case models.APIKeyStatusExpired:
		return nil, fmt.Errorf("API密钥已过期")
	}

	// 检查过期时间
	if keyInfo.ExpiresAt != nil && time.Now().After(*keyInfo.ExpiresAt) {
		m.updateKeyStatus(apiKey, models.APIKeyStatusExpired)
		return nil, fmt.Errorf("API密钥已过期")
	}

	return keyInfo, nil
}

// GetAPIKey 获取API密钥信息
func (m *APIKeyManager) GetAPIKey(apiKey string) (*models.APIKeyInfo, error) {
	m.mu.RLock()
	keyInfo, exists := m.apiKeys[apiKey]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("无效的API密钥")
	}

	return keyInfo, nil
}

// ListAPIKeys 列出所有API密钥
func (m *APIKeyManager) ListAPIKeys() models.APIKeyListResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]models.APIKeyInfo, 0, len(m.apiKeys))
	for _, keyInfo := range m.apiKeys {
		if keyInfo.Status != models.APIKeyStatusDeleted {
			items = append(items, *keyInfo)
		}
	}

	return models.APIKeyListResponse{
		Total: len(items),
		Items: items,
	}
}

// UpdateAPIKey 更新API密钥信息
func (m *APIKeyManager) UpdateAPIKey(apiKey string, req models.UpdateAPIKeyRequest) error {
	m.mu.Lock()

	keyInfo, exists := m.apiKeys[apiKey]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("无效的API密钥")
	}

	if req.Name != "" {
		keyInfo.Name = req.Name
	}

	if req.Status != nil {
		switch *req.Status {
		case models.APIKeyStatusActive, models.APIKeyStatusInactive:
			keyInfo.Status = *req.Status
		default:
			m.mu.Unlock()
			return fmt.Errorf("不支持的状态: %s", *req.Status)
		}
	}

	// 持久化到文件（在持有写锁的情况下直接保存）
	if err := m.saveToDiskLocked(); err != nil {
		log.Printf("保存API密钥到文件失败: %v", err)
	}

	m.mu.Unlock()
	return nil
}

// DeleteAPIKey 删除API密钥
func (m *APIKeyManager) DeleteAPIKey(apiKey string) error {
	m.mu.Lock()

	keyInfo, exists := m.apiKeys[apiKey]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("无效的API密钥")
	}

	keyInfo.Status = models.APIKeyStatusDeleted

	// 持久化到文件（在持有写锁的情况下直接保存）
	if err := m.saveToDiskLocked(); err != nil {
		log.Printf("保存API密钥到文件失败: %v", err)
	}

	m.mu.Unlock()
	return nil
}

// UpdateUsage 更新API密钥使用记录（异步保存）
func (m *APIKeyManager) UpdateUsage(apiKey string) {
	m.mu.Lock()

	keyInfo, exists := m.apiKeys[apiKey]
	if !exists {
		m.mu.Unlock()
		return
	}

	now := time.Now()
	keyInfo.RequestCount++
	keyInfo.LastUsedAt = &now

	m.mu.Unlock()

	// 异步保存到文件
	go m.saveToDisk()
}

// updateKeyStatus 更新密钥状态（内部方法，不加锁）
func (m *APIKeyManager) updateKeyStatus(apiKey string, status models.APIKeyStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if keyInfo, exists := m.apiKeys[apiKey]; exists {
		keyInfo.Status = status
	}
}

// generateKey 生成随机的API密钥
func generateKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
