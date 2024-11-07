package mqtt

import (
	"encoding/json"
	"sync/atomic"
	"time"
)

// PayloadString 获取消息负载的字符串形式
func (m *Message) PayloadString() string {
	return string(m.Payload)
}

// PayloadJSON 将消息负载解析为JSON
func (m *Message) PayloadJSON(v interface{}) error {
	return json.Unmarshal(m.Payload, v)
}

// newRoute 创建新的路由
func newRoute(topic string, session string, manager *Manager, qos byte) *Route {
	return &Route{
		topic:    topic,
		session:  session,
		manager:  manager,
		messages: make(chan *Message, manager.getMessageBufferSize()),
		done:     make(chan struct{}),
		qos:      qos,
		stats:    &RouteStats{},
	}
}

// Stop 停止路由订阅
func (r *Route) Stop() {
	r.once.Do(func() {
		if r.session == "" {
			r.manager.UnsubscribeAll(r.topic)
		} else {
			r.manager.UnsubscribeTo(r.session, r.topic)
		}
		close(r.done)
		if r.messages != nil {
			close(r.messages)
		}
	})
}

// GetStats 获取路由统计信息
func (r *Route) GetStats() RouteStats {
	return RouteStats{
		MessagesReceived: atomic.LoadUint64(&r.stats.MessagesReceived),
		MessagesDropped:  atomic.LoadUint64(&r.stats.MessagesDropped),
		BytesReceived:    atomic.LoadUint64(&r.stats.BytesReceived),
		LastMessageTime:  r.stats.LastMessageTime,
		LastError:        r.stats.LastError,
		ErrorCount:       atomic.LoadUint64(&r.stats.ErrorCount),
	}
}

// Handle 处理所有会话的指定主题消息
func (m *Manager) Handle(topic string, handler func(*Message)) *Route {
	route := newRoute(topic, "", m, 0)

	m.SubscribeAll(topic, func(t string, payload []byte) {
		select {
		case <-route.done:
			return
		default:
			msg := &Message{
				Topic:     t,
				Payload:   payload,
				QoS:       route.qos,
				Timestamp: time.Now(),
			}

			atomic.AddUint64(&route.stats.MessagesReceived, 1)
			atomic.AddUint64(&route.stats.BytesReceived, uint64(len(payload)))
			route.stats.LastMessageTime = msg.Timestamp

			handler(msg)
		}
	}, route.qos)

	return route
}

// HandleTo 处理指定会话的指定主题消息
func (m *Manager) HandleTo(session, topic string, handler func(*Message)) (*Route, error) {
	route := newRoute(topic, session, m, 0)

	err := m.SubscribeTo(session, topic, func(t string, payload []byte) {
		select {
		case <-route.done:
			return
		default:
			msg := &Message{
				Topic:     t,
				Payload:   payload,
				QoS:       route.qos,
				Timestamp: time.Now(),
			}

			atomic.AddUint64(&route.stats.MessagesReceived, 1)
			atomic.AddUint64(&route.stats.BytesReceived, uint64(len(payload)))
			route.stats.LastMessageTime = msg.Timestamp

			handler(msg)
		}
	}, route.qos)
	if err != nil {
		return nil, err
	}

	return route, nil
}

// Listen 监听所有会话的指定主题消息
func (m *Manager) Listen(topic string) (chan *Message, *Route) {
	route := newRoute(topic, "", m, 0)

	m.SubscribeAll(topic, func(t string, payload []byte) {
		select {
		case <-route.done:
			return
		default:
			msg := &Message{
				Topic:     t,
				Payload:   payload,
				QoS:       route.qos,
				Timestamp: time.Now(),
			}

			atomic.AddUint64(&route.stats.MessagesReceived, 1)
			atomic.AddUint64(&route.stats.BytesReceived, uint64(len(payload)))
			route.stats.LastMessageTime = msg.Timestamp

			select {
			case route.messages <- msg:
			default:
				atomic.AddUint64(&route.stats.MessagesDropped, 1)
				m.logger.Warn("Message dropped due to full channel",
					"topic", topic,
					"qos", route.qos)
			}
		}
	}, route.qos)

	return route.messages, route
}

// ListenTo 监听指定会话的指定主题消息
func (m *Manager) ListenTo(session, topic string) (chan *Message, *Route, error) {
	route := newRoute(topic, session, m, 0)

	err := m.SubscribeTo(session, topic, func(t string, payload []byte) {
		select {
		case <-route.done:
			return
		default:
			msg := &Message{
				Topic:     t,
				Payload:   payload,
				QoS:       route.qos,
				Timestamp: time.Now(),
			}

			atomic.AddUint64(&route.stats.MessagesReceived, 1)
			atomic.AddUint64(&route.stats.BytesReceived, uint64(len(payload)))
			route.stats.LastMessageTime = msg.Timestamp

			select {
			case route.messages <- msg:
			default:
				atomic.AddUint64(&route.stats.MessagesDropped, 1)
				m.logger.Warn("Message dropped due to full channel",
					"topic", topic,
					"session", session,
					"qos", route.qos)
			}
		}
	}, route.qos)
	if err != nil {
		return nil, nil, err
	}

	return route.messages, route, nil
}

// getMessageBufferSize 获取消息缓冲区大小
func (m *Manager) getMessageBufferSize() int {
	if m.sessions == nil || len(m.sessions) == 0 {
		return DefaultMessageChanSize
	}

	// 遍历所有会话获取配置的最大缓冲区大小
	maxSize := DefaultMessageChanSize
	for _, session := range m.sessions {
		if session.opts.Performance != nil &&
			int(session.opts.Performance.MessageChanSize) > maxSize {
			maxSize = int(session.opts.Performance.MessageChanSize)
		}
	}
	return maxSize
}
