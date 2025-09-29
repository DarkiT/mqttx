package mqttx

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"time"
)

// randomClientID 生成随机的客户端ID
func randomClientID() int64 {
	b := make([]byte, 8)
	rand.Read(b)
	val := int64(0)
	for i := 0; i < 8; i++ {
		val = (val << 8) | int64(b[i])
	}
	return val & 0x7FFFFFFFFFFFFFFF // 确保为正数
}

// DefaultOptions 返回默认选项
func DefaultOptions() *Options {
	return &Options{
		Name:     "default",
		Brokers:  []string{"tcp://localhost:1883"},
		Username: "",
		Password: "",
		ClientID: fmt.Sprintf("mqttx-%d", randomClientID()),
		Topics:   []TopicConfig{},
		Storage:  DefaultStorageOptions(),
		ConnectProps: &ConnectProps{
			KeepAlive:                DefaultKeepAlive,
			CleanSession:             true,
			AutoReconnect:            true,
			ConnectTimeout:           DefaultConnectTimeout,
			InitialReconnectInterval: DefaultInitialReconnectInterval,
			MaxReconnectInterval:     DefaultMaxReconnectInterval,
			BackoffFactor:            DefaultBackoffFactor,
			WriteTimeout:             DefaultWriteTimeout,
			ResumeSubs:               true,
			PersistentSession:        false,
		},
		Performance: &PerformanceOptions{
			WriteBufferSize:    DefaultWriteBufferSize,
			ReadBufferSize:     DefaultReadBufferSize,
			MessageChanSize:    DefaultMessageChanSize,
			MaxMessageSize:     DefaultMaxMessageSize,
			MaxPendingMessages: DefaultMaxPendingMessages,
			WriteTimeout:       DefaultWriteTimeout,
			ReadTimeout:        DefaultWriteTimeout,
		},
	}
}

// Validate 验证选项
func (o *Options) Validate() error {
	if o.Name == "" {
		return ErrInvalidOptions
	}
	if len(o.Brokers) == 0 {
		return ErrInvalidBroker
	}
	if o.ClientID == "" {
		return ErrInvalidClientID
	}

	// 验证连接属性
	if o.ConnectProps == nil {
		o.ConnectProps = DefaultOptions().ConnectProps
	} else {
		if o.ConnectProps.KeepAlive <= 0 {
			o.ConnectProps.KeepAlive = DefaultKeepAlive
		}
		if o.ConnectProps.ConnectTimeout <= 0 {
			o.ConnectProps.ConnectTimeout = DefaultConnectTimeout
		}
		if o.ConnectProps.InitialReconnectInterval <= 0 {
			o.ConnectProps.InitialReconnectInterval = DefaultInitialReconnectInterval
		}
		if o.ConnectProps.MaxReconnectInterval <= 0 {
			o.ConnectProps.MaxReconnectInterval = DefaultMaxReconnectInterval
		}
		if o.ConnectProps.BackoffFactor <= 0 {
			o.ConnectProps.BackoffFactor = DefaultBackoffFactor
		}
		if o.ConnectProps.WriteTimeout <= 0 {
			o.ConnectProps.WriteTimeout = DefaultWriteTimeout
		}
	}

	// 验证性能选项
	if o.Performance == nil {
		o.Performance = DefaultOptions().Performance
	} else {
		if o.Performance.WriteBufferSize <= 0 {
			o.Performance.WriteBufferSize = DefaultWriteBufferSize
		}
		if o.Performance.ReadBufferSize <= 0 {
			o.Performance.ReadBufferSize = DefaultReadBufferSize
		}
		if o.Performance.MessageChanSize <= 0 {
			o.Performance.MessageChanSize = DefaultMessageChanSize
		}
		if o.Performance.MaxMessageSize <= 0 {
			o.Performance.MaxMessageSize = DefaultMaxMessageSize
		}
		if o.Performance.MaxPendingMessages <= 0 {
			o.Performance.MaxPendingMessages = DefaultMaxPendingMessages
		}
	}

	// 验证预订阅主题
	for _, topic := range o.Topics {
		if topic.Topic == "" {
			return ErrInvalidTopic
		}
		if topic.QoS > 2 {
			return ErrQoSNotSupported
		}
	}

	// 验证存储选项
	if o.Storage == nil {
		o.Storage = DefaultStorageOptions()
	} else {
		// 验证存储类型
		switch o.Storage.Type {
		case StoreTypeFile:
			if o.Storage.Path == "" {
				return errors.New("storage path cannot be empty for file store")
			}
		case StoreTypeRedis:
			if o.Storage.Redis == nil {
				o.Storage.Redis = DefaultRedisOptions()
			} else {
				if o.Storage.Redis.Addr == "" {
					return errors.New("redis address cannot be empty")
				}
				if o.Storage.Redis.TTL <= 0 {
					o.Storage.Redis.TTL = 86400 // 默认24小时
				}
			}
		case StoreTypeMemory:
			// 内存存储不需要额外验证
		default:
			o.Storage.Type = StoreTypeMemory // 默认使用内存存储
		}
	}

	return nil
}

// ConfigureTLS 配置TLS
func (o *Options) ConfigureTLS() (*tls.Config, error) {
	if o.TLS == nil {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: o.TLS.SkipVerify,
		MinVersion:         tls.VersionTLS12,
	}

	// 加载CA证书
	if o.TLS.CAFile != "" {
		caCert, err := os.ReadFile(o.TLS.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
			return nil, errors.New("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	// 加载客户端证书
	if o.TLS.CertFile != "" && o.TLS.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(o.TLS.CertFile, o.TLS.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificates: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

// Clone 克隆选项
func (o *Options) Clone() *Options {
	clone := *o

	// 克隆连接属性
	if o.ConnectProps != nil {
		props := *o.ConnectProps
		clone.ConnectProps = &props
	}

	// 克隆TLS配置
	if o.TLS != nil {
		_tls := *o.TLS
		clone.TLS = &_tls
	}

	// 克隆性能选项
	if o.Performance != nil {
		perf := *o.Performance
		clone.Performance = &perf
	}

	// 克隆预订阅主题
	if o.Topics != nil {
		topics := make([]TopicConfig, len(o.Topics))
		copy(topics, o.Topics)
		clone.Topics = topics
	}

	// 克隆broker地址列表
	if o.Brokers != nil {
		brokers := make([]string, len(o.Brokers))
		copy(brokers, o.Brokers)
		clone.Brokers = brokers
	}

	return &clone
}

// WithTLS 设置TLS配置
func (o *Options) WithTLS(caFile, certFile, keyFile string, skipVerify bool) *Options {
	o.TLS = &TLSConfig{
		CAFile:     caFile,
		CertFile:   certFile,
		KeyFile:    keyFile,
		SkipVerify: skipVerify,
	}
	return o
}

// WithPerformance 设置性能选项
func (o *Options) WithPerformance(writeBuffer, readBuffer, chanSize int) *Options {
	if o.Performance == nil {
		o.Performance = DefaultOptions().Performance
	}
	o.Performance.WriteBufferSize = writeBuffer
	o.Performance.ReadBufferSize = readBuffer
	o.Performance.MessageChanSize = uint(chanSize)
	return o
}

// WithPersistence 设置持久化选项
func (o *Options) WithPersistence(enabled bool) *Options {
	if o.ConnectProps == nil {
		o.ConnectProps = DefaultOptions().ConnectProps
	}
	o.ConnectProps.PersistentSession = enabled
	return o
}

// WithReconnect 设置重连选项
func (o *Options) WithReconnect(autoReconnect bool, initialInterval, maxInterval int64, backoffFactor float64) *Options {
	if o.ConnectProps == nil {
		o.ConnectProps = DefaultOptions().ConnectProps
	}
	o.ConnectProps.AutoReconnect = autoReconnect

	if initialInterval > 0 {
		o.ConnectProps.InitialReconnectInterval = time.Duration(initialInterval) * time.Second
	}

	if maxInterval > 0 {
		o.ConnectProps.MaxReconnectInterval = time.Duration(maxInterval) * time.Second
	}

	if backoffFactor > 0 {
		o.ConnectProps.BackoffFactor = backoffFactor
	}

	return o
}

// WithSimpleReconnect 设置简化版重连选项
func (o *Options) WithSimpleReconnect(autoReconnect bool, maxInterval int64) *Options {
	return o.WithReconnect(autoReconnect, 0, maxInterval, 0)
}

// WithTimeout 设置超时选项
func (o *Options) WithTimeout(connect, write int64) *Options {
	if o.ConnectProps == nil {
		o.ConnectProps = DefaultOptions().ConnectProps
	}
	o.ConnectProps.ConnectTimeout = time.Duration(connect) * time.Second
	o.ConnectProps.WriteTimeout = time.Duration(write) * time.Second
	return o
}

// WithCleanSession 设置清理会话选项
func (o *Options) WithCleanSession(clean bool) *Options {
	if o.ConnectProps == nil {
		o.ConnectProps = DefaultOptions().ConnectProps
	}
	o.ConnectProps.CleanSession = clean
	return o
}

// WithAuth 设置认证信息
func (o *Options) WithAuth(username, password string) *Options {
	o.Username = username
	o.Password = password
	return o
}

// StoreType 存储类型
type StoreType string

const (
	// StoreTypeMemory 内存存储
	StoreTypeMemory StoreType = "memory"
	// StoreTypeFile 文件存储
	StoreTypeFile StoreType = "file"
	// StoreTypeRedis Redis存储
	StoreTypeRedis StoreType = "redis"
)

// StorageOptions 存储选项
type StorageOptions struct {
	// Type 存储类型: "memory", "file", 或 "redis"
	Type StoreType
	// Path 文件存储路径，仅在Type="file"时有效
	Path string
	// Redis Redis存储选项，仅在Type="redis"时有效
	Redis *RedisOptions
}

// RedisOptions Redis配置选项
type RedisOptions struct {
	// Addr Redis服务器地址，格式为 "host:port"
	Addr string
	// Username Redis用户名
	Username string
	// Password Redis密码
	Password string
	// DB Redis数据库索引
	DB int
	// KeyPrefix 键前缀
	KeyPrefix string
	// TTL 会话状态的生存时间
	TTL int64 // 秒
	// PoolSize 连接池大小
	PoolSize int
}

// DefaultRedisOptions 返回默认的Redis选项
func DefaultRedisOptions() *RedisOptions {
	return &RedisOptions{
		Addr:      "localhost:6379",
		Username:  "",
		Password:  "",
		DB:        0,
		KeyPrefix: "mqttx:session:",
		TTL:       86400, // 24小时
		PoolSize:  10,
	}
}

// DefaultStorageOptions 返回默认的存储选项
func DefaultStorageOptions() *StorageOptions {
	return &StorageOptions{
		Type:  StoreTypeMemory,
		Path:  "",
		Redis: DefaultRedisOptions(),
	}
}

// WithStorage 设置存储选项
func (o *Options) WithStorage(storeType StoreType, path string) *Options {
	if o.Storage == nil {
		o.Storage = DefaultStorageOptions()
	}
	o.Storage.Type = storeType
	o.Storage.Path = path
	return o
}

// WithRedisStorage 设置Redis存储选项
func (o *Options) WithRedisStorage(addr, username, password string, db int, keyPrefix string, ttl int64) *Options {
	if o.Storage == nil {
		o.Storage = DefaultStorageOptions()
	}
	o.Storage.Type = StoreTypeRedis

	if o.Storage.Redis == nil {
		o.Storage.Redis = DefaultRedisOptions()
	}

	o.Storage.Redis.Addr = addr
	o.Storage.Redis.Username = username
	o.Storage.Redis.Password = password
	o.Storage.Redis.DB = db

	if keyPrefix != "" {
		o.Storage.Redis.KeyPrefix = keyPrefix
	}

	if ttl > 0 {
		o.Storage.Redis.TTL = ttl
	}

	return o
}
