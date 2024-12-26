package mqttx

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// NewSessionManager 创建新的MQTT管理器
func NewSessionManager() *Manager {
	m := &Manager{
		sessions: make(map[string]*Session),
		events:   newEventManager(),
		logger:   newLogger(),
		metrics:  newMetrics(),
	}

	// 启动指标更新器
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		var lastMessageCount uint64
		var lastByteCount uint64
		lastTime := time.Now()

		for range ticker.C {
			currentTime := time.Now()

			// 更新其他指标
			m.updateMetrics()

			// 获取当前计数
			currentMessages := atomic.LoadUint64(&m.metrics.TotalMessages)
			currentBytes := atomic.LoadUint64(&m.metrics.TotalBytes)

			// 计算时间间隔（秒）
			interval := currentTime.Sub(lastTime).Seconds()
			if interval > 0 {
				// 计算消息速率
				messagesDiff := float64(currentMessages - lastMessageCount)
				messageRate := messagesDiff / interval
				m.metrics.rates.messageRate.Store(fmt.Sprintf("%.2f/s", messageRate))

				// 计算数据速率
				bytesDiff := float64(currentBytes - lastByteCount)

				byteRate := bytesDiff / interval
				m.metrics.rates.byteRate.Store(formatBytes(uint64(byteRate)) + "/s")

				// 计算平均速率
				uptime := currentTime.Sub(m.metrics.startTime).Seconds()
				if uptime > 0 {
					avgMessageRate := float64(currentMessages) / uptime
					m.metrics.rates.avgMessageRate.Store(fmt.Sprintf("%.2f/s", avgMessageRate))
				}
			}

			// 更新上一次的计数
			lastMessageCount = currentMessages
			lastByteCount = currentBytes
			lastTime = currentTime
		}
	}()

	return m
}

// SetLogger 设置日志记录器
func (m *Manager) SetLogger(logger Logger) Logger {
	if logger == nil {
		m.logger = newLogger()
		return m.logger
	}
	m.logger = logger
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

	m.events.emit(Event{
		Type:      EventSessionAdded,
		Session:   opts.Name,
		Timestamp: time.Now(),
	})

	return nil
}

// GetSession 获取指定会话
func (m *Manager) GetSession(name string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[name]
	if !exists {
		return nil, ErrSessionNotFound
	}
	return session, nil
}

// GetAllSessionsStatus 获取所有会话状态
func (m *Manager) GetAllSessionsStatus() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make(map[string]string, len(m.sessions))
	for name, session := range m.sessions {
		status := atomic.LoadUint32(&session.status)
		switch status {
		case StateDisconnected:
			sessions[name] = "disconnected"
		case StateConnecting:
			sessions[name] = "connecting"
		case StateConnected:
			sessions[name] = "connected"
		case StateReconnecting:
			sessions[name] = "reconnecting"
		case StateClosed:
			sessions[name] = "closed"
		default:
			sessions[name] = "unknown"
		}
	}

	return sessions
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

	m.events.emit(Event{
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

// PublishTo 向指定会话发布消息
func (m *Manager) PublishTo(name string, topic string, payload []byte, qos byte) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[name]
	if !exists {
		return ErrSessionNotFound
	}

	if err := session.Publish(topic, payload, qos); err != nil {
		m.metrics.recordError(err)
		return newSessionError(name, err)
	}

	if len(payload) > 0 {
		m.metrics.recordMessage(uint64(len(payload)))
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

// SubscribeTo 向指定会话订阅主题
func (m *Manager) SubscribeTo(name string, topic string, handler MessageHandler, qos byte) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[name]
	if !exists {
		return ErrSessionNotFound
	}

	if err := session.Subscribe(topic, handler, qos); err != nil {
		return newSessionError(name, err)
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
func (m *Manager) OnEvent(eventType string, handler EventHandler) {
	m.events.on(eventType, handler)
}

// GetMetrics 获取管理器指标
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
		return fmt.Errorf("timeout waiting for session %s to connect", name)
	}
}

// WaitForAllSessions 等待所有会话连接成功
func (m *Manager) WaitForAllSessions(timeout time.Duration) error {
	m.logger.Info("Starting to wait for all sessions", "session_count", len(m.ListSessions()), "timeout", timeout)

	sessions := m.ListSessions()
	if len(sessions) == 0 {
		return errors.New("no sessions available")
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
				errChan <- fmt.Errorf("session %s: %w", sessionName, err)
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
			return fmt.Errorf("failed to connect all sessions: %v", _errors)
		}
		return nil
	case <-timer.C:
		return errors.New("timeout waiting for all sessions to connect")
	}
}

// Close 关闭管理器
func (m *Manager) Close() {
	m.DisconnectAll()
}

// updateMetrics 更新管理器级别的指标
func (m *Manager) updateMetrics() {
	var totalMessages, totalBytes uint64
	var totalErrors, totalReconnects uint64

	// 汇总所有会话的指标
	m.mu.RLock()
	for _, session := range m.sessions {
		metrics := session.GetMetrics()
		// 分别获取发送和接收的消息数
		sent := metrics["messages_sent"].(uint64)
		received := metrics["messages_received"].(uint64)
		totalMessages += sent + received

		// 分别获取发送和接收的字节数
		bytesSent := metrics["bytes_sent"].(uint64)
		bytesReceived := metrics["bytes_received"].(uint64)
		totalBytes += bytesSent + bytesReceived

		totalErrors += metrics["errors"].(uint64)
		totalReconnects += metrics["reconnects"].(uint64)
	}
	m.mu.RUnlock()

	// 原子操作更新指标
	atomic.StoreUint64(&m.metrics.TotalMessages, totalMessages)
	atomic.StoreUint64(&m.metrics.TotalBytes, totalBytes)
	atomic.StoreUint64(&m.metrics.ErrorCount, totalErrors)
	atomic.StoreUint64(&m.metrics.ReconnectCount, totalReconnects)

	// 更新最后更新时间
	m.metrics.mu.Lock()
	m.metrics.LastUpdate = time.Now()
	m.metrics.mu.Unlock()
}
