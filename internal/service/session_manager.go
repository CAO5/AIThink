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
	_ "aithink/internal/platform/qwen"  // 通过 init() 注册千问平台
	_ "aithink/internal/platform/zhipu" // 通过 init() 注册智谱平台
)

// SessionState 会话运行状态
type SessionState struct {
	SessionID      string
	Platform       models.Platform
	IsActive       bool
	LastHealthCheck time.Time
	HealthCheckPass bool
	CrashCount     int
	LastCrashTime  time.Time
	CreatedAt      time.Time
	LastRecovered  time.Time
}

// SessionManager 会话管理器（带自动恢复）
type SessionManager struct {
	mu            sync.RWMutex
	browserMgr    *browser.BrowserManager
	cookieStore   *browser.CookieStore
	apiKeyManager *APIKeyManager
	sessions      map[string]*SessionState
	ctx           context.Context
	cancel        context.CancelFunc
	checkInterval time.Duration
	maxCrashCount int
	maxSessions   int           // 最大会话数
	sessionSem    chan struct{} // 信号量控制并发
}

// NewSessionManager 创建会话管理器
func NewSessionManager(browserMgr *browser.BrowserManager, cookieStore *browser.CookieStore, apiKeyManager *APIKeyManager) *SessionManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	maxSessions := 5 // 默认最大5个并发会话
	
	sm := &SessionManager{
		browserMgr:    browserMgr,
		cookieStore:   cookieStore,
		apiKeyManager: apiKeyManager,
		sessions:      make(map[string]*SessionState),
		ctx:           ctx,
		cancel:        cancel,
		checkInterval: 60 * time.Second, // 每60秒检查一次
		maxCrashCount: 5,               // 最大崩溃次数
		maxSessions:   maxSessions,
		sessionSem:    make(chan struct{}, maxSessions),
	}
	
	return sm
}

// Start 启动健康检查循环
func (sm *SessionManager) Start() {
	log.Println("SessionManager: 健康检查循环已启动")
	go sm.healthCheckLoop()
	go sm.autoSaveCookiesLoop()
}

// Stop 停止健康检查
func (sm *SessionManager) Stop() {
	log.Println("SessionManager: 正在停止...")
	sm.cancel()
}

// RegisterSession 注册需要监控的会话
func (sm *SessionManager) RegisterSession(sessionID string, platform models.Platform) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if _, exists := sm.sessions[sessionID]; exists {
		return // 已注册
	}
	
	// 检查是否超过最大会话数
	if len(sm.sessions) >= sm.maxSessions {
		log.Printf("SessionManager: 已达到最大会话数限制(%d)，拒绝注册会话 %s", sm.maxSessions, sessionID)
		return
	}
	
	sm.sessions[sessionID] = &SessionState{
		SessionID:     sessionID,
		Platform:      platform,
		IsActive:      true,
		CreatedAt:     time.Now(),
		HealthCheckPass: true,
	}
	
	// 获取信号量
	select {
	case sm.sessionSem <- struct{}{}:
		log.Printf("SessionManager: 已获取会话槽位 (当前: %d/%d)", len(sm.sessionSem), sm.maxSessions)
	default:
		log.Printf("SessionManager: 无可用会话槽位，等待中...")
		// 阻塞等待槽位
		sm.sessionSem <- struct{}{}
	}
	
	log.Printf("SessionManager: 已注册会话 %s (平台: %s)", sessionID, platform)
}

// UnregisterSession 注销会话
func (sm *SessionManager) UnregisterSession(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	delete(sm.sessions, sessionID)
	
	// 释放信号量
	select {
	case <-sm.sessionSem:
		log.Printf("SessionManager: 已释放会话槽位 (当前: %d/%d)", len(sm.sessionSem), sm.maxSessions)
	default:
	}
	
	log.Printf("SessionManager: 已注销会话 %s", sessionID)
}

// healthCheckLoop 健康检查循环
func (sm *SessionManager) healthCheckLoop() {
	ticker := time.NewTicker(sm.checkInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-sm.ctx.Done():
			log.Println("SessionManager: 健康检查循环已停止")
			return
		case <-ticker.C:
			sm.checkAllSessions()
		}
	}
}

// checkAllSessions 检查所有会话状态
func (sm *SessionManager) checkAllSessions() {
	sm.mu.RLock()
	sessions := make([]*SessionState, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		if s.IsActive {
			sessions = append(sessions, s)
		}
	}
	sm.mu.RUnlock()
	
	for _, session := range sessions {
		sm.checkSingleSession(session)
	}
}

// checkSingleSession 检查单个会话
func (sm *SessionManager) checkSingleSession(state *SessionState) {
	state.LastHealthCheck = time.Now()
	
	// 检查浏览器会话是否存在
	_, err := sm.browserMgr.GetSession(state.SessionID)
	if err != nil {
		log.Printf("SessionManager: 会话 %s 不存在，尝试恢复...", state.SessionID)
		state.HealthCheckPass = false
		sm.recoverSession(state)
		return
	}
	
	// 检查登录状态是否仍然有效
	client, err := sm.createPlatformClient(state.SessionID, state.Platform)
	if err != nil {
		state.HealthCheckPass = false
		sm.recoverSession(state)
		return
	}
	
	if client.CheckLoggedIn() {
		state.HealthCheckPass = true
		state.CrashCount = 0 // 重置崩溃计数
	} else {
		log.Printf("SessionManager: 会话 %s 登录已失效，尝试恢复...", state.SessionID)
		state.HealthCheckPass = false
		sm.recoverSession(state)
	}
}

// recoverSession 恢复会话
func (sm *SessionManager) recoverSession(state *SessionState) {
	// 检查是否超过最大崩溃次数
	now := time.Now()
	if state.CrashCount >= sm.maxCrashCount {
		// 检查是否是最近10分钟内连续崩溃
		if now.Sub(state.LastCrashTime) < 10*time.Minute {
			log.Printf("SessionManager: 会话 %s 崩溃次数过多(%d)，暂停恢复", state.SessionID, state.CrashCount)
			state.IsActive = false
			return
		}
		// 重置计数
		state.CrashCount = 0
	}
	
	state.CrashCount++
	state.LastCrashTime = now
	
	log.Printf("SessionManager: 正在恢复会话 %s (第%d次尝试)...", state.SessionID, state.CrashCount)
	
	// 使用指数退避策略等待
	backoffDuration := time.Duration(state.CrashCount) * 2 * time.Second
	if backoffDuration > 30*time.Second {
		backoffDuration = 30 * time.Second
	}
	log.Printf("SessionManager: 等待 %v 后开始恢复...", backoffDuration)
	time.Sleep(backoffDuration)
	
	// 关闭旧会话（如果存在）
	if err := sm.browserMgr.CloseSession(state.SessionID); err != nil {
		log.Printf("SessionManager: 关闭旧会话失败: %v", err)
	}
	
	// 重新创建会话
	var userDataDir string
	switch state.Platform {
	case models.PlatformZhipu:
		userDataDir = "sessions/zhipu_data"
	case models.PlatformChatGPT:
		userDataDir = "sessions/chatgpt_data"
	case models.PlatformClaude:
		userDataDir = "sessions/claude_data"
	default:
		userDataDir = "sessions/default"
	}
	
	// 重试3次创建会话
	var createErr error
	for attempt := 1; attempt <= 3; attempt++ {
		log.Printf("SessionManager: 创建会话 %s (尝试 %d/3)...", state.SessionID, attempt)
		createErr = sm.browserMgr.CreateSession(state.SessionID, userDataDir, string(state.Platform))
		if createErr == nil {
			break
		}
		log.Printf("SessionManager: 创建会话失败 (尝试 %d/3): %v", attempt, createErr)
		time.Sleep(time.Duration(attempt) * time.Second)
	}
	
	if createErr != nil {
		log.Printf("SessionManager: 创建会话失败，已重试3次: %v", createErr)
		return
	}
	
	// 加载 cookie 恢复
	platformStr := string(state.Platform)
	if !sm.cookieStore.IsCookiesValid(platformStr) {
		log.Printf("SessionManager: 会话 %s cookie无效", state.SessionID)
		return
	}
	
	// 重试2次加载cookie
	var loadErr error
	for attempt := 1; attempt <= 2; attempt++ {
		log.Printf("SessionManager: 加载cookie (尝试 %d/2)...", attempt)
		loadErr = sm.cookieStore.ApplyCookies(sm.browserMgr.GetContext(state.SessionID), platformStr)
		if loadErr == nil {
			break
		}
		log.Printf("SessionManager: 加载cookie失败 (尝试 %d/2): %v", attempt, loadErr)
		time.Sleep(time.Duration(attempt) * time.Second)
	}
	
	if loadErr != nil {
		log.Printf("SessionManager: 加载cookie失败: %v", loadErr)
		return
	}
	
	// 导航到目标网站
	client, err := sm.createPlatformClient(state.SessionID, state.Platform)
	if err != nil {
		log.Printf("SessionManager: 创建平台客户端失败: %v", err)
		return
	}
	
	// 重试2次导航
	for attempt := 1; attempt <= 2; attempt++ {
		if navErr := client.NavigateToHome(); navErr != nil {
			log.Printf("SessionManager: 导航失败 (尝试 %d/2): %v", attempt, navErr)
			time.Sleep(2 * time.Second)
		} else {
			break
		}
	}
	
	time.Sleep(3 * time.Second) // 等待页面加载
	
	if client.CheckLoggedIn() {
		log.Printf("SessionManager: 会话 %s 恢复成功", state.SessionID)
		state.IsActive = true
		state.HealthCheckPass = true
		state.LastRecovered = time.Now()
		return
	}
	
	log.Printf("SessionManager: 会话 %s 恢复失败（cookie无效）", state.SessionID)
}

// autoSaveCookiesLoop 定时保存cookies
func (sm *SessionManager) autoSaveCookiesLoop() {
	ticker := time.NewTicker(5 * time.Minute) // 每5分钟保存一次
	defer ticker.Stop()
	
	for {
		select {
		case <-sm.ctx.Done():
			log.Println("SessionManager: Cookie保存循环已停止")
			return
		case <-ticker.C:
			sm.saveAllCookies()
		}
	}
}

// saveAllCookies 保存所有会话的cookies
func (sm *SessionManager) saveAllCookies() {
	sm.mu.RLock()
	sessions := make([]*SessionState, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		if s.IsActive && s.HealthCheckPass {
			sessions = append(sessions, s)
		}
	}
	sm.mu.RUnlock()
	
	for _, session := range sessions {
		ctx := sm.browserMgr.GetContext(session.SessionID)
		if ctx != nil {
			platformStr := string(session.Platform)
			if err := sm.cookieStore.SaveCookies(ctx, platformStr); err != nil {
				log.Printf("SessionManager: 保存会话 %s 的cookie失败: %v", session.SessionID, err)
			}
		}
	}
	
	if len(sessions) > 0 {
		log.Printf("SessionManager: 已保存 %d 个会话的cookies", len(sessions))
	}
}

// createPlatformClient 通过注册器创建平台客户端
func (sm *SessionManager) createPlatformClient(sessionID string, platformType models.Platform) (platform.PlatformClient, error) {
	session, err := sm.browserMgr.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	client, err := platform.GetRegistry().GetClient(platformType, session)
	if err != nil {
		return nil, fmt.Errorf("获取平台客户端失败: %v", err)
	}
	return client, nil
}

// GetSessionState 获取会话状态
func (sm *SessionManager) GetSessionState(sessionID string) *SessionState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.sessions[sessionID]
}

// GetContext 获取会话上下文
func (sm *SessionManager) GetContext(sessionID string) context.Context {
	return sm.browserMgr.GetContext(sessionID)
}

// IsSessionHealthy 检查会话是否健康
func (sm *SessionManager) IsSessionHealthy(sessionID string) bool {
	state := sm.GetSessionState(sessionID)
	if state == nil {
		return false
	}
	return state.IsActive && state.HealthCheckPass
}
