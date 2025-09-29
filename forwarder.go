package mqttx

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ForwarderConfig 简化的消息转发器配置
type ForwarderConfig struct {
	Name          string            // 转发器名称
	SourceSession string            // 源会话名称
	SourceTopics  []string          // 源主题列表
	TargetSession string            // 目标会话
	TopicMap      map[string]string // 主题映射表（源主题 -> 目标主题）
	QoS           byte              // 服务质量等级
	BufferSize    int               // 缓冲区大小
	Enabled       bool              // 是否启用
}

// Forwarder 简化的消息转发器
type Forwarder struct {
	config      ForwarderConfig     // 转发器配置
	manager     *Manager            // 会话管理器
	messageCh   chan *PooledMessage // 使用池化消息的通道
	stopCh      chan struct{}       // 停止信号通道
	running     bool                // 是否运行中
	msgReceived uint64              // 接收的消息数
	msgSent     uint64              // 发送的消息数
	msgDropped  uint64              // 丢弃的消息数
	lastSent    time.Time           // 最后发送时间
	lastError   time.Time           // 最后错误时间
	mu          sync.RWMutex        // 互斥锁
	stopOnce    sync.Once           // 确保只关闭一次
	stopped     bool                // 标记是否已停止，防止重复关闭通道
}

// NewForwarder 创建新的简化消息转发器
func NewForwarder(config ForwarderConfig, manager *Manager) (*Forwarder, error) {
	// 验证必要的配置
	if config.SourceSession == "" {
		return nil, fmt.Errorf("源会话不能为空")
	}
	if len(config.SourceTopics) == 0 {
		return nil, fmt.Errorf("源主题列表不能为空")
	}
	if config.TargetSession == "" {
		return nil, fmt.Errorf("目标会话不能为空")
	}

	// 设置默认缓冲区大小
	bufferSize := 100
	if config.BufferSize > 0 {
		bufferSize = config.BufferSize
	}

	return &Forwarder{
		config:    config,
		manager:   manager,
		messageCh: make(chan *PooledMessage, bufferSize),
		stopCh:    make(chan struct{}),
	}, nil
}

// Start 启动转发器
func (sf *Forwarder) Start() error {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	if sf.running {
		return fmt.Errorf("转发器已启动")
	}

	// 获取源会话
	session, err := sf.manager.GetSession(sf.config.SourceSession)
	if err != nil {
		return fmt.Errorf("无法获取源会话: %w", err)
	}

	// 为所有源主题订阅消息
	for _, topic := range sf.config.SourceTopics {
		err = session.Subscribe(topic, sf.handleMessage, sf.config.QoS)
		if err != nil {
			return fmt.Errorf("订阅主题 %s 失败: %w", topic, err)
		}
	}

	// 启动消息处理协程
	go sf.processMessages()

	sf.running = true
	return nil
}

// Stop 停止转发器
func (sf *Forwarder) Stop() {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	if !sf.running || sf.stopped {
		return
	}

	// 使用sync.Once确保停止操作只执行一次
	sf.stopOnce.Do(func() {
		// 先设置为非运行状态，避免重复停止
		sf.running = false
		sf.stopped = true

		// 安全地关闭停止信号通道
		close(sf.stopCh)

		// 尝试取消所有主题订阅
		// 安全检查：确保获取到会话后再执行取消订阅操作
		if session, err := sf.manager.GetSession(sf.config.SourceSession); err == nil && session != nil {
			for _, topic := range sf.config.SourceTopics {
				_ = session.Unsubscribe(topic)
			}
		}
	})
}

// IsRunning 检查转发器是否运行中
func (sf *Forwarder) IsRunning() bool {
	sf.mu.RLock()
	defer sf.mu.RUnlock()
	return sf.running
}

// handleMessage 处理接收到的消息
func (sf *Forwarder) handleMessage(topic string, payload []byte) {
	// 检查转发器是否已停止
	sf.mu.RLock()
	stopped := sf.stopped
	sf.mu.RUnlock()

	if stopped {
		// 转发器已停止，直接丢弃消息
		atomic.AddUint64(&sf.msgDropped, 1)
		return
	}

	atomic.AddUint64(&sf.msgReceived, 1)

	// 从对象池获取消息对象
	msg := GetMessage()
	msg.Topic = topic
	msg.SetPayload(payload)
	msg.QoS = sf.config.QoS
	msg.Timestamp = time.Now()

	// 将消息放入通道进行异步处理
	select {
	case sf.messageCh <- msg:
		// 消息成功放入通道
	default:
		// 通道已满，消息被丢弃
		atomic.AddUint64(&sf.msgDropped, 1)
		// 归还消息对象到池中
		PutMessage(msg)
	}
}

// processMessages 处理消息队列
func (sf *Forwarder) processMessages() {
	for {
		select {
		case <-sf.stopCh:
			// 收到停止信号
			return
		case msg := <-sf.messageCh:
			// 处理消息
			sf.forwardMessage(msg)
		}
	}
}

// forwardMessage 转发消息
func (sf *Forwarder) forwardMessage(msg *PooledMessage) {
	// 确保在函数结束时归还消息对象
	defer PutMessage(msg)

	// 确定目标主题
	targetTopic := msg.Topic

	// 检查主题映射
	if sf.config.TopicMap != nil {
		if mapped, exists := sf.config.TopicMap[msg.Topic]; exists {
			targetTopic = mapped
		}
	}

	// 发送消息
	err := sf.manager.PublishTo(sf.config.TargetSession, targetTopic, msg.Payload, sf.config.QoS)

	if err != nil {
		sf.mu.Lock()
		sf.lastError = time.Now()
		sf.mu.Unlock()
	} else {
		sf.mu.Lock()
		sf.lastSent = time.Now()
		sf.mu.Unlock()
		atomic.AddUint64(&sf.msgSent, 1)
	}
}

// GetMetrics 获取转发器指标
func (sf *Forwarder) GetMetrics() map[string]interface{} {
	sf.mu.RLock()
	lastSent := sf.lastSent
	lastError := sf.lastError
	sf.mu.RUnlock()

	return map[string]interface{}{
		"name":           sf.config.Name,
		"source_session": sf.config.SourceSession,
		"target_session": sf.config.TargetSession,
		"topics_count":   len(sf.config.SourceTopics),
		"received":       atomic.LoadUint64(&sf.msgReceived),
		"sent":           atomic.LoadUint64(&sf.msgSent),
		"dropped":        atomic.LoadUint64(&sf.msgDropped),
		"last_sent":      lastSent,
		"last_error":     lastError,
		"running":        sf.IsRunning(),
	}
}

// ForwarderRegistry 简化的转发器注册表
type ForwarderRegistry struct {
	forwarders map[string]*Forwarder
	manager    *Manager
	mu         sync.RWMutex
}

// NewForwarderRegistry 创建新的转发器注册表
func NewForwarderRegistry(manager *Manager) *ForwarderRegistry {
	return &ForwarderRegistry{
		forwarders: make(map[string]*Forwarder),
		manager:    manager,
	}
}

// Register 注册并启动一个新的转发器
func (sfr *ForwarderRegistry) Register(config ForwarderConfig) (*Forwarder, error) {
	sfr.mu.Lock()
	defer sfr.mu.Unlock()

	// 检查名称是否已存在
	if _, exists := sfr.forwarders[config.Name]; exists {
		return nil, fmt.Errorf("转发器名称 '%s' 已存在", config.Name)
	}

	// 创建新的转发器
	forwarder, err := NewForwarder(config, sfr.manager)
	if err != nil {
		return nil, err
	}

	// 如果配置为启用，则启动转发器
	if config.Enabled {
		if err := forwarder.Start(); err != nil {
			return nil, err
		}
	}

	// 添加到注册表
	sfr.forwarders[config.Name] = forwarder

	return forwarder, nil
}

// Unregister 注销并停止一个转发器
func (sfr *ForwarderRegistry) Unregister(name string) error {
	sfr.mu.Lock()
	defer sfr.mu.Unlock()

	forwarder, exists := sfr.forwarders[name]
	if !exists {
		return fmt.Errorf("转发器 '%s' 不存在", name)
	}

	// 停止转发器
	forwarder.Stop()

	// 从注册表中删除
	delete(sfr.forwarders, name)

	return nil
}

// Get 获取指定名称的转发器
func (sfr *ForwarderRegistry) Get(name string) (*Forwarder, error) {
	sfr.mu.RLock()
	defer sfr.mu.RUnlock()

	forwarder, exists := sfr.forwarders[name]
	if !exists {
		return nil, fmt.Errorf("转发器 '%s' 不存在", name)
	}

	return forwarder, nil
}

// List 列出所有转发器的名称
func (sfr *ForwarderRegistry) List() []string {
	sfr.mu.RLock()
	defer sfr.mu.RUnlock()

	names := make([]string, 0, len(sfr.forwarders))
	for name := range sfr.forwarders {
		names = append(names, name)
	}

	return names
}

// GetAllMetrics 获取所有转发器的指标
func (sfr *ForwarderRegistry) GetAllMetrics() map[string]map[string]interface{} {
	sfr.mu.RLock()
	defer sfr.mu.RUnlock()

	metrics := make(map[string]map[string]interface{})
	for name, forwarder := range sfr.forwarders {
		metrics[name] = forwarder.GetMetrics()
	}

	return metrics
}

// StopAll 停止所有转发器
func (sfr *ForwarderRegistry) StopAll() {
	sfr.mu.RLock()
	defer sfr.mu.RUnlock()

	for _, forwarder := range sfr.forwarders {
		// 安全地停止每个转发器
		if forwarder != nil && forwarder.IsRunning() {
			forwarder.Stop()
		}
	}
}
