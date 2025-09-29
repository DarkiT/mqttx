package mqttx

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// BenchmarkSessionCreation 测试会话创建性能
func BenchmarkSessionCreation(b *testing.B) {
	m := NewSessionManager()
	defer m.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		name := fmt.Sprintf("bench-session-%d", i)
		opts := &Options{
			Name:     name,
			Brokers:  []string{"tcp://broker.emqx.io:1883"},
			ClientID: fmt.Sprintf("bench-client-%d", i),
		}
		err := m.AddSession(opts)
		if err != nil {
			b.Fatalf("Failed to add session: %v", err)
		}
		m.RemoveSession(name)
	}
}

// BenchmarkMessagePublish 测试消息发布性能
func BenchmarkMessagePublish(b *testing.B) {
	m := NewSessionManager()
	defer m.Close()

	// 创建一个测试会话
	sessionName := "bench-pub-session"
	opts := &Options{
		Name:     sessionName,
		Brokers:  []string{"tcp://broker.emqx.io:1883"},
		ClientID: "bench-pub-client",
	}
	err := m.AddSession(opts)
	if err != nil {
		b.Fatalf("Failed to add session: %v", err)
	}

	session, err := m.GetSession(sessionName)
	if err != nil {
		b.Fatalf("Failed to get session: %v", err)
	}

	// 设置消息处理函数
	msgCount := 0
	var mu sync.Mutex
	session.handlers.AddMessageHandler("test/bench/#", func(topic string, payload []byte) {
		mu.Lock()
		msgCount++
		mu.Unlock()
	})

	// 准备测试数据
	topics := []string{
		"test/bench/1", "test/bench/2", "test/bench/3",
		"test/bench/4", "test/bench/5",
	}
	payloads := [][]byte{
		[]byte("small payload"),
		[]byte("medium sized payload for benchmark testing"),
		make([]byte, 1024),    // 1KB
		make([]byte, 10*1024), // 10KB
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		topic := topics[i%len(topics)]
		payload := payloads[i%len(payloads)]
		if err := session.Publish(topic, payload, 0); err != nil {
			b.Fatalf("Failed to publish message: %v", err)
		}
	}
}

// BenchmarkConcurrentAccess 测试并发访问性能
func BenchmarkConcurrentAccess(b *testing.B) {
	m := NewSessionManager()
	defer m.Close()

	// 创建多个测试会话
	sessionCount := 50
	for i := 0; i < sessionCount; i++ {
		name := fmt.Sprintf("bench-concurrent-%d", i)
		opts := &Options{
			Name:     name,
			Brokers:  []string{"tcp://broker.emqx.io:1883"},
			ClientID: fmt.Sprintf("bench-concurrent-client-%d", i),
		}
		err := m.AddSession(opts)
		if err != nil {
			b.Fatalf("Failed to add session: %v", err)
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			// 交替执行不同的操作
			switch counter % 3 {
			case 0:
				// 获取会话
				name := fmt.Sprintf("bench-concurrent-%d", counter%sessionCount)
				_, _ = m.GetSession(name)
			case 1:
				// 获取会话状态
				m.GetAllSessionsStatus()
			case 2:
				// 发布消息
				name := fmt.Sprintf("bench-concurrent-%d", counter%sessionCount)
				_ = m.PublishTo(name, "test/bench/concurrent", []byte("test"), 0)
			}
			counter++
		}
	})
}

// BenchmarkSessionStore 测试会话状态存储性能
func BenchmarkSessionStore(b *testing.B) {
	b.Run("MemoryStore", func(b *testing.B) {
		benchmarkSessionStore(b, NewMemoryStore())
	})

	b.Run("FileStore", func(b *testing.B) {
		fileStore, err := NewFileStore("./test_data")
		if err != nil {
			b.Fatalf("Failed to create file store: %v", err)
		}
		benchmarkSessionStore(b, fileStore)
	})
}

func benchmarkSessionStore(b *testing.B, store SessionStore) {
	// 创建测试会话状态
	state := &SessionState{
		Topics: []TopicSubscription{
			{Topic: "test/topic1", QoS: 0},
			{Topic: "test/topic2", QoS: 1},
		},
		Messages: []*Message{
			{Topic: "test/topic1", Payload: []byte("test message 1"), QoS: 0},
			{Topic: "test/topic2", Payload: []byte("test message 2"), QoS: 1},
		},
		LastSequence:     100,
		LastConnected:    time.Now(),
		LastDisconnected: time.Now().Add(-time.Hour),
		QoSMessages:      make(map[uint16]*Message),
		RetainedMessages: make(map[string]*Message),
		ClientID:         "test-client",
		Version:          1,
	}

	// 添加一些QoS消息
	for i := uint16(1); i <= 10; i++ {
		state.QoSMessages[i] = &Message{
			Topic:     fmt.Sprintf("test/qos/%d", i),
			Payload:   []byte(fmt.Sprintf("qos message %d", i)),
			QoS:       1,
			MessageID: i,
		}
	}

	// 添加一些保留消息
	for i := 1; i <= 10; i++ {
		topic := fmt.Sprintf("test/retained/%d", i)
		state.RetainedMessages[topic] = &Message{
			Topic:    topic,
			Payload:  []byte(fmt.Sprintf("retained message %d", i)),
			QoS:      0,
			Retained: true,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sessionName := fmt.Sprintf("bench-store-%d", i%100)

		// 保存状态
		if err := store.SaveState(sessionName, state); err != nil {
			b.Fatalf("Failed to save state: %v", err)
		}

		// 加载状态
		loaded, err := store.LoadState(sessionName)
		if err != nil {
			b.Fatalf("Failed to load state: %v", err)
		}

		// 确保状态加载正确
		if loaded == nil || loaded.ClientID != state.ClientID {
			b.Fatalf("Loaded state is invalid")
		}
	}
}
