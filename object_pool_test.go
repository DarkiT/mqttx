package mqttx

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestMessagePool 测试消息对象池
func TestMessagePool(t *testing.T) {
	t.Run("MessagePoolBasicUsage", func(t *testing.T) {
		// 获取消息对象
		msg := GetMessage()
		if msg == nil {
			t.Fatal("GetMessage returned nil")
		}

		// 设置消息内容
		testPayload := []byte("test message payload")
		msg.Topic = "test/topic"
		msg.SetPayload(testPayload)
		msg.QoS = 1
		msg.Retained = false
		msg.Timestamp = time.Now()

		// 验证消息内容
		if msg.Topic != "test/topic" {
			t.Errorf("Expected topic 'test/topic', got '%s'", msg.Topic)
		}
		if string(msg.Payload) != string(testPayload) {
			t.Errorf("Payload mismatch")
		}
		if msg.QoS != 1 {
			t.Errorf("Expected QoS 1, got %d", msg.QoS)
		}

		// 归还到池中
		PutMessage(msg)

		// 再次获取，应该复用同一对象
		msg2 := GetMessage()
		if msg2 == nil {
			t.Fatal("GetMessage returned nil on reuse")
		}

		// 验证重置后的状态
		if msg2.Topic != "" {
			t.Errorf("Expected empty topic after reset, got '%s'", msg2.Topic)
		}
		if len(msg2.Payload) != 0 {
			t.Errorf("Expected empty payload after reset, got length %d", len(msg2.Payload))
		}

		PutMessage(msg2)
	})

	t.Run("MessageClone", func(t *testing.T) {
		original := GetMessage()
		original.Topic = "original/topic"
		original.SetPayload([]byte("original payload"))
		original.QoS = 2
		original.Retained = true

		clone := original.Clone()
		if clone == nil {
			t.Fatal("Clone returned nil")
		}

		// 验证克隆内容
		if clone.Topic != original.Topic {
			t.Error("Clone topic mismatch")
		}
		if string(clone.Payload) != string(original.Payload) {
			t.Error("Clone payload mismatch")
		}
		if clone.QoS != original.QoS {
			t.Error("Clone QoS mismatch")
		}
		if clone.Retained != original.Retained {
			t.Error("Clone Retained mismatch")
		}

		// 修改克隆不应影响原对象
		clone.Topic = "modified/topic"
		if original.Topic == clone.Topic {
			t.Error("Clone modification affected original")
		}

		PutMessage(original)
		PutMessage(clone)
	})

	t.Run("PayloadReuse", func(t *testing.T) {
		msg := GetMessage()

		// 小载荷
		smallPayload := []byte("small")
		msg.SetPayload(smallPayload)
		initialCap := cap(msg.Payload)

		// 中等载荷（应该复用底层数组）
		mediumPayload := make([]byte, initialCap/2)
		for i := range mediumPayload {
			mediumPayload[i] = byte('M')
		}
		msg.SetPayload(mediumPayload)

		if cap(msg.Payload) != initialCap {
			t.Errorf("Expected capacity reuse, got %d vs %d", cap(msg.Payload), initialCap)
		}

		// 大载荷（应该重新分配）
		largePayload := make([]byte, initialCap*2)
		for i := range largePayload {
			largePayload[i] = byte('L')
		}
		msg.SetPayload(largePayload)

		if cap(msg.Payload) < len(largePayload) {
			t.Error("Large payload capacity not sufficient")
		}

		PutMessage(msg)
	})
}

// TestBufferPool 测试缓冲区对象池
func TestBufferPool(t *testing.T) {
	t.Run("BufferPoolSizes", func(t *testing.T) {
		// 测试小缓冲区
		smallBuf := GetBuffer(256)
		if cap(smallBuf.data) != 512 {
			t.Errorf("Expected small buffer capacity 512, got %d", cap(smallBuf.data))
		}

		// 测试中等缓冲区
		mediumBuf := GetBuffer(2048)
		if cap(mediumBuf.data) != 4096 {
			t.Errorf("Expected medium buffer capacity 4096, got %d", cap(mediumBuf.data))
		}

		// 测试大缓冲区
		largeBuf := GetBuffer(16384)
		if cap(largeBuf.data) != 32768 {
			t.Errorf("Expected large buffer capacity 32768, got %d", cap(largeBuf.data))
		}

		// 归还缓冲区
		PutBuffer(smallBuf)
		PutBuffer(mediumBuf)
		PutBuffer(largeBuf)
	})

	t.Run("BufferOperations", func(t *testing.T) {
		buf := GetBuffer(1024)

		// 写入数据
		testData := []byte("test data for buffer")
		n, err := buf.Write(testData)
		if err != nil {
			t.Fatalf("Buffer write failed: %v", err)
		}
		if n != len(testData) {
			t.Errorf("Expected write %d bytes, got %d", len(testData), n)
		}

		// 验证数据
		if string(buf.Bytes()) != string(testData) {
			t.Error("Buffer data mismatch")
		}

		// 扩容测试
		initialCap := cap(buf.data)
		buf.Grow(initialCap * 2)
		if cap(buf.data) <= initialCap {
			t.Error("Buffer grow failed")
		}

		PutBuffer(buf)
	})
}

// TestMetricsPool 测试指标对象池
func TestMetricsPool(t *testing.T) {
	t.Run("MetricsPoolUsage", func(t *testing.T) {
		// 获取指标对象
		metrics := GetMetrics()
		if metrics == nil {
			t.Fatal("GetMetrics returned nil")
		}

		// 记录一些指标
		metrics.recordMessage(1024)
		metrics.recordError()
		metrics.recordReconnect()

		// 验证指标
		snapshot := metrics.getSnapshot()
		if snapshot["total_messages"] != uint64(1) {
			t.Error("Metrics recording failed")
		}

		// 归还到池中
		PutMetrics(metrics)

		// 再次获取应该是重置的对象
		metrics2 := GetMetrics()
		snapshot2 := metrics2.getSnapshot()
		if snapshot2["total_messages"] != uint64(0) {
			t.Error("Metrics reset failed")
		}

		PutMetrics(metrics2)
	})

	t.Run("SessionMetricsPoolUsage", func(t *testing.T) {
		sessionMetrics := GetSessionMetrics()
		if sessionMetrics == nil {
			t.Fatal("GetSessionMetrics returned nil")
		}

		// 记录会话指标
		sessionMetrics.recordMessage(true, 512)
		sessionMetrics.recordMessage(false, 256)
		sessionMetrics.recordError(nil)

		snapshot := sessionMetrics.getSnapshot()
		if snapshot["messages_sent"] != uint64(1) {
			t.Error("Session metrics recording failed")
		}
		if snapshot["messages_received"] != uint64(1) {
			t.Error("Session metrics recording failed")
		}

		PutSessionMetrics(sessionMetrics)

		// 验证重置
		sessionMetrics2 := GetSessionMetrics()
		snapshot2 := sessionMetrics2.getSnapshot()
		if snapshot2["messages_sent"] != uint64(0) {
			t.Error("Session metrics reset failed")
		}

		PutSessionMetrics(sessionMetrics2)
	})
}

// TestTimerPool 测试定时器对象池
func TestTimerPool(t *testing.T) {
	t.Run("TimerPoolUsage", func(t *testing.T) {
		timer := GetTimer(100 * time.Millisecond)
		if timer == nil {
			t.Fatal("GetTimer returned nil")
		}

		if !timer.active {
			t.Error("Timer should be active after GetTimer")
		}

		// 等待定时器触发
		select {
		case <-timer.C:
			// 定时器正常触发
		case <-time.After(200 * time.Millisecond):
			t.Error("Timer did not fire within expected time")
		}

		// 停止定时器
		timer.Stop()
		if timer.active {
			t.Error("Timer should not be active after Stop")
		}

		PutTimer(timer)
	})

	t.Run("TimerReset", func(t *testing.T) {
		timer := GetTimer(time.Hour) // 长时间定时器

		// 重置为短时间
		timer.Reset(50 * time.Millisecond)

		select {
		case <-timer.C:
			// 重置后的定时器正常触发
		case <-time.After(100 * time.Millisecond):
			t.Error("Reset timer did not fire within expected time")
		}

		timer.Stop()
		PutTimer(timer)
	})
}

// BenchmarkObjectPools 对象池性能基准测试
func BenchmarkObjectPools(b *testing.B) {
	b.Run("MessagePool", func(b *testing.B) {
		testPayload := []byte("benchmark test payload for message pool")

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				msg := GetMessage()
				msg.Topic = "bench/topic"
				msg.SetPayload(testPayload)
				msg.QoS = 1
				msg.Timestamp = time.Now()
				PutMessage(msg)
			}
		})
	})

	b.Run("MessagePoolVsNew", func(b *testing.B) {
		testPayload := []byte("benchmark test payload")

		b.Run("WithPool", func(b *testing.B) {
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					msg := GetMessage()
					msg.SetPayload(testPayload)
					PutMessage(msg)
				}
			})
		})

		b.Run("WithoutPool", func(b *testing.B) {
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					msg := &PooledMessage{
						Payload: make([]byte, len(testPayload)),
					}
					copy(msg.Payload, testPayload)
					_ = msg // 避免优化掉
				}
			})
		})
	})

	b.Run("BufferPool", func(b *testing.B) {
		testData := make([]byte, 1024)
		for i := range testData {
			testData[i] = byte(i % 256)
		}

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				buf := GetBuffer(1024)
				buf.Write(testData)
				PutBuffer(buf)
			}
		})
	})

	b.Run("MetricsPool", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				metrics := GetMetrics()
				metrics.recordMessage(1024)
				metrics.recordError()
				PutMetrics(metrics)
			}
		})
	})
}

// TestObjectPoolConcurrency 并发安全测试
func TestObjectPoolConcurrency(t *testing.T) {
	t.Run("ConcurrentMessagePool", func(t *testing.T) {
		const numGoroutines = 10
		const operationsPerGoroutine = 1000

		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < operationsPerGoroutine; j++ {
					msg := GetMessage()
					msg.SetPayload([]byte("concurrent test"))
					msg.Topic = "test/concurrent"
					clone := msg.Clone()
					PutMessage(msg)
					PutMessage(clone)
				}
			}()
		}

		wg.Wait()
	})

	t.Run("ConcurrentBufferPool", func(t *testing.T) {
		const numGoroutines = 10
		const operationsPerGoroutine = 1000

		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(size int) {
				defer wg.Done()
				expectedSize := 512 << (size % 3) // 512, 1024, 2048
				for j := 0; j < operationsPerGoroutine; j++ {
					buf := GetBuffer(expectedSize)
					buf.Write(make([]byte, expectedSize/2))
					PutBuffer(buf)
				}
			}(i)
		}

		wg.Wait()
	})
}

// TestObjectPoolMemoryEfficiency 内存效率测试
func TestObjectPoolMemoryEfficiency(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过内存效率测试")
	}

	// 强制垃圾回收
	runtime.GC()
	runtime.GC()

	var initialMemStats runtime.MemStats
	runtime.ReadMemStats(&initialMemStats)

	// 使用对象池创建大量对象
	const numObjects = 10000
	messages := make([]*PooledMessage, numObjects)
	buffers := make([]*PooledBuffer, numObjects)

	for i := 0; i < numObjects; i++ {
		msg := GetMessage()
		msg.SetPayload(make([]byte, 1024))
		messages[i] = msg

		buf := GetBuffer(1024)
		buf.Write(make([]byte, 512))
		buffers[i] = buf
	}

	runtime.GC()
	runtime.GC()

	var afterCreateMemStats runtime.MemStats
	runtime.ReadMemStats(&afterCreateMemStats)

	// 归还所有对象
	for i := 0; i < numObjects; i++ {
		PutMessage(messages[i])
		PutBuffer(buffers[i])
	}

	// 清空引用
	messages = nil
	buffers = nil

	runtime.GC()
	runtime.GC()

	var finalMemStats runtime.MemStats
	runtime.ReadMemStats(&finalMemStats)

	memUsed := afterCreateMemStats.Alloc - initialMemStats.Alloc
	memRetained := finalMemStats.Alloc - initialMemStats.Alloc

	t.Logf("Memory used during test: %d bytes", memUsed)
	t.Logf("Memory retained after cleanup: %d bytes", memRetained)
	t.Logf("Memory efficiency: %.2f%% retained",
		float64(memRetained)/float64(memUsed)*100)

	// 允许一定的内存保留（池中的对象）
	maxRetainedRatio := 0.5 // 最多保留50%的内存
	if float64(memRetained) > float64(memUsed)*maxRetainedRatio {
		t.Errorf("Too much memory retained: %d > %d (%.0f%%)",
			memRetained, uint64(float64(memUsed)*maxRetainedRatio), maxRetainedRatio*100)
	}
}
