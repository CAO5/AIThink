package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SetupRouter 设置路由
func SetupRouter() *gin.Engine {
	r := gin.Default()

	// 创建处理器
	handler := NewHandler()

	// 创建OpenAI兼容网关
	gateway := NewOpenAIGateway(handler.GetAPIKeyManager(), handler.aiService)

	// 创建Anthropic兼容网关
	anthropicGateway := NewAnthropicGateway(handler.GetAPIKeyManager(), handler.aiService)

	// API v1 路由组
	v1 := r.Group("/api/v1")
	{
		// 登录接口（手动登录模式：打开浏览器，用户手动完成登录）
		v1.POST("/login", handler.Login)

		// 查询登录状态接口（会主动检测浏览器页面状态）
		v1.GET("/login/status", handler.GetLoginStatus)

		// 提问接口（需先手动登录成功）
		v1.POST("/ask", handler.Ask)

		// 登出接口
		v1.POST("/logout", handler.Logout)

		// 配置管理接口
		v1.GET("/config", handler.GetConfig)
		v1.POST("/config", handler.UpdateConfig)
		
		// 反检测自测接口
		v1.GET("/anti-detection/check", handler.CheckAntiDetection)
	}

	// API Key 管理路由（需要认证？不需要，因为是管理接口，创建时才需要session_id验证）
	apikey := r.Group("/api/v1/apikey")
	{
		// API Key 管理接口（不需要认证）
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

	// OpenAI兼容接口（Claude Code使用）
	v1OpenAI := r.Group("/v1")
	{
		// 聊天补全接口（兼容OpenAI API）
		v1OpenAI.POST("/chat/completions", gateway.ChatCompletions)
		
		// 模型列表接口
		v1OpenAI.GET("/models", gateway.Models)
	}

	// Anthropic兼容接口（Claude Code原生API格式）
	v1Anthropic := r.Group("/v1")
	{
		// 消息接口（兼容Anthropic API）
		v1Anthropic.POST("/messages", anthropicGateway.Messages)
	}

	// 首页
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "AIThink - AI工具API服务",
			"version": "v1.0.0",
			"endpoints": gin.H{
				"health":               "GET /health",
				"login":                "POST /api/v1/login",
				"login_status":         "GET /api/v1/login/status",
				"ask":                  "POST /api/v1/ask",
				"apikey_create":        "POST /api/v1/apikey/create",
				"apikey_list":          "GET /api/v1/apikey/list",
				"apikey_ask":           "POST /api/v1/apikey/ask",
				"openai_chat":          "POST /v1/chat/completions",
				"anthropic_messages":   "POST /v1/messages",
			},
		})
	})

	// 健康检查
	r.GET("/health", handler.HealthCheck)

	return r
}
