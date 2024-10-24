package mqtt

import (
	"fmt"
	"sync/atomic"
	"time"
)

// newMetrics 创建新的指标收集器
func newMetrics() *Metrics {
	return &Metrics{
		LastUpdate: time.Now(),
	}
}

// newSessionMetrics 创建新的会话指标收集器
func newSessionMetrics() *SessionMetrics {
	return &SessionMetrics{
		LastMessage: time.Now(),
	}
}

// recordMessage 记录消息指标
func (m *Metrics) recordMessage(bytes uint64) {
	atomic.AddUint64(&m.TotalMessages, 1)
	atomic.AddUint64(&m.TotalBytes, bytes)
	m.mu.Lock()
	m.LastUpdate = time.Now()
	m.mu.Unlock()
}

// recordError 记录错误指标
func (m *Metrics) recordError() {
	atomic.AddUint64(&m.ErrorCount, 1)
}

// recordReconnect 记录重连指标
func (m *Metrics) recordReconnect() {
	atomic.AddUint64(&m.ReconnectCount, 1)
}

// updateSessionCount 更新会话计数
func (m *Metrics) updateSessionCount(delta int64) {
	atomic.AddInt64(&m.ActiveSessions, delta)
}

// getSnapshot 获取指标快照，返回格式化的指标信息
func (m *Metrics) getSnapshot() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"active_sessions": atomic.LoadInt64(&m.ActiveSessions),
		"total_messages":  atomic.LoadUint64(&m.TotalMessages),
		"total_bytes":     formatBytes(atomic.LoadUint64(&m.TotalBytes)),
		"error_count":     atomic.LoadUint64(&m.ErrorCount),
		"reconnect_count": atomic.LoadUint64(&m.ReconnectCount),
		"last_update":     m.LastUpdate.Format(time.RFC3339),
		"uptime":          time.Since(m.LastUpdate).String(),
		"message_rate":    calculateRate(atomic.LoadUint64(&m.TotalMessages), m.LastUpdate),
		"bytes_rate":      formatBytesRate(atomic.LoadUint64(&m.TotalBytes), m.LastUpdate),
	}
}

// recordMessage 记录会话消息指标
func (m *SessionMetrics) recordMessage(sent bool, bytes uint64) {
	if sent {
		atomic.AddUint64(&m.MessagesSent, 1)
		atomic.AddUint64(&m.BytesSent, bytes)
	} else {
		atomic.AddUint64(&m.MessagesReceived, 1)
		atomic.AddUint64(&m.BytesReceived, bytes)
	}

	m.mu.Lock()
	m.LastMessage = time.Now()
	m.mu.Unlock()
}

// recordError 记录会话错误指标
func (m *SessionMetrics) recordError(_ error) {
	atomic.AddUint64(&m.Errors, 1)
	m.mu.Lock()
	m.LastError = time.Now()
	m.mu.Unlock()
}

// recordReconnect 记录会话重连指标
func (m *SessionMetrics) recordReconnect() {
	atomic.AddUint64(&m.Reconnects, 1)
}

// getSnapshot 获取会话指标快照
func (m *SessionMetrics) getSnapshot() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"messages_sent":     atomic.LoadUint64(&m.MessagesSent),
		"messages_received": atomic.LoadUint64(&m.MessagesReceived),
		"bytes_sent":        atomic.LoadUint64(&m.BytesSent),
		"bytes_received":    atomic.LoadUint64(&m.BytesReceived),
		"errors":            atomic.LoadUint64(&m.Errors),
		"reconnects":        atomic.LoadUint64(&m.Reconnects),
		"last_error":        m.LastError,
		"last_message":      m.LastMessage,
	}
}

// formatBytes 格式化字节大小为可读格式
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB",
		float64(bytes)/float64(div), "KMGTPE"[exp])
}

// calculateRate 计算每秒速率
func calculateRate(count uint64, since time.Time) string {
	duration := time.Since(since).Seconds()
	if duration == 0 {
		return "0/s"
	}
	rate := float64(count) / duration
	return fmt.Sprintf("%.2f/s", rate)
}

// formatBytesRate 格式化字节速率
func formatBytesRate(bytes uint64, since time.Time) string {
	duration := time.Since(since).Seconds()
	if duration == 0 {
		return "0 B/s"
	}
	bytesPerSecond := float64(bytes) / duration

	const unit = 1024
	if bytesPerSecond < unit {
		return fmt.Sprintf("%.2f B/s", bytesPerSecond)
	}
	div, exp := float64(unit), 0
	for n := bytesPerSecond / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB/s",
		bytesPerSecond/div, "KMGTPE"[exp])
}
