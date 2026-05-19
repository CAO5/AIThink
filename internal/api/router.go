package api

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// corsMiddleware CORS中间件，处理cc-Switch Web界面的跨域请求
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		c.Header("Access-Control-Allow-Headers", "Content-Type, x-api-key, Authorization, anthropic-version, anthropic-beta, Accept")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// requestLogMiddleware 请求日志中间件，记录所有API请求（调试cc-Switch兼容性）
func requestLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		if raw != "" {
			path = path + "?" + raw
		}

		method := c.Request.Method
		clientIP := c.ClientIP()
		userAgent := c.Request.UserAgent()

		if strings.HasPrefix(path, "/v1/") || strings.HasPrefix(path, "/messages") {
			log.Printf("[API请求] %s %s %d %v %s UA=%s", method, path, statusCode, latency, clientIP, userAgent)
		}
	}
}

// SetupRouter 设置路由
func SetupRouter() *gin.Engine {
	r := gin.Default()

	r.Use(corsMiddleware())
	r.Use(requestLogMiddleware())

	handler := NewHandler()

	gateway := NewOpenAIGateway(handler.GetAPIKeyManager(), handler.aiService)

	anthropicGateway := NewAnthropicGateway(handler.GetAPIKeyManager(), handler.aiService)

	// API v1 路由组
	v1 := r.Group("/api/v1")
	{
		v1.POST("/login", handler.Login)
		v1.GET("/login/status", handler.GetLoginStatus)
		v1.POST("/ask", handler.Ask)
		v1.POST("/logout", handler.Logout)
		v1.GET("/config", handler.GetConfig)
		v1.POST("/config", handler.UpdateConfig)
		v1.GET("/anti-detection/check", handler.CheckAntiDetection)
	}

	// API Key 管理路由
	apikey := r.Group("/api/v1/apikey")
	{
		apikey.POST("/create", handler.CreateAPIKey)
		apikey.GET("/list", handler.ListAPIKeys)
		apikey.POST("/update/:apikey", handler.UpdateAPIKey)
		apikey.POST("/delete/:apikey", handler.DeleteAPIKey)
	}

	// API Key 提问路由（需要API Key认证）
	apikeyAsk := r.Group("/api/v1/apikey")
	apikeyAsk.Use(APIKeyAuth(handler))
	{
		apikeyAsk.POST("/ask", handler.APIKeyAsk)
		apikeyAsk.POST("/ask/stream", handler.APIKeyAskStream)
	}

	// 记忆管理路由
	memoryGroup := r.Group("/api/v1/memory")
	{
		memoryGroup.GET("/:api_key", handler.GetMemoryStatus)
		memoryGroup.DELETE("/:api_key", handler.ClearMemory)
	}

	// 对话管理路由
	conversationGroup := r.Group("/api/v1/conversation")
	{
		conversationGroup.GET("/:api_key", handler.GetConversationStatus)
		conversationGroup.POST("/:api_key/reset", handler.ResetConversation)
	}

	// 平台管理路由
	platformAPI := r.Group("/api/v1/platforms")
	{
		platformAPI.GET("", handler.ListPlatforms)
		platformAPI.GET("/:name/config", handler.GetPlatformConfig)
	}

	// ==================== cc-Switch / Claude Code 兼容路由 ====================

	// 标准路径：ANTHROPIC_BASE_URL=http://localhost:8081 时，SDK构造 /v1/messages
	v1API := r.Group("/v1")
	{
		// OpenAI兼容接口
		v1API.POST("/chat/completions", gateway.ChatCompletions)

		// Anthropic兼容接口 - POST用于实际消息请求
		v1API.POST("/messages", anthropicGateway.Messages)

		// Anthropic兼容接口 - GET用于健康检查（cc-Switch测试可能用GET探测连通性）
		v1API.GET("/messages", func(c *gin.Context) {
			log.Printf("[健康检查] GET /v1/messages 请求来自 %s", c.ClientIP())
			c.Header("Content-Type", "application/json")
			c.JSON(http.StatusOK, gin.H{
				"id":       "msg_health_check",
				"type":     "message",
				"role":     "assistant",
				"content":  []gin.H{{"type": "text", "text": "AIThink Anthropic API 服务正常运行"}},
				"model":    "aithink-health-check",
				"stop_reason": "end_turn",
				"usage": gin.H{
					"input_tokens":  0,
					"output_tokens": 0,
				},
			})
		})

		// 模型列表 - 优先使用Anthropic格式（cc-Switch测试需要）
		v1API.GET("/models", anthropicGateway.Models)

		// OpenAI Responses API兼容端点（cc-Switch apiFormat=openai_responses时使用）
		v1API.POST("/responses", anthropicGateway.Responses)
		v1API.OPTIONS("/responses", func(c *gin.Context) {
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Methods", "POST, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, x-api-key, Authorization, anthropic-version, anthropic-beta, Accept")
			c.Header("Access-Control-Max-Age", "86400")
			c.Status(http.StatusNoContent)
		})
	}

	// cc-Switch兼容：当用户将base URL配置为 http://localhost:8081/v1/messages 时，
	// SDK会追加 /v1/messages，导致路径变成 /v1/messages/v1/messages
	r.POST("/v1/messages/v1/messages", func(c *gin.Context) {
		log.Printf("[cc-Switch兼容] 检测到重复路径请求: %s，自动转发到 /v1/messages", c.Request.URL.Path)
		c.Request.URL.Path = "/v1/messages"
		r.HandleContext(c)
	})

	r.GET("/v1/messages/v1/messages", func(c *gin.Context) {
		log.Printf("[cc-Switch兼容] 检测到重复路径GET请求: %s，自动转发到 /v1/messages", c.Request.URL.Path)
		c.Request.URL.Path = "/v1/messages"
		r.HandleContext(c)
	})

	r.POST("/v1/v1/messages", func(c *gin.Context) {
		log.Printf("[cc-Switch兼容] 检测到重复路径请求: %s，自动转发到 /v1/messages", c.Request.URL.Path)
		c.Request.URL.Path = "/v1/messages"
		r.HandleContext(c)
	})

	r.GET("/v1/v1/messages", func(c *gin.Context) {
		log.Printf("[cc-Switch兼容] 检测到重复路径GET请求: %s，自动转发到 /v1/messages", c.Request.URL.Path)
		c.Request.URL.Path = "/v1/messages"
		r.HandleContext(c)
	})

	r.GET("/v1/messages/v1/models", func(c *gin.Context) {
		log.Printf("[cc-Switch兼容] 检测到重复路径请求: %s，自动转发到 /v1/models", c.Request.URL.Path)
		c.Request.URL.Path = "/v1/models"
		r.HandleContext(c)
	})

	r.GET("/v1/v1/models", func(c *gin.Context) {
		log.Printf("[cc-Switch兼容] 检测到重复路径请求: %s，自动转发到 /v1/models", c.Request.URL.Path)
		c.Request.URL.Path = "/v1/models"
		r.HandleContext(c)
	})

	r.POST("/v1/messages/v1/chat/completions", func(c *gin.Context) {
		log.Printf("[cc-Switch兼容] 检测到重复路径请求: %s，自动转发到 /v1/chat/completions", c.Request.URL.Path)
		c.Request.URL.Path = "/v1/chat/completions"
		r.HandleContext(c)
	})

	r.POST("/v1/v1/chat/completions", func(c *gin.Context) {
		log.Printf("[cc-Switch兼容] 检测到重复路径请求: %s，自动转发到 /v1/chat/completions", c.Request.URL.Path)
		c.Request.URL.Path = "/v1/chat/completions"
		r.HandleContext(c)
	})

	r.POST("/messages", func(c *gin.Context) {
		log.Printf("[cc-Switch兼容] 检测到根路径messages请求: %s，自动转发到 /v1/messages", c.Request.URL.Path)
		c.Request.URL.Path = "/v1/messages"
		r.HandleContext(c)
	})

	r.GET("/messages", func(c *gin.Context) {
		log.Printf("[cc-Switch兼容] 检测到根路径GET messages请求: %s，自动转发到 /v1/messages", c.Request.URL.Path)
		c.Request.URL.Path = "/v1/messages"
		r.HandleContext(c)
	})

	// 首页
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "AIThink - AI工具API服务",
			"version": "v1.0.0",
			"endpoints": gin.H{
				"health":             "GET /health",
				"login":              "POST /api/v1/login",
				"login_status":       "GET /api/v1/login/status",
				"ask":                "POST /api/v1/ask",
				"apikey_create":      "POST /api/v1/apikey/create",
				"apikey_list":        "GET /api/v1/apikey/list",
				"apikey_ask":         "POST /api/v1/apikey/ask",
				"memory_status":      "GET /api/v1/memory/:api_key",
				"memory_clear":       "DELETE /api/v1/memory/:api_key",
				"conversation_status": "GET /api/v1/conversation/:api_key",
				"conversation_reset": "POST /api/v1/conversation/:api_key/reset",
				"platforms_list":     "GET /api/v1/platforms",
				"platform_config":    "GET /api/v1/platforms/:name/config",
				"openai_chat":        "POST /v1/chat/completions",
				"anthropic_messages": "POST /v1/messages",
				"anthropic_models":   "GET /v1/models",
			},
		})
	})

	r.GET("/health", handler.HealthCheck)

	return r
}
