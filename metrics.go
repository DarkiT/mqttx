package mqttx

import (
	"fmt"
	"sync/atomic"
	"time"
)

// newMetrics 创建新的指标收集器
func newMetrics() *Metrics {
	now := time.Now().UnixNano()
	return &Metrics{
		LastUpdate:      now,
		LastMessageTime: now,
		LastErrorTime:   now,
	}
}

// newSessionMetrics 创建新的会话指标收集器
func newSessionMetrics() *SessionMetrics {
	now := time.Now().UnixNano()
	return &SessionMetrics{
		LastMessage: now,
		LastError:   now,
	}
}

// recordMessage 记录消息指标
func (m *Metrics) recordMessage(bytes uint64) {
	atomic.AddUint64(&m.TotalMessages, 1)
	atomic.AddUint64(&m.TotalBytes, bytes)
	now := time.Now().UnixNano()
	atomic.StoreInt64(&m.LastUpdate, now)
	atomic.StoreInt64(&m.LastMessageTime, now)
}

// recordError 记录错误指标
func (m *Metrics) recordError() {
	atomic.AddUint64(&m.ErrorCount, 1)
	atomic.StoreInt64(&m.LastErrorTime, time.Now().UnixNano())
}

// recordReconnect 记录重连指标
func (m *Metrics) recordReconnect() {
	atomic.AddUint64(&m.ReconnectCount, 1)
}

// reset 重置指标对象以供对象池复用
func (m *Metrics) reset() {
	atomic.StoreUint64(&m.TotalMessages, 0)
	atomic.StoreUint64(&m.TotalBytes, 0)
	atomic.StoreUint64(&m.ErrorCount, 0)
	atomic.StoreUint64(&m.ReconnectCount, 0)
	atomic.StoreInt64(&m.ActiveSessions, 0)
	now := time.Now().UnixNano()
	atomic.StoreInt64(&m.LastUpdate, now)
	atomic.StoreInt64(&m.LastMessageTime, now)
	atomic.StoreInt64(&m.LastErrorTime, now)
}

// updateSessionCount 更新会话计数
func (m *Metrics) updateSessionCount(delta int64) {
	atomic.AddInt64(&m.ActiveSessions, delta)
}

// getSnapshot 获取指标快照，返回格式化的指标信息
func (m *Metrics) getSnapshot() map[string]interface{} {
	return map[string]interface{}{
		"active_sessions":   atomic.LoadInt64(&m.ActiveSessions),
		"total_messages":    atomic.LoadUint64(&m.TotalMessages),
		"total_bytes":       formatBytes(atomic.LoadUint64(&m.TotalBytes)),
		"error_count":       atomic.LoadUint64(&m.ErrorCount),
		"reconnect_count":   atomic.LoadUint64(&m.ReconnectCount),
		"last_update":       time.Unix(0, atomic.LoadInt64(&m.LastUpdate)).Format(time.DateTime),
		"last_message_time": time.Unix(0, atomic.LoadInt64(&m.LastMessageTime)).Format(time.DateTime),
		"last_error_time":   time.Unix(0, atomic.LoadInt64(&m.LastErrorTime)).Format(time.DateTime),
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
	atomic.StoreInt64(&m.LastMessage, time.Now().UnixNano())
}

// recordError 记录会话错误指标
func (m *SessionMetrics) recordError(_ error) {
	atomic.AddUint64(&m.Errors, 1)
	atomic.StoreInt64(&m.LastError, time.Now().UnixNano())
}

// recordReconnect 记录会话重连指标
func (m *SessionMetrics) recordReconnect() {
	atomic.AddUint64(&m.Reconnects, 1)
}

// reset 重置会话指标对象以供对象池复用
func (m *SessionMetrics) reset() {
	atomic.StoreUint64(&m.MessagesSent, 0)
	atomic.StoreUint64(&m.MessagesReceived, 0)
	atomic.StoreUint64(&m.BytesSent, 0)
	atomic.StoreUint64(&m.BytesReceived, 0)
	atomic.StoreUint64(&m.Errors, 0)
	atomic.StoreUint64(&m.Reconnects, 0)
	now := time.Now().UnixNano()
	atomic.StoreInt64(&m.LastMessage, now)
	atomic.StoreInt64(&m.LastError, now)
}

// getSnapshot 获取会话指标快照
func (m *SessionMetrics) getSnapshot() map[string]interface{} {
	return map[string]interface{}{
		"messages_sent":     atomic.LoadUint64(&m.MessagesSent),
		"messages_received": atomic.LoadUint64(&m.MessagesReceived),
		"bytes_sent":        atomic.LoadUint64(&m.BytesSent),
		"bytes_received":    atomic.LoadUint64(&m.BytesReceived),
		"errors":            atomic.LoadUint64(&m.Errors),
		"reconnects":        atomic.LoadUint64(&m.Reconnects),
		"last_error":        time.Unix(0, atomic.LoadInt64(&m.LastError)),
		"last_message":      time.Unix(0, atomic.LoadInt64(&m.LastMessage)),
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
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
