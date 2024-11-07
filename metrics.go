package mqtt

import (
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
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
		resourceStats: &ResourceStats{},
	}
	// 启动速率更新器
	go m.rateUpdater()
	// 启动资源统计
	go m.resourceCollector()
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

	// 获取速率值，确保不为空
	messageRate := m.rates.messageRate.Load()
	if messageRate == nil {
		messageRate = "0.00/s"
	}

	byteRate := m.rates.byteRate.Load()
	if byteRate == nil {
		byteRate = "0 B/s"
	}

	avgRate := m.rates.avgMessageRate.Load()
	if avgRate == nil {
		avgRate = "0.00/s"
	}

	return map[string]interface{}{
		"active_sessions":  atomic.LoadInt64(&m.ActiveSessions),
		"total_messages":   atomic.LoadUint64(&m.TotalMessages),
		"total_bytes":      formatBytes(atomic.LoadUint64(&m.TotalBytes)),
		"message_rate":     messageRate,
		"bytes_rate":       byteRate,
		"avg_message_rate": avgRate,
		"error_count":      atomic.LoadUint64(&m.ErrorCount),
		"reconnect_count":  atomic.LoadUint64(&m.ReconnectCount),
		"last_update":      m.LastUpdate.Format(time.RFC3339),
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

// RateCounter 速率计数器
type RateCounter struct {
	messageRate    *atomic.Value // string
	byteRate       *atomic.Value // string
	avgMessageRate *atomic.Value
}

// ResourceStats 添加新的结构体定义
type ResourceStats struct {
	goroutines int
	heapAlloc  uint64
	heapInUse  uint64
	mu         sync.RWMutex
}

// rateUpdater 更新速率指标
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

		// 计算时间间隔（秒）
		interval := currentTime.Sub(lastTime).Seconds()
		if interval > 0 {
			// 计算消息速率
			messagesDiff := currentMessages - lastMessageCount
			messageRate := float64(messagesDiff) / interval
			m.rates.messageRate.Store(fmt.Sprintf("%.2f/s", messageRate))

			// 计算数据速率
			bytesDiff := currentBytes - lastByteCount
			byteRate := float64(bytesDiff) / interval
			m.rates.byteRate.Store(formatBytes(uint64(byteRate)) + "/s")

			// 计算平均速率
			uptime := currentTime.Sub(m.startTime).Seconds()
			if uptime > 0 {
				avgMessageRate := float64(currentMessages) / uptime
				m.rates.avgMessageRate.Store(fmt.Sprintf("%.2f/s", avgMessageRate))
			}
		}

		lastMessageCount = currentMessages
		lastByteCount = currentBytes
		lastTime = currentTime
	}
}

// resourceCollector 添加新的方法
func (m *Metrics) resourceCollector() {
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		stats := &runtime.MemStats{}
		runtime.ReadMemStats(stats)

		m.resourceStats.mu.Lock()
		m.resourceStats.goroutines = runtime.NumGoroutine()
		m.resourceStats.heapAlloc = stats.HeapAlloc
		m.resourceStats.heapInUse = stats.HeapInuse
		m.resourceStats.mu.Unlock()
	}
}
