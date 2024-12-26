package mqttx

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"time"
)

// DefaultOptions 返回默认选项
func DefaultOptions() *Options {
	return &Options{
		ConnectProps: &ConnectProps{
			KeepAlive:            DefaultKeepAlive,
			CleanSession:         true,
			AutoReconnect:        true,
			ConnectTimeout:       DefaultConnectTimeout,
			MaxReconnectInterval: DefaultMaxReconnectInterval,
			WriteTimeout:         DefaultWriteTimeout,
			ResumeSubs:           true,
			PersistentSession:    false,
		},
		Performance: &PerformanceOptions{
			WriteBufferSize:    4096,
			ReadBufferSize:     4096,
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
		if o.ConnectProps.KeepAlive < 0 {
			return fmt.Errorf("invalid keepalive value: %d", o.ConnectProps.KeepAlive)
		}
		if o.ConnectProps.ConnectTimeout < 0 {
			return fmt.Errorf("invalid connect timeout: %d", o.ConnectProps.ConnectTimeout)
		}
		if o.ConnectProps.MaxReconnectInterval < 0 {
			return fmt.Errorf("invalid max reconnect interval: %d", o.ConnectProps.MaxReconnectInterval)
		}
		if o.ConnectProps.WriteTimeout < 0 {
			return fmt.Errorf("invalid write timeout: %d", o.ConnectProps.WriteTimeout)
		}
	}

	// 验证性能选项
	if o.Performance == nil {
		o.Performance = DefaultOptions().Performance
	} else {
		if o.Performance.WriteBufferSize < 0 {
			return fmt.Errorf("invalid write buffer size: %d", o.Performance.WriteBufferSize)
		}
		if o.Performance.ReadBufferSize < 0 {
			return fmt.Errorf("invalid read buffer size: %d", o.Performance.ReadBufferSize)
		}
		if o.Performance.MessageChanSize < 0 {
			return fmt.Errorf("invalid message channel size: %d", o.Performance.MessageChanSize)
		}
		if o.Performance.MaxMessageSize < 0 {
			return fmt.Errorf("invalid max message size: %d", o.Performance.MaxMessageSize)
		}
		if o.Performance.MaxPendingMessages < 0 {
			return fmt.Errorf("invalid max pending messages: %d", o.Performance.MaxPendingMessages)
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
func (o *Options) WithReconnect(autoReconnect bool, maxInterval int64) *Options {
	if o.ConnectProps == nil {
		o.ConnectProps = DefaultOptions().ConnectProps
	}
	o.ConnectProps.AutoReconnect = autoReconnect
	o.ConnectProps.MaxReconnectInterval = time.Duration(maxInterval) * time.Second
	return o
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
