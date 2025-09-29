package mqttx

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

// BenchmarkAtomicMetrics 测试原子操作的性能提升
func BenchmarkAtomicMetrics(b *testing.B) {
	metrics := newMetrics()
	sessionMetrics := newSessionMetrics()

	b.Run("GlobalMetricsRecordMessage", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				metrics.recordMessage(1024)
			}
		})
	})

	b.Run("GlobalMetricsRecordError", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				metrics.recordError()
			}
		})
	})

	b.Run("SessionMetricsRecordMessage", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				sessionMetrics.recordMessage(i%2 == 0, 1024)
				i++
			}
		})
	})

	b.Run("GetSnapshot", func(b *testing.B) {
		// 预先填充一些数据
		for i := 0; i < 100; i++ {
			metrics.recordMessage(1024)
			sessionMetrics.recordMessage(i%2 == 0, 512)
		}

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = metrics.getSnapshot()
			}
		})
	})
}

// BenchmarkForwarderOperations 测试转发器操作性能
func BenchmarkForwarderOperations(b *testing.B) {
	manager := NewSessionManager()

	config := ForwarderConfig{
		Name:          "bench-forwarder",
		SourceSession: "source",
		SourceTopics:  []string{"bench/topic"},
		TargetSession: "target",
		QoS:           1,
		BufferSize:    1000,
		Enabled:       false,
	}

	forwarder, err := NewForwarder(config, manager)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("HandleMessage", func(b *testing.B) {
		payload := []byte("benchmark test message")

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				forwarder.handleMessage("bench/topic", payload)
			}
		})
	})

	b.Run("GetMetrics", func(b *testing.B) {
		// 预先处理一些消息
		payload := []byte("test")
		for i := 0; i < 100; i++ {
			forwarder.handleMessage("bench/topic", payload)
		}

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = forwarder.GetMetrics()
			}
		})
	})
}

// TestPerformanceImprovement 性能改进验证测试
func TestPerformanceImprovement(t *testing.T) {
	const numOperations = 100000
	const numGoroutines = 10

	// 测试并发指标操作性能
	t.Run("ConcurrentMetricsPerformance", func(t *testing.T) {
		metrics := newMetrics()

		start := time.Now()

		var wg sync.WaitGroup
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < numOperations/numGoroutines; j++ {
					metrics.recordMessage(1024)
					metrics.recordError()
					_ = metrics.getSnapshot()
				}
			}()
		}

		wg.Wait()
		duration := time.Since(start)

		opsPerSec := float64(numOperations*3) / duration.Seconds() // 3 operations per iteration
		t.Logf("Metrics operations: %.0f ops/sec", opsPerSec)

		// 期望性能：至少100k ops/sec
		if opsPerSec < 100000 {
			t.Errorf("Performance below expectation: %.0f ops/sec < 100k ops/sec", opsPerSec)
		}
	})

	// 测试转发器创建销毁性能
	t.Run("ForwarderLifecyclePerformance", func(t *testing.T) {
		manager := NewSessionManager()

		start := time.Now()

		var wg sync.WaitGroup
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 100; j++ { // 每个协程创建100个转发器
					config := ForwarderConfig{
						Name:          "bench-forwarder-" + string(rune(id*100+j)),
						SourceSession: "source",
						SourceTopics:  []string{"topic"},
						TargetSession: "target",
						QoS:           1,
						BufferSize:    10,
						Enabled:       false,
					}

					forwarder, err := NewForwarder(config, manager)
					if err != nil {
						t.Errorf("Failed to create forwarder: %v", err)
						continue
					}

					// 处理一些消息
					for k := 0; k < 10; k++ {
						forwarder.handleMessage("topic", []byte("test"))
					}

					// 停止转发器
					forwarder.Stop()
				}
			}(i)
		}

		wg.Wait()
		duration := time.Since(start)

		totalForwarders := numGoroutines * 100
		forwardersPerSec := float64(totalForwarders) / duration.Seconds()
		t.Logf("Forwarder lifecycle: %.0f forwarders/sec", forwardersPerSec)

		// 期望性能：至少1000 forwarders/sec
		if forwardersPerSec < 1000 {
			t.Errorf("Forwarder performance below expectation: %.0f < 1000 forwarders/sec", forwardersPerSec)
		}
	})
}

// TestMemoryEfficiency 内存效率测试
func TestMemoryEfficiency(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过内存效率测试")
	}

	// 强制垃圾回收
	runtime.GC()
	runtime.GC()

	var initialMemStats runtime.MemStats
	runtime.ReadMemStats(&initialMemStats)

	// 创建大量指标对象
	const numMetrics = 1000
	metrics := make([]*Metrics, numMetrics)
	sessionMetrics := make([]*SessionMetrics, numMetrics)

	for i := 0; i < numMetrics; i++ {
		metrics[i] = newMetrics()
		sessionMetrics[i] = newSessionMetrics()

		// 填充一些数据
		for j := 0; j < 100; j++ {
			metrics[i].recordMessage(1024)
			sessionMetrics[i].recordMessage(j%2 == 0, 512)
		}
	}

	runtime.GC()
	runtime.GC()

	var afterCreateMemStats runtime.MemStats
	runtime.ReadMemStats(&afterCreateMemStats)

	memUsedPerMetric := (afterCreateMemStats.Alloc - initialMemStats.Alloc) / uint64(numMetrics*2)
	t.Logf("Memory per metrics object: %d bytes", memUsedPerMetric)

	// 期望每个指标对象使用不超过1KB内存
	if memUsedPerMetric > 1024 {
		t.Errorf("Memory usage too high: %d bytes per metric > 1024 bytes", memUsedPerMetric)
	}

	// 清理对象
	metrics = nil
	sessionMetrics = nil

	runtime.GC()
	runtime.GC()

	var finalMemStats runtime.MemStats
	runtime.ReadMemStats(&finalMemStats)

	// 检查内存是否被正确释放（允许一定的误差）
	memRetained := finalMemStats.Alloc - initialMemStats.Alloc
	t.Logf("Memory retained after cleanup: %d bytes", memRetained)

	if memRetained > 1024*1024 { // 允许1MB的内存保留
		t.Errorf("Too much memory retained: %d bytes > 1MB", memRetained)
	}
}
