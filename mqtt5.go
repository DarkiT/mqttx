package mqttx

import (
	"errors"
	"fmt"
	"time"
)

// MQTT协议版本常量
const (
	MQTT311 = 4 // MQTT 3.1.1
	MQTT50  = 5 // MQTT 5.0
)

// MQTT 5.0 属性标识符常量
const (
	PropPayloadFormatIndicator      = 1  // 载荷格式指示器
	PropMessageExpiryInterval       = 2  // 消息过期间隔
	PropContentType                 = 3  // 内容类型
	PropResponseTopic               = 8  // 响应主题
	PropCorrelationData             = 9  // 关联数据
	PropSubscriptionIdentifier      = 11 // 订阅标识符
	PropSessionExpiryInterval       = 17 // 会话过期间隔
	PropAssignedClientIdentifier    = 18 // 分配的客户端标识符
	PropServerKeepAlive             = 19 // 服务端保活时间
	PropAuthenticationMethod        = 21 // 认证方法
	PropAuthenticationData          = 22 // 认证数据
	PropRequestProblemInformation   = 23 // 请求问题信息
	PropWillDelayInterval           = 24 // 遗嘱延迟间隔
	PropRequestResponseInformation  = 25 // 请求响应信息
	PropResponseInformation         = 26 // 响应信息
	PropServerReference             = 28 // 服务端引用
	PropReasonString                = 31 // 原因字符串
	PropReceiveMaximum              = 33 // 接收最大值
	PropTopicAliasMaximum           = 34 // 主题别名最大值
	PropTopicAlias                  = 35 // 主题别名
	PropMaximumQoS                  = 36 // 最大QoS
	PropRetainAvailable             = 37 // 保留可用
	PropUserProperty                = 38 // 用户属性
	PropMaximumPacketSize           = 39 // 最大报文大小
	PropWildcardSubscriptionAvail   = 40 // 通配符订阅可用
	PropSubscriptionIdentifierAvail = 41 // 订阅标识符可用
	PropSharedSubscriptionAvail     = 42 // 共享订阅可用
)

// MQTT 5.0 原因码常量
const (
	ReasonSuccess                             = 0   // 成功
	ReasonNormalDisconnection                 = 0   // 正常断开连接
	ReasonGrantedQoS0                         = 0   // 授予QoS 0
	ReasonGrantedQoS1                         = 1   // 授予QoS 1
	ReasonGrantedQoS2                         = 2   // 授予QoS 2
	ReasonDisconnectWithWill                  = 4   // 带遗嘱消息断开连接
	ReasonNoMatchingSubscribers               = 16  // 没有匹配的订阅者
	ReasonNoSubscriptionExisted               = 17  // 没有存在的订阅
	ReasonContinueAuthentication              = 24  // 继续认证
	ReasonReAuthenticate                      = 25  // 重新认证
	ReasonUnspecifiedError                    = 128 // 未指定错误
	ReasonMalformedPacket                     = 129 // 报文格式错误
	ReasonProtocolError                       = 130 // 协议错误
	ReasonImplementationSpecificError         = 131 // 实现特定错误
	ReasonUnsupportedProtocolVersion          = 132 // 不支持的协议版本
	ReasonClientIdentifierNotValid            = 133 // 客户端标识符无效
	ReasonBadUsernameOrPassword               = 134 // 用户名或密码错误
	ReasonNotAuthorized                       = 135 // 未授权
	ReasonServerUnavailable                   = 136 // 服务端不可用
	ReasonServerBusy                          = 137 // 服务端繁忙
	ReasonBanned                              = 138 // 禁止
	ReasonServerShuttingDown                  = 139 // 服务端关闭
	ReasonBadAuthenticationMethod             = 140 // 认证方法错误
	ReasonKeepAliveTimeout                    = 141 // 保活超时
	ReasonSessionTakenOver                    = 142 // 会话被接管
	ReasonTopicFilterInvalid                  = 143 // 主题过滤器无效
	ReasonTopicNameInvalid                    = 144 // 主题名无效
	ReasonPacketIdentifierInUse               = 145 // 报文标识符已使用
	ReasonPacketIdentifierNotFound            = 146 // 报文标识符未找到
	ReasonReceiveMaximumExceeded              = 147 // 超出接收最大值
	ReasonTopicAliasInvalid                   = 148 // 主题别名无效
	ReasonPacketTooLarge                      = 149 // 报文过大
	ReasonMessageRateTooHigh                  = 150 // 消息速率过高
	ReasonQuotaExceeded                       = 151 // 超出配额
	ReasonAdministrativeAction                = 152 // 管理操作
	ReasonPayloadFormatInvalid                = 153 // 载荷格式无效
	ReasonRetainNotSupported                  = 154 // 不支持保留
	ReasonQoSNotSupported                     = 155 // 不支持QoS
	ReasonUseAnotherServer                    = 156 // 使用另一个服务端
	ReasonServerMoved                         = 157 // 服务端已移动
	ReasonSharedSubscriptionsNotSupported     = 158 // 不支持共享订阅
	ReasonConnectionRateExceeded              = 159 // 超出连接速率
	ReasonMaximumConnectTime                  = 160 // 最大连接时间
	ReasonSubscriptionIdentifiersNotSupported = 161 // 不支持订阅标识符
	ReasonWildcardSubscriptionsNotSupported   = 162 // 不支持通配符订阅
)

// MQTT5Config MQTT 5.0配置
type MQTT5Config struct {
	// 基本配置
	ProtocolVersion    byte          // 协议版本，默认为MQTT50
	SessionExpiry      time.Duration // 会话过期间隔
	ReceiveMaximum     uint16        // 接收最大值
	MaximumPacketSize  uint32        // 最大报文大小
	TopicAliasMaximum  uint16        // 主题别名最大值
	RequestRespInfo    bool          // 请求响应信息
	RequestProblemInfo bool          // 请求问题信息

	// 认证配置
	AuthMethod   string // 认证方法
	AuthData     []byte // 认证数据
	EnhancedAuth bool   // 是否启用增强认证

	// 共享订阅配置
	SharedSubAvailable bool // 是否支持共享订阅

	// 用户属性
	UserProperties map[string]string // 用户属性
}

// DefaultMQTT5Config 返回默认MQTT 5.0配置
func DefaultMQTT5Config() *MQTT5Config {
	return &MQTT5Config{
		ProtocolVersion:    MQTT50,
		SessionExpiry:      24 * time.Hour,
		ReceiveMaximum:     65535,
		MaximumPacketSize:  268435455, // 256 MB
		TopicAliasMaximum:  65535,
		RequestRespInfo:    true,
		RequestProblemInfo: true,
		EnhancedAuth:       false,
		SharedSubAvailable: true,
		UserProperties:     make(map[string]string),
	}
}

// MQTT5Auth MQTT 5.0增强认证
type MQTT5Auth struct {
	Method     string            // 认证方法
	Data       []byte            // 认证数据
	Properties map[string]string // 认证属性
}

// NewMQTT5Auth 创建MQTT 5.0增强认证
func NewMQTT5Auth(method string, data []byte) *MQTT5Auth {
	return &MQTT5Auth{
		Method:     method,
		Data:       data,
		Properties: make(map[string]string),
	}
}

// WithProperty 添加认证属性
func (a *MQTT5Auth) WithProperty(key, value string) *MQTT5Auth {
	a.Properties[key] = value
	return a
}

// MQTT5Options MQTT 5.0选项扩展
type MQTT5Options struct {
	// 基本配置
	MQTT5Config *MQTT5Config

	// 安全配置
	TLS         *TLSConfig         // 基本TLS配置
	EnhancedTLS *EnhancedTLSConfig // 增强TLS配置

	// 增强认证
	Auth *MQTT5Auth // 增强认证配置

	// 共享订阅
	SharedSubPrefix string // 共享订阅前缀，默认为"$share"
}

// WithMQTT5 使用MQTT 5.0配置
func (o *Options) WithMQTT5(config *MQTT5Config) *Options {
	// 创建MQTT 5.0选项
	mqtt5Opts := &MQTT5Options{
		MQTT5Config:     config,
		TLS:             o.TLS,
		EnhancedTLS:     o.EnhancedTLS,
		SharedSubPrefix: "$share",
	}

	// 将MQTT 5.0选项存储在用户属性中
	if o.ConnectProps == nil {
		o.ConnectProps = DefaultOptions().ConnectProps
	}

	// 更新协议版本
	o.ProtocolVersion = MQTT50

	// 存储MQTT 5.0选项
	o.MQTT5 = mqtt5Opts

	return o
}

// WithEnhancedAuth 配置增强认证
func (o *Options) WithEnhancedAuth(auth *MQTT5Auth) *Options {
	if o.MQTT5 == nil {
		o.WithMQTT5(DefaultMQTT5Config())
	}
	o.MQTT5.Auth = auth
	return o
}

// WithSharedSubscription 配置共享订阅
func (o *Options) WithSharedSubscription(enable bool, prefix string) *Options {
	if o.MQTT5 == nil {
		o.WithMQTT5(DefaultMQTT5Config())
	}
	o.MQTT5.MQTT5Config.SharedSubAvailable = enable
	if prefix != "" {
		o.MQTT5.SharedSubPrefix = prefix
	}
	return o
}

// WithUserProperty 添加用户属性
func (o *Options) WithUserProperty(key, value string) *Options {
	if o.MQTT5 == nil {
		o.WithMQTT5(DefaultMQTT5Config())
	}
	o.MQTT5.MQTT5Config.UserProperties[key] = value
	return o
}

// WithSessionExpiry 设置会话过期间隔
func (o *Options) WithSessionExpiry(expiry time.Duration) *Options {
	if o.MQTT5 == nil {
		o.WithMQTT5(DefaultMQTT5Config())
	}
	o.MQTT5.MQTT5Config.SessionExpiry = expiry
	return o
}

// SubscribeShared 订阅共享主题
func (s *Session) SubscribeShared(group, topic string, handler MessageHandler, qos byte) error {
	if s.opts.MQTT5 == nil || !s.opts.MQTT5.MQTT5Config.SharedSubAvailable {
		return errors.New("shared subscription not available")
	}

	// 构造共享订阅主题
	sharedTopic := fmt.Sprintf("%s/%s/%s", s.opts.MQTT5.SharedSubPrefix, group, topic)
	return s.Subscribe(sharedTopic, handler, qos)
}

// UnsubscribeShared 取消订阅共享主题
func (s *Session) UnsubscribeShared(group, topic string) error {
	if s.opts.MQTT5 == nil || !s.opts.MQTT5.MQTT5Config.SharedSubAvailable {
		return errors.New("shared subscription not available")
	}

	// 构造共享订阅主题
	sharedTopic := fmt.Sprintf("%s/%s/%s", s.opts.MQTT5.SharedSubPrefix, group, topic)
	return s.Unsubscribe(sharedTopic)
}

// 更新Options结构体，添加MQTT 5.0支持
func init() {
	// 确保Options结构体中有MQTT5字段
	// 这是通过在types.go中更新Options结构体实现的
}
