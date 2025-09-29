package mqttx

import (
	"fmt"
	"strings"
	"time"
)

// SessionBuilder 会话配置构建器，提供流畅的API
type SessionBuilder struct {
	opts   *Options
	errors []error
}

// NewSessionBuilder 创建新的会话构建器
func NewSessionBuilder(name string) *SessionBuilder {
	return &SessionBuilder{
		opts: &Options{
			Name:         name,
			Brokers:      []string{"tcp://localhost:1883"},
			ClientID:     fmt.Sprintf("mqttx-%s-%d", name, randomClientID()),
			ConnectProps: DefaultOptions().ConnectProps,
			Performance:  DefaultOptions().Performance,
			Storage:      DefaultOptions().Storage,
		},
	}
}

// QuickConnect 快速连接配置（最简API）
func QuickConnect(name, broker string) *SessionBuilder {
	return NewSessionBuilder(name).Broker(broker)
}

// SecureConnect 安全连接配置（常用TLS场景）
func SecureConnect(name, broker, caFile string) *SessionBuilder {
	return NewSessionBuilder(name).
		Broker(broker).
		TLS(caFile, "", "", false)
}

// Broker 设置单个broker地址
func (b *SessionBuilder) Broker(addr string) *SessionBuilder {
	if addr = strings.TrimSpace(addr); addr == "" {
		b.addError("broker address cannot be empty")
		return b
	}

	// 自动添加协议前缀
	if !strings.Contains(addr, "://") {
		addr = "tcp://" + addr
	}

	b.opts.Brokers = []string{addr}
	return b
}

// Brokers 设置多个broker地址（高可用）
func (b *SessionBuilder) Brokers(addrs ...string) *SessionBuilder {
	if len(addrs) == 0 {
		b.addError("at least one broker address is required")
		return b
	}

	brokers := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		if addr = strings.TrimSpace(addr); addr != "" {
			// 自动添加协议前缀
			if !strings.Contains(addr, "://") {
				addr = "tcp://" + addr
			}
			brokers = append(brokers, addr)
		}
	}

	if len(brokers) == 0 {
		b.addError("no valid broker addresses provided")
		return b
	}

	b.opts.Brokers = brokers
	return b
}

// ClientID 设置客户端ID
func (b *SessionBuilder) ClientID(id string) *SessionBuilder {
	if id = strings.TrimSpace(id); id == "" {
		b.addError("client ID cannot be empty")
		return b
	}
	b.opts.ClientID = id
	return b
}

// Auth 设置认证信息
func (b *SessionBuilder) Auth(username, password string) *SessionBuilder {
	b.opts.Username = strings.TrimSpace(username)
	b.opts.Password = password
	return b
}

// TLS 配置TLS安全连接
func (b *SessionBuilder) TLS(caFile, certFile, keyFile string, skipVerify bool) *SessionBuilder {
	b.opts.TLS = &TLSConfig{
		CAFile:     strings.TrimSpace(caFile),
		CertFile:   strings.TrimSpace(certFile),
		KeyFile:    strings.TrimSpace(keyFile),
		SkipVerify: skipVerify,
	}
	return b
}

// CleanSession 设置是否清理会话
func (b *SessionBuilder) CleanSession(clean bool) *SessionBuilder {
	b.opts.ConnectProps.CleanSession = clean
	return b
}

// KeepAlive 设置保活时间（秒）
func (b *SessionBuilder) KeepAlive(seconds int) *SessionBuilder {
	if seconds <= 0 {
		b.addError("keep alive must be positive")
		return b
	}
	b.opts.ConnectProps.KeepAlive = seconds
	return b
}

// AutoReconnect 启用自动重连
func (b *SessionBuilder) AutoReconnect() *SessionBuilder {
	b.opts.ConnectProps.AutoReconnect = true
	return b
}

// DisableReconnect 禁用自动重连
func (b *SessionBuilder) DisableReconnect() *SessionBuilder {
	b.opts.ConnectProps.AutoReconnect = false
	return b
}

// ReconnectConfig 详细重连配置
func (b *SessionBuilder) ReconnectConfig(initialSec, maxSec int, backoffFactor float64) *SessionBuilder {
	if initialSec <= 0 {
		initialSec = 2
	}
	if maxSec <= 0 {
		maxSec = 120
	}
	if backoffFactor <= 1.0 {
		backoffFactor = 1.5
	}

	b.opts.ConnectProps.InitialReconnectInterval = time.Duration(initialSec) * time.Second
	b.opts.ConnectProps.MaxReconnectInterval = time.Duration(maxSec) * time.Second
	b.opts.ConnectProps.BackoffFactor = backoffFactor
	return b
}

// Timeouts 设置连接和写入超时（秒）
func (b *SessionBuilder) Timeouts(connectSec, writeSec int) *SessionBuilder {
	if connectSec <= 0 {
		connectSec = 30
	}
	if writeSec <= 0 {
		writeSec = 30
	}

	b.opts.ConnectProps.ConnectTimeout = time.Duration(connectSec) * time.Second
	b.opts.ConnectProps.WriteTimeout = time.Duration(writeSec) * time.Second
	return b
}

// Performance 性能调优配置
func (b *SessionBuilder) Performance(bufferSizeKB, maxPendingMsgs int) *SessionBuilder {
	if bufferSizeKB <= 0 {
		bufferSizeKB = 4
	}
	if maxPendingMsgs <= 0 {
		maxPendingMsgs = 1000
	}

	bufferSize := bufferSizeKB * 1024
	b.opts.Performance.WriteBufferSize = bufferSize
	b.opts.Performance.ReadBufferSize = bufferSize
	b.opts.Performance.MaxPendingMessages = maxPendingMsgs
	return b
}

// MessageChannelSize 设置消息通道大小
func (b *SessionBuilder) MessageChannelSize(size int) *SessionBuilder {
	if size <= 0 {
		size = 100
	}
	b.opts.Performance.MessageChanSize = uint(size)
	return b
}

// Persistent 启用会话持久化
func (b *SessionBuilder) Persistent() *SessionBuilder {
	b.opts.ConnectProps.PersistentSession = true
	b.opts.ConnectProps.CleanSession = false
	return b
}

// FileStorage 使用文件存储
func (b *SessionBuilder) FileStorage(path string) *SessionBuilder {
	if path = strings.TrimSpace(path); path == "" {
		b.addError("storage path cannot be empty")
		return b
	}

	b.opts.Storage = &StorageOptions{
		Type: StoreTypeFile,
		Path: path,
	}
	return b
}

// RedisStorage 使用Redis存储
func (b *SessionBuilder) RedisStorage(addr string) *SessionBuilder {
	if addr = strings.TrimSpace(addr); addr == "" {
		b.addError("redis address cannot be empty")
		return b
	}

	b.opts.Storage = &StorageOptions{
		Type: StoreTypeRedis,
		Redis: &RedisOptions{
			Addr:      addr,
			DB:        0,
			KeyPrefix: "mqttx:session:",
			TTL:       86400, // 24小时
			PoolSize:  10,
		},
	}
	return b
}

// RedisAuth 设置Redis认证
func (b *SessionBuilder) RedisAuth(username, password string, db int) *SessionBuilder {
	if b.opts.Storage == nil || b.opts.Storage.Type != StoreTypeRedis {
		b.addError("redis storage must be configured before setting auth")
		return b
	}

	if b.opts.Storage.Redis == nil {
		b.opts.Storage.Redis = DefaultRedisOptions()
	}

	b.opts.Storage.Redis.Username = strings.TrimSpace(username)
	b.opts.Storage.Redis.Password = password
	b.opts.Storage.Redis.DB = db
	return b
}

// Subscribe 添加预订阅主题
func (b *SessionBuilder) Subscribe(topic string, qos byte, handler MessageHandler) *SessionBuilder {
	if topic = strings.TrimSpace(topic); topic == "" {
		b.addError("topic cannot be empty")
		return b
	}
	if qos > 2 {
		b.addError("QoS must be 0, 1, or 2")
		return b
	}
	if handler == nil {
		b.addError("message handler cannot be nil")
		return b
	}

	b.opts.Topics = append(b.opts.Topics, TopicConfig{
		Topic:   topic,
		QoS:     qos,
		Handler: handler,
	})
	return b
}

// Build 构建配置选项，包含完整验证
func (b *SessionBuilder) Build() (*Options, error) {
	// 检查构建过程中的错误
	if len(b.errors) > 0 {
		return nil, fmt.Errorf("configuration errors: %v", b.errors)
	}

	// 执行完整验证
	if err := b.opts.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return b.opts.Clone(), nil
}

// MustBuild 构建配置选项，出错时panic（用于示例和测试）
func (b *SessionBuilder) MustBuild() *Options {
	opts, err := b.Build()
	if err != nil {
		panic(fmt.Sprintf("SessionBuilder.MustBuild failed: %v", err))
	}
	return opts
}

// addError 添加构建错误
func (b *SessionBuilder) addError(msg string) {
	b.errors = append(b.errors, fmt.Errorf("%s", msg))
}

// Validate 快速验证当前配置
func (b *SessionBuilder) Validate() []error {
	errors := make([]error, len(b.errors))
	copy(errors, b.errors)

	// 添加基本验证错误
	if b.opts.Name == "" {
		errors = append(errors, fmt.Errorf("session name is required"))
	}
	if len(b.opts.Brokers) == 0 {
		errors = append(errors, fmt.Errorf("at least one broker is required"))
	}
	if b.opts.ClientID == "" {
		errors = append(errors, fmt.Errorf("client ID is required"))
	}

	return errors
}

// Reset 重置构建器（复用构建器）
func (b *SessionBuilder) Reset(name string) *SessionBuilder {
	b.opts = &Options{
		Name:         name,
		Brokers:      []string{"tcp://localhost:1883"},
		ClientID:     fmt.Sprintf("mqttx-%s-%d", name, randomClientID()),
		ConnectProps: DefaultOptions().ConnectProps,
		Performance:  DefaultOptions().Performance,
		Storage:      DefaultOptions().Storage,
	}
	b.errors = b.errors[:0]
	return b
}

// ForwarderBuilder 转发器配置构建器
type ForwarderBuilder struct {
	config ForwarderConfig
	errors []error
}

// NewForwarderBuilder 创建转发器构建器
func NewForwarderBuilder(name string) *ForwarderBuilder {
	return &ForwarderBuilder{
		config: ForwarderConfig{
			Name:       name,
			QoS:        1,
			BufferSize: 100,
			Enabled:    true,
		},
	}
}

// Source 设置源会话和主题
func (fb *ForwarderBuilder) Source(session string, topics ...string) *ForwarderBuilder {
	if session = strings.TrimSpace(session); session == "" {
		fb.addError("source session cannot be empty")
		return fb
	}
	if len(topics) == 0 {
		fb.addError("at least one source topic is required")
		return fb
	}

	validTopics := make([]string, 0, len(topics))
	for _, topic := range topics {
		if topic = strings.TrimSpace(topic); topic != "" {
			validTopics = append(validTopics, topic)
		}
	}

	if len(validTopics) == 0 {
		fb.addError("no valid source topics provided")
		return fb
	}

	fb.config.SourceSession = session
	fb.config.SourceTopics = validTopics
	return fb
}

// Target 设置目标会话
func (fb *ForwarderBuilder) Target(session string) *ForwarderBuilder {
	if session = strings.TrimSpace(session); session == "" {
		fb.addError("target session cannot be empty")
		return fb
	}
	fb.config.TargetSession = session
	return fb
}

// QoS 设置转发消息的QoS等级
func (fb *ForwarderBuilder) QoS(qos byte) *ForwarderBuilder {
	if qos > 2 {
		fb.addError("QoS must be 0, 1, or 2")
		return fb
	}
	fb.config.QoS = qos
	return fb
}

// BufferSize 设置消息缓冲区大小
func (fb *ForwarderBuilder) BufferSize(size int) *ForwarderBuilder {
	if size <= 0 {
		size = 100
	}
	fb.config.BufferSize = size
	return fb
}

// TopicMapping 设置主题映射
func (fb *ForwarderBuilder) TopicMapping(mappings map[string]string) *ForwarderBuilder {
	if mappings != nil && len(mappings) > 0 {
		fb.config.TopicMap = make(map[string]string)
		for src, dst := range mappings {
			if src = strings.TrimSpace(src); src != "" {
				if dst = strings.TrimSpace(dst); dst != "" {
					fb.config.TopicMap[src] = dst
				}
			}
		}
	}
	return fb
}

// MapTopic 添加单个主题映射
func (fb *ForwarderBuilder) MapTopic(srcTopic, dstTopic string) *ForwarderBuilder {
	if srcTopic = strings.TrimSpace(srcTopic); srcTopic == "" {
		fb.addError("source topic cannot be empty")
		return fb
	}
	if dstTopic = strings.TrimSpace(dstTopic); dstTopic == "" {
		fb.addError("destination topic cannot be empty")
		return fb
	}

	if fb.config.TopicMap == nil {
		fb.config.TopicMap = make(map[string]string)
	}
	fb.config.TopicMap[srcTopic] = dstTopic
	return fb
}

// Enable 启用转发器
func (fb *ForwarderBuilder) Enable() *ForwarderBuilder {
	fb.config.Enabled = true
	return fb
}

// Disable 禁用转发器
func (fb *ForwarderBuilder) Disable() *ForwarderBuilder {
	fb.config.Enabled = false
	return fb
}

// Build 构建转发器配置
func (fb *ForwarderBuilder) Build() (ForwarderConfig, error) {
	if len(fb.errors) > 0 {
		return ForwarderConfig{}, fmt.Errorf("forwarder configuration errors: %v", fb.errors)
	}

	// 基本验证
	if fb.config.Name == "" {
		return ForwarderConfig{}, fmt.Errorf("forwarder name is required")
	}
	if fb.config.SourceSession == "" {
		return ForwarderConfig{}, fmt.Errorf("source session is required")
	}
	if len(fb.config.SourceTopics) == 0 {
		return ForwarderConfig{}, fmt.Errorf("at least one source topic is required")
	}
	if fb.config.TargetSession == "" {
		return ForwarderConfig{}, fmt.Errorf("target session is required")
	}

	return fb.config, nil
}

// MustBuild 构建转发器配置，出错时panic
func (fb *ForwarderBuilder) MustBuild() ForwarderConfig {
	config, err := fb.Build()
	if err != nil {
		panic(fmt.Sprintf("ForwarderBuilder.MustBuild failed: %v", err))
	}
	return config
}

// addError 添加构建错误
func (fb *ForwarderBuilder) addError(msg string) {
	fb.errors = append(fb.errors, fmt.Errorf("%s", msg))
}
