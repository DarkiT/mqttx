package mqttx

import (
	"context"
	"crypto/tls"
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
		if opts.Storage == nil {
			opts.Storage = DefaultStorageOptions()
		}

		switch opts.Storage.Type {
		case StoreTypeFile:
			// 使用文件存储
			fileStore, err := NewFileStore(opts.Storage.Path)
			if err != nil {
				manager.logger.Warn("Failed to create file store, falling back to memory store",
					"error", err)
				store = NewMemoryStore()
			} else {
				store = fileStore
			}

		case StoreTypeRedis:
			// 使用Redis存储
			if opts.Storage.Redis == nil {
				opts.Storage.Redis = DefaultRedisOptions()
			}

			redisOpts := &GoRedisOptions{
				Addr:     opts.Storage.Redis.Addr,
				Username: opts.Storage.Redis.Username,
				Password: opts.Storage.Redis.Password,
				DB:       opts.Storage.Redis.DB,
				PoolSize: opts.Storage.Redis.PoolSize,
			}

			redisClient := NewGoRedisClient(redisOpts)

			// 测试连接
			if err := redisClient.Ping(context.Background()); err != nil {
				manager.logger.Warn("Failed to connect to Redis, falling back to memory store",
					"error", err)
				store = NewMemoryStore()
			} else {
				redisStoreOpts := &RedisStoreOptions{
					Prefix: opts.Storage.Redis.KeyPrefix,
					TTL:    time.Duration(opts.Storage.Redis.TTL) * time.Second,
				}
				store = NewRedisStore(redisClient, redisStoreOpts)
				manager.logger.Debug("Using Redis store",
					"addr", opts.Storage.Redis.Addr,
					"prefix", opts.Storage.Redis.KeyPrefix)
			}

		default:
			// 默认使用内存存储
			store = NewMemoryStore()
			manager.logger.Debug("Using memory store (default)")
		}
	} else {
		store = NewMemoryStore()
	}

	session := &Session{
		name:           opts.Name,
		opts:           opts,
		manager:        manager,
		handlers:       newHandlerRegistry(),
		status:         stateDisconnected,
		metrics:        newSessionMetrics(),
		store:          store,
		reconnectCount: 0,
		nextReconnect:  opts.ConnectProps.InitialReconnectInterval,
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

	// 确保连接属性存在且设置了默认值
	if s.opts.ConnectProps == nil {
		s.opts.ConnectProps = DefaultOptions().ConnectProps
	}

	props := s.opts.ConnectProps
	// 配置MQTT客户端选项
	mqttOpts.SetKeepAlive(time.Duration(props.KeepAlive) * time.Second)
	mqttOpts.SetCleanSession(props.CleanSession)
	mqttOpts.SetAutoReconnect(props.AutoReconnect)
	mqttOpts.SetConnectTimeout(props.ConnectTimeout)
	// 设置最大重连间隔(实际重连间隔由我们的指数退避策略控制)
	mqttOpts.SetMaxReconnectInterval(props.MaxReconnectInterval)
	mqttOpts.SetWriteTimeout(props.WriteTimeout)

	// 调试日志
	s.manager.logger.Debug("Configuring connection properties",
		"session", s.name,
		"keepalive", props.KeepAlive,
		"clean_session", props.CleanSession,
		"auto_reconnect", props.AutoReconnect,
		"connect_timeout", props.ConnectTimeout,
		"initial_reconnect_interval", props.InitialReconnectInterval,
		"max_reconnect_interval", props.MaxReconnectInterval,
		"backoff_factor", props.BackoffFactor,
		"write_timeout", props.WriteTimeout)

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
	var tlsConfig *tls.Config
	var err error

	// 优先使用增强的TLS配置
	if s.opts.EnhancedTLS != nil {
		tlsConfig, err = s.opts.ConfigureTLSWithEnhanced()
		if err != nil {
			return err
		}
		// 如果启用了自动重新加载证书，启动证书监视器
		if s.opts.EnhancedTLS.AutoReload {
			err = s.opts.EnhancedTLS.StartCertWatcher(s.manager.logger, func() error {
				// 当证书重新加载时，重新连接会话
				s.manager.logger.Info("Certificates reloaded, reconnecting session", "session", s.name)
				s.Disconnect()
				s.connect()
				return nil
			})
			if err != nil {
				s.manager.logger.Warn("Failed to start certificate watcher",
					"session", s.name,
					"error", err)
			}
		}
	} else if s.opts.TLS != nil {
		// 使用基本TLS配置
		tlsConfig, err = s.opts.ConfigureTLS()
		if err != nil {
			return err
		}
	}

	// 设置TLS配置
	if tlsConfig != nil {
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
	s.connect()

	return nil
}

// connect 连接到MQTT服务器
func (s *Session) connect() {
	// 连接前更新状态
	s.setState(stateConnecting)
	s.manager.logger.Debug("Attempting connection",
		"session", s.name,
		"broker", s.opts.Brokers[0],
		"client_id", s.opts.ClientID)

	s.manager.events.emit(&Event{
		Type:      EventSessionConnecting,
		Session:   s.name,
		Timestamp: time.Now(),
	})

	if token := s.client.Connect(); token.Wait() && token.Error() != nil {
		s.manager.logger.Error("Connection failed",
			"session", s.name,
			"error", token.Error())
		s.setState(stateDisconnected)
		if token.Error() != nil {
			s.metrics.recordError(token.Error())
		}
		s.manager.events.emit(&Event{
			Type:      EventSessionDisconnected,
			Session:   s.name,
			Data:      token.Error(),
			Timestamp: time.Now(),
		})
		return
	}

	s.setState(stateConnected)
}

// handleConnect 处理连接成功
func (s *Session) handleConnect(_ mqtt.Client) {
	// 更新状态为已连接
	s.setState(stateConnected)

	// 重置重连计数器和间隔
	s.reconnectMu.Lock()
	wasReconnect := atomic.LoadUint32(&s.reconnectCount) > 0
	atomic.StoreUint32(&s.reconnectCount, 0)
	s.nextReconnect = 0
	s.reconnectMu.Unlock()

	if wasReconnect {
		s.manager.logger.Info("Reconnected successfully", "session", s.name)
	} else {
		s.manager.logger.Info("Connected successfully", "session", s.name)
	}

	// 如果启用了持久化，恢复会话状态
	if s.opts.ConnectProps.PersistentSession && s.opts.ConnectProps.ResumeSubs {
		if err := s.restoreState(); err != nil {
			s.manager.logger.Warn("Failed to restore session state",
				"session", s.name,
				"error", err)
		}
	}

	// 处理预订阅的主题
	var subscribeErrors []error
	if len(s.opts.Topics) > 0 {
		for _, topic := range s.opts.Topics {
			if err := s.Subscribe(topic.Topic, topic.Handler, topic.QoS); err != nil {
				subscribeErrors = append(subscribeErrors, err)
				s.manager.logger.Warn("Failed to subscribe to topic",
					"session", s.name,
					"topic", topic.Topic,
					"error", err)
			}
		}
	}

	s.manager.events.emit(&Event{
		Type:    EventSessionConnected,
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

	s.handlers.connectHandler()
}

// safelyCallHandler 安全地调用消息处理函数，捕获并记录panic
func (s *Session) safelyCallHandler(topic string, payload []byte, handler MessageHandler) {
	defer func() {
		if r := recover(); r != nil {
			err := NewPublishError(topic, fmt.Errorf("panic in message handler: %v", r)).
				WithSession(s.opts.Name)
			s.metrics.recordError(err)
			s.manager.logger.Error("Panic recovered in message handler",
				"session", s.name,
				"topic", topic,
				"error", r)
		}
	}()

	// 调用处理函数
	handler(topic, payload)
}

// handleConnectionLost 处理连接断开
func (s *Session) handleConnectionLost(_ mqtt.Client, err error) {
	// 更新状态为断开连接
	s.setState(stateDisconnected)
	if err != nil {
		// 记录会话级别的错误
		s.metrics.recordError(err)
		// 记录全局级别的错误
		s.manager.RegisterError(s.name, err, CategoryConnection)
	}

	// 详细记录连接断开原因
	s.manager.logger.Warn("Connection lost",
		"session", s.name,
		"error", err,
		"client_id", s.opts.ClientID,
		"broker", s.opts.Brokers[0],
		"time", time.Now().Format(time.RFC3339))

	// 更新最后断开时间并保存状态
	if s.opts.ConnectProps.PersistentSession {
		state, err := s.store.LoadState(s.name)
		if err == nil && state != nil {
			state.LastDisconnected = time.Now()
			if err := s.store.SaveState(s.name, state); err != nil {
				s.manager.logger.Warn("Failed to update session state",
					"session", s.name,
					"error", err)
			}
		}
	}

	// 发送连接断开事件
	s.manager.events.emit(&Event{
		Type:      EventSessionDisconnected,
		Session:   s.name,
		Data:      err,
		Timestamp: time.Now(),
	})

	// 调用用户定义的连接断开处理函数
	if s.handlers.connectLostHandler != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					s.manager.logger.Error("Panic in connection lost handler",
						"session", s.name,
						"error", r)
				}
			}()
			s.handlers.connectLostHandler(err)
		}()
	}
}

// handleReconnecting 处理重连
func (s *Session) handleReconnecting() {
	// 更新状态为重连中
	s.setState(stateReconnecting)

	// 记录会话级别的重连
	s.metrics.recordReconnect()
	// 记录全局级别的重连
	s.manager.metrics.recordReconnect()

	// 安全地计算指数退避重连间隔
	s.reconnectMu.Lock()
	defer s.reconnectMu.Unlock()

	// 增加重连计数器
	currentCount := atomic.AddUint32(&s.reconnectCount, 1)

	// 首次重连尝试，初始化nextReconnect
	if currentCount == 1 || s.nextReconnect == 0 {
		s.nextReconnect = s.opts.ConnectProps.InitialReconnectInterval
	} else {
		// 应用指数退避策略: 当前间隔 * 退避因子
		newInterval := time.Duration(float64(s.nextReconnect) * s.opts.ConnectProps.BackoffFactor)

		// 确保不超过最大重连间隔
		if newInterval > s.opts.ConnectProps.MaxReconnectInterval {
			s.nextReconnect = s.opts.ConnectProps.MaxReconnectInterval
		} else {
			s.nextReconnect = newInterval
		}
	}
	nextInterval := s.nextReconnect

	// 记录日志
	s.manager.logger.Info("Attempting to reconnect",
		"session", s.name,
		"attempt", currentCount,
		"next_interval", nextInterval)

	// 发送事件通知
	s.manager.events.emit(&Event{
		Type:    EventSessionReconnecting,
		Session: s.name,
		Data: map[string]interface{}{
			"attempt":       currentCount,
			"next_interval": nextInterval,
		},
		Timestamp: time.Now(),
	})
}

// setState 设置会话状态
func (s *Session) setState(state uint32) {
	oldState := atomic.LoadUint32(&s.status)
	atomic.StoreUint32(&s.status, state)

	if oldState != state {
		s.manager.events.emit(&Event{
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

// Publish 发布消息，增加错误重试机制
func (s *Session) Publish(topic string, payload []byte, qos byte) error {
	maxRetries := 3
	retryDelay := 500 * time.Millisecond
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		// 检查是否已连接
		if atomic.LoadUint32(&s.status) != stateConnected {
			lastErr = ErrNotConnected
			s.manager.logger.Warn("Cannot publish, session not connected",
				"session", s.name,
				"topic", topic,
				"attempt", i+1,
				"max_attempts", maxRetries)

			// 等待一段时间再重试
			if i < maxRetries-1 {
				time.Sleep(retryDelay)
				continue
			}
			return lastErr
		}

		// 直接使用原始payload
		actualPayload := payload

		token := s.client.Publish(topic, qos, false, actualPayload)
		if token.Wait() && token.Error() != nil {
			lastErr = token.Error()
			if lastErr != nil {
				s.metrics.recordError(lastErr)
			}
			s.manager.logger.Warn("Publish failed, retrying",
				"session", s.name,
				"topic", topic,
				"attempt", i+1,
				"max_attempts", maxRetries,
				"error", lastErr)

			// 等待一段时间再重试
			if i < maxRetries-1 {
				time.Sleep(retryDelay)
				continue
			}
			return lastErr
		}

		// 发布成功
		s.metrics.recordMessage(true, uint64(len(payload)))
		return nil
	}

	return lastErr
}

// Subscribe 订阅主题，增加错误重试机制
func (s *Session) Subscribe(topic string, handler MessageHandler, qos byte) error {
	maxRetries := 3
	retryDelay := 500 * time.Millisecond
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		// 检查是否已连接
		if atomic.LoadUint32(&s.status) != stateConnected {
			lastErr = ErrNotConnected
			s.manager.logger.Warn("Cannot subscribe, session not connected",
				"session", s.name,
				"topic", topic,
				"attempt", i+1,
				"max_attempts", maxRetries)

			// 等待一段时间再重试
			if i < maxRetries-1 {
				time.Sleep(retryDelay)
				continue
			}
			return lastErr
		}

		// 包装处理函数，确保安全调用
		safeHandler := func(c mqtt.Client, msg mqtt.Message) {
			s.safelyCallHandler(msg.Topic(), msg.Payload(), handler)
			s.metrics.recordMessage(false, uint64(len(msg.Payload())))
		}

		token := s.client.Subscribe(topic, qos, safeHandler)
		if token.Wait() && token.Error() != nil {
			lastErr = token.Error()
			if lastErr != nil {
				s.metrics.recordError(lastErr)
			}
			s.manager.logger.Warn("Subscribe failed, retrying",
				"session", s.name,
				"topic", topic,
				"attempt", i+1,
				"max_attempts", maxRetries,
				"error", lastErr)

			// 等待一段时间再重试
			if i < maxRetries-1 {
				time.Sleep(retryDelay)
				continue
			}
			return lastErr
		}

		// 订阅成功
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

		s.manager.logger.Info("Successfully subscribed to topic",
			"session", s.name,
			"topic", topic,
			"qos", qos)
		return nil
	}

	return lastErr
}

// Unsubscribe 取消订阅主题
func (s *Session) Unsubscribe(topics ...string) error {
	// 检查是否已连接
	if atomic.LoadUint32(&s.status) != stateConnected {
		return ErrNotConnected
	}

	token := s.client.Unsubscribe(topics...)
	if token.Wait() && token.Error() != nil {
		if token.Error() != nil {
			s.metrics.recordError(token.Error())
		}
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
	// 停止证书监视器
	if s.opts.EnhancedTLS != nil {
		s.opts.EnhancedTLS.StopCertWatcher()
	}

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

	s.setState(stateClosed)
	s.manager.events.emit(&Event{
		Type:      EventSessionDisconnected,
		Session:   s.name,
		Timestamp: time.Now(),
	})
}

// IsConnected 检查是否已连接
func (s *Session) IsConnected() bool {
	return atomic.LoadUint32(&s.status) == stateConnected
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

	// 计算会话过期时间
	var sessionExpiry time.Time
	if s.opts.ConnectProps.PersistentSession {
		// 如果是Redis存储，使用Redis TTL
		if s.opts.Storage != nil && s.opts.Storage.Type == StoreTypeRedis && s.opts.Storage.Redis != nil {
			sessionExpiry = time.Now().Add(time.Duration(s.opts.Storage.Redis.TTL) * time.Second)
		} else {
			// 默认过期时间为24小时
			sessionExpiry = time.Now().Add(24 * time.Hour)
		}
	}

	state := &SessionState{
		Topics:           topics,
		Messages:         []*Message{},              // 待处理的普通消息
		QoSMessages:      make(map[uint16]*Message), // QoS > 0 的消息
		RetainedMessages: make(map[string]*Message), // 保留消息
		LastSequence:     atomic.LoadUint64(&s.sequence),
		LastConnected:    time.Now(),
		LastDisconnected: time.Time{}, // 将在断开连接时更新
		ClientID:         s.opts.ClientID,
		SessionExpiry:    sessionExpiry,
		Version:          1, // 当前状态版本
	}

	// 这里可以添加逻辑来存储QoS消息和保留消息
	// 例如从缓存中获取等待确认的QoS消息
	// 以及从broker获取的保留消息

	s.manager.logger.Debug("Saving session state",
		"session", s.name,
		"topics", len(topics),
		"client_id", s.opts.ClientID,
		"expiry", sessionExpiry)

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

	// 检查会话是否过期
	if !state.SessionExpiry.IsZero() && time.Now().After(state.SessionExpiry) {
		s.manager.logger.Info("Session expired, not restoring state",
			"session", s.name,
			"expiry", state.SessionExpiry)
		// 删除过期的会话状态
		if err := s.store.DeleteState(s.name); err != nil {
			s.manager.logger.Warn("Failed to delete expired session state",
				"session", s.name,
				"error", err)
		}
		return nil
	}

	// 检查客户端ID是否匹配
	if state.ClientID != "" && state.ClientID != s.opts.ClientID {
		s.manager.logger.Warn("Client ID mismatch in restored session",
			"session", s.name,
			"expected", state.ClientID,
			"actual", s.opts.ClientID)
		// 仍然继续恢复，但记录警告
	}

	// 恢复序列号
	atomic.StoreUint64(&s.sequence, state.LastSequence)

	s.manager.logger.Info("Restoring session state",
		"session", s.name,
		"topics", len(state.Topics),
		"messages", len(state.Messages),
		"qos_messages", len(state.QoSMessages),
		"retained_messages", len(state.RetainedMessages),
		"version", state.Version)

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
			} else {
				s.manager.logger.Debug("Restored subscription",
					"session", s.name,
					"topic", topicName,
					"qos", topic.QoS)
			}
		}
	}

	// 处理待处理的普通消息
	for _, msg := range state.Messages {
		s.manager.logger.Debug("Processing pending message from restored session",
			"session", s.name,
			"topic", msg.Topic,
			"time", msg.Timestamp)

		// 这里可以添加消息处理逻辑
		// 例如将消息分发给相应的处理器
	}

	// 处理QoS消息
	for msgID, msg := range state.QoSMessages {
		s.manager.logger.Debug("Processing QoS message from restored session",
			"session", s.name,
			"topic", msg.Topic,
			"message_id", msgID,
			"qos", msg.QoS)

		// 这里可以添加QoS消息处理逻辑
		// 例如重新发送QoS1/2消息或确认接收
	}

	// 处理保留消息
	for topic, msg := range state.RetainedMessages {
		s.manager.logger.Debug("Processing retained message from restored session",
			"session", s.name,
			"topic", topic,
			"time", msg.Timestamp)

		// 这里可以添加保留消息处理逻辑
	}

	return nil
}

// newHandlerRegistry 创建新的处理函数注册表
func newHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{
		// 设置默认的连接处理函数
		connectHandler: func() {},
		// 设置默认的连接断开处理函数
		connectLostHandler: func(err error) {},
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

// GetStatus 获取会话状态的字符串表示
func (s *Session) GetStatus() string {
	return StatusToString(s.GetSessionStatus())
}

// GetSessionStatus 返回会话的当前状态常量
func (s *Session) GetSessionStatus() uint32 {
	status := atomic.LoadUint32(&s.status)
	switch status {
	case stateDisconnected:
		return StatusDisconnected
	case stateConnecting:
		return StatusConnecting
	case stateConnected:
		return StatusConnected
	case stateReconnecting:
		return StatusReconnecting
	case stateClosed:
		return StatusClosed
	default:
		return StatusDisconnected
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
	atomic.StoreInt64(&s.metrics.LastMessage, time.Now().UnixNano())
}

// GetLastActivity 获取最后活动时间
func (s *Session) GetLastActivity() time.Time {
	return time.Unix(0, atomic.LoadInt64(&s.metrics.LastMessage))
}

// String 返回会话的字符串表示
func (s *Session) String() string {
	return fmt.Sprintf("Session{name: %s, client_id: %s, status: %s}", s.name, s.opts.ClientID, s.GetStatus())
}

// PrometheusMetrics 格式的指标导出
func (s *Session) PrometheusMetrics() string {
	metrics := s.GetMetrics()
	var sb strings.Builder

	// 基础计数指标
	writeGaugeMetric(&sb, "mqtt_session_messages_sent_total", s.name, metrics["messages_sent"])
	writeGaugeMetric(&sb, "mqtt_session_messages_received_total", s.name, metrics["messages_received"])
	writeGaugeMetric(&sb, "mqtt_session_bytes_sent_total", s.name, metrics["bytes_sent"])
	writeGaugeMetric(&sb, "mqtt_session_bytes_received_total", s.name, metrics["bytes_received"])
	writeGaugeMetric(&sb, "mqtt_session_errors_total", s.name, metrics["errors"])
	writeGaugeMetric(&sb, "mqtt_session_reconnects_total", s.name, metrics["reconnects"])

	// 状态指标
	writeGaugeMetric(&sb, "mqtt_session_connected", s.name, map[bool]float64{true: 1, false: 0}[s.IsConnected()])
	writeGaugeMetric(&sb, "mqtt_session_subscriptions", s.name, s.GetSubscriptionCount())

	// 时间戳指标
	if lastMsg, ok := metrics["last_message"].(time.Time); ok {
		writeGaugeMetric(&sb, "mqtt_session_last_message_timestamp_seconds", s.name, float64(lastMsg.Unix()))
	}
	if lastErr, ok := metrics["last_error"].(time.Time); ok && !lastErr.IsZero() {
		writeGaugeMetric(&sb, "mqtt_session_last_error_timestamp_seconds", s.name, float64(lastErr.Unix()))
	}

	// 会话属性指标
	writeGaugeMetric(&sb, "mqtt_session_persistent", s.name, map[bool]float64{true: 1, false: 0}[s.IsPersistent()])
	writeGaugeMetric(&sb, "mqtt_session_clean_session", s.name, map[bool]float64{true: 1, false: 0}[s.opts.ConnectProps.CleanSession])
	writeGaugeMetric(&sb, "mqtt_session_auto_reconnect", s.name, map[bool]float64{true: 1, false: 0}[s.opts.ConnectProps.AutoReconnect])
	writeGaugeMetric(&sb, "mqtt_session_status", s.name, float64(atomic.LoadUint32(&s.status)))

	return sb.String()
}

// 辅助函数
func writeGaugeMetric(sb *strings.Builder, name, session string, value interface{}) {
	sb.WriteString(fmt.Sprintf("%s{session=\"%s\"} %v\n", name, session, value))
}
