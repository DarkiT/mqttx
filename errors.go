package mqttx

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// 基础错误定义
var (
	ErrSessionNotFound      = errors.New("session not found")
	ErrSessionExists        = errors.New("session already exists")
	ErrNotConnected         = errors.New("not connected")
	ErrSessionClosed        = errors.New("session closed")
	ErrInvalidOptions       = errors.New("invalid options")
	ErrInvalidBroker        = errors.New("invalid broker address")
	ErrInvalidClientID      = errors.New("invalid client ID")
	ErrInvalidTopic         = errors.New("invalid topic")
	ErrTimeout              = errors.New("operation timeout")
	ErrInvalidPayload       = errors.New("invalid payload")
	ErrSubscribeFailed      = errors.New("subscribe failed")
	ErrUnsubscribeFailed    = errors.New("unsubscribe failed")
	ErrPublishFailed        = errors.New("publish failed")
	ErrQoSNotSupported      = errors.New("QoS level not supported")
	ErrStorageFailure       = errors.New("storage operation failed")
	ErrInvalidConfig        = errors.New("invalid configuration")
	ErrConnectionRefused    = errors.New("connection refused")
	ErrAuthenticationFailed = errors.New("authentication failed")
	ErrForwarderFailed      = errors.New("forwarder operation failed")
)

// ErrorSeverity 错误严重程度
type ErrorSeverity string

const (
	SeverityInfo     ErrorSeverity = "info"     // 信息级别（可忽略）
	SeverityWarning  ErrorSeverity = "warning"  // 警告级别（需关注）
	SeverityError    ErrorSeverity = "error"    // 错误级别（需处理）
	SeverityCritical ErrorSeverity = "critical" // 严重级别（需立即处理）
)

// ErrorType 错误类型
type ErrorType string

const (
	TypeConnection     ErrorType = "connection"     // 连接相关错误
	TypeAuthentication ErrorType = "authentication" // 认证相关错误
	TypeSubscription   ErrorType = "subscription"   // 订阅相关错误
	TypePublish        ErrorType = "publish"        // 发布相关错误
	TypeConfiguration  ErrorType = "configuration"  // 配置相关错误
	TypeStorage        ErrorType = "storage"        // 存储相关错误
	TypeForwarder      ErrorType = "forwarder"      // 转发器相关错误
	TypeInternal       ErrorType = "internal"       // 内部错误
)

// MQTTXError 统一的错误结构
type MQTTXError struct {
	Type      ErrorType      `json:"type"`              // 错误类型
	Severity  ErrorSeverity  `json:"severity"`          // 错误严重程度
	Code      string         `json:"code,omitempty"`    // 错误代码
	Message   string         `json:"message"`           // 错误消息
	Session   string         `json:"session,omitempty"` // 相关会话
	Topic     string         `json:"topic,omitempty"`   // 相关主题
	Timestamp time.Time      `json:"timestamp"`         // 错误发生时间
	Context   map[string]any `json:"context,omitempty"` // 额外上下文信息
	Cause     error          `json:"-"`                 // 底层原因（不序列化）
}

// Error 实现error接口
func (e *MQTTXError) Error() string {
	var parts []string

	if e.Session != "" {
		parts = append(parts, fmt.Sprintf("session=%s", e.Session))
	}
	if e.Topic != "" {
		parts = append(parts, fmt.Sprintf("topic=%s", e.Topic))
	}
	if e.Code != "" {
		parts = append(parts, fmt.Sprintf("code=%s", e.Code))
	}

	contextStr := ""
	if len(parts) > 0 {
		contextStr = " (" + strings.Join(parts, ", ") + ")"
	}

	message := fmt.Sprintf("[%s:%s] %s%s", e.Type, e.Severity, e.Message, contextStr)

	if e.Cause != nil {
		message += ": " + e.Cause.Error()
	}

	return message
}

// Unwrap 返回底层错误
func (e *MQTTXError) Unwrap() error {
	return e.Cause
}

// Is 检查错误类型
func (e *MQTTXError) Is(target error) bool {
	if t, ok := target.(*MQTTXError); ok {
		return e.Type == t.Type && e.Code == t.Code
	}
	return errors.Is(e.Cause, target)
}

// WithContext 添加上下文信息
func (e *MQTTXError) WithContext(key string, value any) *MQTTXError {
	if e.Context == nil {
		e.Context = make(map[string]any)
	}
	e.Context[key] = value
	return e
}

// WithSession 设置相关会话
func (e *MQTTXError) WithSession(session string) *MQTTXError {
	e.Session = session
	return e
}

// WithTopic 设置相关主题
func (e *MQTTXError) WithTopic(topic string) *MQTTXError {
	e.Topic = topic
	return e
}

// IsCritical 判断是否为严重错误
func (e *MQTTXError) IsCritical() bool {
	return e.Severity == SeverityCritical
}

// IsRecoverable 判断是否为可恢复错误
func (e *MQTTXError) IsRecoverable() bool {
	return e.Severity == SeverityInfo || e.Severity == SeverityWarning
}

// NewError 创建新的错误
func NewError(errType ErrorType, severity ErrorSeverity, message string) *MQTTXError {
	return &MQTTXError{
		Type:      errType,
		Severity:  severity,
		Message:   message,
		Timestamp: time.Now(),
	}
}

// NewConnectionError 创建连接错误
func NewConnectionError(message string, cause error) *MQTTXError {
	return &MQTTXError{
		Type:      TypeConnection,
		Severity:  SeverityError,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
	}
}

// NewConfigError 创建配置错误
func NewConfigError(message string, cause error) *MQTTXError {
	return &MQTTXError{
		Type:      TypeConfiguration,
		Severity:  SeverityCritical,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
	}
}

// NewPublishError 创建发布错误
func NewPublishError(topic string, cause error) *MQTTXError {
	return &MQTTXError{
		Type:      TypePublish,
		Severity:  SeverityError,
		Message:   "failed to publish message",
		Topic:     topic,
		Cause:     cause,
		Timestamp: time.Now(),
	}
}

// NewSubscribeError 创建订阅错误
func NewSubscribeError(topic string, cause error) *MQTTXError {
	return &MQTTXError{
		Type:      TypeSubscription,
		Severity:  SeverityError,
		Message:   "failed to subscribe to topic",
		Topic:     topic,
		Cause:     cause,
		Timestamp: time.Now(),
	}
}

// NewForwarderError 创建转发器错误
func NewForwarderError(message string, cause error) *MQTTXError {
	return &MQTTXError{
		Type:      TypeForwarder,
		Severity:  SeverityError,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
	}
}

// SessionError 会话相关错误（保持向后兼容）
type SessionError struct {
	SessionName string
	Err         error
}

func (e *SessionError) Error() string {
	return fmt.Sprintf("session %s: %v", e.SessionName, e.Err)
}

func (e *SessionError) Unwrap() error {
	return e.Err
}

// newSessionError 创建新的会话错误
func newSessionError(name string, err error) *SessionError {
	return &SessionError{
		SessionName: name,
		Err:         err,
	}
}

// wrapError 用指定的格式封装错误
func wrapError(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf(format+": %w", append(args, err)...)
}

// ValidationErrors 配置验证错误集合
type ValidationErrors struct {
	Errors []error `json:"errors"`
}

func (ve *ValidationErrors) Error() string {
	if len(ve.Errors) == 0 {
		return "no validation errors"
	}
	if len(ve.Errors) == 1 {
		return fmt.Sprintf("validation error: %v", ve.Errors[0])
	}

	var messages []string
	for i, err := range ve.Errors {
		messages = append(messages, fmt.Sprintf("%d. %v", i+1, err))
	}
	return fmt.Sprintf("validation errors:\n%s", strings.Join(messages, "\n"))
}

func (ve *ValidationErrors) Add(err error) {
	if err != nil {
		ve.Errors = append(ve.Errors, err)
	}
}

func (ve *ValidationErrors) HasErrors() bool {
	return len(ve.Errors) > 0
}

// NewValidationErrors 创建验证错误集合
func NewValidationErrors() *ValidationErrors {
	return &ValidationErrors{
		Errors: make([]error, 0),
	}
}

// IsTemporary 判断是否为临时错误（可重试）
func IsTemporary(err error) bool {
	if mqttErr, ok := err.(*MQTTXError); ok {
		return mqttErr.Type == TypeConnection && mqttErr.Severity != SeverityCritical
	}

	// 检查底层错误是否实现了Temporary接口
	if temp, ok := err.(interface{ Temporary() bool }); ok {
		return temp.Temporary()
	}

	return false
}

// IsTimeout 判断是否为超时错误
func IsTimeout(err error) bool {
	if errors.Is(err, ErrTimeout) {
		return true
	}

	// 检查底层错误是否实现了Timeout接口
	if timeout, ok := err.(interface{ Timeout() bool }); ok {
		return timeout.Timeout()
	}

	return false
}

// GetErrorCode 获取错误代码
func GetErrorCode(err error) string {
	if mqttErr, ok := err.(*MQTTXError); ok {
		return mqttErr.Code
	}
	return ""
}

// GetErrorType 获取错误类型
func GetErrorType(err error) ErrorType {
	if mqttErr, ok := err.(*MQTTXError); ok {
		return mqttErr.Type
	}
	return TypeInternal
}

// GetErrorSeverity 获取错误严重程度
func GetErrorSeverity(err error) ErrorSeverity {
	if mqttErr, ok := err.(*MQTTXError); ok {
		return mqttErr.Severity
	}
	return SeverityError
}
