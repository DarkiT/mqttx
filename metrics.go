package mqtt

import (
	"fmt"
	"reflect"
	"sync/atomic"
	"time"
)

// 添加速率格式化常量
const (
	UnitMsg  = "msg/s"
	UnitByte = "B/s"
)

// newMetrics 创建新的指标收集器
func newMetrics() *Metrics {
	now := time.Now()
	m := &Metrics{
		startTime:  now,
		LastUpdate: now,
		rates: &RateCounter{
			messageRate:    &atomic.Value{},
			byteRate:       &atomic.Value{},
			avgMessageRate: &atomic.Value{},
		},
	}
	// 启动速率更新器
	go m.rateUpdater()
	return m
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
func (m *Metrics) recordError(err error) {
	atomic.AddUint64(&m.ErrorCount, 1)
	errType := reflect.TypeOf(err).String()
	if v, ok := m.errorTypes.Load(errType); ok {
		m.errorTypes.Store(errType, v.(uint64)+1)
	} else {
		m.errorTypes.Store(errType, uint64(1))
	}
}

// updateSessionCount 更新会话计数
func (m *Metrics) updateSessionCount(delta int64) {
	atomic.AddInt64(&m.ActiveSessions, delta)
}

// getSnapshot 获取指标快照，返回格式化的指标信息
func (m *Metrics) getSnapshot() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	messageRate := m.rates.messageRate.Load()
	if messageRate == nil {
		messageRate = formatRate(0)
	}

	byteRate := m.rates.byteRate.Load()
	if byteRate == nil {
		byteRate = formatByteRate(0)
	}

	avgRate := m.rates.avgMessageRate.Load()
	if avgRate == nil {
		avgRate = formatRate(0)
	}

	totalBytes := atomic.LoadUint64(&m.TotalBytes)

	return map[string]interface{}{
		"active_sessions":  atomic.LoadInt64(&m.ActiveSessions),
		"total_messages":   atomic.LoadUint64(&m.TotalMessages),
		"total_bytes":      formatBytes(totalBytes),
		"message_rate":     messageRate,
		"bytes_rate":       byteRate,
		"avg_message_rate": avgRate,
		"error_count":      atomic.LoadUint64(&m.ErrorCount),
		"reconnect_count":  atomic.LoadUint64(&m.ReconnectCount),
		"last_update":      m.LastUpdate.Format(time.DateTime),
		"uptime":           time.Since(m.startTime).String(),
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

// rateUpdater 优化后的速率更新器
func (m *Metrics) rateUpdater() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var lastMessageCount uint64
	var lastByteCount uint64
	var lastTime = time.Now()

	for range ticker.C {
		currentTime := time.Now()
		currentMessages := atomic.LoadUint64(&m.TotalMessages)
		currentBytes := atomic.LoadUint64(&m.TotalBytes)

		interval := currentTime.Sub(lastTime).Seconds()
		if interval > 0 {
			// 计算消息速率
			messagesDiff := currentMessages - lastMessageCount
			messageRate := float64(messagesDiff) / interval
			m.rates.messageRate.Store(formatRate(messageRate))

			// 计算数据速率
			bytesDiff := currentBytes - lastByteCount
			byteRate := uint64(float64(bytesDiff) / interval)
			m.rates.byteRate.Store(formatByteRate(byteRate))

			// 计算平均速率
			uptime := currentTime.Sub(m.startTime).Seconds()
			if uptime > 0 {
				avgMessageRate := float64(currentMessages) / uptime
				m.rates.avgMessageRate.Store(formatRate(avgMessageRate))
			}
		}

		lastMessageCount = currentMessages
		lastByteCount = currentBytes
		lastTime = currentTime
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

// formatRate 格式化速率
func formatRate(value float64) string {
	switch {
	case value >= 1000000:
		return fmt.Sprintf("%.2f M%s", value/1000000, UnitMsg)
	case value >= 1000:
		return fmt.Sprintf("%.2f k%s", value/1000, UnitMsg)
	default:
		return fmt.Sprintf("%.2f %s", value, UnitMsg)
	}
}

// formatByteRate 格式化字节速率
func formatByteRate(bytesPerSecond uint64) string {
	const unit = 1024
	if bytesPerSecond < unit {
		return fmt.Sprintf("%d %s", bytesPerSecond, UnitByte)
	}
	div, exp := uint64(unit), 0
	for n := bytesPerSecond / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB/s", float64(bytesPerSecond)/float64(div), "KMGTPE"[exp])
}
