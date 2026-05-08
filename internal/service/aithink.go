package service

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"aithink/internal/browser"
	"aithink/internal/models"
)

// AIService AI服务
type AIService struct {
	browserMgr  *browser.BrowserManager
	loginStates map[string]*LoginState
	mu          sync.RWMutex
}

// LoginState 登录状态跟踪
type LoginState struct {
	Status    models.LoginStatus
	Message   string
	Error     error
	UpdatedAt time.Time
}

var (
	aiServiceInstance *AIService
	aiServiceOnce     sync.Once
)

// GetAIService 获取单例AI服务
func GetAIService() *AIService {
	aiServiceOnce.Do(func() {
		aiServiceInstance = &AIService{
			browserMgr:  browser.GetBrowserManager(),
			loginStates: make(map[string]*LoginState),
		}
	})
	return aiServiceInstance
}

// Login 启动登录流程（手动登录模式）
// 打开浏览器并导航到登录页面，用户手动完成登录
func (s *AIService) Login(req *models.LoginRequest) (*models.LoginResponse, error) {
	// 生成会话ID（如果未提供）
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("%s_%d", req.Platform, getCurrentTimestamp())
	}

	// 创建浏览器会话 - 使用固定的user_data_dir保持登录状态
	userDataDir := req.UserDataDir
	if userDataDir == "" {
		// 为不同平台使用固定的user_data_dir（保持登录状态）
		switch req.Platform {
		case models.PlatformZhipu:
			userDataDir = "sessions/zhipu_data" // 智谱清言固定目录
		case models.PlatformChatGPT:
			userDataDir = "sessions/chatgpt_data"
		case models.PlatformClaude:
			userDataDir = "sessions/claude_data"
		default:
			userDataDir = "sessions/default"
		}
		log.Printf("[%s] 使用固定user_data_dir: %s", sessionID, userDataDir)
	}

	if err := s.browserMgr.CreateSession(sessionID, userDataDir); err != nil {
		return nil, fmt.Errorf("创建会话失败: %v", err)
	}

	// 初始化登录状态
	s.mu.Lock()
	s.loginStates[sessionID] = &LoginState{
		Status:    models.LoginStatusPending,
		Message:   "浏览器已打开，请手动完成登录",
		UpdatedAt: time.Now(),
	}
	s.mu.Unlock()

	// 异步打开登录页面
	go s.openLoginPage(sessionID, req.Platform)

	log.Printf("[%s] 登录流程已启动，请手动在浏览器中完成登录", sessionID)

	return &models.LoginResponse{
		SessionID: sessionID,
		Status:    models.LoginStatusPending,
		Message:   "浏览器已打开，请手动完成登录。完成后调用 /api/v1/login/status 查询状态",
	}, nil
}

// openLoginPage 打开登录页面（异步）
func (s *AIService) openLoginPage(sessionID string, platform models.Platform) {
	log.Printf("[%s] openLoginPage 开始执行", sessionID)

	session, err := s.browserMgr.GetSession(sessionID)
	log.Printf("[%s] GetSession 完成, err=%v", sessionID, err)
	if err != nil {
		s.updateLoginState(sessionID, models.LoginStatusFailed, fmt.Sprintf("获取会话失败: %v", err), err)
		return
	}

	// 尝试加载已保存的cookies
	platformStr := string(platform)
	if s.browserMgr.HasValidCookies(platformStr) {
		log.Printf("[%s] 发现有效的cookies，尝试加载...", sessionID)
		err = s.browserMgr.LoadCookies(sessionID, platformStr)
		if err != nil {
			log.Printf("[%s] 加载cookies失败: %v", sessionID, err)
		} else {
			log.Printf("[%s] Cookies已加载", sessionID)
		}
	}

	// 检查是否已登录（加载cookie后检查）
	log.Printf("[%s] 开始检查登录状态", sessionID)
	client := browser.NewZhipuClient(session)
	if client.CheckLoggedIn() {
		log.Printf("[%s] 检测到已登录，跳过登录流程", sessionID)
		log.Printf("[%s] 检测到已登录状态，跳过登录流程", sessionID)
		s.updateLoginState(sessionID, models.LoginStatusSuccess, "已登录（复用已有会话）", nil)
		return
	}

	// 打开登录页面供用户手动登录
	err = client.OpenLoginPage()
	if err != nil {
		s.updateLoginState(sessionID, models.LoginStatusFailed, fmt.Sprintf("打开登录页面失败: %v", err), err)
		s.browserMgr.CloseSession(sessionID)
		return
	}

	// 更新状态：等待手动登录
	s.updateLoginState(sessionID, models.LoginStatusWaitingCode, "请在浏览器中手动完成登录", nil)

	// 启动自动检测登录状态的协程
	go s.monitorLoginStatus(sessionID)
}

// monitorLoginStatus 监控登录状态（自动检测用户是否已完成登录）
func (s *AIService) monitorLoginStatus(sessionID string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeout := time.After(30 * time.Minute) // 30分钟超时

	for {
		select {
		case <-ticker.C:
			session, err := s.browserMgr.GetSession(sessionID)
			if err != nil {
				log.Printf("[%s] 会话已失效: %v", sessionID, err)
				s.updateLoginState(sessionID, models.LoginStatusFailed, "会话已失效", err)
				return
			}

			client := browser.NewZhipuClient(session)
			if client.CheckLoggedIn() {
				log.Printf("[%s] 检测到手动登录成功！", sessionID)

				// 登录成功后保存cookies
				platform := s.getPlatformFromSessionID(sessionID)
				if platform != "" {
					err = s.browserMgr.SaveCookies(sessionID, platform)
					if err != nil {
						log.Printf("[%s] 保存cookies失败: %v", sessionID, err)
					} else {
						log.Printf("[%s] Cookies已保存", sessionID)
					}
				}

				s.updateLoginState(sessionID, models.LoginStatusSuccess, "登录成功（手动登录已确认）", nil)
				return
			}

		case <-timeout:
			log.Printf("[%s] 登录超时（30分钟）", sessionID)
			s.updateLoginState(sessionID, models.LoginStatusFailed, "登录超时", fmt.Errorf("超时"))
			return
		}
	}
}

// getPlatformFromSessionID 从sessionID提取平台名称
func (s *AIService) getPlatformFromSessionID(sessionID string) string {
	if strings.HasPrefix(sessionID, "zhipu") {
		return "zhipu"
	} else if strings.HasPrefix(sessionID, "chatgpt") {
		return "chatgpt"
	} else if strings.HasPrefix(sessionID, "claude") {
		return "claude"
	}
	return ""
}

// GetLoginStatus 查询登录状态
func (s *AIService) GetLoginStatus(sessionID string) (*models.LoginStatusResponse, error) {
	s.mu.RLock()
	state, exists := s.loginStates[sessionID]
	s.mu.RUnlock()

	if !exists || state == nil {
		return nil, fmt.Errorf("会话不存在: %s", sessionID)
	}

	// 如果状态是 waiting，主动检查一次
	if state.Status == models.LoginStatusWaitingCode {
		session, err := s.browserMgr.GetSession(sessionID)
		if err == nil {
			client := browser.NewZhipuClient(session)
			if client.CheckLoggedIn() {
				log.Printf("[%s] 查询时检测到登录成功", sessionID)
				s.updateLoginState(sessionID, models.LoginStatusSuccess, "登录成功", nil)
				state = s.loginStates[sessionID]
			}
		}

		return &models.LoginStatusResponse{
			SessionID: sessionID,
			Status:    state.Status,
			Message:   state.Message,
		}, nil
	}

	return &models.LoginStatusResponse{
		SessionID: sessionID,
		Status:    state.Status,
		Message:   state.Message,
	}, nil
}

// Ask 向AI平台提问
func (s *AIService) Ask(req *models.AskRequest) (*models.AskResponse, error) {
	// 检查登录状态
	s.mu.RLock()
	state := s.loginStates[req.SessionID]
	s.mu.RUnlock()

	if state == nil {
		return nil, fmt.Errorf("会话不存在或未登录，请先登录，session_id: %s", req.SessionID)
	}

	if state.Status != models.LoginStatusSuccess {
		return nil, fmt.Errorf("会话未登录或登录未完成，当前状态: %v", state.Status)
	}

	// 获取浏览器会话
	session, err := s.browserMgr.GetSession(req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("获取会话失败: %v", err)
	}

	// 根据平台类型执行提问（如果未指定平台，默认使用智谱）
	platform := req.Platform
	if platform == "" {
		platform = models.PlatformZhipu // 默认智谱清言
		log.Printf("[%s] 未指定平台，默认使用: %s", req.SessionID, platform)
	}

	switch platform {
	case models.PlatformZhipu:
		client := browser.NewZhipuClient(session)
		result, err := client.Ask(req.Question)
		if err != nil {
			return nil, fmt.Errorf("提问失败: %v", err)
		}

		// 打印流式内容（如果有）
		if result.StreamChan != nil {
			log.Println("开始接收流式回复...")
			for content := range result.StreamChan {
				log.Printf("📝 流式内容: %s", content)
			}
			log.Println("流式回复接收完成")
		}

		return &models.AskResponse{
			Answer:     result.Answer,
			SessionID:  req.SessionID,
			IsBot:      result.IsBot,
			DetectInfo: result.DetectInfo,
		}, nil
	default:
		return nil, fmt.Errorf("不支持的平台: %s", platform)
	}
}

// Logout 登出并关闭会话
func (s *AIService) Logout(sessionID string) error {
	s.mu.Lock()
	delete(s.loginStates, sessionID)
	s.mu.Unlock()

	return s.browserMgr.CloseSession(sessionID)
}

// CheckAntiDetection 检查反检测是否生效（自测功能）
// 返回检测结果、详细信息和错误
func (s *AIService) CheckAntiDetection(sessionID string) (bool, map[string]interface{}, error) {
	// 获取浏览器会话
	session, err := s.browserMgr.GetSession(sessionID)
	if err != nil {
		return false, nil, fmt.Errorf("获取会话失败: %v", err)
	}

	// 调用BrowserSession的CheckAntiDetection方法
	return session.CheckAntiDetection()
}

// updateLoginState 更新登录状态
func (s *AIService) updateLoginState(sessionID string, status models.LoginStatus, message string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.loginStates[sessionID] = &LoginState{
		Status:    status,
		Message:   message,
		Error:     err,
		UpdatedAt: time.Now(),
	}
}

// getCurrentTimestamp 获取当前时间戳（辅助函数）
func getCurrentTimestamp() int64 {
	return time.Now().UnixNano() / 1e6 // 毫秒时间戳
}
