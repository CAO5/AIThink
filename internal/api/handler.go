package api

import (
	"log"
	"net/http"
	"strings"

	"aithink/internal/config"
	"aithink/internal/memory"
	"aithink/internal/models"
	"aithink/internal/platform"
	"aithink/internal/service"

	"github.com/gin-gonic/gin"
)

// Handler API处理器
type Handler struct {
	aiService     *service.AIService
	apiKeyManager *service.APIKeyManager
}

// NewHandler 创建API处理器
func NewHandler() *Handler {
	return &Handler{
		aiService:     service.GetAIService(),
		apiKeyManager: service.NewAPIKeyManager(),
	}
}

// GetAPIKeyManager 获取API密钥管理器
func (h *Handler) GetAPIKeyManager() *service.APIKeyManager {
	return h.apiKeyManager
}

// Login 登录接口（手动登录模式）
// @Summary AI平台登录（手动登录）
// @Description 打开浏览器并导航到登录页面，用户手动完成登录
// @Accept json
// @Produce json
// @Param request body models.LoginRequest true "登录请求"
// @Success 200 {object} models.APIResponse
// @Router /api/v1/login [post]
func (h *Handler) Login(c *gin.Context) {
	var req models.LoginRequest

	// 解析请求体
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	// 验证平台类型
	if !isValidPlatform(req.Platform) {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "不支持的平台类型",
		})
		return
	}

	// 启动登录（打开浏览器，等待手动登录）
	resp, err := h.aiService.Login(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    0,
		Message: "浏览器已打开，请手动完成登录",
		Data:    resp,
	})
}

// GetLoginStatus 查询登录状态接口
// @Summary 查询登录状态
// @Description 查询指定会话的登录状态（会主动检测浏览器页面）
// @Produce json
// @Param session_id query string true "会话ID"
// @Success 200 {object} models.APIResponse
// @Router /api/v1/login/status [get]
func (h *Handler) GetLoginStatus(c *gin.Context) {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "session_id不能为空",
		})
		return
	}

	resp, err := h.aiService.GetLoginStatus(sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    0,
		Message: "查询成功",
		Data:    resp,
	})
}

// Ask 提问接口
// @Summary AI平台提问
// @Description 向指定的AI平台提问（需先手动登录成功）
// @Accept json
// @Produce json
// @Param request body models.AskRequest true "提问请求"
// @Success 200 {object} models.APIResponse
// @Router /api/v1/ask [post]
func (h *Handler) Ask(c *gin.Context) {
	var req models.AskRequest

	// 解析请求体
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	// 验证参数
	if req.SessionID == "" {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "session_id不能为空",
		})
		return
	}

	if req.Question == "" {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "question不能为空",
		})
		return
	}

	// 执行提问
	resp, err := h.aiService.Ask(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    0,
		Message: "提问成功",
		Data:    resp,
	})
}

// Logout 登出接口
// @Summary 登出AI平台
// @Description 关闭指定的会话
// @Produce json
// @Param session_id query string true "会话ID"
// @Success 200 {object} models.APIResponse
// @Router /api/v1/logout [post]
func (h *Handler) Logout(c *gin.Context) {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "session_id不能为空",
		})
		return
	}

	// 执行登出
	if err := h.aiService.Logout(sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    0,
		Message: "登出成功",
	})
}

// HealthCheck 健康检查接口
// @Summary 健康检查
// @Description 检查服务是否正常运行
// @Produce json
// @Success 200 {object} models.APIResponse
// @Router /health [get]
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, models.APIResponse{
		Code:    0,
		Message: "服务正常运行",
	})
}

// CheckAntiDetection 反检测自测接口
// @Summary 检查反检测是否生效
// @Description 检查当前会话的反检测措施是否生效
// @Produce json
// @Param session_id query string true "会话ID"
// @Success 200 {object} models.APIResponse
// @Router /api/v1/anti-detection/check [get]
func (h *Handler) CheckAntiDetection(c *gin.Context) {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "session_id不能为空",
		})
		return
	}

	// 调用service层检查反检测
	passed, details, err := h.aiService.CheckAntiDetection(sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	// 构建响应
	message := "反检测检查完成"
	if passed {
		message = "✅ 反检测检查通过！所有关键检测点都已正确伪装"
	} else {
		message = "❌ 反检测检查未完全通过，请查看详细信息"
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    0,
		Message: message,
		Data: gin.H{
			"passed":  passed,
			"details": details,
		},
	})
}

// isValidPlatform 验证平台类型是否有效
func isValidPlatform(platform models.Platform) bool {
	validPlatforms := []models.Platform{
		models.PlatformZhipu,
		models.PlatformChatGPT,
		models.PlatformClaude,
	}

	for _, p := range validPlatforms {
		if strings.EqualFold(string(p), string(platform)) {
			return true
		}
	}
	return false
}

// ==================== 配置管理接口 ====================

// GetConfig 获取当前配置
// @Summary 获取配置
// @Description 获取当前系统配置（不包含敏感信息）
// @Produce json
// @Success 200 {object} models.APIResponse
// @Router /api/v1/config [get]
func (h *Handler) GetConfig(c *gin.Context) {
	cfgMgr := config.GetConfigManager()
	cfg := cfgMgr.GetConfig()

	// 构建响应（不返回敏感信息）
	resp := models.ConfigResponse{
		ImageAI: models.ImageAIConfigResponse{
			Provider: cfg.ImageAI.Provider,
		},
	}

	// OpenAI配置（不返回APIKey）
	if cfg.ImageAI.Provider == "openai" {
		resp.ImageAI.OpenAI = models.OpenAIResponse{
			BaseURL: cfg.ImageAI.OpenAI.BaseURL,
			Model:   cfg.ImageAI.OpenAI.Model,
			HasKey:  cfg.ImageAI.OpenAI.APIKey != "",
		}
	}

	// 百度OCR配置（不返回密钥）
	if cfg.ImageAI.Provider == "baidu" {
		resp.ImageAI.Baidu = models.BaiduResponse{
			HasAPIKey:    cfg.ImageAI.Baidu.APIKey != "",
			HasSecretKey: cfg.ImageAI.Baidu.SecretKey != "",
		}
	}

	// 腾讯云OCR配置（不返回密钥）
	if cfg.ImageAI.Provider == "tencent" {
		resp.ImageAI.Tencent = models.TencentResponse{
			HasSecretID:  cfg.ImageAI.Tencent.SecretID != "",
			HasSecretKey: cfg.ImageAI.Tencent.SecretKey != "",
		}
	}

	// 自定义API配置（不返回密钥）
	if cfg.ImageAI.Provider == "custom" {
		resp.ImageAI.Custom = models.CustomResponse{
			URL:    cfg.ImageAI.Custom.URL,
			HasKey: cfg.ImageAI.Custom.APIKey != "",
		}
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    0,
		Message: "获取配置成功",
		Data:    resp,
	})
}

// UpdateConfig 更新配置
// @Summary 更新配置
// @Description 更新系统配置
// @Accept json
// @Produce json
// @Param request body models.ImageAIConfigRequest true "配置请求"
// @Success 200 {object} models.APIResponse
// @Router /api/v1/config [post]
func (h *Handler) UpdateConfig(c *gin.Context) {
	var req models.ImageAIConfigRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	cfgMgr := config.GetConfigManager()
	cfg := cfgMgr.GetConfig()

	// 更新Provider
	cfg.ImageAI.Provider = req.Provider

	// 更新OpenAI配置
	if req.Provider == "openai" && req.OpenAI.APIKey != "" {
		cfg.ImageAI.OpenAI.APIKey = req.OpenAI.APIKey
		if req.OpenAI.BaseURL != "" {
			cfg.ImageAI.OpenAI.BaseURL = req.OpenAI.BaseURL
		}
		if req.OpenAI.Model != "" {
			cfg.ImageAI.OpenAI.Model = req.OpenAI.Model
		}
	}

	// 更新百度OCR配置
	if req.Provider == "baidu" {
		if req.Baidu.APIKey != "" {
			cfg.ImageAI.Baidu.APIKey = req.Baidu.APIKey
		}
		if req.Baidu.SecretKey != "" {
			cfg.ImageAI.Baidu.SecretKey = req.Baidu.SecretKey
		}
	}

	// 更新腾讯云OCR配置
	if req.Provider == "tencent" {
		if req.Tencent.SecretID != "" {
			cfg.ImageAI.Tencent.SecretID = req.Tencent.SecretID
		}
		if req.Tencent.SecretKey != "" {
			cfg.ImageAI.Tencent.SecretKey = req.Tencent.SecretKey
		}
	}

	// 更新自定义API配置
	if req.Provider == "custom" {
		if req.Custom.URL != "" {
			cfg.ImageAI.Custom.URL = req.Custom.URL
		}
		if req.Custom.APIKey != "" {
			cfg.ImageAI.Custom.APIKey = req.Custom.APIKey
		}
		if req.Custom.Headers != nil {
			cfg.ImageAI.Custom.Headers = req.Custom.Headers
		}
	}

	// 保存配置
	cfgMgr.UpdateImageAIConfig(cfg.ImageAI)
	if err := cfgMgr.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Code:    500,
			Message: "保存配置失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    0,
		Message: "配置更新成功",
	})
}

// ==================== API Key 管理接口 ====================

// CreateAPIKey 创建API密钥
// @Summary 创建API密钥
// @Description 为已登录的会话创建API密钥
// @Accept json
// @Produce json
// @Param request body models.CreateAPIKeyRequest true "创建请求"
// @Success 200 {object} models.APIResponse
// @Router /api/v1/apikey/create [post]
func (h *Handler) CreateAPIKey(c *gin.Context) {
	var req models.CreateAPIKeyRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	resp, err := h.apiKeyManager.GenerateAPIKey(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    0,
		Message: "API密钥创建成功，请妥善保存（仅显示一次）",
		Data:    resp,
	})
}

// ListAPIKeys 列出所有API密钥
// @Summary 列出API密钥
// @Description 列出所有已创建的API密钥
// @Produce json
// @Success 200 {object} models.APIResponse
// @Router /api/v1/apikey/list [get]
func (h *Handler) ListAPIKeys(c *gin.Context) {
	resp := h.apiKeyManager.ListAPIKeys()

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    0,
		Message: "查询成功",
		Data:    resp,
	})
}

// UpdateAPIKey 更新API密钥
// @Summary 更新API密钥
// @Description 更新API密钥的名称或状态
// @Accept json
// @Produce json
// @Param apikey path string true "API密钥"
// @Param request body models.UpdateAPIKeyRequest true "更新请求"
// @Success 200 {object} models.APIResponse
// @Router /api/v1/apikey/update [post]
func (h *Handler) UpdateAPIKey(c *gin.Context) {
	apiKey := c.Param("apikey")
	if apiKey == "" {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "API密钥不能为空",
		})
		return
	}

	var req models.UpdateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	if err := h.apiKeyManager.UpdateAPIKey(apiKey, req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    0,
		Message: "API密钥更新成功",
	})
}

// DeleteAPIKey 删除API密钥
// @Summary 删除API密钥
// @Description 删除指定的API密钥
// @Param apikey path string true "API密钥"
// @Success 200 {object} models.APIResponse
// @Router /api/v1/apikey/delete [post]
func (h *Handler) DeleteAPIKey(c *gin.Context) {
	apiKey := c.Param("apikey")
	if apiKey == "" {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "API密钥不能为空",
		})
		return
	}

	if err := h.apiKeyManager.DeleteAPIKey(apiKey); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    0,
		Message: "API密钥已删除",
	})
}

// ==================== API Key 认证中间件 ====================

// APIKeyAuth API密钥认证中间件
func APIKeyAuth(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := extractAPIKey(c)
		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, models.APIResponse{
				Code:    401,
				Message: "缺少API密钥，请在请求头中提供 X-API-Key 或在查询参数中提供 api_key",
			})
			c.Abort()
			return
		}

		keyInfo, err := h.apiKeyManager.ValidateAPIKey(apiKey)
		if err != nil {
			c.JSON(http.StatusUnauthorized, models.APIResponse{
				Code:    401,
				Message: err.Error(),
			})
			c.Abort()
			return
		}

		// 将API密钥信息存储到上下文中，供后续使用
		c.Set("api_key_info", keyInfo)
		c.Next()
	}
}

// extractAPIKey 从请求中提取API密钥
func extractAPIKey(c *gin.Context) string {
	// 优先从请求头获取
	apiKey := c.GetHeader("X-API-Key")
	if apiKey != "" {
		return apiKey
	}

	// 从查询参数获取
	apiKey = c.Query("api_key")
	if apiKey != "" {
		return apiKey
	}

	return ""
}

// APIKeyAsk API Key方式提问接口
// @Summary API Key提问
// @Description 使用API密钥进行提问，自动关联到对应的浏览器会话
// @Accept json
// @Produce json
// @Param request body models.APIKeyAskRequest true "提问请求"
// @Success 200 {object} models.APIResponse
// @Router /api/v1/apikey/ask [post]
func (h *Handler) APIKeyAsk(c *gin.Context) {
	// 从上下文获取API密钥信息
	keyInfo, exists := c.Get("api_key_info")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.APIResponse{
			Code:    401,
			Message: "API密钥未验证",
		})
		return
	}

	keyInfoTyped := keyInfo.(*models.APIKeyInfo)

	var req models.APIKeyAskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	// 调试：打印接收到的 question 信息
	log.Printf("[API] 接收到question长度: %d", len(req.Question))
	qLen := len(req.Question)
	if qLen > 10 {
		qLen = 10
	}
	log.Printf("[API] question前10个字符: %s", req.Question[:qLen])
	// 打印每个字符的 rune 值
	count := 0
	for _, r := range req.Question {
		if count >= 5 {
			break
		}
		log.Printf("[API] 字符[%d]: rune=U+%04X, char=%c", count, r, r)
		count++
	}

	if req.Question == "" {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "question不能为空",
		})
		return
	}

	// 构建标准提问请求
	askReq := &models.AskRequest{
		Platform:  keyInfoTyped.Platform,
		SessionID: keyInfoTyped.SessionID,
		Question:  req.Question,
	}

	// 执行提问
	resp, err := h.aiService.Ask(askReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	// 更新使用记录
	h.apiKeyManager.UpdateUsage(keyInfoTyped.APIKey)

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    0,
		Message: "提问成功",
		Data:    resp,
	})
}

// APIKeyAskStream API Key方式提问接口（流式）
// @Summary API Key提问（流式）
// @Description 使用API密钥进行提问，流式返回结果
// @Accept json
// @Produce json
// @Param request body models.APIKeyAskRequest true "提问请求"
// @Success 200 {object} models.APIResponse
// @Router /api/v1/apikey/ask/stream [post]
func (h *Handler) APIKeyAskStream(c *gin.Context) {
	// 使用与普通API Key提问相同的逻辑
	h.APIKeyAsk(c)
}

// ==================== 记忆管理接口 ====================

// GetMemoryStatus 查看记忆状态
// @Summary 查看记忆状态
// @Description 查看指定API Key的对话记忆状态
// @Produce json
// @Param api_key path string true "API Key"
// @Success 200 {object} models.APIResponse
// @Router /api/v1/memory/{api_key} [get]
func (h *Handler) GetMemoryStatus(c *gin.Context) {
	apiKey := c.Param("api_key")
	if apiKey == "" {
		c.JSON(http.StatusBadRequest, models.APIResponse{Code: 400, Message: "api_key不能为空"})
		return
	}

	state := memory.GetGlobalMemoryManager().GetConversationState(apiKey)
	if state == nil {
		c.JSON(http.StatusOK, models.APIResponse{Code: 0, Message: "查询成功，无记忆数据", Data: nil})
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Code: 0, Message: "查询成功", Data: state})
}

// ClearMemory 清除记忆
// @Summary 清除记忆
// @Description 清除指定API Key的所有记忆数据
// @Produce json
// @Param api_key path string true "API Key"
// @Success 200 {object} models.APIResponse
// @Router /api/v1/memory/{api_key} [delete]
func (h *Handler) ClearMemory(c *gin.Context) {
	apiKey := c.Param("api_key")
	if apiKey == "" {
		c.JSON(http.StatusBadRequest, models.APIResponse{Code: 400, Message: "api_key不能为空"})
		return
	}

	memory.GetGlobalMemoryManager().ClearMemory(apiKey)
	c.JSON(http.StatusOK, models.APIResponse{Code: 0, Message: "记忆已清除"})
}

// ==================== 对话管理接口 ====================

// GetConversationStatus 查看对话状态
// @Summary 查看对话状态
// @Description 查看指定API Key的对话生命周期状态（包含记忆、会话ID等）
// @Produce json
// @Param api_key path string true "API Key"
// @Success 200 {object} models.APIResponse
// @Router /api/v1/conversation/{api_key} [get]
func (h *Handler) GetConversationStatus(c *gin.Context) {
	apiKey := c.Param("api_key")
	if apiKey == "" {
		c.JSON(http.StatusBadRequest, models.APIResponse{Code: 400, Message: "api_key不能为空"})
		return
	}

	convManager := memory.NewConversationManager(memory.GetGlobalMemoryManager())
	info := convManager.GetConversationInfo(apiKey)
	if info == nil {
		c.JSON(http.StatusOK, models.APIResponse{Code: 0, Message: "查询成功，无对话数据", Data: nil})
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Code: 0, Message: "查询成功", Data: info})
}

// ResetConversation 重置对话
// @Summary 重置对话
// @Description 重置指定API Key的对话状态，下次请求时会自动重建
// @Produce json
// @Param api_key path string true "API Key"
// @Success 200 {object} models.APIResponse
// @Router /api/v1/conversation/{api_key}/reset [post]
func (h *Handler) ResetConversation(c *gin.Context) {
	apiKey := c.Param("api_key")
	if apiKey == "" {
		c.JSON(http.StatusBadRequest, models.APIResponse{Code: 400, Message: "api_key不能为空"})
		return
	}

	convManager := memory.NewConversationManager(memory.GetGlobalMemoryManager())
	convManager.ResetConversation(apiKey)
	c.JSON(http.StatusOK, models.APIResponse{Code: 0, Message: "对话已重置"})
}

// ==================== 平台管理接口 ====================

// ListPlatforms 列出所有已注册平台
// @Summary 列出已注册平台
// @Description 获取所有已注册的AI平台列表
// @Produce json
// @Success 200 {object} models.APIResponse
// @Router /api/v1/platforms [get]
func (h *Handler) ListPlatforms(c *gin.Context) {
	platforms := platform.GetRegistry().ListPlatforms()
	c.JSON(http.StatusOK, models.APIResponse{Code: 0, Message: "查询成功", Data: platforms})
}

// GetPlatformConfig 获取平台配置
// @Summary 获取平台配置
// @Description 获取指定平台的详细配置信息
// @Produce json
// @Param name path string true "平台名称"
// @Success 200 {object} models.APIResponse
// @Router /api/v1/platforms/{name}/config [get]
func (h *Handler) GetPlatformConfig(c *gin.Context) {
	platformName := c.Param("name")
	if platformName == "" {
		c.JSON(http.StatusBadRequest, models.APIResponse{Code: 400, Message: "平台名称不能为空"})
		return
	}

	config, err := platform.GetRegistry().GetConfig(models.Platform(platformName))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Code: 400, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Code: 0, Message: "查询成功", Data: config})
}

