package models

// AI平台类型枚举
type Platform string

const (
	PlatformZhipu   Platform = "zhipu"   // 智谱清言
	PlatformChatGPT Platform = "chatgpt" // ChatGPT
	PlatformClaude  Platform = "claude"  // Claude
)

// 登录状态
type LoginStatus string

const (
	LoginStatusPending         LoginStatus = "pending"          // 登录进行中（浏览器已打开）
	LoginStatusWaitingCode     LoginStatus = "waiting_for_code" // 等待用户手动登录
	LoginStatusSuccess         LoginStatus = "success"          // 登录成功
	LoginStatusFailed          LoginStatus = "failed"           // 登录失败
)

// 登录请求
type LoginRequest struct {
	Platform     Platform `json:"platform"`
	Username     string   `json:"username"`
	Password     string   `json:"password"`
	SessionID    string   `json:"session_id,omitempty"`        // 可选，用于复用会话
	UserDataDir  string   `json:"user_data_dir,omitempty"`    // Chrome用户数据目录，用于保持登录状态
}

// 提交验证码请求
type SubmitCodeRequest struct {
	SessionID string `json:"session_id"`
	Code      string `json:"code"`
}

// 登录状态查询响应
type LoginStatusResponse struct {
	SessionID string      `json:"session_id"`
	Status    LoginStatus `json:"status"`
	Message   string      `json:"message"`
}

// 提问请求
type AskRequest struct {
	Platform  Platform `json:"platform"`
	SessionID string   `json:"session_id"` // 会话ID
	Question  string   `json:"question"`
}

// API响应
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// 登录响应
type LoginResponse struct {
	SessionID string      `json:"session_id"`
	Status    LoginStatus `json:"status"`
	Message   string      `json:"message"`
}

// 提问响应
type AskResponse struct {
	Answer     string `json:"answer"`
	SessionID  string `json:"session_id"`
	IsBot      bool   `json:"is_bot,omitempty"`      // 是否被检测为机器人
	DetectInfo string `json:"detect_info,omitempty"` // 检测信息
}

// ==================== 配置管理相关类型 ====================

// ImageAIConfigRequest 图片识别AI配置请求
type ImageAIConfigRequest struct {
	Provider string              `json:"provider"` // openai, baidu, tencent, custom
	OpenAI   OpenAIConfigRequest `json:"openai,omitempty"`
	Baidu    BaiduConfigRequest `json:"baidu,omitempty"`
	Tencent  TencentConfigRequest `json:"tencent,omitempty"`
	Custom   CustomConfigRequest `json:"custom,omitempty"`
}

// OpenAIConfigRequest OpenAI配置请求
type OpenAIConfigRequest struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url,omitempty"`
	Model   string `json:"model,omitempty"`
}

// BaiduConfigRequest 百度OCR配置请求
type BaiduConfigRequest struct {
	APIKey    string `json:"api_key"`
	SecretKey string `json:"secret_key"`
}

// TencentConfigRequest 腾讯云OCR配置请求
type TencentConfigRequest struct {
	SecretID  string `json:"secret_id"`
	SecretKey string `json:"secret_key"`
}

// CustomConfigRequest 自定义API配置请求
type CustomConfigRequest struct {
	URL     string            `json:"url"`
	APIKey  string            `json:"api_key,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// ConfigResponse 配置响应
type ConfigResponse struct {
	ImageAI ImageAIConfigResponse `json:"image_ai"`
}

// ImageAIConfigResponse 图片识别AI配置响应（不返回敏感信息）
type ImageAIConfigResponse struct {
	Provider string `json:"provider"`
	OpenAI   OpenAIResponse `json:"openai,omitempty"`
	Baidu    BaiduResponse `json:"baidu,omitempty"`
	Tencent  TencentResponse `json:"tencent,omitempty"`
	Custom   CustomResponse `json:"custom,omitempty"`
}

// OpenAIResponse OpenAI配置响应（不返回APIKey）
type OpenAIResponse struct {
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
	HasKey  bool   `json:"has_key"` // 只告知是否配置了key，不返回key本身
}

// BaiduResponse 百度OCR配置响应（不返回密钥）
type BaiduResponse struct {
	HasAPIKey    bool `json:"has_api_key"`
	HasSecretKey bool `json:"has_secret_key"`
}

// TencentResponse 腾讯云OCR配置响应（不返回密钥）
type TencentResponse struct {
	HasSecretID  bool `json:"has_secret_id"`
	HasSecretKey bool `json:"has_secret_key"`
}

// CustomResponse 自定义API配置响应（不返回密钥）
type CustomResponse struct {
	URL    string `json:"url"`
	HasKey bool   `json:"has_key"`
}
