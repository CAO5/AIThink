package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"aithink/internal/models"
)

// 全局记忆管理器单例
var (
	globalManager     *MemoryManager
	globalManagerOnce sync.Once
)

// GetGlobalMemoryManager 获取全局记忆管理器单例
// 首次调用时初始化记忆存储和管理器，并启动清理循环
// 后续调用返回同一实例，确保 Handler 和 Gateway 共享同一个 MemoryManager
func GetGlobalMemoryManager() *MemoryManager {
	globalManagerOnce.Do(func() {
		store := NewMemoryStore("data/memory")
		globalManager = NewMemoryManager(store, MemoryConfig{
			MaxEntries:          20,
			RepeatThreshold:     3,
			ConversationTimeout: 30 * time.Minute,
		})
		globalManager.StartCleanupLoop()
	})
	return globalManager
}

// MemoryEntry 记忆条目
// 表示一条被记录的对话记忆，包含内容、指纹、重复计数等信息
type MemoryEntry struct {
	ID          string    `json:"id"`            // 唯一ID
	Content     string    `json:"content"`       // 记忆内容
	Fingerprint string    `json:"fingerprint"`   // 内容指纹（SHA256前8位）
	RepeatCount int       `json:"repeat_count"`  // 重复出现次数
	IsCompacted bool      `json:"is_compacted"`  // 是否已精简
	CreatedAt   time.Time `json:"created_at"`    // 创建时间
	LastUsedAt  time.Time `json:"last_used_at"`  // 最后使用时间
}

// ConvStatus 对话状态类型
type ConvStatus string

const (
	ConvStatusActive  ConvStatus = "active"  // 活跃
	ConvStatusExpired ConvStatus = "expired" // 过期
	ConvStatusLost    ConvStatus = "lost"    // 丢失
)

// ConversationState 对话状态
// 记录一个API Key关联的完整对话上下文，包括平台信息、记忆列表和状态
type ConversationState struct {
	APIKey         string         `json:"api_key"`          // 关联的API Key
	Platform       models.Platform `json:"platform"`        // 平台类型
	SessionID      string         `json:"session_id"`       // 浏览器会话ID
	ConversationID string         `json:"conversation_id"`  // 浏览器内对话ID
	FixedPrompt    string         `json:"fixed_prompt"`     // 已发送的固定提示词
	PromptHash     string         `json:"prompt_hash"`      // 提示词指纹
	Memories       []MemoryEntry  `json:"memories"`         // 记忆列表
	Status         ConvStatus     `json:"status"`           // 对话状态
	LastActiveAt   time.Time      `json:"last_active_at"`   // 最后活跃时间
	CreatedAt      time.Time      `json:"created_at"`       // 创建时间
}

// MemoryConfig 记忆管理配置
type MemoryConfig struct {
	MaxEntries          int           `json:"max_entries"`           // 记忆条目上限（默认20）
	RepeatThreshold     int           `json:"repeat_threshold"`      // 重复阈值（默认3）
	ConversationTimeout time.Duration `json:"conversation_timeout"`  // 对话超时时间（默认30分钟）
}

// DefaultMemoryConfig 返回默认记忆管理配置
func DefaultMemoryConfig() MemoryConfig {
	return MemoryConfig{
		MaxEntries:          20,
		RepeatThreshold:     3,
		ConversationTimeout: 30 * time.Minute,
	}
}

// MemoryManager 记忆管理器
// 管理所有API Key对应的对话状态和记忆条目，支持并发安全访问
type MemoryManager struct {
	mu            sync.RWMutex
	store         *MemoryStore
	config        MemoryConfig
	conversations map[string]*ConversationState // apiKey -> ConversationState
	stopCh        chan struct{}                 // 停止信号通道
	stopped       bool                          // 是否已停止
}

// NewMemoryManager 创建记忆管理器实例
// store 为持久化存储，config 为配置项
// 启动时会从存储中加载已有的对话状态
func NewMemoryManager(store *MemoryStore, config MemoryConfig) *MemoryManager {
	mgr := &MemoryManager{
		store:         store,
		config:        config,
		conversations: make(map[string]*ConversationState),
		stopCh:        make(chan struct{}),
	}

	// 从持久化存储加载已有数据
	if store != nil {
		loaded, err := store.LoadAll()
		if err != nil {
			log.Printf("[MemoryManager] 加载持久化数据失败: %v", err)
		} else if len(loaded) > 0 {
			mgr.conversations = loaded
			log.Printf("[MemoryManager] 已加载 %d 个对话状态", len(loaded))
		}
	}

	return mgr
}

// GetOrCreateConversation 获取或创建对话状态
// 如果已有对话且状态为active，直接返回
// 如果已有对话但状态为expired/lost，标记需要重建
// 如果没有对话，创建新的ConversationState
func (m *MemoryManager) GetOrCreateConversation(apiKey string, platform models.Platform) *ConversationState {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv, exists := m.conversations[apiKey]
	if exists {
		switch conv.Status {
		case ConvStatusActive:
			// 活跃对话直接返回
			log.Printf("[MemoryManager] 返回活跃对话: apiKey=%s...%s", maskAPIKey(apiKey), apiKey[len(apiKey)-4:])
			return conv
		case ConvStatusExpired, ConvStatusLost:
			// 过期或丢失的对话需要重建
			log.Printf("[MemoryManager] 对话状态为%s，需要重建: apiKey=%s...%s",
				conv.Status, maskAPIKey(apiKey), apiKey[len(apiKey)-4:])
			// 保留记忆，重置其他状态
			conv.Status = ConvStatusActive
			conv.Platform = platform
			conv.SessionID = ""
			conv.ConversationID = ""
			conv.FixedPrompt = ""
			conv.PromptHash = ""
			conv.LastActiveAt = time.Now()
			return conv
		}
	}

	// 创建新对话
	now := time.Now()
	conv = &ConversationState{
		APIKey:       apiKey,
		Platform:     platform,
		Memories:     []MemoryEntry{},
		Status:       ConvStatusActive,
		LastActiveAt: now,
		CreatedAt:    now,
	}
	m.conversations[apiKey] = conv

	log.Printf("[MemoryManager] 创建新对话: apiKey=%s...%s, platform=%s",
		maskAPIKey(apiKey), apiKey[len(apiKey)-4:], platform)

	return conv
}

// GetActiveConversation 获取活跃的对话状态
// 如果不存在或非active状态，返回nil
func (m *MemoryManager) GetActiveConversation(apiKey string) *ConversationState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conv, exists := m.conversations[apiKey]
	if !exists || conv.Status != ConvStatusActive {
		return nil
	}
	return conv
}

// AddMemory 添加记忆条目
// 计算内容指纹，如果已有相同指纹的记忆则增加重复计数，否则新增记忆条目
func (m *MemoryManager) AddMemory(apiKey string, content string) MemoryEntry {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv, exists := m.conversations[apiKey]
	if !exists {
		// 如果对话不存在，创建一个新的
		now := time.Now()
		conv = &ConversationState{
			APIKey:       apiKey,
			Memories:     []MemoryEntry{},
			Status:       ConvStatusActive,
			LastActiveAt: now,
			CreatedAt:    now,
		}
		m.conversations[apiKey] = conv
	}

	fingerprint := m.computeFingerprint(content)
	now := time.Now()

	// 检查是否已有相同指纹的记忆
	for i := range conv.Memories {
		if conv.Memories[i].Fingerprint == fingerprint {
			// 已存在相同指纹，增加重复计数并更新最后使用时间
			conv.Memories[i].RepeatCount++
			conv.Memories[i].LastUsedAt = now
			log.Printf("[MemoryManager] 记忆重复命中: apiKey=%s...%s, fingerprint=%s, repeatCount=%d",
				maskAPIKey(apiKey), apiKey[len(apiKey)-4:], fingerprint, conv.Memories[i].RepeatCount)
			return conv.Memories[i]
		}
	}

	// 新增记忆条目
	entry := MemoryEntry{
		ID:          m.generateEntryID(),
		Content:     content,
		Fingerprint: fingerprint,
		RepeatCount: 1,
		IsCompacted: false,
		CreatedAt:   now,
		LastUsedAt:  now,
	}
	conv.Memories = append(conv.Memories, entry)

	log.Printf("[MemoryManager] 新增记忆: apiKey=%s...%s, id=%s, fingerprint=%s",
		maskAPIKey(apiKey), apiKey[len(apiKey)-4:], entry.ID, fingerprint)

	return entry
}

// CompactMemories 精简记忆列表
// 1. RepeatCount > config.RepeatThreshold 的：保留最近一条完整内容，旧条目精简为首行摘要，标记IsCompacted=true
// 2. 总条数 > config.MaxEntries 时：按LRU策略淘汰最久未使用的
func (m *MemoryManager) CompactMemories(apiKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv, exists := m.conversations[apiKey]
	if !exists || len(conv.Memories) == 0 {
		return
	}

	compactedCount := 0

	// 第一轮：精简重复次数超过阈值的记忆
	for i := range conv.Memories {
		if conv.Memories[i].RepeatCount > m.config.RepeatThreshold && !conv.Memories[i].IsCompacted {
			// 精简为首行摘要
			content := conv.Memories[i].Content
			firstLine := extractFirstLine(content)
			if firstLine != content {
				conv.Memories[i].Content = firstLine
				conv.Memories[i].IsCompacted = true
				compactedCount++
			}
		}
	}

	// 第二轮：如果总条数超过上限，按LRU策略淘汰
	if len(conv.Memories) > m.config.MaxEntries {
		// 按LastUsedAt排序（升序），最久未使用的排在前面
		sort.Slice(conv.Memories, func(i, j int) bool {
			return conv.Memories[i].LastUsedAt.Before(conv.Memories[j].LastUsedAt)
		})

		// 保留最近使用的条目
		removeCount := len(conv.Memories) - m.config.MaxEntries
		conv.Memories = conv.Memories[removeCount:]

		log.Printf("[MemoryManager] LRU淘汰: apiKey=%s...%s, 淘汰=%d条, 保留=%d条",
			maskAPIKey(apiKey), apiKey[len(apiKey)-4:], removeCount, len(conv.Memories))
	}

	if compactedCount > 0 {
		log.Printf("[MemoryManager] 记忆精简: apiKey=%s...%s, 精简=%d条",
			maskAPIKey(apiKey), apiKey[len(apiKey)-4:], compactedCount)
	}
}

// MarkConversationExpired 标记对话为过期状态
func (m *MemoryManager) MarkConversationExpired(apiKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv, exists := m.conversations[apiKey]
	if !exists {
		return
	}

	conv.Status = ConvStatusExpired
	log.Printf("[MemoryManager] 对话已标记为过期: apiKey=%s...%s", maskAPIKey(apiKey), apiKey[len(apiKey)-4:])
}

// MarkConversationLost 标记对话为丢失状态
func (m *MemoryManager) MarkConversationLost(apiKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv, exists := m.conversations[apiKey]
	if !exists {
		return
	}

	conv.Status = ConvStatusLost
	log.Printf("[MemoryManager] 对话已标记为丢失: apiKey=%s...%s", maskAPIKey(apiKey), apiKey[len(apiKey)-4:])
}

// UpdateConversationActivity 更新对话的最后活跃时间
func (m *MemoryManager) UpdateConversationActivity(apiKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv, exists := m.conversations[apiKey]
	if !exists {
		return
	}

	conv.LastActiveAt = time.Now()
}

// UpdateConversationIDs 更新对话的SessionID和ConversationID
// 用于新建/重建对话后更新浏览器返回的会话标识
// 传入空字符串的字段不会被更新，支持只更新其中一个ID
func (m *MemoryManager) UpdateConversationIDs(apiKey string, sessionID string, conversationID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv, exists := m.conversations[apiKey]
	if !exists {
		return
	}

	if sessionID != "" {
		conv.SessionID = sessionID
	}
	if conversationID != "" {
		conv.ConversationID = conversationID
	}

	log.Printf("[MemoryManager] 更新对话ID: apiKey=%s...%s, sessionID=%s, conversationID=%s",
		maskAPIKey(apiKey), apiKey[len(apiKey)-4:], sessionID, conversationID)
}

// ClearConversationIDs 清除对话的SessionID和ConversationID
// 用于重置对话时清除旧的会话标识，下次请求时会自动重建
func (m *MemoryManager) ClearConversationIDs(apiKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv, exists := m.conversations[apiKey]
	if !exists {
		return
	}

	conv.SessionID = ""
	conv.ConversationID = ""

	log.Printf("[MemoryManager] 清除对话ID: apiKey=%s...%s", maskAPIKey(apiKey), apiKey[len(apiKey)-4:])
}

// SetFixedPrompt 设置对话的固定提示词
// prompt 为提示词内容，promptHash 为提示词指纹
func (m *MemoryManager) SetFixedPrompt(apiKey string, prompt string, promptHash string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv, exists := m.conversations[apiKey]
	if !exists {
		return
	}

	conv.FixedPrompt = prompt
	conv.PromptHash = promptHash
	log.Printf("[MemoryManager] 设置固定提示词: apiKey=%s...%s, hash=%s",
		maskAPIKey(apiKey), apiKey[len(apiKey)-4:], promptHash)
}

// BuildMessage 根据对话状态组装发送内容
// active对话 → 仅发送 CurrentRequest
// 新建/重建对话 → 固定提示词 + 精简记忆 + CurrentRequest
// 记忆格式：每条记忆前加 [记忆N] 前缀
func (m *MemoryManager) BuildMessage(apiKey string, parsedReq *ParsedRequest) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conv, exists := m.conversations[apiKey]
	if !exists || parsedReq == nil {
		// 没有对话状态或请求为空，直接返回当前请求
		if parsedReq != nil {
			return parsedReq.CurrentRequest
		}
		return ""
	}

	// 活跃对话：仅发送当前请求
	if conv.Status == ConvStatusActive && conv.ConversationID != "" {
		log.Printf("[MemoryManager] 活跃对话，仅发送当前请求: apiKey=%s...%s",
			maskAPIKey(apiKey), apiKey[len(apiKey)-4:])
		return parsedReq.CurrentRequest
	}

	// 新建/重建对话：组装完整消息
	var parts []string

	// 1. 固定提示词
	if parsedReq.FixedPrompt != "" {
		parts = append(parts, parsedReq.FixedPrompt)
	}

	// 2. 精简记忆
	if len(conv.Memories) > 0 {
		var memoryLines []string
		for i, mem := range conv.Memories {
			memoryLines = append(memoryLines, fmt.Sprintf("[记忆%d] %s", i+1, mem.Content))
		}
		parts = append(parts, strings.Join(memoryLines, "\n"))
	}

	// 3. 当前请求
	if parsedReq.CurrentRequest != "" {
		parts = append(parts, parsedReq.CurrentRequest)
	}

	log.Printf("[MemoryManager] 新建/重建对话，组装完整消息: apiKey=%s...%s, 记忆数=%d",
		maskAPIKey(apiKey), apiKey[len(apiKey)-4:], len(conv.Memories))

	return strings.Join(parts, "\n")
}

// CheckConversationTimeout 检查对话是否超时
// 返回true表示已超时
func (m *MemoryManager) CheckConversationTimeout(apiKey string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conv, exists := m.conversations[apiKey]
	if !exists {
		return false
	}

	// 非活跃状态不需要检查超时
	if conv.Status != ConvStatusActive {
		return false
	}

	// 检查是否超过超时时间
	timeout := m.config.ConversationTimeout
	if timeout <= 0 {
		timeout = 30 * time.Minute // 默认30分钟
	}

	elapsed := time.Since(conv.LastActiveAt)
	if elapsed > timeout {
		log.Printf("[MemoryManager] 对话超时: apiKey=%s...%s, 已空闲=%v, 超时阈值=%v",
			maskAPIKey(apiKey), apiKey[len(apiKey)-4:], elapsed, timeout)
		return true
	}

	return false
}

// GetConversationState 获取对话状态（只读）
// 返回对话状态的副本，避免外部修改内部状态
func (m *MemoryManager) GetConversationState(apiKey string) *ConversationState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conv, exists := m.conversations[apiKey]
	if !exists {
		return nil
	}
	return conv
}

// ClearMemory 清除指定API Key的所有记忆
// 同时从持久化存储中删除对应文件
func (m *MemoryManager) ClearMemory(apiKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv, exists := m.conversations[apiKey]
	if !exists {
		return
	}

	// 清空记忆列表
	conv.Memories = []MemoryEntry{}
	conv.FixedPrompt = ""
	conv.PromptHash = ""

	// 从持久化存储中删除
	if m.store != nil {
		if err := m.store.DeleteConversation(apiKey); err != nil {
			log.Printf("[MemoryManager] 删除持久化文件失败: apiKey=%s...%s, err=%v",
				maskAPIKey(apiKey), apiKey[len(apiKey)-4:], err)
		}
	}

	log.Printf("[MemoryManager] 已清除记忆: apiKey=%s...%s", maskAPIKey(apiKey), apiKey[len(apiKey)-4:])
}

// computeFingerprint 计算内容的指纹
// 使用SHA256哈希取前8位，用于判断内容是否相同
func (m *MemoryManager) computeFingerprint(content string) string {
	// 预处理：去除首尾空白，统一换行符
	normalized := strings.TrimSpace(content)
	normalized = strings.ReplaceAll(normalized, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	// 空内容返回空指纹
	if normalized == "" {
		return ""
	}

	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:])[:8]
}

// generateEntryID 生成唯一的记忆条目ID
// 格式：mem_{纳秒时间戳}
func (m *MemoryManager) generateEntryID() string {
	return fmt.Sprintf("mem_%d", time.Now().UnixNano())
}

// StartCleanupLoop 启动定期清理协程
// 每分钟检查一次对话超时，超时的标记为expired
// 定期保存所有对话状态到持久化存储
func (m *MemoryManager) StartCleanupLoop() {
	go func() {
		// 超时检查间隔：1分钟
		timeoutTicker := time.NewTicker(1 * time.Minute)
		defer timeoutTicker.Stop()

		// 持久化保存间隔：5分钟
		saveTicker := time.NewTicker(5 * time.Minute)
		defer saveTicker.Stop()

		log.Printf("[MemoryManager] 清理协程已启动")

		for {
			select {
			case <-timeoutTicker.C:
				m.checkAllTimeouts()
			case <-saveTicker.C:
				m.saveAllConversations()
			case <-m.stopCh:
				log.Printf("[MemoryManager] 清理协程收到停止信号")
				return
			}
		}
	}()
}

// Stop 停止清理协程并保存所有数据
func (m *MemoryManager) Stop() {
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return
	}
	m.stopped = true
	m.mu.Unlock()

	// 发送停止信号
	close(m.stopCh)

	// 保存所有数据
	m.saveAllConversations()

	log.Printf("[MemoryManager] 记忆管理器已停止")
}

// checkAllTimeouts 检查所有对话的超时状态
func (m *MemoryManager) checkAllTimeouts() {
	m.mu.Lock()
	defer m.mu.Unlock()

	timeout := m.config.ConversationTimeout
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}

	now := time.Now()
	expiredCount := 0

	for apiKey, conv := range m.conversations {
		if conv.Status != ConvStatusActive {
			continue
		}

		if now.Sub(conv.LastActiveAt) > timeout {
			conv.Status = ConvStatusExpired
			expiredCount++
			log.Printf("[MemoryManager] 对话超时，标记为expired: apiKey=%s...%s, 空闲时间=%v",
				maskAPIKey(apiKey), apiKey[len(apiKey)-4:], now.Sub(conv.LastActiveAt))
		}
	}

	if expiredCount > 0 {
		log.Printf("[MemoryManager] 超时检查完成: 过期=%d", expiredCount)
	}
}

// saveAllConversations 保存所有对话状态到持久化存储
func (m *MemoryManager) saveAllConversations() {
	if m.store == nil {
		return
	}

	m.mu.RLock()
	// 复制一份map避免长时间持锁
	snapshot := make(map[string]*ConversationState, len(m.conversations))
	for k, v := range m.conversations {
		snapshot[k] = v
	}
	m.mu.RUnlock()

	if err := m.store.SaveAll(snapshot); err != nil {
		log.Printf("[MemoryManager] 保存所有对话状态失败: %v", err)
	}
}

// extractFirstLine 提取内容的第一行作为摘要
// 用于记忆精简时保留关键信息
func extractFirstLine(content string) string {
	// 按换行符分割，取第一行
	lines := strings.SplitN(content, "\n", 2)
	firstLine := strings.TrimSpace(lines[0])

	// 如果第一行过长，截断到200字符
	if len(firstLine) > 200 {
		firstLine = firstLine[:200] + "..."
	}

	return firstLine
}
