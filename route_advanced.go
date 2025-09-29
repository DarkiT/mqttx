package mqttx

import (
	"sync"
	"sync/atomic"
	"time"
)

// AdvancedRoute 高级路由配置
// 支持消息过滤和转换
type AdvancedRoute struct {
	Route
	filters      []MessageFilter      // 消息过滤器列表
	transformers []MessageTransformer // 消息转换器列表
	mu           sync.RWMutex
	handler      func(*Message) // 消息处理函数
}

// RouteConfig 路由配置选项
type RouteConfig struct {
	Topic        string               // 主题
	QoS          byte                 // QoS级别
	Filters      []MessageFilter      // 过滤器列表
	Transformers []MessageTransformer // 转换器列表
	BufferSize   int                  // 消息缓冲区大小，仅在Listen模式下有效
	Description  string               // 路由描述
	SessionName  string               // 会话名称，空字符串表示所有会话
	Handler      func(*Message)       // 消息处理函数
}

// NewAdvancedRoute 创建高级路由
func newAdvancedRoute(config RouteConfig, manager *Manager) *AdvancedRoute {
	if config.BufferSize <= 0 {
		config.BufferSize = DefaultMessageChanSize
	}

	route := &AdvancedRoute{
		Route: Route{
			topic:    config.Topic,
			session:  config.SessionName,
			manager:  manager,
			messages: make(chan *Message, config.BufferSize),
			done:     make(chan struct{}),
			qos:      config.QoS,
			stats:    &RouteStats{},
		},
		filters:      config.Filters,
		transformers: config.Transformers,
		handler:      config.Handler,
	}

	return route
}

// AddFilter 添加消息过滤器
func (r *AdvancedRoute) AddFilter(filter MessageFilter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.filters = append(r.filters, filter)
}

// AddTransformer 添加消息转换器
func (r *AdvancedRoute) AddTransformer(transformer MessageTransformer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transformers = append(r.transformers, transformer)
}

// filterMessage 根据过滤器过滤消息
func (r *AdvancedRoute) filterMessage(message *Message) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 如果没有过滤器，则接受所有消息
	if len(r.filters) == 0 {
		return true
	}

	// 所有过滤器都必须匹配
	for _, filter := range r.filters {
		if !filter.Match(message) {
			return false
		}
	}
	return true
}

// transformMessage 使用转换器转换消息
func (r *AdvancedRoute) transformMessage(message *Message) (*Message, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 如果没有转换器，则直接返回原始消息
	if len(r.transformers) == 0 {
		return message, nil
	}

	current := message
	var err error

	// 按顺序应用每个转换器
	for _, transformer := range r.transformers {
		current, err = transformer.Transform(current)
		if err != nil {
			return nil, wrapError(err, "transformer error")
		}
		if current == nil {
			return nil, nil
		}
	}

	return current, nil
}

// processMessage 处理收到的消息
// 1. 过滤消息
// 2. 转换消息
// 3. 发送到处理函数或消息通道
func (r *AdvancedRoute) processMessage(topic string, payload []byte) {
	select {
	case <-r.done:
		return
	default:
		msg := &Message{
			Topic:     topic,
			Payload:   payload,
			QoS:       r.qos,
			Timestamp: time.Now(),
		}

		// 记录原始消息统计
		atomic.AddUint64(&r.stats.MessagesReceived, 1)
		atomic.AddUint64(&r.stats.BytesReceived, uint64(len(payload)))
		r.stats.LastMessageTime = msg.Timestamp

		// 过滤消息
		if !r.filterMessage(msg) {
			return
		}

		// 转换消息
		transformedMsg, err := r.transformMessage(msg)
		if err != nil {
			r.stats.LastError = time.Now()
			atomic.AddUint64(&r.stats.ErrorCount, 1)
			r.manager.logger.Error("Message transformation error",
				"topic", topic,
				"error", err)
			return
		}

		// 如果消息被转换器过滤掉
		if transformedMsg == nil {
			return
		}

		// 如果有处理函数，直接调用
		if r.handler != nil {
			r.handler(transformedMsg)
			return
		}

		// 否则发送到消息通道
		select {
		case r.messages <- transformedMsg:
		default:
			atomic.AddUint64(&r.stats.MessagesDropped, 1)
			r.manager.logger.Warn("Message dropped due to full channel",
				"topic", topic,
				"session", r.session,
				"qos", r.qos)
		}
	}
}

// HandleWithConfig 使用高级配置处理所有会话的指定主题消息
func (m *Manager) HandleWithConfig(config RouteConfig, handler func(*Message)) *AdvancedRoute {
	route := newAdvancedRoute(config, m)
	route.handler = handler

	// 根据会话名称决定订阅方式
	if config.SessionName == "" {
		// 订阅所有会话
		m.mu.RLock()
		for _, session := range m.sessions {
			if session.status == stateConnected {
				_ = session.Subscribe(config.Topic, route.processMessage, config.QoS)
			}
		}
		m.mu.RUnlock()
	} else {
		// 订阅指定会话
		m.mu.RLock()
		session, ok := m.sessions[config.SessionName]
		if ok && session.status == stateConnected {
			_ = session.Subscribe(config.Topic, route.processMessage, config.QoS)
		}
		m.mu.RUnlock()
	}

	return route
}

// HandleToWithConfig 使用高级配置处理指定会话的指定主题消息
func (m *Manager) HandleToWithConfig(config RouteConfig) (*AdvancedRoute, error) {
	if config.SessionName == "" {
		return nil, ErrSessionNotFound
	}

	route := newAdvancedRoute(config, m)
	route.handler = config.Handler

	// 订阅指定会话
	m.mu.RLock()
	session, ok := m.sessions[config.SessionName]
	m.mu.RUnlock()

	if !ok {
		return nil, ErrSessionNotFound
	}

	if session.status != stateConnected {
		return nil, ErrNotConnected
	}

	err := session.Subscribe(config.Topic, route.processMessage, config.QoS)
	if err != nil {
		return nil, err
	}

	return route, nil
}

// ListenWithConfig 使用高级配置监听所有会话的指定主题消息
func (m *Manager) ListenWithConfig(config RouteConfig) (chan *Message, *AdvancedRoute) {
	route := newAdvancedRoute(config, m)

	// 根据会话名称决定订阅方式
	if config.SessionName == "" {
		// 订阅所有会话
		m.mu.RLock()
		for _, session := range m.sessions {
			if session.status == stateConnected {
				_ = session.Subscribe(config.Topic, route.processMessage, config.QoS)
			}
		}
		m.mu.RUnlock()
	} else {
		// 订阅指定会话
		m.mu.RLock()
		session, ok := m.sessions[config.SessionName]
		if ok && session.status == stateConnected {
			_ = session.Subscribe(config.Topic, route.processMessage, config.QoS)
		}
		m.mu.RUnlock()
	}

	return route.messages, route
}

// ListenToWithConfig 使用高级配置监听指定会话的指定主题消息
func (m *Manager) ListenToWithConfig(config RouteConfig) (chan *Message, *AdvancedRoute, error) {
	if config.SessionName == "" {
		return nil, nil, ErrSessionNotFound
	}

	route := newAdvancedRoute(config, m)

	// 订阅指定会话
	m.mu.RLock()
	session, ok := m.sessions[config.SessionName]
	m.mu.RUnlock()

	if !ok {
		return nil, nil, ErrSessionNotFound
	}

	if session.status != stateConnected {
		return nil, nil, ErrNotConnected
	}

	err := session.Subscribe(config.Topic, route.processMessage, config.QoS)
	if err != nil {
		return nil, nil, err
	}

	return route.messages, route, nil
}
