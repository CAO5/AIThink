package memory

import (
	"log"

	"aithink/internal/models"
)

// ConversationAction 对话操作决策类型
type ConversationAction string

const (
	// ActionSendOnly 仅发送新需求（活跃对话，提示词未变化）
	ActionSendOnly ConversationAction = "send_only"
	// ActionCreateAndSend 新建对话并发送（首次创建）
	ActionCreateAndSend ConversationAction = "create_and_send"
	// ActionRebuildAndSend 重建对话并发送（过期/丢失/提示词变化）
	ActionRebuildAndSend ConversationAction = "rebuild_and_send"
)

// ConversationDecision 对话决策结果
// 包含操作决策、待发送消息、以及浏览器会话和重建相关的标志
type ConversationDecision struct {
	Action         ConversationAction // 操作决策
	Message        string            // 要发送的完整消息
	NeedNewBrowser bool              // 是否需要新建浏览器会话
	NeedRebuild    bool              // 是否需要重建对话
}

// ConversationManager 对话生命周期管理器
// 负责跟踪对话状态、处理超时、决定何时重建对话。
// 仅依赖 MemoryManager，不依赖 browser 和 service 包，
// 浏览器会话管理由 AIService 层负责。
type ConversationManager struct {
	memoryManager *MemoryManager
}

// NewConversationManager 创建对话生命周期管理器
// memoryManager 为记忆管理器实例，不能为nil
func NewConversationManager(memoryManager *MemoryManager) *ConversationManager {
	if memoryManager == nil {
		log.Printf("[ConversationManager] 警告: MemoryManager为nil，对话管理功能将不可用")
	}
	return &ConversationManager{
		memoryManager: memoryManager,
	}
}

// Decide 核心决策方法，根据对话状态决定操作
//
// 决策逻辑：
//   - 对话active且PromptHash与当前一致 → ActionSendOnly，仅发送CurrentRequest
//   - 对话active但PromptHash变化 → ActionRebuildAndSend，重建对话（提示词变了）
//   - 对话expired/lost → ActionRebuildAndSend，重建对话
//   - 没有对话 → ActionCreateAndSend，新建对话
//
// 对于新建/重建的对话，会自动设置固定提示词并组装完整消息
func (cm *ConversationManager) Decide(apiKey string, platform models.Platform, parsedReq *ParsedRequest) *ConversationDecision {
	if cm.memoryManager == nil {
		// MemoryManager不可用，默认新建对话
		log.Printf("[ConversationManager] MemoryManager不可用，默认新建对话")
		message := ""
		if parsedReq != nil {
			message = parsedReq.CurrentRequest
		}
		return &ConversationDecision{
			Action:         ActionCreateAndSend,
			Message:        message,
			NeedNewBrowser: true,
			NeedRebuild:    false,
		}
	}

	// 第一步：检查对话是否超时，超时则标记为expired
	if cm.memoryManager.CheckConversationTimeout(apiKey) {
		cm.memoryManager.MarkConversationExpired(apiKey)
		log.Printf("[ConversationManager] 对话超时，已标记为expired: apiKey=%s...%s",
			maskAPIKey(apiKey), apiKey[len(apiKey)-4:])
	}

	// 第二步：尝试获取活跃对话
	activeConv := cm.memoryManager.GetActiveConversation(apiKey)
	if activeConv != nil {
		return cm.decideForActiveConversation(apiKey, platform, parsedReq, activeConv)
	}

	// 第三步：没有活跃对话，检查是否存在非活跃对话
	convState := cm.memoryManager.GetConversationState(apiKey)
	if convState != nil {
		return cm.decideForInactiveConversation(apiKey, platform, parsedReq, convState)
	}

	// 第四步：没有任何对话，新建对话
	return cm.decideForNewConversation(apiKey, platform, parsedReq)
}

// decideForActiveConversation 活跃对话的决策逻辑
// 比较提示词指纹，决定是仅发送当前请求还是重建对话
func (cm *ConversationManager) decideForActiveConversation(
	apiKey string,
	platform models.Platform,
	parsedReq *ParsedRequest,
	activeConv *ConversationState,
) *ConversationDecision {
	// 获取当前请求的提示词指纹
	currentHash := ""
	if parsedReq != nil {
		currentHash = parsedReq.PromptHash
	}

	// 提示词未变化，仅发送当前请求
	if activeConv.PromptHash == currentHash {
		message := ""
		if parsedReq != nil {
			message = parsedReq.CurrentRequest
		}
		log.Printf("[ConversationManager] 活跃对话，提示词未变化，仅发送当前请求: apiKey=%s...%s",
			maskAPIKey(apiKey), apiKey[len(apiKey)-4:])
		return &ConversationDecision{
			Action:         ActionSendOnly,
			Message:        message,
			NeedNewBrowser: false,
			NeedRebuild:    false,
		}
	}

	// 提示词变化，需要重建对话
	log.Printf("[ConversationManager] 活跃对话但提示词变化，需要重建: apiKey=%s...%s, 旧hash=%s, 新hash=%s",
		maskAPIKey(apiKey), apiKey[len(apiKey)-4:], activeConv.PromptHash, currentHash)

	// 标记为过期，然后通过GetOrCreateConversation重置为活跃状态
	// GetOrCreateConversation会保留记忆，但重置SessionID、ConversationID、FixedPrompt、PromptHash
	cm.memoryManager.MarkConversationExpired(apiKey)
	cm.memoryManager.GetOrCreateConversation(apiKey, platform)

	// 设置新的固定提示词（用于下次请求时的PromptHash比对）
	if parsedReq != nil {
		cm.memoryManager.SetFixedPrompt(apiKey, parsedReq.FixedPrompt, parsedReq.PromptHash)
	}

	// 组装完整消息（固定提示词+记忆+当前请求）
	message := cm.memoryManager.BuildMessage(apiKey, parsedReq)

	return &ConversationDecision{
		Action:         ActionRebuildAndSend,
		Message:        message,
		NeedNewBrowser: false, // 浏览器会话可能仍然有效，由AIService层判断
		NeedRebuild:    true,
	}
}

// decideForInactiveConversation 非活跃对话（expired/lost）的决策逻辑
// 重置对话状态为活跃，保留记忆，重新组装消息
func (cm *ConversationManager) decideForInactiveConversation(
	apiKey string,
	platform models.Platform,
	parsedReq *ParsedRequest,
	convState *ConversationState,
) *ConversationDecision {
	log.Printf("[ConversationManager] 对话状态为%s，需要重建: apiKey=%s...%s",
		convState.Status, maskAPIKey(apiKey), apiKey[len(apiKey)-4:])

	// 通过GetOrCreateConversation重置对话状态为活跃
	// 保留记忆，重置SessionID、ConversationID、FixedPrompt、PromptHash
	cm.memoryManager.GetOrCreateConversation(apiKey, platform)

	// 设置新的固定提示词
	if parsedReq != nil {
		cm.memoryManager.SetFixedPrompt(apiKey, parsedReq.FixedPrompt, parsedReq.PromptHash)
	}

	// 组装完整消息
	message := cm.memoryManager.BuildMessage(apiKey, parsedReq)

	return &ConversationDecision{
		Action:         ActionRebuildAndSend,
		Message:        message,
		NeedNewBrowser: true, // 过期/丢失的对话，浏览器会话很可能也失效了
		NeedRebuild:    true,
	}
}

// decideForNewConversation 全新对话的决策逻辑
// 创建新对话状态，组装完整消息
func (cm *ConversationManager) decideForNewConversation(
	apiKey string,
	platform models.Platform,
	parsedReq *ParsedRequest,
) *ConversationDecision {
	log.Printf("[ConversationManager] 无对话记录，新建对话: apiKey=%s...%s, platform=%s",
		maskAPIKey(apiKey), apiKey[len(apiKey)-4:], platform)

	// 创建新对话
	cm.memoryManager.GetOrCreateConversation(apiKey, platform)

	// 设置固定提示词
	if parsedReq != nil {
		cm.memoryManager.SetFixedPrompt(apiKey, parsedReq.FixedPrompt, parsedReq.PromptHash)
	}

	// 组装完整消息
	message := cm.memoryManager.BuildMessage(apiKey, parsedReq)

	return &ConversationDecision{
		Action:         ActionCreateAndSend,
		Message:        message,
		NeedNewBrowser: true,
		NeedRebuild:    false,
	}
}

// HandlePostAsk 请求完成后的处理
//   - 将AI回复添加到记忆
//   - 更新对话活跃时间
//   - 如果提供了sessionID/conversationID，更新对话ID（适用于新建/重建的对话）
//   - 精简记忆（防止记忆过多）
func (cm *ConversationManager) HandlePostAsk(apiKey string, answer string, sessionID string, conversationID string) {
	if cm.memoryManager == nil {
		log.Printf("[ConversationManager] MemoryManager不可用，跳过PostAsk处理")
		return
	}

	// 1. 将AI回复添加到记忆
	if answer != "" {
		cm.memoryManager.AddMemory(apiKey, answer)
		log.Printf("[ConversationManager] AI回复已添加到记忆: apiKey=%s...%s",
			maskAPIKey(apiKey), apiKey[len(apiKey)-4:])
	}

	// 2. 更新对话活跃时间
	cm.memoryManager.UpdateConversationActivity(apiKey)

	// 3. 更新SessionID和ConversationID（适用于新建/重建的对话）
	if sessionID != "" || conversationID != "" {
		cm.memoryManager.UpdateConversationIDs(apiKey, sessionID, conversationID)
		log.Printf("[ConversationManager] 已更新对话ID: apiKey=%s...%s, sessionID=%s, conversationID=%s",
			maskAPIKey(apiKey), apiKey[len(apiKey)-4:], sessionID, conversationID)
	}

	// 4. 精简记忆（防止记忆过多）
	cm.memoryManager.CompactMemories(apiKey)
}

// ResetConversation 重置对话
// 标记为expired，清除对话ID，下次请求时会自动重建
func (cm *ConversationManager) ResetConversation(apiKey string) {
	if cm.memoryManager == nil {
		log.Printf("[ConversationManager] MemoryManager不可用，跳过重置操作")
		return
	}

	// 标记为过期
	cm.memoryManager.MarkConversationExpired(apiKey)

	// 清除对话ID
	cm.memoryManager.ClearConversationIDs(apiKey)

	log.Printf("[ConversationManager] 对话已重置: apiKey=%s...%s",
		maskAPIKey(apiKey), apiKey[len(apiKey)-4:])
}

// GetConversationInfo 获取对话信息（只读副本）
// 返回对话状态的深拷贝，避免外部修改内部状态
func (cm *ConversationManager) GetConversationInfo(apiKey string) *ConversationState {
	if cm.memoryManager == nil {
		return nil
	}

	conv := cm.memoryManager.GetConversationState(apiKey)
	if conv == nil {
		return nil
	}

	// 创建深拷贝，防止外部修改影响内部状态
	result := &ConversationState{
		APIKey:         conv.APIKey,
		Platform:       conv.Platform,
		SessionID:      conv.SessionID,
		ConversationID: conv.ConversationID,
		FixedPrompt:    conv.FixedPrompt,
		PromptHash:     conv.PromptHash,
		Status:         conv.Status,
		LastActiveAt:   conv.LastActiveAt,
		CreatedAt:      conv.CreatedAt,
	}

	// 深拷贝记忆列表
	if len(conv.Memories) > 0 {
		result.Memories = make([]MemoryEntry, len(conv.Memories))
		copy(result.Memories, conv.Memories)
	}

	return result
}
