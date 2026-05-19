package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"aithink/internal/browser"
	"aithink/internal/models"
	"aithink/internal/platform"
	_ "aithink/internal/platform/deepseek" // 通过 init() 注册DeepSeek平台
	_ "aithink/internal/platform/doubao"   // 通过 init() 注册豆包平台
	_ "aithink/internal/platform/gpt"   // 通过 init() 注册ChatGPT平台
	_ "aithink/internal/platform/qwen"  // 通过 init() 注册千问平台
	_ "aithink/internal/platform/zhipu" // 通过 init() 注册智谱平台

	"github.com/chromedp/chromedp"
)

// AIService AI服务
type AIService struct {
	browserMgr     *browser.BrowserManager
	cookieStore    *browser.CookieStore
	sessionManager *SessionManager
	loginStates    map[string]*LoginState
	mu             sync.RWMutex
}

// LoginState 登录状态跟踪
type LoginState struct {
	Status    models.LoginStatus
	Message   string
	Error     error
	UpdatedAt time.Time
	Platform  models.Platform // 关联的AI平台类型
}

var (
	aiServiceInstance *AIService
	aiServiceOnce     sync.Once
)

// GetAIService 获取单例AI服务
func GetAIService() *AIService {
	aiServiceOnce.Do(func() {
		browserMgr := browser.GetBrowserManager()
		cookieStore := browser.NewCookieStore("sessions/cookies")
		apiKeyMgr := NewAPIKeyManager()
		
		aiServiceInstance = &AIService{
			browserMgr:     browserMgr,
			cookieStore:    cookieStore,
			sessionManager: NewSessionManager(browserMgr, cookieStore, apiKeyMgr),
			loginStates:    make(map[string]*LoginState),
		}
		
		// 注入cookie store到browser manager
		browserMgr.SetCookieStore(cookieStore)
		
		// 启动会话管理器
		aiServiceInstance.sessionManager.Start()
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

	if err := s.browserMgr.CreateSession(sessionID, userDataDir, string(req.Platform)); err != nil {
		return nil, fmt.Errorf("创建会话失败: %v", err)
	}

	// 初始化登录状态
	s.mu.Lock()
	s.loginStates[sessionID] = &LoginState{
		Status:    models.LoginStatusPending,
		Message:   "浏览器已打开，请手动完成登录",
		UpdatedAt: time.Now(),
		Platform:  req.Platform,
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
func (s *AIService) openLoginPage(sessionID string, platformType models.Platform) {
	log.Printf("[%s] openLoginPage 开始执行", sessionID)

	session, err := s.browserMgr.GetSession(sessionID)
	log.Printf("[%s] GetSession 完成, err=%v", sessionID, err)
	if err != nil {
		s.updateLoginState(sessionID, models.LoginStatusFailed, fmt.Sprintf("获取会话失败: %v", err), err)
		return
	}

	// 尝试加载已保存的cookies
	platformStr := string(platformType)
	// 通过注册器获取平台客户端（替代硬编码的ZhipuClient）
	client, err := platform.GetRegistry().GetClient(platformType, session)
	if err != nil {
		s.updateLoginState(sessionID, models.LoginStatusFailed, fmt.Sprintf("获取平台客户端失败: %v", err), err)
		return
	}
	
	if s.browserMgr.HasValidCookies(platformStr) {
		log.Printf("[%s] 发现有效的cookies，尝试加载...", sessionID)
		err = s.browserMgr.LoadCookies(sessionID, platformStr)
		if err != nil {
			log.Printf("[%s] 加载cookies失败: %v", sessionID, err)
		} else {
			log.Printf("[%s] Cookies已加载", sessionID)
		}
		
		// 加载cookies后，先导航到目标网站让cookies生效
		log.Printf("[%s] 导航到目标网站使cookies生效...", sessionID)
		if navErr := client.NavigateToHome(); navErr != nil {
			log.Printf("[%s] 导航失败: %v", sessionID, navErr)
		} else {
			log.Printf("[%s] 导航完成，等待页面加载...", sessionID)
		}
		
		// 检查是否已登录（加载cookie后检查）
		log.Printf("[%s] 开始检查登录状态", sessionID)
		if client.CheckLoggedIn() {
			log.Printf("[%s] 检测到已登录，跳过登录流程", sessionID)
			log.Printf("[%s] 检测到已登录状态，跳过登录流程", sessionID)
			s.updateLoginState(sessionID, models.LoginStatusSuccess, "已登录（复用已有会话）", nil)
			
			// 注册会话到SessionManager进行健康监控
			s.sessionManager.RegisterSession(sessionID, platformType)
			log.Printf("[%s] 会话已注册到SessionManager", sessionID)
			return
		}
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

			// 从登录状态中获取平台信息
			s.mu.RLock()
			loginState, exists := s.loginStates[sessionID]
			s.mu.RUnlock()
			if !exists || loginState.Platform == "" {
				log.Printf("[%s] 无法获取平台信息", sessionID)
				s.updateLoginState(sessionID, models.LoginStatusFailed, "无法获取平台信息", fmt.Errorf("平台信息缺失"))
				return
			}

			// 通过注册器获取平台客户端
			client, clientErr := platform.GetRegistry().GetClient(loginState.Platform, session)
			if clientErr != nil {
				log.Printf("[%s] 获取平台客户端失败: %v", sessionID, clientErr)
				s.updateLoginState(sessionID, models.LoginStatusFailed, fmt.Sprintf("获取平台客户端失败: %v", clientErr), clientErr)
				return
			}

			if client.CheckLoggedIn() {
				log.Printf("[%s] 检测到手动登录成功！", sessionID)

				// 登录成功后保存cookies
				platformStr := string(loginState.Platform)
				err = s.browserMgr.SaveCookies(sessionID, platformStr)
				if err != nil {
					log.Printf("[%s] 保存cookies失败: %v", sessionID, err)
				} else {
					log.Printf("[%s] Cookies已保存", sessionID)
				}
				
				// 注册会话到SessionManager进行健康监控
				s.sessionManager.RegisterSession(sessionID, loginState.Platform)
				log.Printf("[%s] 会话已注册到SessionManager", sessionID)

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
			// 通过注册器获取平台客户端
			client, clientErr := platform.GetRegistry().GetClient(state.Platform, session)
			if clientErr == nil && client.CheckLoggedIn() {
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

// Ask 向AI平台提问（带超时和重试）
// 默认行为：新建对话。可通过 req.ConversationMode 指定对话模式
func (s *AIService) Ask(req *models.AskRequest) (*models.AskResponse, error) {
	// 如果指定了对话模式，委托给 AskWithConversation 处理
	if req.ConversationMode != "" {
		return s.AskWithConversation(req, req.ConversationMode)
	}

	// 先检查会话状态
	s.mu.RLock()
	state := s.loginStates[req.SessionID]
	s.mu.RUnlock()

	if state == nil || state.Status != models.LoginStatusSuccess {
		// 会话不存在或未登录，尝试自动创建会话（通过cookie恢复）
		log.Printf("[%s] 会话不存在或未登录，尝试自动创建会话...", req.SessionID)
		if err := s.autoCreateSession(req.SessionID, req.Platform); err != nil {
			return nil, fmt.Errorf("自动创建会话失败: %v", err)
		}
		// 再次检查状态
		s.mu.RLock()
		state = s.loginStates[req.SessionID]
		s.mu.RUnlock()
		if state == nil || state.Status != models.LoginStatusSuccess {
			return nil, fmt.Errorf("会话创建失败，请手动登录，session_id: %s", req.SessionID)
		}
	}

	// 检查会话是否健康
	if !s.sessionManager.IsSessionHealthy(req.SessionID) {
		log.Printf("[%s] 会话不健康，尝试恢复...", req.SessionID)
		sessionState := s.sessionManager.GetSessionState(req.SessionID)
		if sessionState != nil {
			s.sessionManager.UnregisterSession(req.SessionID)
		}
		if err := s.autoCreateSession(req.SessionID, req.Platform); err != nil {
			return nil, fmt.Errorf("恢复会话失败: %v", err)
		}
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

	// 带重试的提问逻辑
	maxRetries := 2
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("[%s] 提问尝试 %d/%d", req.SessionID, attempt, maxRetries)
		
		result, err := s.executeAsk(session, platform, req.Question, req.SessionID)
		if err == nil {
			return result, nil
		}
		
		lastErr = err
		log.Printf("[%s] 提问失败 (尝试 %d/%d): %v", req.SessionID, attempt, maxRetries, err)
		
		// 如果不是最后一次尝试，等待后重试
		if attempt < maxRetries {
			log.Printf("[%s] 等待2秒后重试...", req.SessionID)
			time.Sleep(2 * time.Second)
			
			// 检查会话是否还有效
			if _, sessionErr := s.browserMgr.GetSession(req.SessionID); sessionErr != nil {
				log.Printf("[%s] 会话已失效，尝试恢复...", req.SessionID)
				if recoverErr := s.autoCreateSession(req.SessionID, platform); recoverErr != nil {
					return nil, fmt.Errorf("恢复会话失败: %v, 原始错误: %v", recoverErr, err)
				}
				// 重新获取会话
				session, err = s.browserMgr.GetSession(req.SessionID)
				if err != nil {
					return nil, fmt.Errorf("重新获取会话失败: %v", err)
				}
			}
		}
	}

	return nil, fmt.Errorf("提问失败，已重试%d次: %v", maxRetries, lastErr)
}

// AskWithConversation 根据对话模式向AI平台提问
// conversationMode 决定提问方式：新建对话、在已有对话中继续
func (s *AIService) AskWithConversation(req *models.AskRequest, conversationMode models.ConversationMode) (*models.AskResponse, error) {
	// 先检查会话状态
	s.mu.RLock()
	state := s.loginStates[req.SessionID]
	s.mu.RUnlock()

	if state == nil || state.Status != models.LoginStatusSuccess {
		// 会话不存在或未登录，尝试自动创建会话（通过cookie恢复）
		log.Printf("[%s] 会话不存在或未登录，尝试自动创建会话...", req.SessionID)
		if err := s.autoCreateSession(req.SessionID, req.Platform); err != nil {
			return nil, fmt.Errorf("自动创建会话失败: %v", err)
		}
		// 再次检查状态
		s.mu.RLock()
		state = s.loginStates[req.SessionID]
		s.mu.RUnlock()
		if state == nil || state.Status != models.LoginStatusSuccess {
			return nil, fmt.Errorf("会话创建失败，请手动登录，session_id: %s", req.SessionID)
		}
	}

	// 检查会话是否健康
	if !s.sessionManager.IsSessionHealthy(req.SessionID) {
		log.Printf("[%s] 会话不健康，尝试恢复...", req.SessionID)
		sessionState := s.sessionManager.GetSessionState(req.SessionID)
		if sessionState != nil {
			s.sessionManager.UnregisterSession(req.SessionID)
		}
		if err := s.autoCreateSession(req.SessionID, req.Platform); err != nil {
			return nil, fmt.Errorf("恢复会话失败: %v", err)
		}
	}

	// 获取浏览器会话
	session, err := s.browserMgr.GetSession(req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("获取会话失败: %v", err)
	}

	// 确定平台类型
	platformType := req.Platform
	if platformType == "" {
		platformType = models.PlatformZhipu // 默认智谱清言
		log.Printf("[%s] 未指定平台，默认使用: %s", req.SessionID, platformType)
	}

	// 通过注册器获取平台客户端
	client, clientErr := platform.GetRegistry().GetClient(platformType, session)
	if clientErr != nil {
		return nil, fmt.Errorf("获取平台客户端失败: %v", clientErr)
	}

	// 创建超时上下文
	timeoutCtx, cancel := context.WithTimeout(session.Ctx, 5*time.Minute)
	defer cancel()

	var result *platform.AskResult
	var askErr error

	// 根据对话模式选择不同的提问方式
	done := make(chan bool, 1)
	go func() {
		switch conversationMode {
		case models.ConversationModeExisting:
			// 在已有对话中继续提问
			log.Printf("[%s] 在已有对话中继续提问", req.SessionID)
			result, askErr = client.AskInConversation(req.Question)
		case models.ConversationModeNew:
			// 新建对话并发送初始消息
			log.Printf("[%s] 新建对话并发送初始消息", req.SessionID)
			result, askErr = client.StartNewConversation(req.Question)
		default:
			// 默认行为：新建对话
			log.Printf("[%s] 默认模式：新建对话提问", req.SessionID)
			result, askErr = client.Ask(req.Question)
		}
		done <- true
	}()

	select {
	case <-done:
		if askErr != nil {
			return nil, fmt.Errorf("提问失败: %v", askErr)
		}

		// 打印流式内容（如果有）
		if result.StreamChan != nil {
			log.Println("开始接收流式回复...")
			for content := range result.StreamChan {
				log.Printf("流式内容: %s", content)
			}
			log.Println("流式回复接收完成")
		}

		return &models.AskResponse{
			Answer:     result.Answer,
			Thinking:   result.Thinking,
			SessionID:  req.SessionID,
			IsBot:      result.IsBot,
			DetectInfo: result.DetectInfo,
		}, nil
	case <-timeoutCtx.Done():
		return nil, fmt.Errorf("提问超时（5分钟限制）")
	}
}

// executeAsk 执行提问（内部方法）
func (s *AIService) executeAsk(session *browser.BrowserSession, platformType models.Platform, question, sessionID string) (*models.AskResponse, error) {
	// 创建超时上下文
	timeoutCtx, cancel := context.WithTimeout(session.Ctx, 5*time.Minute)
	defer cancel()

	var result *platform.AskResult
	var err error

	// 通过注册器获取平台客户端
	client, clientErr := platform.GetRegistry().GetClient(platformType, session)
	if clientErr != nil {
		return nil, fmt.Errorf("获取平台客户端失败: %v", clientErr)
	}

	// 在超时上下文中执行提问
	done := make(chan bool, 1)
	go func() {
		result, err = client.Ask(question)
		done <- true
	}()

	select {
	case <-done:
		// 提问完成
		if err != nil {
			return nil, fmt.Errorf("提问失败: %v", err)
		}

		// 打印流式内容（如果有）
		if result.StreamChan != nil {
			log.Println("开始接收流式回复...")
			for content := range result.StreamChan {
				log.Printf("流式内容: %s", content)
			}
			log.Println("流式回复接收完成")
		}

		return &models.AskResponse{
			Answer:     result.Answer,
			Thinking:   result.Thinking,
			SessionID:  sessionID,
			IsBot:      result.IsBot,
			DetectInfo: result.DetectInfo,
		}, nil
	case <-timeoutCtx.Done():
		return nil, fmt.Errorf("提问超时（5分钟限制）")
	}
}

// Logout 登出并关闭会话
func (s *AIService) Logout(sessionID string) error {
	// 从SessionManager注销会话
	s.sessionManager.UnregisterSession(sessionID)
	
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

	// 保留原有的平台信息
	existingState, exists := s.loginStates[sessionID]
	var platform models.Platform
	if exists && existingState != nil {
		platform = existingState.Platform
	}

	s.loginStates[sessionID] = &LoginState{
		Status:    status,
		Message:   message,
		Error:     err,
		UpdatedAt: time.Now(),
		Platform:  platform,
	}
}

// getCurrentTimestamp 获取当前时间戳（辅助函数）
func getCurrentTimestamp() int64 {
	return time.Now().UnixNano() / 1e6 // 毫秒时间戳
}

// autoCreateSession 自动创建会话（通过cookie恢复）
func (s *AIService) autoCreateSession(sessionID string, platformType models.Platform) error {
	var userDataDir string
	switch platformType {
	case models.PlatformZhipu:
		userDataDir = "sessions/zhipu_data"
	case models.PlatformChatGPT:
		userDataDir = "sessions/chatgpt_data"
	case models.PlatformClaude:
		userDataDir = "sessions/claude_data"
	default:
		userDataDir = "sessions/default"
	}

	log.Printf("[%s] 自动创建会话，userDataDir: %s", sessionID, userDataDir)

	// 创建浏览器会话
	if err := s.browserMgr.CreateSession(sessionID, userDataDir, string(platformType)); err != nil {
		return fmt.Errorf("创建会话失败: %v", err)
	}

	// 初始化登录状态为pending
	s.mu.Lock()
	s.loginStates[sessionID] = &LoginState{
		Status:    models.LoginStatusPending,
		Message:   "正在通过cookie恢复会话...",
		UpdatedAt: time.Now(),
		Platform:  platformType,
	}
	s.mu.Unlock()

	// 尝试加载cookie并导航到目标网站
	platformStr := string(platformType)
	if s.browserMgr.HasValidCookies(platformStr) {
		log.Printf("[%s] 发现有效cookie，加载...", sessionID)
		
		// 获取会话
		session, err := s.browserMgr.GetSession(sessionID)
		if err != nil {
			return err
		}
		
		// 通过注册器获取平台客户端
		client, clientErr := platform.GetRegistry().GetClient(platformType, session)
		if clientErr != nil {
			log.Printf("[%s] 获取平台客户端失败: %v", sessionID, clientErr)
			return fmt.Errorf("获取平台客户端失败: %v", clientErr)
		}

		// 先导航到目标网站，让cookie可以应用
		if err := client.NavigateToHome(); err != nil {
			log.Printf("[%s] 导航失败: %v", sessionID, err)
		} else {
			log.Printf("[%s] 导航完成，准备应用cookie...", sessionID)
		}
		
		// 再加载cookie
		if err := s.browserMgr.LoadCookies(sessionID, platformStr); err != nil {
			log.Printf("[%s] 加载cookie失败: %v", sessionID, err)
			return fmt.Errorf("加载cookie失败: %v", err)
		}
		
		// 刷新页面使cookie生效
		chromedp.Run(session.Ctx, chromedp.Reload())
		time.Sleep(2 * time.Second)
		
		// 检查是否已登录
		if client.CheckLoggedIn() {
			log.Printf("[%s] cookie恢复成功", sessionID)
			
			// 注册会话到SessionManager进行健康监控
			s.sessionManager.RegisterSession(sessionID, platformType)
			log.Printf("[%s] 会话已注册到SessionManager", sessionID)
			
			s.mu.Lock()
			s.loginStates[sessionID] = &LoginState{
				Status:    models.LoginStatusSuccess,
				Message:   "cookie恢复成功",
				UpdatedAt: time.Now(),
				Platform:  platformType,
			}
			s.mu.Unlock()
			return nil
		}
	}

	return fmt.Errorf("cookie无效或已过期")
}
