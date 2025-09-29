package mqttx

import (
	"sync"
	"time"
)

// NewSessionManager 创建新的会话管理器
func NewSessionManager() *Manager {
	m := &Manager{
		sessions: make(map[string]*Session),
		events:   newEventManager(),
		logger:   NewDefaultLogger(),
		metrics:  newMetrics(),
	}

	// 初始化错误恢复管理器
	m.recovery = NewRecoveryManager(m)

	return m
}

// RegisterError 注册错误并启动恢复流程
func (m *Manager) RegisterError(sessionName string, err error, category ErrorCategory) *ErrorInfo {
	if err != nil {
		m.metrics.recordError()
	}
	return m.recovery.RegisterError(sessionName, err, category)
}

// GetActiveErrors 获取当前活跃的错误
func (m *Manager) GetActiveErrors() []*ErrorInfo {
	return m.recovery.GetActiveErrors()
}

// GetErrorStats 获取错误统计信息
func (m *Manager) GetErrorStats() map[string]interface{} {
	return m.recovery.GetErrorStats()
}

// SetMaxRetries 设置特定类别错误的最大重试次数
func (m *Manager) SetMaxRetries(category ErrorCategory, count int) {
	m.recovery.SetMaxRetries(category, count)
}

// ClearRecoveredErrors 清理已恢复的错误
func (m *Manager) ClearRecoveredErrors() int {
	return m.recovery.ClearRecoveredErrors()
}

// PublishTo 发布消息到指定会话，增加错误处理
func (m *Manager) PublishTo(sessionName, topic string, payload []byte, qos byte) error {
	// 如果设置了测试函数，则使用测试函数
	if m.publishToFunc != nil {
		return m.publishToFunc(sessionName, topic, payload, qos)
	}

	// 正常逻辑
	session, err := m.GetSession(sessionName)
	if err != nil {
		return err
	}

	err = session.Publish(topic, payload, qos)
	if err != nil {
		// 注册错误并启动恢复流程
		m.RegisterError(sessionName, err, CategoryPublish)
	}
	return err
}

// SubscribeTo 订阅主题，增加错误处理
func (m *Manager) SubscribeTo(sessionName, topic string, handler MessageHandler, qos byte) error {
	session, err := m.GetSession(sessionName)
	if err != nil {
		return err
	}

	err = session.Subscribe(topic, handler, qos)
	if err != nil {
		// 注册错误并启动恢复流程
		m.RegisterError(sessionName, err, CategorySubscription)
	}
	return err
}

// SetLogger 设置日志记录器
func (m *Manager) SetLogger(logger Logger) Logger {
	if logger == nil {
		m.logger = NewDefaultLogger()
		return m.logger
	} else {
		m.logger = logger
	}
	return m.logger
}

// AddSession 添加新的MQTT会话
func (m *Manager) AddSession(opts *Options) error {
	if err := opts.Validate(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[opts.Name]; exists {
		return ErrSessionExists
	}

	session := newSession(opts, m)
	if err := session.init(); err != nil {
		return err
	}

	m.sessions[opts.Name] = session
	m.metrics.updateSessionCount(1)

	m.events.emit(&Event{
		Type:      EventSessionAdded,
		Session:   opts.Name,
		Timestamp: time.Now(),
	})

	return nil
}

// GetSession 获取指定会话
func (m *Manager) GetSession(name string) (*Session, error) {
	// 如果设置了测试函数，则使用测试函数
	if m.getSessionFunc != nil {
		return m.getSessionFunc(name)
	}

	// 正常逻辑
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[name]
	if !exists {
		return nil, ErrSessionNotFound
	}
	return session, nil
}

// GetAllSessionsStatus 获取所有会话的状态字符串
func (m *Manager) GetAllSessionsStatus() map[string]string {
	statusCodes := m.GetAllSessionsStatusCode()
	result := make(map[string]string)

	for name, code := range statusCodes {
		result[name] = StatusToString(code)
	}

	return result
}

// GetAllSessionsStatusCode 获取所有会话的状态常量
func (m *Manager) GetAllSessionsStatusCode() map[string]uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]uint32)
	for name, session := range m.sessions {
		result[name] = session.GetSessionStatus()
	}
	return result
}

// RemoveSession 移除会话
func (m *Manager) RemoveSession(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[name]
	if !exists {
		return ErrSessionNotFound
	}

	session.Disconnect()
	delete(m.sessions, name)
	m.metrics.updateSessionCount(-1)

	m.events.emit(&Event{
		Type:      EventSessionRemoved,
		Session:   name,
		Timestamp: time.Now(),
	})

	return nil
}

// ListSessions 列出所有会话
func (m *Manager) ListSessions() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]string, 0, len(m.sessions))
	for name := range m.sessions {
		sessions = append(sessions, name)
	}
	return sessions
}

// PublishToAll 向所有会话发布消息
func (m *Manager) PublishToAll(topic string, payload []byte, qos byte) []error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var _errors []error
	for name, session := range m.sessions {
		if err := session.Publish(topic, payload, qos); err != nil {
			_errors = append(_errors, newSessionError(name, err))
		}
	}

	if len(payload) > 0 {
		m.metrics.recordMessage(uint64(len(payload)))
	}

	if len(_errors) > 0 {
		return _errors
	}
	return nil
}

// SubscribeAll 在所有会话中订阅主题
func (m *Manager) SubscribeAll(topic string, handler MessageHandler, qos byte) []error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var _errors []error
	for name, session := range m.sessions {
		if err := session.Subscribe(topic, handler, qos); err != nil {
			_errors = append(_errors, newSessionError(name, err))
		}
	}

	if len(_errors) > 0 {
		return _errors
	}
	return nil
}

// UnsubscribeAll 取消所有会话的主题订阅
func (m *Manager) UnsubscribeAll(topic string) []error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var _errors []error
	for name, session := range m.sessions {
		if err := session.Unsubscribe(topic); err != nil {
			_errors = append(_errors, newSessionError(name, err))
		}
	}

	if len(_errors) > 0 {
		return _errors
	}
	return nil
}

// UnsubscribeTo 取消指定会话的主题订阅
func (m *Manager) UnsubscribeTo(name string, topic string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[name]
	if !exists {
		return ErrSessionNotFound
	}

	if err := session.Unsubscribe(topic); err != nil {
		return newSessionError(name, err)
	}
	return nil
}

// DisconnectAll 断开所有会话连接
func (m *Manager) DisconnectAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, session := range m.sessions {
		session.Disconnect()
	}
}

// OnEvent 注册事件处理函数
func (m *Manager) OnEvent(eventType string, handler func(event Event)) {
	// 转换为接受*Event参数的处理函数
	wrappedHandler := func(event *Event) {
		handler(*event)
	}
	m.events.on(eventType, wrappedHandler)
}

// GetMetrics 获取指标信息
func (m *Manager) GetMetrics() map[string]interface{} {
	return m.metrics.getSnapshot()
}

// WaitForSession 等待指定会话连接成功
func (m *Manager) WaitForSession(name string, timeout time.Duration) error {
	session, err := m.GetSession(name)
	if err != nil {
		return err
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	// 如果已经连接，直接返回
	if session.IsConnected() {
		return nil
	}

	// 创建一个通道来接收连接成功事件
	connected := make(chan struct{})
	var once sync.Once

	// 注册事件处理器
	handler := func(event Event) {
		if event.Type == EventSessionConnected && event.Session == name {
			once.Do(func() {
				close(connected)
			})
		}
	}

	m.OnEvent(EventSessionConnected, handler)

	// 等待连接成功或超时
	select {
	case <-connected:
		return nil
	case <-timer.C:
		return NewConnectionError("timeout waiting for session to connect", ErrTimeout).
			WithSession(name)
	}
}

// WaitForAllSessions 等待所有会话连接成功
func (m *Manager) WaitForAllSessions(timeout time.Duration) error {
	m.logger.Info("Starting to wait for all sessions", "session_count", len(m.ListSessions()), "timeout", timeout)

	sessions := m.ListSessions()
	if len(sessions) == 0 {
		return NewConfigError("no sessions available", nil)
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	var wg sync.WaitGroup
	errChan := make(chan error, len(sessions))

	// 为每个会话创建一个等待协程
	for _, name := range sessions {
		wg.Add(1)
		go func(sessionName string) {
			defer wg.Done()
			if err := m.WaitForSession(sessionName, timeout); err != nil {
				if mqttErr, ok := err.(*MQTTXError); ok {
					mqttErr.WithSession(sessionName)
					errChan <- mqttErr
				} else {
					errChan <- NewConnectionError("session connection failed", err).WithSession(sessionName)
				}
			}
		}(name)
	}

	// 等待所有协程完成或超时
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 检查是否有错误
		close(errChan)
		var _errors []error
		for err := range errChan {
			_errors = append(_errors, err)
		}
		if len(_errors) > 0 {
			validationErrs := NewValidationErrors()
			for _, err := range _errors {
				validationErrs.Add(err)
			}
			return validationErrs
		}
		return nil
	case <-timer.C:
		return NewConnectionError("timeout waiting for all sessions to connect", ErrTimeout)
	}
}

// Close 关闭管理器
func (m *Manager) Close() {
	m.DisconnectAll()
}
