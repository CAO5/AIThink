package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"aithink/internal/memory"
	"aithink/internal/models"
	_ "aithink/internal/platform/qwen"  // 确保千问平台注册（init）
	_ "aithink/internal/platform/zhipu" // 确保智谱平台注册（init）
	"aithink/internal/service"

	"github.com/gin-gonic/gin"
)

// AnthropicMessageContent 消息内容块（支持text、image、tool_use、tool_result）
type AnthropicMessageContent struct {
	Type      string                    `json:"type"`
	Text      string                    `json:"text,omitempty"`
	Source    *AnthropicContentSource   `json:"source,omitempty"`
	ID        string                    `json:"id,omitempty"`
	Name      string                    `json:"name,omitempty"`
	Input     map[string]interface{}    `json:"input,omitempty"`
	ToolUseID string                    `json:"tool_use_id,omitempty"`
	Output    []AnthropicMessageContent `json:"output,omitempty"`
}

// AnthropicContentSource 图片/文件源
type AnthropicContentSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

// AnthropicMessage Anthropic消息格式
type AnthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

// AnthropicChatRequest Anthropic聊天请求
type AnthropicChatRequest struct {
	Model         string                `json:"model"`
	Messages      []AnthropicMessage    `json:"messages"`
	MaxTokens     int                   `json:"max_tokens"`
	Temperature   *float64              `json:"temperature,omitempty"`
	TopP          *float64              `json:"top_p,omitempty"`
	TopK          *int                  `json:"top_k,omitempty"`
	Stream        bool                  `json:"stream,omitempty"`
	System        interface{}           `json:"system,omitempty"`
	StopSequences []string              `json:"stop_sequences,omitempty"`
	Tools         []AnthropicTool       `json:"tools,omitempty"`
	ToolChoice    *AnthropicToolChoice  `json:"tool_choice,omitempty"`
	Metadata      *AnthropicMetadata    `json:"metadata,omitempty"`
	Thinking      *AnthropicThinking    `json:"thinking,omitempty"`
}

// AnthropicThinking 扩展思考配置
type AnthropicThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

// AnthropicMetadata 请求元数据（cc-Switch/Claude Code会发送）
type AnthropicMetadata struct {
	UserID string `json:"user_id,omitempty"`
}

// AnthropicTool 工具定义
type AnthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// AnthropicToolChoice 工具选择
type AnthropicToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// AnthropicContent 响应内容块
type AnthropicContent struct {
	Type      string                 `json:"type"`
	Text      *string                `json:"text,omitempty"`
	Thinking  *string                `json:"thinking,omitempty"`
	Signature *string                `json:"signature,omitempty"`
	ID        string                 `json:"id,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
	ToolUseID string                 `json:"tool_use_id,omitempty"`
}

// AnthropicChatResponse Anthropic聊天响应
type AnthropicChatResponse struct {
	ID           string             `json:"id"`
	Type         string             `json:"type"`
	Role         string             `json:"role"`
	Content      []AnthropicContent `json:"content"`
	Model        string             `json:"model"`
	StopReason   *string            `json:"stop_reason,omitempty"`
	StopSequence *string            `json:"stop_sequence,omitempty"`
	Usage        *AnthropicUsage    `json:"usage,omitempty"`
}

// AnthropicUsage Anthropic使用统计
type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// AnthropicStreamEvent 流式事件
type AnthropicStreamEvent struct {
	Type         string                    `json:"type"`
	Index        *int                      `json:"index,omitempty"`
	Delta        *AnthropicStreamDelta     `json:"delta,omitempty"`
	Usage        *AnthropicUsage           `json:"usage,omitempty"`
	ContentBlock *AnthropicContentBlock    `json:"content_block,omitempty"`
	Message      *AnthropicStreamMessage   `json:"message,omitempty"`
}

// AnthropicStreamMessage 流式消息开始时的message字段
type AnthropicStreamMessage struct {
	ID           string             `json:"id"`
	Type         string             `json:"type"`
	Role         string             `json:"role"`
	Model        string             `json:"model"`
	Content      []AnthropicContent `json:"content"`
	StopReason   *string            `json:"stop_reason"`
	StopSequence *string            `json:"stop_sequence"`
	Usage        *AnthropicUsage    `json:"usage"`
}

// AnthropicContentBlock 内容块
type AnthropicContentBlock struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	Thinking  string `json:"thinking,omitempty"`
	Signature string `json:"signature,omitempty"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
}

// AnthropicStreamDelta 流式增量
type AnthropicStreamDelta struct {
	Type         string `json:"type,omitempty"`
	Text         string `json:"text,omitempty"`
	Thinking     string `json:"thinking,omitempty"`
	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
}

// AnthropicError Anthropic错误响应
type AnthropicError struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// AnthropicModelsResponse Anthropic模型列表响应
type AnthropicModelsResponse struct {
	Data []AnthropicModelInfo `json:"data"`
}

// AnthropicModelInfo Anthropic模型信息
type AnthropicModelInfo struct {
	ID         string `json:"id"`
	Object     string `json:"object"`
	Created    int64  `json:"created"`
	OwnedBy    string `json:"owned_by"`
	DisplayName string `json:"display_name,omitempty"`
}

// AnthropicGateway Anthropic兼容网关
type AnthropicGateway struct {
	apiKeyManager       *service.APIKeyManager
	aiService           *service.AIService
	memoryManager       *memory.MemoryManager       // 记忆管理器
	conversationManager *memory.ConversationManager  // 对话生命周期管理器
	messageParser       *memory.MessageParser        // 消息解析器
}

// NewAnthropicGateway 创建Anthropic兼容网关
// 使用全局记忆管理器单例，确保 Handler 和 Gateway 共享同一个 MemoryManager 实例
// 初始化对话管理器和消息解析器
func NewAnthropicGateway(apiKeyManager *service.APIKeyManager, aiService *service.AIService) *AnthropicGateway {
	// 使用全局单例获取记忆管理器，避免重复创建
	memManager := memory.GetGlobalMemoryManager()

	// 创建对话管理器
	convManager := memory.NewConversationManager(memManager)

	return &AnthropicGateway{
		apiKeyManager:       apiKeyManager,
		aiService:           aiService,
		memoryManager:       memManager,
		conversationManager: convManager,
		messageParser:       memory.NewMessageParser(),
	}
}

// extractAPIKey 从请求中提取API Key，兼容多种认证方式
// cc-Switch可能发送 x-api-key 或 Authorization: Bearer 两种方式
func (g *AnthropicGateway) extractAPIKey(c *gin.Context) string {
	apiKey := c.GetHeader("x-api-key")
	if apiKey != "" {
		return apiKey
	}

	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		apiKey = strings.TrimPrefix(authHeader, "Bearer ")
	}

	return apiKey
}

// setCommonHeaders 设置cc-Switch/Claude Code所需的通用响应头
func (g *AnthropicGateway) setCommonHeaders(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	c.Header("X-Request-Id", fmt.Sprintf("req_%d", time.Now().UnixNano()))
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Content-Type, x-api-key, Authorization, anthropic-version, anthropic-beta")
}

// handleCORS 处理CORS预检请求（cc-Switch Web界面可能发送）
func (g *AnthropicGateway) handleCORS(c *gin.Context) {
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Content-Type, x-api-key, Authorization, anthropic-version, anthropic-beta")
	c.Header("Access-Control-Max-Age", "86400")
	c.Status(http.StatusNoContent)
}

// extractSystemPrompt 从system字段提取系统提示词
// system字段可以是string或[]interface{}（Anthropic新格式）
func (g *AnthropicGateway) extractSystemPrompt(system interface{}) string {
	if system == nil {
		return ""
	}
	switch s := system.(type) {
	case string:
		return s
	case []interface{}:
		var texts []string
		for _, item := range s {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if itemMap["type"] == "text" {
					if text, ok := itemMap["text"].(string); ok {
						texts = append(texts, text)
					}
				}
			}
		}
		return strings.Join(texts, "\n")
	default:
		return fmt.Sprintf("%v", system)
	}
}

// Messages Anthropic兼容的消息接口
func (g *AnthropicGateway) Messages(c *gin.Context) {
	// 处理CORS预检请求
	if c.Request.Method == http.MethodOptions {
		g.handleCORS(c)
		return
	}

	// 记录请求信息（调试cc-Switch兼容性）
	anthropicVersion := c.GetHeader("anthropic-version")
	anthropicBeta := c.GetHeader("anthropic-beta")
	betaParam := c.Query("beta")
	log.Printf("[Anthropic网关] 收到请求: %s %s, anthropic-version=%s, anthropic-beta=%s, beta=%s",
		c.Request.Method, c.Request.URL.Path, anthropicVersion, anthropicBeta, betaParam)

	// 提取API Key（兼容x-api-key和Authorization: Bearer两种方式）
	apiKey := g.extractAPIKey(c)
	if apiKey == "" {
		g.setCommonHeaders(c)
		c.JSON(http.StatusUnauthorized, g.newError("authentication_error", "缺少API Key，请在请求头中提供 x-api-key 或 Authorization: Bearer <api_key>"))
		return
	}

	// 验证API Key
	keyInfo, err := g.apiKeyManager.ValidateAPIKey(apiKey)
	if err != nil {
		g.setCommonHeaders(c)
		c.JSON(http.StatusUnauthorized, g.newError("authentication_error", err.Error()))
		return
	}

	// 解析请求
	var req AnthropicChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		g.setCommonHeaders(c)
		c.JSON(http.StatusBadRequest, g.newError("invalid_request_error", "请求格式错误: "+err.Error()))
		return
	}

	// 验证必填字段
	if req.Model == "" {
		g.setCommonHeaders(c)
		c.JSON(http.StatusBadRequest, g.newError("invalid_request_error", "缺少必填字段: model"))
		return
	}

	if len(req.Messages) == 0 {
		g.setCommonHeaders(c)
		c.JSON(http.StatusBadRequest, g.newError("invalid_request_error", "缺少必填字段: messages"))
		return
	}

	// 提取用户消息（兼容性保留，用于长度检查）
	userMessage := g.extractUserMessage(req.Messages)
	if userMessage == "" {
		g.setCommonHeaders(c)
		c.JSON(http.StatusBadRequest, g.newError("invalid_request_error", "缺少用户消息"))
		return
	}

	// 输入字符长度限制检查
	maxInputLength := 8000
	if len(userMessage) > maxInputLength {
		log.Printf("[Anthropic网关] 输入内容过长: %d字符，最大支持%d字符", len(userMessage), maxInputLength)
		g.setCommonHeaders(c)
		c.JSON(http.StatusBadRequest, g.newError("invalid_request_error", fmt.Sprintf("输入内容过长（%d字符），最大支持%d字符", len(userMessage), maxInputLength)))
		return
	}

	// ===== 记忆管理流程 =====
	// 1. 解析请求：拆分为固定提示词/记忆/新需求
	inputMessages := make([]memory.InputMessage, len(req.Messages))
	for i, msg := range req.Messages {
		inputMessages[i] = memory.InputMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}
	parsedReq := g.messageParser.ParseMessages(inputMessages, req.System)

	// 2. 对话决策：根据对话状态决定发送内容
	decision := g.conversationManager.Decide(keyInfo.APIKey, keyInfo.Platform, parsedReq)

	// 3. 根据决策设置对话模式和发送内容
	var conversationMode models.ConversationMode
	var questionToSend string
	switch decision.Action {
	case memory.ActionSendOnly:
		// 活跃对话，提示词未变化，仅在当前对话中继续发送
		conversationMode = models.ConversationModeExisting
		questionToSend = parsedReq.CurrentRequest
		log.Printf("[Anthropic网关] 记忆决策: 继续现有对话, apiKey=%s...%s",
			keyInfo.APIKey[:4], keyInfo.APIKey[len(keyInfo.APIKey)-4:])
	case memory.ActionCreateAndSend, memory.ActionRebuildAndSend:
		// 新建或重建对话，需要发送完整消息（固定提示词+记忆+当前请求）
		conversationMode = models.ConversationModeNew
		questionToSend = decision.Message
		log.Printf("[Anthropic网关] 记忆决策: %s对话, apiKey=%s...%s",
			decision.Action, keyInfo.APIKey[:4], keyInfo.APIKey[len(keyInfo.APIKey)-4:])
	default:
		// 未知决策，默认新建对话
		conversationMode = models.ConversationModeNew
		questionToSend = decision.Message
		log.Printf("[Anthropic网关] 记忆决策: 未知操作(%s)，默认新建对话, apiKey=%s...%s",
			decision.Action, keyInfo.APIKey[:4], keyInfo.APIKey[len(keyInfo.APIKey)-4:])
	}

	// 如果是流式请求
	if req.Stream {
		g.handleStreamRequest(c, keyInfo, questionToSend, req, conversationMode)
		return
	}

	// 非流式请求
	g.handleNonStreamRequest(c, keyInfo, questionToSend, req, conversationMode)
}

// extractUserMessage 从消息数组中提取用户消息
func (g *AnthropicGateway) extractUserMessage(messages []AnthropicMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			switch content := messages[i].Content.(type) {
			case string:
				return content
			case []interface{}:
				var texts []string
				for _, item := range content {
					if itemMap, ok := item.(map[string]interface{}); ok {
						if itemMap["type"] == "text" {
							if text, ok := itemMap["text"].(string); ok {
								texts = append(texts, text)
							}
						}
						// 处理tool_result类型
						if itemMap["type"] == "tool_result" {
							if content, ok := itemMap["content"]; ok {
								switch c := content.(type) {
								case string:
									texts = append(texts, c)
								case []interface{}:
									for _, subItem := range c {
										if subMap, ok := subItem.(map[string]interface{}); ok {
											if subMap["type"] == "text" {
												if text, ok := subMap["text"].(string); ok {
													texts = append(texts, text)
												}
											}
										}
									}
								}
							}
						}
					}
				}
				return strings.Join(texts, "\n")
			}
		}
	}
	return ""
}

// handleNonStreamRequest 处理非流式请求
// conversationMode 对话模式：new（新建对话）或 existing（继续现有对话）
func (g *AnthropicGateway) handleNonStreamRequest(c *gin.Context, keyInfo *models.APIKeyInfo, question string, req AnthropicChatRequest, conversationMode models.ConversationMode) {
	askReq := &models.AskRequest{
		Platform:         keyInfo.Platform,
		SessionID:        keyInfo.SessionID,
		Question:         question,
		ConversationMode: conversationMode,
	}

	// 注意：记忆管理流程中，question已经包含了固定提示词和记忆内容
	// 不再需要额外拼接systemPrompt
	systemPrompt := g.extractSystemPrompt(req.System)
	if systemPrompt != "" && conversationMode == models.ConversationModeExisting {
		// 仅在继续现有对话时，将systemPrompt拼接到问题前面
		// 新建/重建对话时，systemPrompt已通过记忆管理流程包含在decision.Message中
		askReq.Question = systemPrompt + "\n\n" + question
	}

	resp, err := g.aiService.Ask(askReq)
	if err != nil {
		errMsg := err.Error()
		// 输入超限错误返回400
		if strings.Contains(errMsg, "输入内容超限") || strings.Contains(errMsg, "字数限制") {
			g.setCommonHeaders(c)
			c.JSON(http.StatusBadRequest, g.newError("invalid_request_error", errMsg))
			return
		}
		// 人机验证错误返回403
		if strings.Contains(errMsg, "人机验证") {
			g.setCommonHeaders(c)
			c.JSON(http.StatusForbidden, g.newError("authentication_error", errMsg))
			return
		}
		g.setCommonHeaders(c)
		c.JSON(http.StatusInternalServerError, g.newError("api_error", "提问失败: "+errMsg))
		return
	}

	g.apiKeyManager.UpdateUsage(keyInfo.APIKey)

	answer := resp.Answer
	if answer == "" {
		answer = "抱歉，未能获取到回复。"
	}

	// 更新记忆：将AI回复添加到记忆，更新对话活跃时间
	g.conversationManager.HandlePostAsk(keyInfo.APIKey, answer, keyInfo.SessionID, "")

	thinking := resp.Thinking
	hasThinking := thinking != ""

	stopReason := "end_turn"
	messageID := fmt.Sprintf("msg_%d", time.Now().UnixNano())

	var contentBlocks []AnthropicContent
	if hasThinking {
		sig := fmt.Sprintf("ErUBk%d", time.Now().UnixNano())
		contentBlocks = append(contentBlocks, AnthropicContent{
			Type:      "thinking",
			Thinking:  &thinking,
			Signature: &sig,
		})
	}
	contentBlocks = append(contentBlocks, AnthropicContent{
		Type: "text",
		Text: &answer,
	})

	anthropicResp := AnthropicChatResponse{
		ID:      messageID,
		Type:    "message",
		Role:    "assistant",
		Content: contentBlocks,
		Model:      req.Model,
		StopReason: &stopReason,
		Usage: &AnthropicUsage{
			InputTokens:  len(question) / 4,
			OutputTokens: len(answer) / 4,
		},
	}

	g.setCommonHeaders(c)
	c.JSON(http.StatusOK, anthropicResp)
}

// handleStreamRequest 处理流式请求
// 实现真正的流式响应：先立即发送message_start/ping事件让客户端确认连接成功，
// 然后异步获取AI回复并逐步发送内容块
// conversationMode 对话模式：new（新建对话）或 existing（继续现有对话）
func (g *AnthropicGateway) handleStreamRequest(c *gin.Context, keyInfo *models.APIKeyInfo, question string, req AnthropicChatRequest, conversationMode models.ConversationMode) {
	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Header("X-Request-Id", fmt.Sprintf("req_%d", time.Now().UnixNano()))
	c.Header("Access-Control-Allow-Origin", "*")

	messageID := fmt.Sprintf("msg_%d", time.Now().UnixNano())

	// 立即发送 message_start 事件，让客户端确认连接成功
	// 这是cc-Switch Stream Check判定成功的关键：只要收到首个SSE事件就算连接成功
	g.sendSSEEvent(c, "message_start", map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":            messageID,
			"type":          "message",
			"role":          "assistant",
			"model":         req.Model,
			"content":       []interface{}{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]int{
				"input_tokens":  len(question) / 4,
				"output_tokens": 0,
			},
		},
	})

	// 发送 ping 事件（cc-Switch/Claude Code需要保持连接）
	g.sendSSEEvent(c, "ping", map[string]interface{}{})

	// 异步获取AI回复
	askReq := &models.AskRequest{
		Platform:         keyInfo.Platform,
		SessionID:        keyInfo.SessionID,
		Question:         question,
		ConversationMode: conversationMode,
	}

	// 注意：记忆管理流程中，question已经包含了固定提示词和记忆内容
	// 不再需要额外拼接systemPrompt
	systemPrompt := g.extractSystemPrompt(req.System)
	if systemPrompt != "" && conversationMode == models.ConversationModeExisting {
		// 仅在继续现有对话时，将systemPrompt拼接到问题前面
		// 新建/重建对话时，systemPrompt已通过记忆管理流程包含在decision.Message中
		askReq.Question = systemPrompt + "\n\n" + question
	}

	resp, err := g.aiService.Ask(askReq)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "输入内容超限") || strings.Contains(errMsg, "字数限制") {
			g.sendSSEError(c, "invalid_request_error", errMsg)
			return
		}
		if strings.Contains(errMsg, "人机验证") {
			g.sendSSEError(c, "authentication_error", errMsg)
			return
		}
		g.sendSSEError(c, "api_error", errMsg)
		return
	}

	g.apiKeyManager.UpdateUsage(keyInfo.APIKey)

	answer := resp.Answer
	if answer == "" {
		answer = "抱歉，未能获取到回复。"
	}

	// 更新记忆：将AI回复添加到记忆，更新对话活跃时间
	g.conversationManager.HandlePostAsk(keyInfo.APIKey, answer, keyInfo.SessionID, "")

	thinking := resp.Thinking
	hasThinking := thinking != ""

	blockIndex := 0

	// 如果有思考过程，先发送thinking内容块
	if hasThinking {
		thinkingBlockID := fmt.Sprintf("blk_%d", time.Now().UnixNano())
		sig := fmt.Sprintf("ErUBk%d", time.Now().UnixNano())
		g.sendSSEEvent(c, "content_block_start", map[string]interface{}{
			"type": "content_block_start",
			"index": blockIndex,
			"content_block": map[string]interface{}{
				"type":      "thinking",
				"thinking":  "",
				"signature": sig,
				"id":        thinkingBlockID,
			},
		})

		// 流式发送思考过程
		thinkingRunes := []rune(thinking)
		thinkingChunkSize := 10
		for i := 0; i < len(thinkingRunes); i += thinkingChunkSize {
			end := i + thinkingChunkSize
			if end > len(thinkingRunes) {
				end = len(thinkingRunes)
			}
			chunk := string(thinkingRunes[i:end])

			g.sendSSEEvent(c, "content_block_delta", map[string]interface{}{
				"type":  "content_block_delta",
				"index": blockIndex,
				"delta": map[string]interface{}{
					"type":     "thinking_delta",
					"thinking": chunk,
				},
			})
		}

		g.sendSSEEvent(c, "content_block_stop", map[string]interface{}{
			"type":  "content_block_stop",
			"index": blockIndex,
		})

		blockIndex++
	}

	// 发送 text 内容块
	contentBlockID := fmt.Sprintf("blk_%d", time.Now().UnixNano())
	g.sendSSEEvent(c, "content_block_start", map[string]interface{}{
		"type": "content_block_start",
		"index": blockIndex,
		"content_block": map[string]interface{}{
			"type": "text",
			"text": "",
			"id":   contentBlockID,
		},
	})

	// 按字符分割答案，流式发送
	runes := []rune(answer)
	chunkSize := 5
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunk := string(runes[i:end])

		g.sendSSEEvent(c, "content_block_delta", map[string]interface{}{
			"type":  "content_block_delta",
			"index": blockIndex,
			"delta": map[string]interface{}{
				"type": "text_delta",
				"text": chunk,
			},
		})
	}

	// 发送 content_block_stop 事件
	g.sendSSEEvent(c, "content_block_stop", map[string]interface{}{
		"type":  "content_block_stop",
		"index": blockIndex,
	})

	// 发送 message_delta 事件
	stopReason := "end_turn"
	g.sendSSEEvent(c, "message_delta", map[string]interface{}{
		"type": "message_delta",
		"delta": map[string]interface{}{
			"stop_reason":   stopReason,
			"stop_sequence": nil,
		},
		"usage": map[string]int{
			"output_tokens": len(answer) / 4,
		},
	})

	// 发送 message_stop 事件
	g.sendSSEEvent(c, "message_stop", map[string]interface{}{
		"type": "message_stop",
	})
}

// Models 返回Anthropic格式的模型列表（cc-Switch测试需要）
func (g *AnthropicGateway) Models(c *gin.Context) {
	g.setCommonHeaders(c)

	models := []AnthropicModelInfo{
		{
			ID:          "claude-sonnet-4-5-20250929",
			Object:      "model",
			Created:     time.Now().Unix(),
			OwnedBy:     "aithink",
			DisplayName: "Claude Sonnet 4.5 (via AIThink)",
		},
		{
			ID:          "claude-sonnet-4-20250514",
			Object:      "model",
			Created:     time.Now().Unix(),
			OwnedBy:     "aithink",
			DisplayName: "Claude Sonnet 4 (via AIThink)",
		},
		{
			ID:          "claude-haiku-4-5-20251001",
			Object:      "model",
			Created:     time.Now().Unix(),
			OwnedBy:     "aithink",
			DisplayName: "Claude Haiku 4.5 (via AIThink)",
		},
		{
			ID:          "zhipu-glm-5",
			Object:      "model",
			Created:     time.Now().Unix(),
			OwnedBy:     "aithink",
			DisplayName: "智谱GLM-5 (via AIThink)",
		},
		{
			ID:          "chatgpt-gpt-4",
			Object:      "model",
			Created:     time.Now().Unix(),
			OwnedBy:     "aithink",
			DisplayName: "GPT-4 (via AIThink)",
		},
	}

	c.JSON(http.StatusOK, AnthropicModelsResponse{
		Data: models,
	})
}

// sendSSEEvent 发送SSE事件
func (g *AnthropicGateway) sendSSEEvent(c *gin.Context, eventType string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", eventType, jsonData)
	c.Writer.Flush()
}

// sendSSEError 发送SSE错误事件
func (g *AnthropicGateway) sendSSEError(c *gin.Context, errorType string, message string) {
	errorData := map[string]interface{}{
		"type": "error",
		"error": map[string]interface{}{
			"type":    errorType,
			"message": message,
		},
	}
	g.sendSSEEvent(c, "error", errorData)
}

// Responses OpenAI Responses API兼容端点（cc-Switch apiFormat=openai_responses时使用）
// 将OpenAI Responses格式转换为内部处理，返回Anthropic SSE流式响应
func (g *AnthropicGateway) Responses(c *gin.Context) {
	if c.Request.Method == http.MethodOptions {
		g.handleCORS(c)
		return
	}

	log.Printf("[Responses网关] 收到请求: %s %s", c.Request.Method, c.Request.URL.Path)

	apiKey := g.extractAPIKey(c)
	if apiKey == "" {
		g.setCommonHeaders(c)
		c.JSON(http.StatusUnauthorized, g.newError("authentication_error", "缺少API Key"))
		return
	}

	keyInfo, err := g.apiKeyManager.ValidateAPIKey(apiKey)
	if err != nil {
		g.setCommonHeaders(c)
		c.JSON(http.StatusUnauthorized, g.newError("authentication_error", err.Error()))
		return
	}

	var reqBody map[string]interface{}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		g.setCommonHeaders(c)
		c.JSON(http.StatusBadRequest, g.newError("invalid_request_error", "请求格式错误: "+err.Error()))
		return
	}

	input := ""
	if inputVal, ok := reqBody["input"]; ok {
		switch v := inputVal.(type) {
		case string:
			input = v
		case []interface{}:
			var texts []string
			for _, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if role, ok := itemMap["role"].(string); ok && role == "user" {
						if content, ok := itemMap["content"].(string); ok {
							texts = append(texts, content)
						}
					}
				}
			}
			input = strings.Join(texts, "\n")
		}
	}

	if input == "" {
		g.setCommonHeaders(c)
		c.JSON(http.StatusBadRequest, g.newError("invalid_request_error", "缺少输入内容"))
		return
	}

	model := "gpt-4"
	if m, ok := reqBody["model"].(string); ok && m != "" {
		model = m
	}

	stream := true
	if s, ok := reqBody["stream"].(bool); ok {
		stream = s
	}

	askReq := &models.AskRequest{
		Platform:  keyInfo.Platform,
		SessionID: keyInfo.SessionID,
		Question:  input,
	}

	resp, err := g.aiService.Ask(askReq)
	if err != nil {
		g.setCommonHeaders(c)
		c.JSON(http.StatusInternalServerError, g.newError("api_error", "请求失败: "+err.Error()))
		return
	}

	g.apiKeyManager.UpdateUsage(keyInfo.APIKey)

	answer := resp.Answer
	if answer == "" {
		answer = "抱歉，未能获取到回复。"
	}

	if stream {
		g.handleResponsesStream(c, answer, model, input)
	} else {
		g.handleResponsesNonStream(c, answer, model, input)
	}
}

// handleResponsesNonStream 处理OpenAI Responses API非流式响应
func (g *AnthropicGateway) handleResponsesNonStream(c *gin.Context, answer string, model string, input string) {
	g.setCommonHeaders(c)
	responseID := fmt.Sprintf("resp_%d", time.Now().UnixNano())
	c.JSON(http.StatusOK, gin.H{
		"id":     responseID,
		"object": "response",
		"model":  model,
		"output": []gin.H{
			{
				"type":    "message",
				"id":      fmt.Sprintf("msg_%d", time.Now().UnixNano()),
				"role":    "assistant",
				"content": []gin.H{{"type": "output_text", "text": answer}},
			},
		},
		"usage": gin.H{
			"input_tokens":  len(input) / 4,
			"output_tokens": len(answer) / 4,
		},
	})
}

// handleResponsesStream 处理OpenAI Responses API流式响应
func (g *AnthropicGateway) handleResponsesStream(c *gin.Context, answer string, model string, input string) {
	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Header("X-Request-Id", fmt.Sprintf("req_%d", time.Now().UnixNano()))
	c.Header("Access-Control-Allow-Origin", "*")

	responseID := fmt.Sprintf("resp_%d", time.Now().UnixNano())
	messageID := fmt.Sprintf("msg_%d", time.Now().UnixNano())

	// 发送response.created事件
	g.sendSSEEvent(c, "response.created", map[string]interface{}{
		"type":   "response.created",
		"response": map[string]interface{}{
			"id":     responseID,
			"object": "response",
			"model":  model,
			"status": "in_progress",
		},
	})

	// 发送response.output_item.added事件
	g.sendSSEEvent(c, "response.output_item.added", map[string]interface{}{
		"type":   "response.output_item.added",
		"output_index": 0,
		"item": map[string]interface{}{
			"type":    "message",
			"id":      messageID,
			"role":    "assistant",
			"content": []interface{}{},
		},
	})

	// 发送content部分添加事件
	contentID := fmt.Sprintf("cnt_%d", time.Now().UnixNano())
	g.sendSSEEvent(c, "response.content_part.added", map[string]interface{}{
		"type":         "response.content_part.added",
		"output_index": 0,
		"content_index": 0,
		"part": map[string]interface{}{
			"type": "output_text",
			"text": "",
			"id":   contentID,
		},
	})

	// 流式发送文本
	runes := []rune(answer)
	chunkSize := 5
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunk := string(runes[i:end])

		g.sendSSEEvent(c, "response.output_text.delta", map[string]interface{}{
			"type":           "response.output_text.delta",
			"output_index":   0,
			"content_index":  0,
			"delta":          chunk,
		})
	}

	// 发送text done事件
	g.sendSSEEvent(c, "response.output_text.done", map[string]interface{}{
		"type":          "response.output_text.done",
		"output_index":  0,
		"content_index": 0,
		"text":          answer,
	})

	// 发送content part done事件
	g.sendSSEEvent(c, "response.content_part.done", map[string]interface{}{
		"type":          "response.content_part.done",
		"output_index":  0,
		"content_index": 0,
		"part": map[string]interface{}{
			"type": "output_text",
			"text": answer,
			"id":   contentID,
		},
	})

	// 发送output item done事件
	g.sendSSEEvent(c, "response.output_item.done", map[string]interface{}{
		"type":         "response.output_item.done",
		"output_index": 0,
		"item": map[string]interface{}{
			"type":    "message",
			"id":      messageID,
			"role":    "assistant",
			"content": []gin.H{{"type": "output_text", "text": answer}},
		},
	})

	// 发送response.completed事件
	g.sendSSEEvent(c, "response.completed", map[string]interface{}{
		"type":   "response.completed",
		"response": map[string]interface{}{
			"id":     responseID,
			"object": "response",
			"model":  model,
			"status": "completed",
			"output": []gin.H{
				{
					"type":    "message",
					"id":      messageID,
					"role":    "assistant",
					"content": []gin.H{{"type": "output_text", "text": answer}},
				},
			},
			"usage": gin.H{
				"input_tokens":  len(input) / 4,
				"output_tokens": len(answer) / 4,
			},
		},
	})
}

// newError 创建Anthropic格式错误响应
func (g *AnthropicGateway) newError(errorType string, message string) AnthropicError {
	var errResp AnthropicError
	errResp.Type = "error"
	errResp.Error.Type = errorType
	errResp.Error.Message = message
	return errResp
}
