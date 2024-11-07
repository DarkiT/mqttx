package mqtt

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// newSession 创建新的会话
func newSession(opts *Options, manager *Manager) *Session {
	var store SessionStore
	if opts.ConnectProps != nil && opts.ConnectProps.PersistentSession {
		if opts.StoragePath != "" {
			// 使用文件存储
			fileStore, err := NewFileStore(opts.StoragePath)
			if err != nil {
				manager.logger.Warn("Failed to create file store, falling back to memory store",
					"error", err)
				store = NewMemoryStore()
			} else {
				store = fileStore
			}
		} else {
			// 默认使用内存存储
			store = NewMemoryStore()
		}
	} else {
		store = NewMemoryStore()
	}

	session := &Session{
		name:     opts.Name,
		opts:     opts,
		manager:  manager,
		handlers: newHandlerRegistry(),
		status:   StateDisconnected,
		metrics:  newSessionMetrics(),
		store:    store,
	}

	// 如果启用了持久化，尝试恢复状态
	if opts.ConnectProps != nil && opts.ConnectProps.PersistentSession {
		if err := session.restoreState(); err != nil {
			manager.logger.Warn("Failed to restore session state",
				"session", opts.Name,
				"error", err)
		}
	}

	return session
}

// init 初始化会话
func (s *Session) init() error {
	mqttOpts := mqtt.NewClientOptions()

	// 配置broker地址
	for _, broker := range s.opts.Brokers {
		mqttOpts.AddBroker(broker)
	}

	// 基本配置
	mqttOpts.SetClientID(s.opts.ClientID)
	mqttOpts.SetUsername(s.opts.Username)
	mqttOpts.SetPassword(s.opts.Password)

	if s.opts.ConnectProps == nil {
		s.opts.ConnectProps = DefaultOptions().ConnectProps
	}

	// 配置连接属性
	props := s.opts.ConnectProps
	mqttOpts.SetKeepAlive(time.Duration(props.KeepAlive) * time.Second)
	mqttOpts.SetCleanSession(props.CleanSession)
	mqttOpts.SetAutoReconnect(props.AutoReconnect)
	mqttOpts.SetConnectTimeout(time.Duration(props.ConnectTimeout) * time.Second)
	mqttOpts.SetMaxReconnectInterval(time.Duration(props.MaxReconnectInterval) * time.Second)
	mqttOpts.SetWriteTimeout(time.Duration(props.WriteTimeout) * time.Second)

	// 配置性能选项
	if s.opts.Performance != nil {
		perf := s.opts.Performance
		if perf.MessageChanSize > 0 {
			mqttOpts.SetMessageChannelDepth(perf.MessageChanSize)
		}
		if perf.WriteTimeout > 0 {
			mqttOpts.SetWriteTimeout(perf.WriteTimeout)
		}
	}

	// 配置TLS
	if tlsConfig, err := s.opts.ConfigureTLS(); err != nil {
		return err
	} else if tlsConfig != nil {
		mqttOpts.SetTLSConfig(tlsConfig)
	}

	// 设置回调
	mqttOpts.SetOnConnectHandler(s.handleConnect)
	mqttOpts.SetConnectionLostHandler(s.handleConnectionLost)
	mqttOpts.SetReconnectingHandler(func(c mqtt.Client, opts *mqtt.ClientOptions) {
		s.handleReconnecting()
	})

	s.client = mqtt.NewClient(mqttOpts)

	// 启动连接
	go s.connect()

	return nil
}

// connect 连接到MQTT服务器
func (s *Session) connect() {
	s.setState(StateConnecting)
	s.manager.events.emit(Event{
		Type:      EventSessionConnecting,
		Session:   s.name,
		Timestamp: time.Now(),
	})

	token := s.client.Connect()
	if token.Wait() && token.Error() != nil {
		s.manager.logger.Error("Connection failed",
			"session", s.name,
			"error", token.Error())
		s.setState(StateDisconnected)
		s.metrics.recordError(token.Error())
		s.manager.events.emit(Event{
			Type:      EventSessionDisconnected,
			Session:   s.name,
			Data:      token.Error(),
			Timestamp: time.Now(),
		})
		return
	}

	s.setState(StateConnected)
}

// handleConnect 处理连接成功
func (s *Session) handleConnect(_ mqtt.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.manager.logger.Info("Connected successfully", "session", s.name)

	// 发送连接成功事件
	s.manager.events.emit(Event{
		Type:      EventSessionConnected,
		Session:   s.name,
		Timestamp: time.Now(),
	})

	// 重新订阅主题
	var subscribeErrors []error
	if s.opts.ConnectProps.ResumeSubs {
		s.handlers.mu.RLock()
		for topic, handler := range s.handlers.messageHandlers {
			if err := s.Subscribe(topic, handler, 0); err != nil {
				subscribeErrors = append(subscribeErrors, err)
				s.manager.logger.Warn("Failed to resubscribe",
					"session", s.name,
					"topic", topic,
					"error", err)
			}
		}
		s.handlers.mu.RUnlock()
	}

	// 保存会话状态
	if s.opts.ConnectProps.PersistentSession {
		if err := s.saveState(); err != nil {
			s.manager.logger.Warn("Failed to save session state",
				"session", s.name,
				"error", err)
		}
	}

	// 设置为已连接状态
	s.setState(StateConnected)

	// 发送就绪事件
	s.manager.events.emit(Event{
		Type:    EventSessionReady,
		Session: s.name,
		Data: map[string]interface{}{
			"client_id":        s.opts.ClientID,
			"clean_session":    s.opts.ConnectProps.CleanSession,
			"auto_reconnect":   s.opts.ConnectProps.AutoReconnect,
			"persistent":       s.opts.ConnectProps.PersistentSession,
			"subscriptions":    len(s.handlers.messageHandlers),
			"connected_broker": s.opts.Brokers[0],
			"sub_errors":       len(subscribeErrors) > 0,
		},
		Timestamp: time.Now(),
	})

	s.handlers.connectHandler(s)
}

// handleConnectionLost 处理连接断开
func (s *Session) handleConnectionLost(_ mqtt.Client, err error) {
	s.setState(StateDisconnected)
	s.metrics.recordError(err)
	s.manager.logger.Warn("Connection lost",
		"session", s.name,
		"error", err)

	// 更新最后断开时间并保存状态
	if s.opts.ConnectProps.PersistentSession {
		state, _ := s.store.LoadState(s.name)
		if state != nil {
			state.LastDisconnected = time.Now()
			if err := s.store.SaveState(s.name, state); err != nil {
				s.manager.logger.Warn("Failed to update session disconnect time",
					"session", s.name,
					"error", err)
			}
		}
	}

	s.manager.events.emit(Event{
		Type:      EventSessionDisconnected,
		Session:   s.name,
		Data:      err,
		Timestamp: time.Now(),
	})

	s.handlers.connectLostHandler(s, err)
}

// handleReconnecting 处理重连
func (s *Session) handleReconnecting() {
	s.setState(StateReconnecting)
	s.metrics.recordReconnect()
	s.manager.logger.Info("Attempting to reconnect", "session", s.name)

	s.manager.events.emit(Event{
		Type:      EventSessionReconnecting,
		Session:   s.name,
		Timestamp: time.Now(),
	})
}

// setState 设置会话状态
func (s *Session) setState(state uint32) {
	oldState := atomic.LoadUint32(&s.status)
	atomic.StoreUint32(&s.status, state)

	if oldState != state {
		s.manager.events.emit(Event{
			Type:    EventStateChanged,
			Session: s.name,
			Data: map[string]interface{}{
				"old_state": oldState,
				"new_state": state,
			},
			Timestamp: time.Now(),
		})
	}
}

// Publish 发布消息
func (s *Session) Publish(topic string, payload []byte, qos byte) error {
	if atomic.LoadUint32(&s.status) != StateConnected {
		return ErrNotConnected
	}

	token := s.client.Publish(topic, qos, false, payload)
	if token.Wait() && token.Error() != nil {
		s.metrics.recordError(token.Error())
		return token.Error()
	}

	s.metrics.recordMessage(true, uint64(len(payload)))
	return nil
}

// Subscribe 订阅主题
func (s *Session) Subscribe(topic string, handler MessageHandler, qos byte) error {
	if atomic.LoadUint32(&s.status) != StateConnected {
		return ErrNotConnected
	}

	token := s.client.Subscribe(topic, qos, func(c mqtt.Client, msg mqtt.Message) {
		handler(msg.Topic(), msg.Payload())
		s.metrics.recordMessage(false, uint64(len(msg.Payload())))
	})

	if token.Wait() && token.Error() != nil {
		s.metrics.recordError(token.Error())
		return token.Error()
	}

	s.handlers.mu.Lock()
	s.handlers.messageHandlers[topic] = handler
	s.handlers.mu.Unlock()

	// 如果启用了持久化，保存订阅状态
	if s.opts.ConnectProps.PersistentSession {
		if err := s.saveState(); err != nil {
			s.manager.logger.Warn("Failed to save subscription state",
				"session", s.name,
				"error", err)
		}
	}

	return nil
}

// Unsubscribe 取消订阅主题
func (s *Session) Unsubscribe(topics ...string) error {
	if atomic.LoadUint32(&s.status) != StateConnected {
		return ErrNotConnected
	}

	token := s.client.Unsubscribe(topics...)
	if token.Wait() && token.Error() != nil {
		s.metrics.recordError(token.Error())
		return token.Error()
	}

	s.handlers.mu.Lock()
	for _, topic := range topics {
		delete(s.handlers.messageHandlers, topic)
	}
	s.handlers.mu.Unlock()

	// 如果启用了持久化，保存状态
	if s.opts.ConnectProps.PersistentSession {
		if err := s.saveState(); err != nil {
			s.manager.logger.Warn("Failed to save subscription state",
				"session", s.name,
				"error", err)
		}
	}

	return nil
}

// Disconnect 断开连接
func (s *Session) Disconnect() {
	if s.client != nil && s.client.IsConnected() {
		// 如果启用了持久化，保存最终状态
		if s.opts.ConnectProps.PersistentSession {
			if err := s.saveState(); err != nil {
				s.manager.logger.Warn("Failed to save final session state",
					"session", s.name,
					"error", err)
			}
		}

		s.client.Disconnect(250)
	}

	s.setState(StateClosed)
	s.manager.events.emit(Event{
		Type:      EventSessionDisconnected,
		Session:   s.name,
		Timestamp: time.Now(),
	})
}

// IsConnected 检查是否已连接
func (s *Session) IsConnected() bool {
	return atomic.LoadUint32(&s.status) == StateConnected
}

// GetMetrics 获取会话指标
func (s *Session) GetMetrics() map[string]interface{} {
	return s.metrics.getSnapshot()
}

// saveState 保存会话状态
func (s *Session) saveState() error {
	// 收集所有订阅的主题信息
	s.handlers.mu.RLock()
	topics := make([]TopicSubscription, 0, len(s.handlers.messageHandlers))
	for topic := range s.handlers.messageHandlers {
		topics = append(topics, TopicSubscription{
			Topic: topic,
			QoS:   0, // 可以根据需要存储实际的 QoS 值
		})
	}
	s.handlers.mu.RUnlock()

	state := &SessionState{
		Topics:           topics,
		Messages:         []*Message{}, // 如果需要，可以保存待处理的消息
		LastSequence:     atomic.LoadUint64(&s.sequence),
		LastConnected:    time.Now(),
		LastDisconnected: time.Time{}, // 将在断开连接时更新
	}

	return s.store.SaveState(s.name, state)
}

// restoreState 恢复会话状态
func (s *Session) restoreState() error {
	state, err := s.store.LoadState(s.name)
	if err != nil {
		return err
	}
	if state == nil {
		return nil
	}

	// 恢复序列号
	atomic.StoreUint64(&s.sequence, state.LastSequence)

	// 如果连接可用，重新订阅主题
	if s.IsConnected() {
		for _, topic := range state.Topics {
			// 使用一个闭包来捕获主题
			topicName := topic.Topic
			err := s.Subscribe(topicName, func(t string, payload []byte) {
				s.manager.logger.Debug("Restored subscription received message",
					"session", s.name,
					"topic", topicName)
			}, topic.QoS)
			if err != nil {
				s.manager.logger.Warn("Failed to restore subscription",
					"session", s.name,
					"topic", topicName,
					"error", err)
			}
		}
	}

	// 如果有待处理的消息，可以在这里处理
	for _, msg := range state.Messages {
		s.manager.logger.Debug("Found pending message in restored session",
			"session", s.name,
			"topic", msg.Topic,
			"time", msg.Timestamp)
	}

	return nil
}

// newHandlerRegistry 创建新的处理函数注册表
func newHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{
		// 设置默认的连接处理函数
		connectHandler: func(session *Session) {},
		// 设置默认的连接断开处理函数
		connectLostHandler: func(session *Session, err error) {},
		// 初始化消息处理函数映射
		messageHandlers: make(map[string]MessageHandler),
	}
}

// SetConnectHandler 设置连接成功处理函数
func (h *HandlerRegistry) SetConnectHandler(handler ConnectHandler) {
	h.mu.Lock()
	h.connectHandler = handler
	h.mu.Unlock()
}

// SetConnectLostHandler 设置连接断开处理函数
func (h *HandlerRegistry) SetConnectLostHandler(handler ConnectLostHandler) {
	h.mu.Lock()
	h.connectLostHandler = handler
	h.mu.Unlock()
}

// AddMessageHandler 添加消息处理函数
func (h *HandlerRegistry) AddMessageHandler(topic string, handler MessageHandler) {
	h.mu.Lock()
	h.messageHandlers[topic] = handler
	h.mu.Unlock()
}

// RemoveMessageHandler 移除消息处理函数
func (h *HandlerRegistry) RemoveMessageHandler(topic string) {
	h.mu.Lock()
	delete(h.messageHandlers, topic)
	h.mu.Unlock()
}

// GetMessageHandler 获取消息处理函数
func (h *HandlerRegistry) GetMessageHandler(topic string) (MessageHandler, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	handler, exists := h.messageHandlers[topic]
	return handler, exists
}

// GetAllTopics 获取所有订阅的主题
func (h *HandlerRegistry) GetAllTopics() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	topics := make([]string, 0, len(h.messageHandlers))
	for topic := range h.messageHandlers {
		topics = append(topics, topic)
	}
	return topics
}

// ClearMessageHandlers 清除所有消息处理函数
func (h *HandlerRegistry) ClearMessageHandlers() {
	h.mu.Lock()
	h.messageHandlers = make(map[string]MessageHandler)
	h.mu.Unlock()
}

// CountMessageHandlers 获取消息处理函数数量
func (h *HandlerRegistry) CountMessageHandlers() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.messageHandlers)
}

// IsSubscribed 检查主题是否已订阅
func (s *Session) IsSubscribed(topic string) bool {
	s.handlers.mu.RLock()
	defer s.handlers.mu.RUnlock()
	_, exists := s.handlers.messageHandlers[topic]
	return exists
}

// GetSubscribedTopics 获取所有已订阅的主题
func (s *Session) GetSubscribedTopics() []string {
	return s.handlers.GetAllTopics()
}

// GetSubscriptionCount 获取订阅数量
func (s *Session) GetSubscriptionCount() int {
	return s.handlers.CountMessageHandlers()
}

// ResetMetrics 重置会话指标
func (s *Session) ResetMetrics() {
	s.metrics = newSessionMetrics()
}

// GetStatus 获取会话状态
func (s *Session) GetStatus() string {
	status := atomic.LoadUint32(&s.status)
	switch status {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateReconnecting:
		return "reconnecting"
	case StateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// GetClientID 获取客户端ID
func (s *Session) GetClientID() string {
	return s.opts.ClientID
}

// GetName 获取会话名称
func (s *Session) GetName() string {
	return s.name
}

// GetBrokers 获取已配置的Broker地址列表
func (s *Session) GetBrokers() []string {
	return s.opts.Brokers
}

// GetOptions 获取会话选项
func (s *Session) GetOptions() *Options {
	return s.opts
}

// IsPersistent 检查是否为持久会话
func (s *Session) IsPersistent() bool {
	return s.opts.ConnectProps != nil && s.opts.ConnectProps.PersistentSession
}

// UpdateLastActivity 更新最后活动时间
func (s *Session) UpdateLastActivity() {
	s.metrics.mu.Lock()
	s.metrics.LastMessage = time.Now()
	s.metrics.mu.Unlock()
}

// GetLastActivity 获取最后活动时间
func (s *Session) GetLastActivity() time.Time {
	s.metrics.mu.RLock()
	defer s.metrics.mu.RUnlock()
	return s.metrics.LastMessage
}

// String 返回会话的字符串表示
func (s *Session) String() string {
	return fmt.Sprintf("Session{name: %s, client_id: %s, status: %s}",
		s.name, s.opts.ClientID, s.GetStatus())
}

// PrometheusMetrics 格式的指标导出
func (s *Session) PrometheusMetrics() string {
	metrics := s.GetMetrics()
	var sb strings.Builder

	// 消息相关指标
	sb.WriteString(fmt.Sprintf("mqtt_session_messages_sent_total{session=\"%s\"} %v\n",
		s.name, metrics["messages_sent"]))
	sb.WriteString(fmt.Sprintf("mqtt_session_messages_received_total{session=\"%s\"} %v\n",
		s.name, metrics["messages_received"]))

	// 字节相关指标
	sb.WriteString(fmt.Sprintf("mqtt_session_bytes_sent_total{session=\"%s\"} %v\n",
		s.name, metrics["bytes_sent"]))
	sb.WriteString(fmt.Sprintf("mqtt_session_bytes_received_total{session=\"%s\"} %v\n",
		s.name, metrics["bytes_received"]))

	// 错误和重连指标
	sb.WriteString(fmt.Sprintf("mqtt_session_errors_total{session=\"%s\"} %v\n",
		s.name, metrics["errors"]))
	sb.WriteString(fmt.Sprintf("mqtt_session_reconnects_total{session=\"%s\"} %v\n",
		s.name, metrics["reconnects"]))

	// 状态指标
	sb.WriteString(fmt.Sprintf("mqtt_session_connected{session=\"%s\"} %v\n",
		s.name, s.IsConnected()))
	sb.WriteString(fmt.Sprintf("mqtt_session_subscriptions{session=\"%s\"} %v\n",
		s.name, s.GetSubscriptionCount()))

	// 时间相关指标
	if lastMsg, ok := metrics["last_message"].(time.Time); ok {
		sb.WriteString(fmt.Sprintf("mqtt_session_last_message_timestamp_seconds{session=\"%s\"} %v\n",
			s.name, lastMsg.Unix()))
	}
	if lastErr, ok := metrics["last_error"].(time.Time); ok && !lastErr.IsZero() {
		sb.WriteString(fmt.Sprintf("mqtt_session_last_error_timestamp_seconds{session=\"%s\"} %v\n",
			s.name, lastErr.Unix()))
	}

	// 会话属性指标
	sb.WriteString(fmt.Sprintf("mqtt_session_persistent{session=\"%s\"} %v\n",
		s.name, s.IsPersistent()))
	sb.WriteString(fmt.Sprintf("mqtt_session_clean_session{session=\"%s\"} %v\n",
		s.name, s.opts.ConnectProps.CleanSession))
	sb.WriteString(fmt.Sprintf("mqtt_session_auto_reconnect{session=\"%s\"} %v\n",
		s.name, s.opts.ConnectProps.AutoReconnect))

	// 状态码指标
	sb.WriteString(fmt.Sprintf("mqtt_session_status{session=\"%s\"} %v\n",
		s.name, atomic.LoadUint32(&s.status)))

	// 速率指标
	if rate, ok := metrics["message_rate"].(string); ok {
		// 去掉 "/s" 后缀，只保留数值
		rate = strings.TrimSuffix(rate, "/s")
		sb.WriteString(fmt.Sprintf("mqtt_session_message_rate{session=\"%s\"} %s\n",
			s.name, rate))
	}
	if rate, ok := metrics["bytes_rate"].(string); ok {
		// 去掉单位和 "/s" 后缀，只保留数值
		rate = strings.TrimSuffix(strings.Fields(rate)[0], "/s")
		sb.WriteString(fmt.Sprintf("mqtt_session_bytes_rate{session=\"%s\"} %s\n",
			s.name, rate))
	}

	// 资源统计指标
	if stats, ok := metrics["resource_stats"].(map[string]interface{}); ok {
		if goroutines, exists := stats["goroutines"].(int); exists {
			sb.WriteString(fmt.Sprintf("mqtt_session_goroutines{session=\"%s\"} %d\n",
				s.name, goroutines))
		}
		if heapAlloc, exists := stats["heap_alloc"].(string); exists {
			// 转换为字节数
			bytes := parseBytes(heapAlloc)
			sb.WriteString(fmt.Sprintf("mqtt_session_heap_alloc_bytes{session=\"%s\"} %d\n",
				s.name, bytes))
		}
		if heapInUse, exists := stats["heap_inuse"].(string); exists {
			// 转换为字节数
			bytes := parseBytes(heapInUse)
			sb.WriteString(fmt.Sprintf("mqtt_session_heap_inuse_bytes{session=\"%s\"} %d\n",
				s.name, bytes))
		}
	}

	return sb.String()
}

// parseBytes 辅助函数：将可读的字节大小字符串转换为字节数
func parseBytes(s string) uint64 {
	var value float64
	var unit string
	fmt.Sscanf(s, "%f %s", &value, &unit)

	multiplier := map[string]uint64{
		"B":  1,
		"KB": 1024,
		"MB": 1024 * 1024,
		"GB": 1024 * 1024 * 1024,
		"TB": 1024 * 1024 * 1024 * 1024,
		"PB": 1024 * 1024 * 1024 * 1024 * 1024,
	}

	if m, ok := multiplier[unit]; ok {
		return uint64(value * float64(m))
	}
	return uint64(value)
}
