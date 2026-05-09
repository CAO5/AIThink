package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"aithink/internal/models"
)

// APIKeyManager API密钥管理器
type APIKeyManager struct {
	mu      sync.RWMutex
	apiKeys map[string]*models.APIKeyInfo
}

// NewAPIKeyManager 创建API密钥管理器
func NewAPIKeyManager() *APIKeyManager {
	return &APIKeyManager{
		apiKeys: make(map[string]*models.APIKeyInfo),
	}
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
	defer m.mu.Unlock()

	keyInfo, exists := m.apiKeys[apiKey]
	if !exists {
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
			return fmt.Errorf("不支持的状态: %s", *req.Status)
		}
	}

	return nil
}

// DeleteAPIKey 删除API密钥
func (m *APIKeyManager) DeleteAPIKey(apiKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	keyInfo, exists := m.apiKeys[apiKey]
	if !exists {
		return fmt.Errorf("无效的API密钥")
	}

	keyInfo.Status = models.APIKeyStatusDeleted
	return nil
}

// UpdateUsage 更新API密钥使用记录
func (m *APIKeyManager) UpdateUsage(apiKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	keyInfo, exists := m.apiKeys[apiKey]
	if !exists {
		return
	}

	now := time.Now()
	keyInfo.RequestCount++
	keyInfo.LastUsedAt = &now
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
