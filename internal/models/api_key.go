package models

import "time"

// APIKeyInfo API密钥信息
type APIKeyInfo struct {
	APIKey       string    `json:"api_key"`
	Platform     Platform  `json:"platform"`          // 绑定的AI平台
	Name         string    `json:"name"`              // 密钥名称（用户自定义）
	SessionID    string    `json:"session_id"`        // 关联的浏览器会话ID
	Status       APIKeyStatus `json:"status"`         // 密钥状态
	CreatedAt    time.Time `json:"created_at"`        // 创建时间
	LastUsedAt   *time.Time `json:"last_used_at"`     // 最后使用时间
	ExpiresAt    *time.Time `json:"expires_at"`       // 过期时间（可选）
	RequestCount int64     `json:"request_count"`     // 请求次数
}

// APIKey状态
type APIKeyStatus string

const (
	APIKeyStatusActive   APIKeyStatus = "active"    // 活跃状态
	APIKeyStatusInactive APIKeyStatus = "inactive"  // 停用状态
	APIKeyStatusExpired  APIKeyStatus = "expired"   // 已过期
	APIKeyStatusDeleted  APIKeyStatus = "deleted"   // 已删除
)

// 创建API密钥请求
type CreateAPIKeyRequest struct {
	Platform  Platform  `json:"platform"`           // 绑定的AI平台
	Name      string    `json:"name"`               // 密钥名称
	SessionID string    `json:"session_id"`         // 关联的会话ID（需已登录）
	ExpiresAt *time.Time `json:"expires_at"`        // 过期时间（可选）
}

// 创建API密钥响应
type CreateAPIKeyResponse struct {
	APIKeyInfo
	FullAPIKey string `json:"full_api_key"`  // 完整密钥（仅创建时返回一次）
}

// API密钥列表响应
type APIKeyListResponse struct {
	Total int           `json:"total"`
	Items []APIKeyInfo  `json:"items"`
}

// 更新API密钥请求
type UpdateAPIKeyRequest struct {
	Name      string       `json:"name,omitempty"`
	Status    *APIKeyStatus `json:"status,omitempty"`
}

// APIKey请求
type APIKeyAskRequest struct {
	Question string `json:"question"`
}
