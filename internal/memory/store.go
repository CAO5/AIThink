package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// MemoryStore 记忆持久化存储
// 负责将对话状态以JSON文件形式持久化到磁盘
type MemoryStore struct {
	mu      sync.RWMutex
	dataDir string // 存储目录
}

// NewMemoryStore 创建记忆持久化存储实例
// dataDir 为存储目录路径，如果目录不存在会自动创建
func NewMemoryStore(dataDir string) *MemoryStore {
	store := &MemoryStore{
		dataDir: dataDir,
	}

	// 确保存储目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("[MemoryStore] 创建存储目录失败: %v", err)
	} else {
		log.Printf("[MemoryStore] 存储目录已就绪: %s", dataDir)
	}

	return store
}

// SaveConversation 保存单个对话状态到JSON文件
// 对apiKey做SHA256哈希取前16位作为文件名，避免直接暴露API Key
func (s *MemoryStore) SaveConversation(state *ConversationState) error {
	if state == nil {
		return fmt.Errorf("对话状态不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 序列化为JSON
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化对话状态失败: %v", err)
	}

	// 生成文件路径
	filePath := filepath.Join(s.dataDir, s.apiKeyToFileName(state.APIKey))

	// 写入文件（原子写入：先写临时文件再重命名）
	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("写入临时文件失败: %v", err)
	}

	// 重命名临时文件为目标文件（原子操作）
	if err := os.Rename(tmpPath, filePath); err != nil {
		// 重命名失败时清理临时文件
		os.Remove(tmpPath)
		return fmt.Errorf("重命名文件失败: %v", err)
	}

	log.Printf("[MemoryStore] 已保存对话状态: apiKey=%s...%s, file=%s",
		maskAPIKey(state.APIKey), state.APIKey[len(state.APIKey)-4:], filepath.Base(filePath))

	return nil
}

// LoadConversation 根据apiKey加载对话状态
// 通过apiKey哈希查找对应的JSON文件并反序列化
func (s *MemoryStore) LoadConversation(apiKey string) (*ConversationState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filePath := filepath.Join(s.dataDir, s.apiKeyToFileName(apiKey))

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // 文件不存在时返回nil而非错误
		}
		return nil, fmt.Errorf("读取对话文件失败: %v", err)
	}

	var state ConversationState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("反序列化对话状态失败: %v", err)
	}

	return &state, nil
}

// DeleteConversation 删除指定API Key对应的对话文件
func (s *MemoryStore) DeleteConversation(apiKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := filepath.Join(s.dataDir, s.apiKeyToFileName(apiKey))

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在不算错误
		}
		return fmt.Errorf("删除对话文件失败: %v", err)
	}

	log.Printf("[MemoryStore] 已删除对话文件: apiKey=%s...%s",
		maskAPIKey(apiKey), apiKey[len(apiKey)-4:])

	return nil
}

// ListConversations 列出所有已存储的API Key哈希
// 返回文件名列表（不含.json后缀），即API Key的SHA256哈希前16位
func (s *MemoryStore) ListConversations() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, fmt.Errorf("读取存储目录失败: %v", err)
	}

	var hashes []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// 只匹配.json文件
		if filepath.Ext(name) == ".json" {
			hash := name[:len(name)-5] // 去掉.json后缀
			hashes = append(hashes, hash)
		}
	}

	return hashes, nil
}

// SaveAll 保存所有对话状态
// 遍历conversations map，逐个保存每个对话状态
func (s *MemoryStore) SaveAll(conversations map[string]*ConversationState) error {
	if conversations == nil {
		return nil
	}

	var lastErr error
	savedCount := 0

	for apiKey, state := range conversations {
		if state == nil {
			continue
		}
		// 确保APIKey字段正确
		state.APIKey = apiKey
		if err := s.SaveConversation(state); err != nil {
			log.Printf("[MemoryStore] 保存对话失败: apiKey=%s...%s, err=%v",
				maskAPIKey(apiKey), apiKey[len(apiKey)-4:], err)
			lastErr = err
		} else {
			savedCount++
		}
	}

	log.Printf("[MemoryStore] 批量保存完成: 成功=%d, 总数=%d", savedCount, len(conversations))

	return lastErr
}

// LoadAll 加载所有对话状态
// 读取存储目录下所有JSON文件，反序列化为ConversationState
func (s *MemoryStore) LoadAll() (map[string]*ConversationState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, fmt.Errorf("读取存储目录失败: %v", err)
	}

	conversations := make(map[string]*ConversationState)
	loadErrors := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}

		filePath := filepath.Join(s.dataDir, name)
		data, err := os.ReadFile(filePath)
		if err != nil {
			log.Printf("[MemoryStore] 读取文件失败: %s, err=%v", name, err)
			loadErrors++
			continue
		}

		var state ConversationState
		if err := json.Unmarshal(data, &state); err != nil {
			log.Printf("[MemoryStore] 反序列化失败: %s, err=%v", name, err)
			loadErrors++
			continue
		}

		// 使用文件中存储的APIKey作为map的键
		if state.APIKey != "" {
			conversations[state.APIKey] = &state
		}
	}

	log.Printf("[MemoryStore] 批量加载完成: 成功=%d, 失败=%d", len(conversations), loadErrors)

	return conversations, nil
}

// apiKeyToFileName 将API Key转换为文件名
// 使用SHA256哈希前16位 + ".json"后缀，避免文件名直接暴露API Key
func (s *MemoryStore) apiKeyToFileName(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])[:16] + ".json"
}

// maskAPIKey 遮蔽API Key中间部分，仅显示前4位和后4位
// 用于日志输出，避免完整暴露API Key
func maskAPIKey(apiKey string) string {
	if len(apiKey) <= 8 {
		return "****"
	}
	return apiKey[:4] + "****"
}
