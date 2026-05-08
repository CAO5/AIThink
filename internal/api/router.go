package api

import "github.com/gin-gonic/gin"

// SetupRouter 设置路由
func SetupRouter() *gin.Engine {
	r := gin.Default()

	// 创建处理器
	handler := NewHandler()

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

	// 健康检查
	r.GET("/health", handler.HealthCheck)

	return r
}
