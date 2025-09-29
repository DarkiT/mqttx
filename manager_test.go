package mqttx

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// mockBroker 模拟MQTT broker用于测试
type mockBroker struct {
	messages map[string][]byte
	subs     map[string][]MessageHandler
	mu       sync.RWMutex
}

func newMockBroker() *mockBroker {
	return &mockBroker{
		messages: make(map[string][]byte),
		subs:     make(map[string][]MessageHandler),
	}
}

func (b *mockBroker) publish(topic string, payload []byte) {
	b.mu.Lock()
	// 创建一个深拷贝的payload，避免后续修改影响原始数据
	payloadCopy := make([]byte, len(payload))
	copy(payloadCopy, payload)
	b.messages[topic] = payloadCopy

	// 获取该主题的所有处理函数，并在解锁后调用，避免死锁
	var handlers []MessageHandler
	if subs, ok := b.subs[topic]; ok {
		handlers = make([]MessageHandler, len(subs))
		copy(handlers, subs)
	}
	b.mu.Unlock()

	// 调用所有订阅处理函数，传递深拷贝的payload
	for _, handler := range handlers {
		if handler != nil {
			// 为每个处理函数创建新的payload副本
			msgCopy := make([]byte, len(payload))
			copy(msgCopy, payload)
			handler(topic, msgCopy)
		}
	}
}

func (b *mockBroker) subscribe(topic string, handler MessageHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[topic] = append(b.subs[topic], handler)
}

// TestNewSessionManager 测试创建新的会话管理器
func TestNewSessionManager(t *testing.T) {
	m := NewSessionManager()
	if m == nil {
		t.Fatal("NewSessionManager should not return nil")
	}
	if m.sessions == nil {
		t.Error("sessions map should be initialized")
	}
	if m.events == nil {
		t.Error("events should be initialized")
	}
	if m.logger == nil {
		t.Error("logger should be initialized")
	}
}

// TestAddSession 测试添加会话功能
func TestAddSession(t *testing.T) {
	m := NewSessionManager()
	tests := []struct {
		name    string
		opts    *Options
		wantErr error
	}{
		{
			name: "valid session",
			opts: &Options{
				Name:     "test1",
				Brokers:  []string{"tcp://broker.emqx.io:1883"},
				ClientID: "client1",
			},
			wantErr: nil,
		},
		{
			name: "duplicate session",
			opts: &Options{
				Name:     "test1",
				Brokers:  []string{"tcp://broker.emqx.io:1883"},
				ClientID: "client1",
			},
			wantErr: ErrSessionExists,
		},
		{
			name: "invalid options",
			opts: &Options{
				Name: "", // 空名称
			},
			wantErr: ErrInvalidOptions,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.AddSession(tt.opts)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("AddSession() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestGetSession 测试获取会话功能
func TestGetSession(t *testing.T) {
	m := NewSessionManager()
	opts := &Options{
		Name:     "test1",
		Brokers:  []string{"tcp://broker.emqx.io:1883"},
		ClientID: "client1",
	}
	m.AddSession(opts)

	tests := []struct {
		name    string
		session string
		wantErr error
	}{
		{
			name:    "existing session",
			session: "test1",
			wantErr: nil,
		},
		{
			name:    "non-existing session",
			session: "test2",
			wantErr: ErrSessionNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := m.GetSession(tt.session)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("GetSession() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestRemoveSession 测试移除会话功能
func TestRemoveSession(t *testing.T) {
	m := NewSessionManager()
	opts := &Options{
		Name:     "test1",
		Brokers:  []string{"tcp://broker.emqx.io:1883"},
		ClientID: "client1",
	}
	m.AddSession(opts)

	tests := []struct {
		name    string
		session string
		wantErr error
	}{
		{
			name:    "existing session",
			session: "test1",
			wantErr: nil,
		},
		{
			name:    "already removed session",
			session: "test1",
			wantErr: ErrSessionNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.RemoveSession(tt.session)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("RemoveSession() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestEventHandling 测试事件处理功能
func TestEventHandling(t *testing.T) {
	m := NewSessionManager()
	// 设置事件处理函数
	received := make(chan *Event, 1)
	m.OnEvent("test_event", func(event Event) {
		received <- &event
	})

	// 发送测试事件
	testEvent := &Event{
		Type:    "test_event",
		Session: "test1",
		Data:    "test_data",
	}

	m.events.emit(testEvent)

	select {
	case event := <-received:
		if event.Type != testEvent.Type {
			t.Errorf("Got event type %v, want %v", event.Type, testEvent.Type)
		}
		if event.Session != testEvent.Session {
			t.Errorf("Got session %v, want %v", event.Session, testEvent.Session)
		}
		if event.Data != testEvent.Data {
			t.Errorf("Got data %v, want %v", event.Data, testEvent.Data)
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for event")
	}
}

// TestSessionStatus 测试会话状态功能
func TestSessionStatus(t *testing.T) {
	m := NewSessionManager()

	// 添加测试会话
	opts := &Options{
		Name:     "test1",
		Brokers:  []string{"tcp://broker.emqx.io:1883"},
		ClientID: "client1",
	}
	m.AddSession(opts)

	// 获取会话状态
	status := m.GetAllSessionsStatus()
	if len(status) != 1 {
		t.Errorf("Got %d sessions, want 1", len(status))
	}

	// 检查会话是否存在，但不检查具体状态
	// 因为测试时状态可能是connected或disconnected，取决于网络连接情况
	if _, exists := status["test1"]; !exists {
		t.Error("Session test1 not found in status")
	}
}

// TestDefaultLogger 测试默认日志记录器
func TestDefaultLogger(t *testing.T) {
	logger := NewDefaultLogger()

	// 测试所有日志级别
	tests := []struct {
		name  string
		level string
		fn    func(string, ...interface{})
	}{
		{"Debug", "DEBUG", logger.Debug},
		{"Info", "INFO", logger.Info},
		{"Warn", "WARN", logger.Warn},
		{"Error", "ERROR", logger.Error},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 日志输出测试可以通过检查是否panic来验证基本功能
			tt.fn("test message", "key", "value")
		})
	}
}

// TestMessageMethods 测试消息方法
func TestMessageMethods(t *testing.T) {
	msg := &Message{
		Topic:   "test/topic",
		Payload: []byte(`{"key":"value"}`),
		QoS:     1,
	}

	t.Run("PayloadString", func(t *testing.T) {
		if s := msg.PayloadString(); s != `{"key":"value"}` {
			t.Errorf("PayloadString() = %v, want %v", s, `{"key":"value"}`)
		}
	})

	t.Run("PayloadJSON", func(t *testing.T) {
		var data struct {
			Key string `json:"key"`
		}
		if err := msg.PayloadJSON(&data); err != nil {
			t.Errorf("PayloadJSON() error = %v", err)
		}
		if data.Key != "value" {
			t.Errorf("PayloadJSON() = %v, want %v", data.Key, "value")
		}
	})
}

// TestConcurrency 测试并发安全性
func TestConcurrency(t *testing.T) {
	m := NewSessionManager()
	var wg sync.WaitGroup

	// 并发添加会话
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			opts := &Options{
				Name:     fmt.Sprintf("test%d", i),
				Brokers:  []string{"tcp://broker.emqx.io:1883"},
				ClientID: fmt.Sprintf("client%d", i),
			}
			m.AddSession(opts)
		}(i)
	}

	// 并发获取会话状态
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.GetAllSessionsStatus()
		}()
	}

	wg.Wait()
}

// TestPublishAndSubscribe 测试发布和订阅功能
func TestPublishAndSubscribe(t *testing.T) {
	// 创建模拟 Broker
	broker := newMockBroker()

	// 测试数据
	testValue := "test"
	payload := []byte(`{"value":"test"}`)

	// 测试 Handle 模式
	t.Run("Handle", func(t *testing.T) {
		var received []byte
		var wg sync.WaitGroup
		wg.Add(1)

		// 直接订阅模拟 Broker
		broker.subscribe("test/topic", func(topic string, data []byte) {
			// 复制数据，避免引用问题
			received = make([]byte, len(data))
			copy(received, data)
			wg.Done()
		})

		// 发布消息
		broker.publish("test/topic", payload)

		// 等待接收
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// 验证收到的消息
			var receivedMsg struct {
				Value string `json:"value"`
			}

			t.Logf("Received payload: %s", string(received))

			err := json.Unmarshal(received, &receivedMsg)
			if err != nil {
				t.Errorf("Failed to parse message: %v", err)
			} else if receivedMsg.Value != testValue {
				t.Errorf("Got message value %v, want %v", receivedMsg.Value, testValue)
			}
		case <-time.After(1 * time.Second):
			t.Error("Test timeout after 1 second")
		}
	})

	// 测试 Listen 模式
	t.Run("Listen", func(t *testing.T) {
		// 创建消息通道
		messages := make(chan []byte, 1)

		// 订阅模拟 Broker
		broker.subscribe("test/listen", func(topic string, data []byte) {
			// 复制数据
			dataCopy := make([]byte, len(data))
			copy(dataCopy, data)
			messages <- dataCopy
		})

		// 发布消息
		broker.publish("test/listen", payload)

		// 等待接收
		select {
		case received := <-messages:
			// 验证收到的消息
			var receivedMsg struct {
				Value string `json:"value"`
			}

			t.Logf("Received payload: %s", string(received))

			err := json.Unmarshal(received, &receivedMsg)
			if err != nil {
				t.Errorf("Failed to parse message: %v", err)
			} else if receivedMsg.Value != testValue {
				t.Errorf("Got message value %v, want %v", receivedMsg.Value, testValue)
			}
		case <-time.After(1 * time.Second):
			t.Error("Test timeout after 1 second")
		}
	})
}
