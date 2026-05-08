package api

import (
	"net/http"
	"strings"

	"aithink/internal/config"
	"aithink/internal/models"
	"aithink/internal/service"

	"github.com/gin-gonic/gin"
)

// Handler API处理器
type Handler struct {
	aiService *service.AIService
}

// NewHandler 创建API处理器
func NewHandler() *Handler {
	return &Handler{
		aiService: service.GetAIService(),
	}
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
