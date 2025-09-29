package mqttx

import (
	"sync"
	"time"
)

// MessagePool 消息对象池，减少内存分配
type MessagePool struct {
	pool sync.Pool
}

// PooledMessage 池化消息对象
type PooledMessage struct {
	Topic     string
	Payload   []byte
	QoS       byte
	Retained  bool
	Timestamp time.Time
	original  []byte // 原始payload存储，避免切片复制
}

// Reset 重置消息对象以供复用
func (m *PooledMessage) Reset() {
	m.Topic = ""
	m.Payload = m.Payload[:0] // 保留底层数组，清空切片
	m.QoS = 0
	m.Retained = false
	m.Timestamp = time.Time{}
}

// SetPayload 设置消息载荷，复用底层数组
func (m *PooledMessage) SetPayload(data []byte) {
	// 如果容量足够，直接复用
	if cap(m.Payload) >= len(data) {
		m.Payload = m.Payload[:len(data)]
		copy(m.Payload, data)
	} else {
		// 容量不足时重新分配，但保留原有容量的1.5倍以减少后续分配
		newCap := len(data)
		if oldCap := cap(m.Payload); oldCap > 0 {
			newCap = max(len(data), oldCap*3/2)
		}
		m.Payload = make([]byte, len(data), newCap)
		copy(m.Payload, data)
	}
}

// Clone 克隆消息（深拷贝）
func (m *PooledMessage) Clone() *PooledMessage {
	clone := GetMessage()
	clone.Topic = m.Topic
	clone.SetPayload(m.Payload)
	clone.QoS = m.QoS
	clone.Retained = m.Retained
	clone.Timestamp = m.Timestamp
	return clone
}

// 全局消息池实例
var globalMessagePool = &MessagePool{
	pool: sync.Pool{
		New: func() interface{} {
			return &PooledMessage{
				Payload: make([]byte, 0, 1024), // 预分配1KB容量
			}
		},
	},
}

// GetMessage 从池中获取消息对象
func GetMessage() *PooledMessage {
	msg := globalMessagePool.pool.Get().(*PooledMessage)
	msg.Reset()
	return msg
}

// PutMessage 将消息对象返回池中
func PutMessage(msg *PooledMessage) {
	if msg != nil {
		msg.Reset()
		globalMessagePool.pool.Put(msg)
	}
}

// BufferPool 字节缓冲区对象池
type BufferPool struct {
	pool sync.Pool
}

// PooledBuffer 池化缓冲区
type PooledBuffer struct {
	data []byte
}

// Reset 重置缓冲区
func (b *PooledBuffer) Reset() {
	b.data = b.data[:0]
}

// Bytes 获取字节数据
func (b *PooledBuffer) Bytes() []byte {
	return b.data
}

// Write 写入数据
func (b *PooledBuffer) Write(p []byte) (n int, err error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

// Grow 扩容缓冲区
func (b *PooledBuffer) Grow(n int) {
	if cap(b.data)-len(b.data) < n {
		newCap := len(b.data) + n
		if newCap < cap(b.data)*2 {
			newCap = cap(b.data) * 2
		}
		newData := make([]byte, len(b.data), newCap)
		copy(newData, b.data)
		b.data = newData
	}
}

// 不同大小的缓冲区池
var (
	smallBufferPool = &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &PooledBuffer{
					data: make([]byte, 0, 512), // 512B
				}
			},
		},
	}

	mediumBufferPool = &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &PooledBuffer{
					data: make([]byte, 0, 4096), // 4KB
				}
			},
		},
	}

	largeBufferPool = &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &PooledBuffer{
					data: make([]byte, 0, 32768), // 32KB
				}
			},
		},
	}
)

// GetBuffer 根据预期大小获取合适的缓冲区
func GetBuffer(expectedSize int) *PooledBuffer {
	switch {
	case expectedSize <= 512:
		return smallBufferPool.pool.Get().(*PooledBuffer)
	case expectedSize <= 4096:
		return mediumBufferPool.pool.Get().(*PooledBuffer)
	default:
		return largeBufferPool.pool.Get().(*PooledBuffer)
	}
}

// PutBuffer 将缓冲区返回对应的池中
func PutBuffer(buf *PooledBuffer) {
	if buf == nil {
		return
	}

	buf.Reset()

	// 根据容量返回到对应的池
	switch cap(buf.data) {
	case 512:
		smallBufferPool.pool.Put(buf)
	case 4096:
		mediumBufferPool.pool.Put(buf)
	case 32768:
		largeBufferPool.pool.Put(buf)
	default:
		// 容量不匹配的缓冲区不回收，让GC处理
	}
}

// MetricsPool 指标对象池
type MetricsPool struct {
	metricsPool        sync.Pool
	sessionMetricsPool sync.Pool
}

// 全局指标池
var globalMetricsPool = &MetricsPool{
	metricsPool: sync.Pool{
		New: func() interface{} {
			return newMetrics()
		},
	},
	sessionMetricsPool: sync.Pool{
		New: func() interface{} {
			return newSessionMetrics()
		},
	},
}

// GetMetrics 获取指标对象
func GetMetrics() *Metrics {
	metrics := globalMetricsPool.metricsPool.Get().(*Metrics)
	metrics.reset() // 重置状态
	return metrics
}

// PutMetrics 归还指标对象
func PutMetrics(metrics *Metrics) {
	if metrics != nil {
		globalMetricsPool.metricsPool.Put(metrics)
	}
}

// GetSessionMetrics 获取会话指标对象
func GetSessionMetrics() *SessionMetrics {
	metrics := globalMetricsPool.sessionMetricsPool.Get().(*SessionMetrics)
	metrics.reset() // 重置状态
	return metrics
}

// PutSessionMetrics 归还会话指标对象
func PutSessionMetrics(metrics *SessionMetrics) {
	if metrics != nil {
		globalMetricsPool.sessionMetricsPool.Put(metrics)
	}
}

// TimerPool 定时器对象池
type TimerPool struct {
	pool sync.Pool
}

// PooledTimer 池化定时器
type PooledTimer struct {
	*time.Timer
	active bool
}

// Reset 重置定时器
func (t *PooledTimer) Reset(d time.Duration) bool {
	t.active = true
	return t.Timer.Reset(d)
}

// Stop 停止定时器
func (t *PooledTimer) Stop() bool {
	t.active = false
	return t.Timer.Stop()
}

// 全局定时器池
var globalTimerPool = &TimerPool{
	pool: sync.Pool{
		New: func() interface{} {
			return &PooledTimer{
				Timer:  time.NewTimer(time.Hour), // 创建一个长时间的定时器
				active: false,
			}
		},
	},
}

// GetTimer 获取定时器
func GetTimer(d time.Duration) *PooledTimer {
	timer := globalTimerPool.pool.Get().(*PooledTimer)
	timer.Reset(d)
	return timer
}

// PutTimer 归还定时器
func PutTimer(timer *PooledTimer) {
	if timer != nil && !timer.active {
		// 确保定时器已停止
		timer.Stop()
		// 清空通道中可能存在的信号
		select {
		case <-timer.C:
		default:
		}
		globalTimerPool.pool.Put(timer)
	}
}

// PoolStats 对象池统计信息
type PoolStats struct {
	MessagePoolHits   uint64 `json:"message_pool_hits"`
	MessagePoolMisses uint64 `json:"message_pool_misses"`
	BufferPoolHits    uint64 `json:"buffer_pool_hits"`
	BufferPoolMisses  uint64 `json:"buffer_pool_misses"`
	MetricsPoolHits   uint64 `json:"metrics_pool_hits"`
	MetricsPoolMisses uint64 `json:"metrics_pool_misses"`
	TimerPoolHits     uint64 `json:"timer_pool_hits"`
	TimerPoolMisses   uint64 `json:"timer_pool_misses"`
}

// 全局池统计
var poolStats PoolStats

// GetPoolStats 获取池统计信息
func GetPoolStats() PoolStats {
	return poolStats
}

// ResetPoolStats 重置池统计
func ResetPoolStats() {
	poolStats = PoolStats{}
}

// max 返回两个整数中的较大值
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
