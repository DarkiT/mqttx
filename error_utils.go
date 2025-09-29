package mqttx

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// ErrorHandler 错误处理器接口
type ErrorHandler interface {
	HandleError(ctx context.Context, err error) bool // 返回true表示错误已处理，false表示需要继续传播
}

// DefaultErrorHandler 默认错误处理器
type DefaultErrorHandler struct {
	logger *slog.Logger
}

// NewDefaultErrorHandler 创建默认错误处理器
func NewDefaultErrorHandler(logger *slog.Logger) *DefaultErrorHandler {
	return &DefaultErrorHandler{
		logger: logger,
	}
}

// HandleError 处理错误
func (h *DefaultErrorHandler) HandleError(ctx context.Context, err error) bool {
	if err == nil {
		return true
	}

	// 根据错误类型采取不同的处理策略
	if mqttErr, ok := err.(*MQTTXError); ok {
		return h.handleMQTTXError(ctx, mqttErr)
	}

	// 处理其他类型的错误
	h.logger.Error("Unhandled error", "error", err)
	return false
}

// handleMQTTXError 处理 MQTTXError 类型的错误
func (h *DefaultErrorHandler) handleMQTTXError(ctx context.Context, err *MQTTXError) bool {
	// 根据错误严重程度选择日志级别
	switch err.Severity {
	case SeverityInfo:
		h.logger.Info("MQTT info",
			"type", err.Type,
			"message", err.Message,
			"session", err.Session,
			"topic", err.Topic)
		return true

	case SeverityWarning:
		h.logger.Warn("MQTT warning",
			"type", err.Type,
			"message", err.Message,
			"session", err.Session,
			"topic", err.Topic,
			"context", err.Context)
		// 警告级别错误通常可以继续处理
		return !err.IsCritical()

	case SeverityError:
		h.logger.Error("MQTT error",
			"type", err.Type,
			"message", err.Message,
			"session", err.Session,
			"topic", err.Topic,
			"context", err.Context,
			"cause", err.Cause)
		// 一般错误需要上报但不一定停止处理
		return IsTemporary(err)

	case SeverityCritical:
		h.logger.Error("MQTT critical error",
			"type", err.Type,
			"message", err.Message,
			"session", err.Session,
			"topic", err.Topic,
			"context", err.Context,
			"cause", err.Cause)
		// 严重错误需要立即处理
		return false

	default:
		h.logger.Error("Unknown severity MQTT error", "error", err)
		return false
	}
}

// ErrorReporter 错误报告接口
type ErrorReporter interface {
	ReportError(err error)
	GetErrorStats() map[string]interface{}
}

// InMemoryErrorReporter 内存错误报告器
type InMemoryErrorReporter struct {
	errors []error
	stats  map[string]int
}

// NewInMemoryErrorReporter 创建内存错误报告器
func NewInMemoryErrorReporter() *InMemoryErrorReporter {
	return &InMemoryErrorReporter{
		errors: make([]error, 0),
		stats:  make(map[string]int),
	}
}

// ReportError 报告错误
func (r *InMemoryErrorReporter) ReportError(err error) {
	if err == nil {
		return
	}

	r.errors = append(r.errors, err)

	if mqttErr, ok := err.(*MQTTXError); ok {
		r.stats[fmt.Sprintf("type_%s", mqttErr.Type)]++
		r.stats[fmt.Sprintf("severity_%s", mqttErr.Severity)]++
	} else {
		r.stats["unknown_type"]++
	}
}

// GetErrorStats 获取错误统计
func (r *InMemoryErrorReporter) GetErrorStats() map[string]interface{} {
	stats := make(map[string]interface{})
	stats["total_errors"] = len(r.errors)

	for key, count := range r.stats {
		stats[key] = count
	}

	return stats
}

// GetRecentErrors 获取最近的错误
func (r *InMemoryErrorReporter) GetRecentErrors(limit int) []error {
	if limit <= 0 || len(r.errors) == 0 {
		return nil
	}

	start := 0
	if len(r.errors) > limit {
		start = len(r.errors) - limit
	}

	return r.errors[start:]
}

// WrapError 封装错误，保留原始错误信息
func WrapError(err error, errType ErrorType, message string) *MQTTXError {
	if err == nil {
		return nil
	}

	// 如果已经是 MQTTXError，增强它
	if mqttErr, ok := err.(*MQTTXError); ok {
		mqttErr.Message = message + ": " + mqttErr.Message
		return mqttErr
	}

	// 根据错误类型确定严重程度
	severity := SeverityError
	switch errType {
	case TypeConfiguration:
		severity = SeverityCritical
	case TypeConnection, TypeAuthentication:
		if IsTimeout(err) {
			severity = SeverityWarning
		} else {
			severity = SeverityError
		}
	case TypePublish, TypeSubscription:
		severity = SeverityWarning
	case TypeInternal:
		severity = SeverityCritical
	}

	return NewError(errType, severity, message).
		WithContext("original_error", err.Error())
}

// ChainErrors 链接多个错误
func ChainErrors(errors ...error) error {
	validationErrs := NewValidationErrors()

	for _, err := range errors {
		if err != nil {
			validationErrs.Add(err)
		}
	}

	if !validationErrs.HasErrors() {
		return nil
	}

	return validationErrs
}

// FilterErrors 过滤特定类型的错误
func FilterErrors(errors []error, errType ErrorType) []*MQTTXError {
	var filtered []*MQTTXError

	for _, err := range errors {
		if mqttErr, ok := err.(*MQTTXError); ok && mqttErr.Type == errType {
			filtered = append(filtered, mqttErr)
		}
	}

	return filtered
}

// ErrorMetrics 错误度量收集器
type ErrorMetrics struct {
	TotalErrors      uint64            `json:"total_errors"`
	ErrorsByType     map[string]uint64 `json:"errors_by_type"`
	ErrorsBySeverity map[string]uint64 `json:"errors_by_severity"`
	LastError        string            `json:"last_error"`
	LastErrorTime    int64             `json:"last_error_time"`
}

// NewErrorMetrics 创建错误度量收集器
func NewErrorMetrics() *ErrorMetrics {
	return &ErrorMetrics{
		ErrorsByType:     make(map[string]uint64),
		ErrorsBySeverity: make(map[string]uint64),
	}
}

// RecordError 记录错误
func (m *ErrorMetrics) RecordError(err error) {
	if err == nil {
		return
	}

	m.TotalErrors++
	m.LastError = err.Error()
	m.LastErrorTime = getCurrentTimestamp()

	if mqttErr, ok := err.(*MQTTXError); ok {
		m.ErrorsByType[string(mqttErr.Type)]++
		m.ErrorsBySeverity[string(mqttErr.Severity)]++
	} else {
		m.ErrorsByType["unknown"]++
		m.ErrorsBySeverity["unknown"]++
	}
}

// GetSnapshot 获取度量快照
func (m *ErrorMetrics) GetSnapshot() map[string]interface{} {
	snapshot := make(map[string]interface{})
	snapshot["total_errors"] = m.TotalErrors
	snapshot["errors_by_type"] = m.ErrorsByType
	snapshot["errors_by_severity"] = m.ErrorsBySeverity
	snapshot["last_error"] = m.LastError
	snapshot["last_error_time"] = m.LastErrorTime
	return snapshot
}

// Reset 重置度量
func (m *ErrorMetrics) Reset() {
	m.TotalErrors = 0
	m.ErrorsByType = make(map[string]uint64)
	m.ErrorsBySeverity = make(map[string]uint64)
	m.LastError = ""
	m.LastErrorTime = 0
}

// getCurrentTimestamp 获取当前时间戳
func getCurrentTimestamp() int64 {
	return time.Now().UnixNano()
}

// HasCriticalErrors 检查是否有严重错误
func HasCriticalErrors(errors []error) bool {
	for _, err := range errors {
		if mqttErr, ok := err.(*MQTTXError); ok && mqttErr.IsCritical() {
			return true
		}
	}
	return false
}

// CountErrorsBySeverity 按严重程度统计错误
func CountErrorsBySeverity(errors []error) map[ErrorSeverity]int {
	counts := make(map[ErrorSeverity]int)

	for _, err := range errors {
		if mqttErr, ok := err.(*MQTTXError); ok {
			counts[mqttErr.Severity]++
		} else {
			counts[SeverityError]++ // 未知错误默认为错误级别
		}
	}

	return counts
}

// FormatErrorSummary 格式化错误摘要
func FormatErrorSummary(errors []error) string {
	if len(errors) == 0 {
		return "No errors"
	}

	counts := CountErrorsBySeverity(errors)
	summary := fmt.Sprintf("Total: %d errors", len(errors))

	if critical := counts[SeverityCritical]; critical > 0 {
		summary += fmt.Sprintf(", Critical: %d", critical)
	}
	if errCount := counts[SeverityError]; errCount > 0 {
		summary += fmt.Sprintf(", Error: %d", errCount)
	}
	if warnings := counts[SeverityWarning]; warnings > 0 {
		summary += fmt.Sprintf(", Warning: %d", warnings)
	}
	if info := counts[SeverityInfo]; info > 0 {
		summary += fmt.Sprintf(", Info: %d", info)
	}

	return summary
}

// RetryableError 可重试的错误包装器
type RetryableError struct {
	*MQTTXError
	MaxRetries   int   `json:"max_retries"`
	CurrentRetry int   `json:"current_retry"`
	RetryDelay   int64 `json:"retry_delay"` // 纳秒
}

// NewRetryableError 创建可重试错误
func NewRetryableError(err *MQTTXError, maxRetries int, retryDelay int64) *RetryableError {
	return &RetryableError{
		MQTTXError:   err,
		MaxRetries:   maxRetries,
		CurrentRetry: 0,
		RetryDelay:   retryDelay,
	}
}

// CanRetry 检查是否可以重试
func (r *RetryableError) CanRetry() bool {
	return r.CurrentRetry < r.MaxRetries && IsTemporary(r.MQTTXError)
}

// NextRetry 准备下次重试
func (r *RetryableError) NextRetry() {
	r.CurrentRetry++
	r.WithContext("retry_count", r.CurrentRetry)
}

// GetRetryDelay 获取重试延迟（纳秒）
func (r *RetryableError) GetRetryDelay() int64 {
	// 指数退避策略
	delay := r.RetryDelay
	for i := 0; i < r.CurrentRetry; i++ {
		delay *= 2
	}
	return delay
}
