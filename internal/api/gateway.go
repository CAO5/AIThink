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

// OpenAIMessage OpenAI消息格式
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIChatRequest OpenAI聊天请求
type OpenAIChatRequest struct {
	Model       string         `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Stream      bool           `json:"stream"`
	Temperature *float64       `json:"temperature,omitempty"`
	MaxTokens   *int           `json:"max_tokens,omitempty"`
}

// OpenAIChoice OpenAI选择
type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message,omitempty"`
	Delta        *OpenAIMessage `json:"delta,omitempty"`
	FinishReason *string       `json:"finish_reason,omitempty"`
}

// OpenAIChatResponse OpenAI聊天响应
type OpenAIChatResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   *OpenAIUsage   `json:"usage,omitempty"`
}

// OpenAIUsage OpenAI使用统计
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAIStreamChunk 流式响应块
type OpenAIStreamChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
}

// OpenAIGateway OpenAI兼容网关
type OpenAIGateway struct {
	apiKeyManager *service.APIKeyManager
	aiService     *service.AIService
}

// NewOpenAIGateway 创建OpenAI兼容网关
func NewOpenAIGateway(apiKeyManager *service.APIKeyManager, aiService *service.AIService) *OpenAIGateway {
	return &OpenAIGateway{
		apiKeyManager: apiKeyManager,
		aiService:     aiService,
	}
}

// ChatCompletions OpenAI兼容的聊天接口
func (g *OpenAIGateway) ChatCompletions(c *gin.Context) {
	// 验证API Key
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"message": "缺少API Key，请在请求头中提供 Authorization: Bearer <api_key>",
				"type":    "authentication_error",
			},
		})
		return
	}

	// 提取API Key（Bearer token）
	apiKey := strings.TrimPrefix(authHeader, "Bearer ")
	if apiKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"message": "无效的API Key",
				"type":    "authentication_error",
			},
		})
		return
	}

	// 验证API Key
	keyInfo, err := g.apiKeyManager.ValidateAPIKey(apiKey)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"message": err.Error(),
				"type":    "authentication_error",
			},
		})
		return
	}

	// 解析请求
	var req OpenAIChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "请求格式错误: " + err.Error(),
				"type":    "invalid_request_error",
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
			"error": gin.H{
				"message": "缺少用户消息",
				"type":    "invalid_request_error",
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
func (g *OpenAIGateway) handleNonStreamRequest(c *gin.Context, keyInfo *models.APIKeyInfo, question string, model string) {
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
			"error": gin.H{
				"message": "提问失败: " + err.Error(),
				"type":    "api_error",
			},
		})
		return
	}

	// 更新使用记录
	g.apiKeyManager.UpdateUsage(keyInfo.APIKey)

	// 构建OpenAI格式响应
	finishReason := "stop"
	answer := resp.Answer

	// 如果答案为空，使用默认值
	if answer == "" {
		answer = "抱歉，未能获取到回复。"
	}

	openAIResp := OpenAIChatResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []OpenAIChoice{
			{
				Index: 0,
				Message: OpenAIMessage{
					Role:    "assistant",
					Content: answer,
				},
				FinishReason: &finishReason,
			},
		},
		Usage: &OpenAIUsage{
			PromptTokens:     len(question) / 4, // 粗略估算
			CompletionTokens: len(answer) / 4,
			TotalTokens:      (len(question) + len(answer)) / 4,
		},
	}

	c.JSON(http.StatusOK, openAIResp)
}

// handleStreamRequest 处理流式请求
func (g *OpenAIGateway) handleStreamRequest(c *gin.Context, keyInfo *models.APIKeyInfo, question string, model string) {
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
		// 发送错误
		errorChunk := OpenAIStreamChunk{
			ID:      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   model,
			Choices: []OpenAIChoice{
				{
					Index: 0,
					Delta: &OpenAIMessage{
						Role:    "assistant",
						Content: "错误: " + err.Error(),
					},
					FinishReason: strPtr("stop"),
				},
			},
		}
		data, _ := json.Marshal(errorChunk)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		c.Writer.Flush()
		fmt.Fprint(c.Writer, "data: [DONE]\n\n")
		return
	}

	// 更新使用记录
	g.apiKeyManager.UpdateUsage(keyInfo.APIKey)

	// 流式返回答案（按行分割）
	answer := resp.Answer
	if answer == "" {
		answer = "抱歉，未能获取到回复。"
	}

	// 按字符分块返回（模拟流式）
	lines := strings.Split(answer, "\n")
	for i, line := range lines {
		chunk := OpenAIStreamChunk{
			ID:      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   model,
			Choices: []OpenAIChoice{
				{
					Index: 0,
					Delta: &OpenAIMessage{
						Role:    "",
						Content: line,
					},
					FinishReason: nil,
				},
			},
		}
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		c.Writer.Flush()

		// 如果不是最后一行，添加换行
		if i < len(lines)-1 {
			newlineChunk := OpenAIStreamChunk{
				ID:      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   model,
				Choices: []OpenAIChoice{
					{
						Index: 0,
						Delta: &OpenAIMessage{
							Role:    "",
							Content: "\n",
						},
						FinishReason: nil,
					},
				},
			}
			data, _ := json.Marshal(newlineChunk)
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			c.Writer.Flush()
		}
	}

	// 发送结束标记
	finishChunk := OpenAIStreamChunk{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []OpenAIChoice{
			{
				Index: 0,
				Delta: nil,
				FinishReason: strPtr("stop"),
			},
		},
	}
	data, _ := json.Marshal(finishChunk)
	fmt.Fprintf(c.Writer, "data: %s\n\n", data)
	c.Writer.Flush()
	fmt.Fprint(c.Writer, "data: [DONE]\n\n")
}

// Models 返回可用模型列表
func (g *OpenAIGateway) Models(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data": []gin.H{
			{
				"id":       "zhipu-glm-5",
				"object":   "model",
				"created":  time.Now().Unix(),
				"owned_by": "aithink",
			},
			{
				"id":       "claude-sonnet",
				"object":   "model",
				"created":  time.Now().Unix(),
				"owned_by": "aithink",
			},
			{
				"id":       "chatgpt-gpt-4",
				"object":   "model",
				"created":  time.Now().Unix(),
				"owned_by": "aithink",
			},
		},
	})
}

// strPtr 返回字符串指针
func strPtr(s string) *string {
	return &s
}
