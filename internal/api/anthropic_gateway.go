package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"aithink/internal/models"
	"aithink/internal/service"

	"github.com/gin-gonic/gin"
)

// AnthropicMessage Anthropic消息格式
type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AnthropicChatRequest Anthropic聊天请求
type AnthropicChatRequest struct {
	Model       string             `json:"model"`
	Messages    []AnthropicMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
	Temperature *float64           `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
	System      string             `json:"system,omitempty"`
}

// AnthropicContent 内容块
type AnthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// AnthropicChatResponse Anthropic聊天响应
type AnthropicChatResponse struct {
	ID         string              `json:"id"`
	Type       string              `json:"type"`
	Role       string              `json:"role"`
	Content    []AnthropicContent  `json:"content"`
	Model      string              `json:"model"`
	StopReason *string             `json:"stop_reason,omitempty"`
	Usage      *AnthropicUsage     `json:"usage,omitempty"`
}

// AnthropicUsage Anthropic使用统计
type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// AnthropicStreamEvent 流式事件
type AnthropicStreamEvent struct {
	Type  string                 `json:"type"`
	Index *int                   `json:"index,omitempty"`
	Delta *AnthropicStreamDelta  `json:"delta,omitempty"`
	Usage *AnthropicUsage        `json:"usage,omitempty"`
}

// AnthropicStreamDelta 流式增量
type AnthropicStreamDelta struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

// AnthropicGateway Anthropic兼容网关
type AnthropicGateway struct {
	apiKeyManager *service.APIKeyManager
	aiService     *service.AIService
}

// NewAnthropicGateway 创建Anthropic兼容网关
func NewAnthropicGateway(apiKeyManager *service.APIKeyManager, aiService *service.AIService) *AnthropicGateway {
	return &AnthropicGateway{
		apiKeyManager: apiKeyManager,
		aiService:     aiService,
	}
}

// Messages Anthropic兼容的消息接口
func (g *AnthropicGateway) Messages(c *gin.Context) {
	// 验证API Key
	apiKey := c.GetHeader("x-api-key")
	if apiKey == "" {
		// 也支持 Authorization 头
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			apiKey = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	if apiKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"type":    "error",
			"error": gin.H{
				"type":    "authentication_error",
				"message": "缺少API Key，请在请求头中提供 x-api-key 或 Authorization: Bearer <api_key>",
			},
		})
		return
	}

	// 验证API Key
	keyInfo, err := g.apiKeyManager.ValidateAPIKey(apiKey)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"type":    "error",
			"error": gin.H{
				"type":    "authentication_error",
				"message": err.Error(),
			},
		})
		return
	}

	// 解析请求
	var req AnthropicChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"type":    "error",
			"error": gin.H{
				"type":    "invalid_request_error",
				"message": "请求格式错误: " + err.Error(),
			},
		})
		return
	}

	// 提取用户消息（获取最后一条用户消息）
	var userMessage string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			userMessage = req.Messages[i].Content
			break
		}
	}

	if userMessage == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"type":    "error",
			"error": gin.H{
				"type":    "invalid_request_error",
				"message": "缺少用户消息",
			},
		})
		return
	}

	// 如果是流式请求
	if req.Stream {
		g.handleStreamRequest(c, keyInfo, userMessage, req.Model)
		return
	}

	// 非流式请求
	g.handleNonStreamRequest(c, keyInfo, userMessage, req.Model)
}

// handleNonStreamRequest 处理非流式请求
func (g *AnthropicGateway) handleNonStreamRequest(c *gin.Context, keyInfo *models.APIKeyInfo, question string, model string) {
	// 构建提问请求
	askReq := &models.AskRequest{
		Platform:  keyInfo.Platform,
		SessionID: keyInfo.SessionID,
		Question:  question,
	}

	// 执行提问
	resp, err := g.aiService.Ask(askReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"type":    "error",
			"error": gin.H{
				"type":    "api_error",
				"message": "提问失败: " + err.Error(),
			},
		})
		return
	}

	// 更新使用记录
	g.apiKeyManager.UpdateUsage(keyInfo.APIKey)

	// 构建Anthropic格式响应
	answer := resp.Answer
	if answer == "" {
		answer = "抱歉，未能获取到回复。"
	}

	stopReason := "end_turn"
	anthropicResp := AnthropicChatResponse{
		ID:   fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		Type: "message",
		Role: "assistant",
		Content: []AnthropicContent{
			{
				Type: "text",
				Text: answer,
			},
		},
		Model:      model,
		StopReason: &stopReason,
		Usage: &AnthropicUsage{
			InputTokens:  len(question) / 4,
			OutputTokens: len(answer) / 4,
		},
	}

	c.JSON(http.StatusOK, anthropicResp)
}

// handleStreamRequest 处理流式请求
func (g *AnthropicGateway) handleStreamRequest(c *gin.Context, keyInfo *models.APIKeyInfo, question string, model string) {
	// 设置SSE响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// 构建提问请求
	askReq := &models.AskRequest{
		Platform:  keyInfo.Platform,
		SessionID: keyInfo.SessionID,
		Question:  question,
	}

	// 执行提问
	resp, err := g.aiService.Ask(askReq)
	if err != nil {
		// 发送错误事件
		errorEvent := map[string]interface{}{
			"type": "error",
			"error": gin.H{
				"type":    "api_error",
				"message": err.Error(),
			},
		}
		data, _ := json.Marshal(errorEvent)
		fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n", data)
		c.Writer.Flush()
		return
	}

	// 更新使用记录
	g.apiKeyManager.UpdateUsage(keyInfo.APIKey)

	answer := resp.Answer
	if answer == "" {
		answer = "抱歉，未能获取到回复。"
	}

	// 发送 message_start 事件
	messageStart := AnthropicStreamEvent{
		Type: "message_start",
	}
	data, _ := json.Marshal(messageStart)
	fmt.Fprintf(c.Writer, "event: message_start\ndata: %s\n\n", data)
	c.Writer.Flush()

	// 按行分割答案，流式发送
	lines := strings.Split(answer, "\n")
	for i, line := range lines {
		// 发送 content_block_delta 事件
		contentDelta := AnthropicStreamEvent{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: &AnthropicStreamDelta{
				Type: "text_delta",
				Text: line,
			},
		}
		data, _ := json.Marshal(contentDelta)
		fmt.Fprintf(c.Writer, "event: content_block_delta\ndata: %s\n\n", data)
		c.Writer.Flush()

		// 如果不是最后一行，发送换行
		if i < len(lines)-1 {
			newlineDelta := AnthropicStreamEvent{
				Type:  "content_block_delta",
				Index: intPtr(0),
				Delta: &AnthropicStreamDelta{
					Type: "text_delta",
					Text: "\n",
				},
			}
			data, _ := json.Marshal(newlineDelta)
			fmt.Fprintf(c.Writer, "event: content_block_delta\ndata: %s\n\n", data)
			c.Writer.Flush()
		}
	}

	// 发送 content_block_stop 事件
	contentStop := AnthropicStreamEvent{
		Type:  "content_block_stop",
		Index: intPtr(0),
	}
	data, _ = json.Marshal(contentStop)
	fmt.Fprintf(c.Writer, "event: content_block_stop\ndata: %s\n\n", data)
	c.Writer.Flush()

	// 发送 message_delta 事件
	messageDelta := AnthropicStreamEvent{
		Type: "message_delta",
		Delta: &AnthropicStreamDelta{
			Type: "text_delta",
			Text: "",
		},
		Usage: &AnthropicUsage{
			InputTokens:  len(question) / 4,
			OutputTokens: len(answer) / 4,
		},
	}
	data, _ = json.Marshal(messageDelta)
	fmt.Fprintf(c.Writer, "event: message_delta\ndata: %s\n\n", data)
	c.Writer.Flush()

	// 发送 message_stop 事件
	messageStop := AnthropicStreamEvent{
		Type: "message_stop",
	}
	data, _ = json.Marshal(messageStop)
	fmt.Fprintf(c.Writer, "event: message_stop\ndata: %s\n\n", data)
	c.Writer.Flush()
}

// intPtr 返回整数指针
func intPtr(i int) *int {
	return &i
}
