package mqttx

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestConcurrentMetricsAccess 测试指标并发访问安全性
func TestConcurrentMetricsAccess(t *testing.T) {
	metrics := newMetrics()
	sessionMetrics := newSessionMetrics()

	const numGoroutines = 100
	const numOperations = 1000

	var wg sync.WaitGroup

	// 测试Metrics并发访问
	t.Run("GlobalMetrics", func(t *testing.T) {
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					// 并发写入
					metrics.recordMessage(uint64(j))
					metrics.recordError()
					metrics.recordReconnect()
					metrics.updateSessionCount(1)

					// 并发读取
					snapshot := metrics.getSnapshot()
					if snapshot == nil {
						t.Errorf("Goroutine %d: got nil snapshot", id)
						return
					}

					// 验证数据一致性
					if j%100 == 0 {
						totalMessages := atomic.LoadUint64(&metrics.TotalMessages)
						if totalMessages == 0 {
							t.Errorf("Goroutine %d: unexpected zero total messages", id)
						}
					}
				}
			}(i)
		}
		wg.Wait()

		// 验证最终状态
		finalSnapshot := metrics.getSnapshot()
		if finalSnapshot["total_messages"].(uint64) == 0 {
			t.Error("Expected non-zero total messages")
		}
	})

	// 测试SessionMetrics并发访问
	t.Run("SessionMetrics", func(t *testing.T) {
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					// 交替发送和接收
					sessionMetrics.recordMessage(j%2 == 0, uint64(j))
					sessionMetrics.recordError(fmt.Errorf("test error %d", j))
					sessionMetrics.recordReconnect()

					// 并发读取
					snapshot := sessionMetrics.getSnapshot()
					if snapshot == nil {
						t.Errorf("Goroutine %d: got nil session snapshot", id)
						return
					}
				}
			}(i)
		}
		wg.Wait()

		// 验证最终状态
		finalSnapshot := sessionMetrics.getSnapshot()
		totalSent := finalSnapshot["messages_sent"].(uint64)
		totalReceived := finalSnapshot["messages_received"].(uint64)

		if totalSent == 0 && totalReceived == 0 {
			t.Error("Expected non-zero message counts")
		}
	})
}

// TestForwarderConcurrentOperations 测试转发器并发操作
func TestForwarderConcurrentOperations(t *testing.T) {
	// 创建管理器
	manager := NewSessionManager()

	// 创建转发器注册表
	_ = NewForwarderRegistry(manager)

	const numForwarders = 50
	const numMessages = 100

	var wg sync.WaitGroup
	var createdCount int32
	var stoppedCount int32

	// 并发创建和启动转发器
	t.Run("ConcurrentForwarderLifecycle", func(t *testing.T) {
		for i := 0; i < numForwarders; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				config := ForwarderConfig{
					Name:          fmt.Sprintf("forwarder-%d", id),
					SourceSession: "source",
					SourceTopics:  []string{fmt.Sprintf("topic/%d", id)},
					TargetSession: "target",
					QoS:           1,
					BufferSize:    10,
					Enabled:       true,
				}

				// 创建转发器
				forwarder, err := NewForwarder(config, manager)
				if err != nil {
					t.Errorf("Failed to create forwarder %d: %v", id, err)
					return
				}
				atomic.AddInt32(&createdCount, 1)

				// 模拟消息处理
				for j := 0; j < numMessages; j++ {
					forwarder.handleMessage(
						fmt.Sprintf("topic/%d", id),
						[]byte(fmt.Sprintf("message-%d-%d", id, j)),
					)
				}

				// 并发停止
				forwarder.Stop()
				atomic.AddInt32(&stoppedCount, 1)
			}(i)
		}
		wg.Wait()

		if atomic.LoadInt32(&createdCount) != numForwarders {
			t.Errorf("Expected %d forwarders created, got %d", numForwarders, createdCount)
		}

		if atomic.LoadInt32(&stoppedCount) != numForwarders {
			t.Errorf("Expected %d forwarders stopped, got %d", numForwarders, stoppedCount)
		}
	})
}

// TestMemoryLeakDetection 内存泄漏检测测试
func TestMemoryLeakDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过内存泄漏测试（耗时）")
	}

	// 记录初始内存使用
	runtime.GC()
	runtime.GC() // 两次GC确保清理完成

	var initialMemStats runtime.MemStats
	runtime.ReadMemStats(&initialMemStats)

	t.Run("ErrorRecoveryMemoryLeak", func(t *testing.T) {
		manager := NewSessionManager()

		// 创建大量错误然后清理
		for i := 0; i < 10000; i++ {
			err := fmt.Errorf("test error %d", i)
			manager.RegisterError("test-session", err, CategoryConnection)

			if i%1000 == 999 {
				// 定期清理已恢复的错误
				cleared := manager.ClearRecoveredErrors()
				t.Logf("Cleared %d recovered errors at iteration %d", cleared, i)
			}
		}

		// 最终清理
		manager.ClearRecoveredErrors()

		// 强制垃圾回收
		runtime.GC()
		runtime.GC()

		var afterMemStats runtime.MemStats
		runtime.ReadMemStats(&afterMemStats)

		// 检查内存增长
		memGrowth := afterMemStats.Alloc - initialMemStats.Alloc
		t.Logf("Memory growth: %d bytes", memGrowth)

		// 允许一定的内存增长（小于1MB）
		if memGrowth > 1024*1024 {
			t.Errorf("Potential memory leak: %d bytes growth", memGrowth)
		}
	})

	t.Run("ForwarderMemoryLeak", func(t *testing.T) {
		manager := NewSessionManager()

		// 创建和销毁大量转发器
		for i := 0; i < 1000; i++ {
			config := ForwarderConfig{
				Name:          fmt.Sprintf("temp-forwarder-%d", i),
				SourceSession: "source",
				SourceTopics:  []string{"temp/topic"},
				TargetSession: "target",
				QoS:           1,
				BufferSize:    10,
				Enabled:       false,
			}

			forwarder, err := NewForwarder(config, manager)
			if err != nil {
				t.Errorf("Failed to create forwarder %d: %v", i, err)
				continue
			}

			// 模拟使用
			for j := 0; j < 10; j++ {
				forwarder.handleMessage("temp/topic", []byte("test"))
			}

			// 立即停止
			forwarder.Stop()
		}

		runtime.GC()
		runtime.GC()

		var finalMemStats runtime.MemStats
		runtime.ReadMemStats(&finalMemStats)

		memGrowth := finalMemStats.Alloc - initialMemStats.Alloc
		t.Logf("Forwarder memory growth: %d bytes", memGrowth)

		if memGrowth > 2*1024*1024 { // 允许2MB增长
			t.Errorf("Potential forwarder memory leak: %d bytes growth", memGrowth)
		}
	})
}

// BenchmarkMetricsPerformance 性能基准测试
func BenchmarkMetricsPerformance(b *testing.B) {
	metrics := newMetrics()

	b.Run("RecordMessage", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				metrics.recordMessage(1024)
			}
		})
	})

	b.Run("RecordError", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				metrics.recordError()
			}
		})
	})

	b.Run("GetSnapshot", func(b *testing.B) {
		// 预先填充一些数据
		for i := 0; i < 1000; i++ {
			metrics.recordMessage(1024)
		}

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = metrics.getSnapshot()
			}
		})
	})
}

// TestRaceConditionDetection 竞态条件检测（需要-race标志）
func TestRaceConditionDetection(t *testing.T) {
	if !testing.Short() {
		t.Log("运行竞态条件检测测试。使用 'go test -race' 来检测竞态条件")
	}

	manager := NewSessionManager()
	sessionMetrics := newSessionMetrics()

	const numWorkers = 50
	const duration = 2 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var wg sync.WaitGroup

	// 启动多个工作者并发访问指标
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			counter := 0
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// 并发操作
					sessionMetrics.recordMessage(counter%2 == 0, uint64(counter))
					sessionMetrics.recordError(fmt.Errorf("worker %d error %d", workerID, counter))
					sessionMetrics.recordReconnect()

					// 读取指标
					snapshot := sessionMetrics.getSnapshot()
					_ = snapshot

					// 管理器错误操作
					if counter%10 == 0 {
						err := fmt.Errorf("worker %d error %d", workerID, counter)
						manager.RegisterError("test-session", err, CategoryConnection)
					}

					counter++

					// 短暂休眠避免CPU过载
					time.Sleep(time.Microsecond * 10)
				}
			}
		}(i)
	}

	wg.Wait()
	t.Log("竞态条件检测测试完成")
}
