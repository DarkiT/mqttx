package mqtt

import (
	"sync"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// 会话状态常量
const (
	StateDisconnected uint32 = iota
	StateConnecting
	StateConnected
	StateReconnecting
	StateClosed
)

// 事件类型常量
const (
	EventSessionConnecting   = "session_connecting"    // 会话正在连接
	EventSessionConnected    = "session_connected"     // 会话已连接
	EventSessionDisconnected = "session_disconnected"  // 会话已断开
	EventSessionReconnecting = "session_reconnecting"  // 会话重连中
	EventSessionAdded        = "session_added"         // 会话已添加
	EventSessionRemoved      = "session_removed"       // 会话已移除
	EventSessionReady        = "session_ready"         // 会话准备就绪（已成功连接）
	EventStateChanged        = "session_state_changed" // 会话状态已改变
)

// LogLevel 日志级别
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Manager 管理多个MQTT会话
type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	logger   Logger
	events   *EventManager
	metrics  *Metrics
}

// Session 表示单个MQTT会话
type Session struct {
	name     string
	client   mqtt.Client
	status   uint32
	opts     *Options
	manager  *Manager
	handlers *HandlerRegistry
	metrics  *SessionMetrics
	store    SessionStore
	sequence uint64
	mu       sync.RWMutex
}

// Options MQTT会话配置选项
type Options struct {
	Name         string              // 会话名称
	Brokers      []string            // MQTT broker地址列表
	Username     string              // 用户名
	Password     string              // 密码
	ClientID     string              // 客户端ID
	TLS          *TLSConfig          // TLS配置
	ConnectProps *ConnectProps       // 连接属性
	Topics       []TopicConfig       // 预订阅的主题
	Performance  *PerformanceOptions // 性能相关配置
	StoragePath  string              // 状态存储路径，为空则使用内存存储
}

// TLSConfig TLS配置
type TLSConfig struct {
	CAFile     string // CA证书文件路径
	CertFile   string // 客户端证书文件路径
	KeyFile    string // 客户端密钥文件路径
	SkipVerify bool   // 是否跳过服务器证书验证
}

// ConnectProps 连接属性
type ConnectProps struct {
	KeepAlive            uint16
	CleanSession         bool
	AutoReconnect        bool
	ConnectTimeout       int64
	MaxReconnectInterval int64
	WriteTimeout         int64
	ResumeSubs           bool // 重连后是否恢复订阅
	PersistentSession    bool // 是否持久化会话
}

// PerformanceOptions 性能相关配置
type PerformanceOptions struct {
	WriteBufferSize    int           // 写缓冲区大小
	ReadBufferSize     int           // 读缓冲区大小
	MessageChanSize    uint          // 消息通道大小
	WriteTimeout       time.Duration // 写超时
	ReadTimeout        time.Duration // 读超时
	MaxMessageSize     int64         // 最大消息大小
	MaxPendingMessages int           // 最大待处理消息数
}

// TopicConfig 主题配置
type TopicConfig struct {
	Topic   string
	QoS     byte
	Handler MessageHandler
}

// HandlerRegistry 处理函数注册表
type HandlerRegistry struct {
	connectHandler     ConnectHandler
	connectLostHandler ConnectLostHandler
	messageHandlers    map[string]MessageHandler
	mu                 sync.RWMutex
}

// EventManager 事件管理器
type EventManager struct {
	handlers map[string][]EventHandler
	mu       sync.RWMutex
}

// Handler types
type (
	MessageHandler     func(topic string, payload []byte)
	ConnectHandler     func(session *Session)
	ConnectLostHandler func(session *Session, err error)
	EventHandler       func(event Event)
)

// Message MQTT消息包装
type Message struct {
	Topic     string
	Payload   []byte
	QoS       byte
	Retained  bool
	Duplicate bool
	MessageID uint16
	Timestamp time.Time
}

// Route 表示一个消息路由
type Route struct {
	topic    string
	session  string
	manager  *Manager
	messages chan *Message
	done     chan struct{}
	once     sync.Once
	qos      byte
	stats    *RouteStats
}

// RouteStats 路由统计信息
type RouteStats struct {
	MessagesReceived uint64
	MessagesDropped  uint64
	BytesReceived    uint64
	LastMessageTime  time.Time
	LastError        time.Time
	ErrorCount       uint64
}

// Metrics 全局指标收集
type Metrics struct {
	ActiveSessions int64
	TotalMessages  uint64
	TotalBytes     uint64
	ErrorCount     uint64
	ReconnectCount uint64
	startTime      time.Time
	LastUpdate     time.Time
	mu             sync.RWMutex
	rates          *RateCounter
	errorTypes     sync.Map
}

// RateCounter 速率计数器
type RateCounter struct {
	messageRate    *atomic.Value // string
	byteRate       *atomic.Value // string
	avgMessageRate *atomic.Value
}

// SessionMetrics 单个会话的指标
type SessionMetrics struct {
	MessagesSent     uint64
	MessagesReceived uint64
	BytesSent        uint64
	BytesReceived    uint64
	Errors           uint64
	Reconnects       uint64
	LastError        time.Time
	LastMessage      time.Time
	mu               sync.RWMutex
}

// SessionState 会话状态
type SessionState struct {
	Topics           []TopicSubscription `json:"topics"`            // 订阅的主题列表
	Messages         []*Message          `json:"messages"`          // 待处理的消息
	LastSequence     uint64              `json:"last_sequence"`     // 最后序列号
	LastConnected    time.Time           `json:"last_connected"`    // 最后连接时间
	LastDisconnected time.Time           `json:"last_disconnected"` // 最后断开时间
}

// TopicSubscription 主题订阅信息
type TopicSubscription struct {
	Topic string `json:"topic"` // 主题名称
	QoS   byte   `json:"qos"`   // QoS 级别
}

// SessionStore 会话存储接口
type SessionStore interface {
	SaveState(sessionName string, state *SessionState) error
	LoadState(sessionName string) (*SessionState, error)
	DeleteState(sessionName string) error
}

// Logger 日志接口
type Logger interface {
	Debug(msg string, args ...interface{})
	Debugf(format string, args ...interface{})
	Info(msg string, args ...interface{})
	Infof(format string, args ...interface{})
	Warn(msg string, args ...interface{})
	Warnf(format string, args ...interface{})
	Error(msg string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// Event 事件结构
type Event struct {
	Type      string
	Session   string
	Data      interface{}
	Timestamp time.Time
}

// 默认配置常量
const (
	DefaultKeepAlive            = 60
	DefaultConnectTimeout       = 30
	DefaultMaxReconnectInterval = 120
	DefaultWriteTimeout         = 30
	DefaultMessageChanSize      = 100
	DefaultMaxMessageSize       = 32 * 1024 // 32KB
	DefaultMaxPendingMessages   = 1000
)
