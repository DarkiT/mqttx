package mqttx

import (
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// 默认配置常量
const (
	DefaultKeepAlive                = 60                // 默认保活时间(秒)
	DefaultConnectTimeout           = 30 * time.Second  // 默认连接超时时间
	DefaultInitialReconnectInterval = 2 * time.Second   // 默认初始重连间隔
	DefaultMaxReconnectInterval     = 120 * time.Second // 默认最大重连间隔
	DefaultBackoffFactor            = 1.5               // 默认退避因子
	DefaultWriteTimeout             = 30 * time.Second  // 默认写超时时间
	DefaultMessageChanSize          = 100               // 默认消息通道大小
	DefaultMaxMessageSize           = 32 * 1024         // 默认最大消息大小(32KB)
	DefaultMaxPendingMessages       = 1000              // 默认最大待处理消息数
	DefaultWriteBufferSize          = 4096              // 默认写缓冲区大小
	DefaultReadBufferSize           = 4096              // 默认读缓冲区大小
)

// 内部会话状态常量
const (
	stateDisconnected uint32 = iota // 已断开连接
	stateConnecting                 // 正在连接
	stateConnected                  // 已连接
	stateReconnecting               // 正在重连
	stateClosed                     // 已关闭
)

// 会话状态常量 - 供外部代码使用
const (
	StatusDisconnected uint32 = 0 // 已断开连接
	StatusConnecting   uint32 = 1 // 正在连接
	StatusConnected    uint32 = 2 // 已连接
	StatusReconnecting uint32 = 3 // 正在重连
	StatusClosed       uint32 = 4 // 已关闭
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

// Manager 管理多个MQTT会话
type Manager struct {
	sessions map[string]*Session // 会话映射表，按名称索引
	mu       sync.RWMutex        // 全局锁，用于管理会话表
	logger   Logger              // 日志记录器
	events   *EventManager       // 事件管理器
	metrics  *Metrics            // 指标收集器
	recovery *RecoveryManager    // 错误恢复管理器

	// 以下字段仅用于测试
	getSessionFunc func(name string) (*Session, error)                             // 用于测试的获取会话函数
	publishToFunc  func(sessionName, topic string, payload []byte, qos byte) error // 用于测试的发布函数
}

// Session 表示单个MQTT会话
type Session struct {
	name           string           // 会话名称
	client         mqtt.Client      // MQTT客户端实例
	status         uint32           // 会话状态(连接、断开等)
	opts           *Options         // 会话配置选项
	manager        *Manager         // 所属的管理器
	handlers       *HandlerRegistry // 处理函数注册表
	metrics        *SessionMetrics  // 会话指标统计
	store          SessionStore     // 会话状态存储
	sequence       uint64           // 消息序列号
	reconnectCount uint32           // 重连尝试次数
	reconnectMu    sync.Mutex       // 重连锁
	nextReconnect  time.Duration    // 下次重连间隔
}

// Options MQTT会话配置选项
type Options struct {
	Name            string              // 会话名称
	Brokers         []string            // MQTT broker地址列表
	Username        string              // 用户名
	Password        string              // 密码
	ClientID        string              // 客户端ID
	TLS             *TLSConfig          // TLS配置
	EnhancedTLS     *EnhancedTLSConfig  // 增强的TLS配置
	ConnectProps    *ConnectProps       // 连接属性
	Topics          []TopicConfig       // 预订阅的主题
	Performance     *PerformanceOptions // 性能相关配置
	Storage         *StorageOptions     // 存储选项
	ProtocolVersion byte                // 协议版本: MQTT311 (4) 或 MQTT50 (5)
	MQTT5           *MQTT5Options       // MQTT 5.0 特定选项
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
	KeepAlive                int           // 保活时间(秒)
	CleanSession             bool          // 是否清理会话
	AutoReconnect            bool          // 是否自动重连
	ConnectTimeout           time.Duration // 连接超时时间
	InitialReconnectInterval time.Duration // 初始重连间隔
	MaxReconnectInterval     time.Duration // 最大重连间隔
	BackoffFactor            float64       // 退避因子
	WriteTimeout             time.Duration // 写超时时间
	ResumeSubs               bool          // 是否恢复订阅
	PersistentSession        bool          // 是否持久化会话
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
	Topic   string         // 主题名称
	QoS     byte           // 服务质量等级
	Handler MessageHandler // 消息处理函数
}

// HandlerRegistry 处理函数注册表
type HandlerRegistry struct {
	connectHandler     ConnectHandler            // 连接成功处理函数
	connectLostHandler ConnectLostHandler        // 连接断开处理函数
	messageHandlers    map[string]MessageHandler // 消息处理函数映射，按主题索引
	mu                 sync.RWMutex              // 读写锁，保护处理函数映射
}

// EventManager 事件管理器
type EventManager struct {
	handlers map[string][]EventHandler // 事件处理函数映射，按事件类型索引
	mu       sync.RWMutex              // 读写锁，保护事件处理函数映射
}

// Event 事件结构
type Event struct {
	Type      string      // 事件类型
	Session   string      // 相关会话名称
	Data      interface{} // 事件数据
	Timestamp time.Time   // 事件发生时间
}

// EventHandler 事件处理函数类型
type EventHandler func(event *Event)

// ConnectHandler 连接处理函数类型
type ConnectHandler func()

// ConnectLostHandler 连接断开处理函数类型
type ConnectLostHandler func(err error)

// MessageHandler 消息处理函数类型
type MessageHandler func(topic string, payload []byte)

// Message 消息结构
type Message struct {
	Topic      string                 `json:"topic"`                // 主题名称
	Payload    []byte                 `json:"payload"`              // 消息内容
	QoS        byte                   `json:"qos"`                  // 服务质量等级
	Retained   bool                   `json:"retained"`             // 是否为保留消息
	Duplicate  bool                   `json:"duplicate"`            // 是否为重复消息
	MessageID  uint16                 `json:"message_id"`           // 消息ID
	Timestamp  time.Time              `json:"timestamp"`            // 消息时间戳
	Properties map[string]interface{} `json:"properties,omitempty"` // MQTT 5.0属性
}

// Route 表示MQTT主题的路由
type Route struct {
	topic    string        // 主题名称
	session  string        // 会话名称
	manager  *Manager      // 管理器引用
	messages chan *Message // 消息通道
	done     chan struct{} // 关闭信号通道
	qos      byte          // 服务质量等级
	stats    *RouteStats   // 路由统计信息
	once     sync.Once     // 确保只关闭一次
}

// RouteStats 路由统计信息
type RouteStats struct {
	MessagesReceived uint64    // 接收的消息数量
	MessagesDropped  uint64    // 丢弃的消息数量
	BytesReceived    uint64    // 接收的字节数
	LastMessageTime  time.Time // 最后消息时间
	LastError        time.Time // 最后错误时间
	ErrorCount       uint64    // 错误计数
}

// Metrics 指标收集器
type Metrics struct {
	TotalMessages   uint64 // 总消息数
	TotalBytes      uint64 // 总字节数
	ErrorCount      uint64 // 错误计数
	ReconnectCount  uint64 // 重连次数
	ActiveSessions  int64  // 活跃会话数
	LastUpdate      int64  // 最后更新时间(Unix纳秒时间戳，原子操作安全)
	LastMessageTime int64  // 最后消息时间(Unix纳秒时间戳，原子操作安全)
	LastErrorTime   int64  // 最后错误时间(Unix纳秒时间戳，原子操作安全)
}

// SessionMetrics 会话指标
type SessionMetrics struct {
	MessagesSent     uint64 // 发送的消息数
	MessagesReceived uint64 // 接收的消息数
	BytesSent        uint64 // 发送的字节数
	BytesReceived    uint64 // 接收的字节数
	Errors           uint64 // 错误计数
	Reconnects       uint64 // 重连次数
	LastMessage      int64  // 最后消息时间(Unix纳秒时间戳，原子操作安全)
	LastError        int64  // 最后错误时间(Unix纳秒时间戳，原子操作安全)
}

// SessionState 会话状态
type SessionState struct {
	Topics           []TopicSubscription `json:"topics"`            // 订阅的主题列表
	Messages         []*Message          `json:"messages"`          // 待处理的消息
	LastSequence     uint64              `json:"last_sequence"`     // 最后序列号
	LastConnected    time.Time           `json:"last_connected"`    // 最后连接时间
	LastDisconnected time.Time           `json:"last_disconnected"` // 最后断开时间
	QoSMessages      map[uint16]*Message `json:"qos_messages"`      // QoS > 0 的消息，按MessageID索引
	RetainedMessages map[string]*Message `json:"retained_messages"` // 保留消息，按主题索引
	ClientID         string              `json:"client_id"`         // 客户端ID
	SessionExpiry    time.Time           `json:"session_expiry"`    // 会话过期时间
	Version          int                 `json:"version"`           // 状态版本，用于兼容性
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

// StatusToString 将状态常量转换为状态字符串
func StatusToString(status uint32) string {
	switch status {
	case StatusDisconnected:
		return "disconnected"
	case StatusConnecting:
		return "connecting"
	case StatusConnected:
		return "connected"
	case StatusReconnecting:
		return "reconnecting"
	case StatusClosed:
		return "closed"
	default:
		return "unknown"
	}
}
