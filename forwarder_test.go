package mqttx

import (
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

// 模拟会话用于测试
type mockSession struct {
	name          string
	subscriptions map[string]func(string, []byte)
	publishedMsgs []mockMessage
	mu            sync.Mutex
	onSubscribe   func(string, func(string, []byte), byte) error
	onUnsubscribe func(string) error
	onPublish     func(string, []byte, byte) error
}

type mockMessage struct {
	topic   string
	payload []byte
	qos     byte
}

func newMockSession(name string) *mockSession {
	return &mockSession{
		name:          name,
		subscriptions: make(map[string]func(string, []byte)),
		publishedMsgs: make([]mockMessage, 0),
	}
}

func (m *mockSession) Subscribe(topic string, callback func(string, []byte), qos byte) error {
	if m.onSubscribe != nil {
		return m.onSubscribe(topic, callback, qos)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscriptions[topic] = callback
	return nil
}

func (m *mockSession) Unsubscribe(topic string) error {
	if m.onUnsubscribe != nil {
		return m.onUnsubscribe(topic)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.subscriptions, topic)
	return nil
}

func (m *mockSession) Publish(topic string, payload []byte, qos byte) error {
	if m.onPublish != nil {
		return m.onPublish(topic, payload, qos)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publishedMsgs = append(m.publishedMsgs, mockMessage{
		topic:   topic,
		payload: payload,
		qos:     qos,
	})
	return nil
}

func (m *mockSession) simulateIncomingMessage(topic string, payload []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if handler, ok := m.subscriptions[topic]; ok {
		handler(topic, payload)
	}
}

func (m *mockSession) countPublishedMessages() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.publishedMsgs)
}

func (m *mockSession) getLastPublishedMessage() (mockMessage, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.publishedMsgs) == 0 {
		return mockMessage{}, false
	}
	return m.publishedMsgs[len(m.publishedMsgs)-1], true
}

// 模拟Manager用于测试
type mockManager struct {
	sessions map[string]*mockSession
}

func newMockManager() *mockManager {
	return &mockManager{
		sessions: make(map[string]*mockSession),
	}
}

func (m *mockManager) addMockSession(session *mockSession) {
	m.sessions[session.name] = session
}

func (m *mockManager) GetSession(name string) (*Session, error) {
	if _, ok := m.sessions[name]; ok {
		// 我们返回nil，因为在测试中实际上不会用到Session对象
		// 但会检查错误是否为nil
		return nil, nil
	}
	return nil, fmt.Errorf("会话 '%s' 不存在", name)
}

func (m *mockManager) PublishTo(sessionName, topic string, payload []byte, qos byte) error {
	if session, ok := m.sessions[sessionName]; ok {
		return session.Publish(topic, payload, qos)
	}
	return fmt.Errorf("会话 '%s' 不存在", sessionName)
}

// 使用不安全的方式修改结构体的私有字段（仅测试使用）
func setForwarderField(f *Forwarder, fieldName string, value interface{}) {
	// 获取结构体的反射值
	val := reflect.ValueOf(f).Elem()

	// 找到要修改的字段
	field := val.FieldByName(fieldName)

	if field.IsValid() {
		// 获取字段的指针
		ptr := unsafe.Pointer(field.UnsafeAddr())

		// 创建一个新的反射值指向该地址
		reflect.NewAt(field.Type(), ptr).Elem().Set(reflect.ValueOf(value))
	}
}

// 自定义的处理消息函数，使用模拟会话
func customForwardMessage(msg *Message, forwarder *Forwarder, targetSession *mockSession) {
	// 确定目标主题
	targetTopic := msg.Topic

	// 检查主题映射
	if forwarder.config.TopicMap != nil {
		if mapped, exists := forwarder.config.TopicMap[msg.Topic]; exists {
			targetTopic = mapped
		}
	}

	// 直接调用模拟目标会话的Publish方法
	err := targetSession.Publish(targetTopic, msg.Payload, forwarder.config.QoS)

	if err != nil {
		// 更新统计信息
	} else {
		atomic.AddUint64(&forwarder.msgSent, 1)
	}
}

// 测试转发器
func TestForwarder(t *testing.T) {
	// 创建模拟会话和管理器
	sourceSession := newMockSession("source")
	targetSession := newMockSession("target")

	mockMgr := newMockManager()
	mockMgr.addMockSession(sourceSession)
	mockMgr.addMockSession(targetSession)

	// 创建简化的转发器配置
	config := ForwarderConfig{
		Name:          "test-forwarder",
		SourceSession: "source",
		SourceTopics:  []string{"sensors/temp", "sensors/humidity"},
		TargetSession: "target",
		TopicMap: map[string]string{
			"sensors/temp":     "processed/temperature",
			"sensors/humidity": "processed/humidity",
		},
		QoS:        1,
		BufferSize: 10,
		Enabled:    true,
	}

	// 创建一个Manager实例并设置测试函数
	manager := &Manager{
		getSessionFunc: mockMgr.GetSession,
		publishToFunc:  mockMgr.PublishTo,
	}

	// 创建转发器
	forwarder, err := NewForwarder(config, manager)
	assert.NoError(t, err)
	assert.NotNil(t, forwarder)

	// 创建自定义的转发消息处理函数
	msgHandler := func(msg *Message) {
		customForwardMessage(msg, forwarder, targetSession)
	}

	// 手动设置转发器的运行状态
	forwarder.running = true

	// 确保stopCh已创建，避免关闭nil通道
	if forwarder.stopCh == nil {
		forwarder.stopCh = make(chan struct{})
	}

	// 使用反射修改私有字段
	setForwarderField(forwarder, "forwardMessage", msgHandler)

	// 手动设置订阅
	for _, topic := range config.SourceTopics {
		sourceSession.subscriptions[topic] = func(topic string, payload []byte) {
			forwarder.handleMessage(topic, payload)
		}
	}

	// 启动消息处理协程
	go forwarder.processMessages()

	// 确认转发器已启动
	assert.True(t, forwarder.IsRunning())

	// 模拟接收消息
	sourceSession.simulateIncomingMessage("sensors/temp", []byte("25.5"))

	// 等待消息处理
	time.Sleep(100 * time.Millisecond)

	// 验证消息是否被正确转发
	assert.Equal(t, 1, targetSession.countPublishedMessages())

	lastMsg, ok := targetSession.getLastPublishedMessage()
	assert.True(t, ok)
	assert.Equal(t, "processed/temperature", lastMsg.topic)
	assert.Equal(t, []byte("25.5"), lastMsg.payload)
	assert.Equal(t, byte(1), lastMsg.qos)

	// 模拟另一条消息
	sourceSession.simulateIncomingMessage("sensors/humidity", []byte("45%"))

	// 等待消息处理
	time.Sleep(100 * time.Millisecond)

	// 验证第二条消息
	assert.Equal(t, 2, targetSession.countPublishedMessages())

	lastMsg, ok = targetSession.getLastPublishedMessage()
	assert.True(t, ok)
	assert.Equal(t, "processed/humidity", lastMsg.topic)
	assert.Equal(t, []byte("45%"), lastMsg.payload)

	// 检查指标
	metrics := forwarder.GetMetrics()
	assert.Equal(t, uint64(2), metrics["received"])
	assert.Equal(t, uint64(2), metrics["sent"])
	assert.Equal(t, uint64(0), metrics["dropped"])

	// 安全地停止转发器
	// 直接设置运行状态，模拟停止
	forwarder.running = false
	close(forwarder.stopCh)

	// 验证转发器已停止
	assert.False(t, forwarder.IsRunning())
}

// 转发器注册表测试
func TestForwarderRegistry(t *testing.T) {
	// 创建模拟会话和管理器
	sourceSession := newMockSession("source")
	targetSession := newMockSession("target")

	mockMgr := newMockManager()
	mockMgr.addMockSession(sourceSession)
	mockMgr.addMockSession(targetSession)

	// 创建一个Manager实例并设置测试函数
	manager := &Manager{
		getSessionFunc: mockMgr.GetSession,
		publishToFunc:  mockMgr.PublishTo,
	}

	// 创建注册表
	registry := NewForwarderRegistry(manager)
	assert.NotNil(t, registry)

	// 添加测试转发器
	config1 := ForwarderConfig{
		Name:          "forwarder1",
		SourceSession: "source",
		SourceTopics:  []string{"topic1"},
		TargetSession: "target",
		QoS:           0,
		Enabled:       true,
	}

	// 创建转发器并设置运行状态
	forwarder1, err := NewForwarder(config1, manager)
	assert.NoError(t, err)
	forwarder1.running = true

	// 确保stopCh已初始化
	forwarder1.stopCh = make(chan struct{})

	// 手动添加到注册表
	registry.forwarders["forwarder1"] = forwarder1

	// 创建第二个转发器
	config2 := ForwarderConfig{
		Name:          "forwarder2",
		SourceSession: "source",
		SourceTopics:  []string{"topic2"},
		TargetSession: "target",
		QoS:           1,
		Enabled:       false,
	}

	forwarder2, err := NewForwarder(config2, manager)
	assert.NoError(t, err)

	// 初始化stopCh通道
	forwarder2.stopCh = make(chan struct{})

	// 手动添加到注册表
	registry.forwarders["forwarder2"] = forwarder2

	// 列出所有转发器
	forwarders := registry.List()
	assert.Len(t, forwarders, 2)
	assert.Contains(t, forwarders, "forwarder1")
	assert.Contains(t, forwarders, "forwarder2")

	// 获取指定转发器
	f1, err := registry.Get("forwarder1")
	assert.NoError(t, err)
	assert.Equal(t, "forwarder1", f1.config.Name)

	// 获取所有指标
	metrics := registry.GetAllMetrics()
	assert.Len(t, metrics, 2)
	assert.Contains(t, metrics, "forwarder1")
	assert.Contains(t, metrics, "forwarder2")

	// 安全地注销一个转发器
	forwarder1.running = false
	close(forwarder1.stopCh)
	delete(registry.forwarders, "forwarder1")

	// 确认转发器已被移除
	forwarders = registry.List()
	assert.Len(t, forwarders, 1)
	assert.Contains(t, forwarders, "forwarder2")

	// 安全地停止所有转发器
	for _, f := range registry.forwarders {
		if f.running {
			f.running = false
			close(f.stopCh)
		}
	}
}
