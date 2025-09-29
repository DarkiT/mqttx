package mqttx

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// 使用 errors.go 中定义的 ErrorSeverity 类型

// ErrorCategory 错误类别
type ErrorCategory string

const (
	// CategoryConnection 连接相关错误
	CategoryConnection ErrorCategory = "connection"
	// CategorySubscription 订阅相关错误
	CategorySubscription ErrorCategory = "subscription"
	// CategoryPublish 发布相关错误
	CategoryPublish ErrorCategory = "publish"
	// CategoryAuthentication 认证相关错误
	CategoryAuthentication ErrorCategory = "authentication"
	// CategoryInternal 内部错误
	CategoryInternal ErrorCategory = "internal"
)

// RecoveryStrategy 恢复策略
type RecoveryStrategy int

const (
	// StrategyRetry 重试策略
	StrategyRetry RecoveryStrategy = iota
	// StrategyReconnect 重新连接策略
	StrategyReconnect
	// StrategyNotify 仅通知策略
	StrategyNotify
)

// ErrorInfo 错误信息
type ErrorInfo struct {
	Error      error            // 原始错误
	Time       time.Time        // 错误发生时间
	Session    string           // 相关会话
	Category   ErrorCategory    // 错误类别
	Severity   ErrorSeverity    // 错误严重程度
	Strategy   RecoveryStrategy // 恢复策略
	RetryCount int              // 重试次数
	Recovered  bool             // 是否已恢复
}

// RecoveryManager 恢复管理器
type RecoveryManager struct {
	errors     map[string]*ErrorInfo // 错误信息映射
	manager    *Manager              // 会话管理器
	maxRetries map[ErrorCategory]int // 每种错误类别的最大重试次数
	mu         sync.RWMutex          // 读写锁
	logger     Logger                // 日志记录器
	errorLimit int                   // 错误数量限制
	errorCount int32                 // 当前错误计数
}

// NewRecoveryManager 创建新的恢复管理器
func NewRecoveryManager(manager *Manager) *RecoveryManager {
	rm := &RecoveryManager{
		errors:     make(map[string]*ErrorInfo),
		manager:    manager,
		maxRetries: make(map[ErrorCategory]int),
		logger:     manager.logger,
		errorLimit: 100, // 最多存储100个错误
	}

	// 设置默认的最大重试次数
	rm.maxRetries[CategoryConnection] = 3
	rm.maxRetries[CategorySubscription] = 2
	rm.maxRetries[CategoryPublish] = 2
	rm.maxRetries[CategoryAuthentication] = 1
	rm.maxRetries[CategoryInternal] = 1

	return rm
}

// RegisterError 注册错误
func (rm *RecoveryManager) RegisterError(sessionName string, err error, category ErrorCategory) *ErrorInfo {
	if err == nil {
		return nil
	}

	// 如果错误数量超过限制，先清理已恢复的错误
	if atomic.LoadInt32(&rm.errorCount) >= int32(rm.errorLimit) {
		rm.ClearRecoveredErrors()

		// 如果清理后仍然达到上限，则移除最旧的错误
		rm.mu.Lock()
		if len(rm.errors) >= rm.errorLimit {
			var oldestID string
			var oldestTime time.Time
			first := true

			// 找到最旧的错误
			for id, info := range rm.errors {
				if first || info.Time.Before(oldestTime) {
					oldestID = id
					oldestTime = info.Time
					first = false
				}
			}

			// 移除最旧的错误
			if oldestID != "" {
				delete(rm.errors, oldestID)
				atomic.AddInt32(&rm.errorCount, -1)
				rm.logger.Debug("Removed oldest error to stay within limit",
					"error_id", oldestID,
					"error_time", oldestTime)
			}
		}
		rm.mu.Unlock()
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	// 创建错误ID
	errorID := fmt.Sprintf("%s:%s:%v", sessionName, category, err)

	// 检查是否已存在相同错误
	if info, exists := rm.errors[errorID]; exists {
		// 更新已有错误信息
		info.RetryCount++
		rm.logger.Debug("Updated existing error",
			"session", sessionName,
			"category", category,
			"error", err,
			"retry_count", info.RetryCount)
		return info
	}

	// 确定错误严重程度和恢复策略
	severity := rm.determineSeverity(err, category)
	strategy := rm.determineStrategy(category, severity)

	// 创建新的错误信息
	info := &ErrorInfo{
		Error:      err,
		Time:       time.Now(),
		Session:    sessionName,
		Category:   category,
		Severity:   severity,
		Strategy:   strategy,
		RetryCount: 0,
		Recovered:  false,
	}

	rm.errors[errorID] = info
	atomic.AddInt32(&rm.errorCount, 1)

	rm.logger.Info("Registered new error",
		"session", sessionName,
		"category", category,
		"severity", severity,
		"strategy", strategy,
		"error", err)

	// 根据策略执行恢复操作
	go rm.executeRecovery(info)

	return info
}

// determineSeverity 确定错误严重程度
func (rm *RecoveryManager) determineSeverity(err error, category ErrorCategory) ErrorSeverity {
	// 简化严重程度判断
	switch category {
	case CategoryAuthentication, CategoryInternal:
		return SeverityCritical
	default:
		return SeverityWarning
	}
}

// determineStrategy 确定恢复策略
func (rm *RecoveryManager) determineStrategy(category ErrorCategory, severity ErrorSeverity) RecoveryStrategy {
	// 简化恢复策略判断
	switch {
	case category == CategoryConnection && severity == SeverityWarning:
		return StrategyReconnect
	case category == CategorySubscription || category == CategoryPublish:
		return StrategyRetry
	default:
		return StrategyNotify
	}
}

// executeRecovery 执行恢复操作
func (rm *RecoveryManager) executeRecovery(info *ErrorInfo) {
	// 获取会话
	session, err := rm.manager.GetSession(info.Session)
	if err != nil {
		rm.logger.Warn("Cannot execute recovery, session not found",
			"session", info.Session,
			"error", err)
		return
	}

	// 根据恢复策略执行操作
	switch info.Strategy {
	case StrategyRetry:
		rm.executeRetry(info, session)
	case StrategyReconnect:
		// 会话的自动重连机制会处理重连
		rm.logger.Info("Reconnection will be handled by session's auto-reconnect mechanism",
			"session", info.Session)
	case StrategyNotify:
		rm.executeNotify(info)
	}
}

// executeRetry 执行重试策略
func (rm *RecoveryManager) executeRetry(info *ErrorInfo, session *Session) {
	maxRetries := rm.maxRetries[info.Category]
	if info.RetryCount >= maxRetries {
		rm.logger.Warn("Max retries reached",
			"session", info.Session,
			"category", info.Category,
			"error", info.Error,
			"retry_count", info.RetryCount,
			"max_retries", maxRetries)
		return
	}

	// 指数退避重试
	backoff := time.Duration(1<<uint(info.RetryCount)) * 100 * time.Millisecond
	if backoff > 10*time.Second {
		backoff = 10 * time.Second
	}

	rm.logger.Info("Scheduling retry",
		"session", info.Session,
		"category", info.Category,
		"retry_count", info.RetryCount+1,
		"max_retries", maxRetries,
		"backoff", backoff)

	time.Sleep(backoff)

	// 更新重试信息
	rm.mu.Lock()
	info.RetryCount++
	rm.mu.Unlock()
}

// executeNotify 执行通知策略
func (rm *RecoveryManager) executeNotify(info *ErrorInfo) {
	rm.logger.Warn("Error requires attention",
		"session", info.Session,
		"category", info.Category,
		"severity", info.Severity,
		"error", info.Error,
		"time", info.Time.Format(time.RFC3339))

	// 发送错误事件
	rm.manager.events.emit(&Event{
		Type:    "error_notification",
		Session: info.Session,
		Data: map[string]interface{}{
			"error":       info.Error.Error(),
			"category":    info.Category,
			"severity":    info.Severity,
			"time":        info.Time,
			"retry_count": info.RetryCount,
		},
		Timestamp: time.Now(),
	})
}

// GetActiveErrors 获取活跃错误
func (rm *RecoveryManager) GetActiveErrors() []*ErrorInfo {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	activeErrors := make([]*ErrorInfo, 0, len(rm.errors))
	for _, info := range rm.errors {
		if !info.Recovered {
			activeErrors = append(activeErrors, info)
		}
	}

	return activeErrors
}

// MarkErrorRecovered 标记错误已恢复
func (rm *RecoveryManager) MarkErrorRecovered(sessionName string, category ErrorCategory, err error) bool {
	if err == nil {
		return false
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	// 创建错误ID
	errorID := fmt.Sprintf("%s:%s:%v", sessionName, category, err)

	// 查找并标记错误
	if info, exists := rm.errors[errorID]; exists {
		info.Recovered = true
		rm.logger.Info("Marked error as recovered",
			"session", sessionName,
			"category", category,
			"error", err)
		return true
	}

	return false
}

// ClearRecoveredErrors 清理已恢复的错误
func (rm *RecoveryManager) ClearRecoveredErrors() int {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	count := 0
	for id, info := range rm.errors {
		if info.Recovered {
			delete(rm.errors, id)
			count++
		}
	}

	if count > 0 {
		atomic.AddInt32(&rm.errorCount, -int32(count))
		rm.logger.Debug("Cleared recovered errors", "count", count)
	}

	return count
}

// SetMaxRetries 设置最大重试次数
func (rm *RecoveryManager) SetMaxRetries(category ErrorCategory, count int) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.maxRetries[category] = count
}

// GetErrorStats 获取错误统计信息
func (rm *RecoveryManager) GetErrorStats() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	stats := make(map[string]interface{})
	categoryCounts := make(map[string]int)
	severityCounts := make(map[string]int)
	recoveryCounts := make(map[string]int)
	totalErrors := len(rm.errors)
	activeErrors := 0

	for _, info := range rm.errors {
		categoryCounts[string(info.Category)]++
		severityCounts[string(info.Severity)]++

		if info.Recovered {
			recoveryCounts["recovered"]++
		} else {
			recoveryCounts["active"]++
			activeErrors++
		}
	}

	stats["total"] = totalErrors
	stats["active"] = activeErrors
	stats["by_category"] = categoryCounts
	stats["by_severity"] = severityCounts
	stats["by_recovery"] = recoveryCounts

	return stats
}
