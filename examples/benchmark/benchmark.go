package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/darkit/mqttx"
)

// BenchmarkConfig 基准测试配置
type BenchmarkConfig struct {
	BrokerURL    string        // MQTT代理地址
	ClientCount  int           // 客户端数量
	MessageCount int           // 每个客户端发送的消息数
	MessageSize  int           // 消息大小(字节)
	QoS          byte          // 服务质量 (0, 1, 2)
	Interval     time.Duration // 发布间隔
	Parallel     int           // 并行度
	Topic        string        // 测试主题前缀
}

// 统计信息
type Stats struct {
	StartTime        time.Time       // 开始时间
	EndTime          time.Time       // 结束时间
	MessagesSent     int64           // 发送的消息数
	MessagesReceived int64           // 接收的消息数
	PublishErrors    int64           // 发布错误数
	Latencies        []time.Duration // 延迟记录
	mu               sync.Mutex      // 互斥锁
}

func main() {
	// 解析命令行参数
	brokerURL := flag.String("broker", "tcp://broker.emqx.io:1883", "MQTT代理地址")
	clientCount := flag.Int("clients", 10, "客户端数量")
	messageCount := flag.Int("messages", 100, "每个客户端发送的消息数")
	messageSize := flag.Int("size", 256, "消息大小(字节)")
	qos := flag.Int("qos", 0, "服务质量 (0, 1, 2)")
	interval := flag.Int("interval", 100, "发布间隔(毫秒)")
	parallel := flag.Int("parallel", runtime.NumCPU(), "并行度")
	topic := flag.String("topic", "mqttx/benchmark", "测试主题前缀")
	flag.Parse()

	// 创建配置
	config := &BenchmarkConfig{
		BrokerURL:    *brokerURL,
		ClientCount:  *clientCount,
		MessageCount: *messageCount,
		MessageSize:  *messageSize,
		QoS:          byte(*qos),
		Interval:     time.Duration(*interval) * time.Millisecond,
		Parallel:     *parallel,
		Topic:        *topic,
	}

	// 打印配置信息
	log.Printf("基准测试配置:")
	log.Printf("代理地址: %s", config.BrokerURL)
	log.Printf("客户端数量: %d", config.ClientCount)
	log.Printf("每客户端消息数: %d", config.MessageCount)
	log.Printf("消息大小: %d 字节", config.MessageSize)
	log.Printf("QoS: %d", config.QoS)
	log.Printf("发布间隔: %v", config.Interval)
	log.Printf("并行度: %d", config.Parallel)
	log.Printf("总计划消息数: %d", config.ClientCount*config.MessageCount)
	log.Printf("测试主题前缀: %s", config.Topic)

	// 运行测试
	RunBenchmark(config)
}

// 运行基准测试
func RunBenchmark(config *BenchmarkConfig) {
	// 创建消息载荷
	payload := make([]byte, config.MessageSize)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	// 创建统计对象
	stats := &Stats{
		StartTime: time.Now(),
		Latencies: make([]time.Duration, 0, config.ClientCount*config.MessageCount),
	}

	// 创建会话管理器
	manager := mqttx.NewSessionManager()
	defer manager.Close()

	// 创建客户端
	log.Println("正在创建并连接客户端...")
	var wg sync.WaitGroup

	// 限制同时连接的客户端数量
	semaphore := make(chan struct{}, 50)
	for i := 0; i < config.ClientCount; i++ {
		semaphore <- struct{}{}
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			defer func() { <-semaphore }()

			// 创建会话选项
			opts := mqttx.DefaultOptions()
			opts.Name = fmt.Sprintf("bench-%d", clientID)
			opts.ClientID = fmt.Sprintf("bench-client-%d", clientID)
			opts.Brokers = []string{config.BrokerURL}

			// 配置性能选项
			opts.Performance = &mqttx.PerformanceOptions{
				WriteBufferSize:    8192,
				ReadBufferSize:     8192,
				MessageChanSize:    1000,
				MaxPendingMessages: 5000,
			}

			// 添加会话
			err := manager.AddSession(opts)
			if err != nil {
				log.Printf("客户端 %d 创建失败: %v", clientID, err)
				return
			}

			// 等待连接成功
			err = manager.WaitForSession(opts.Name, 5*time.Second)
			if err != nil {
				log.Printf("客户端 %d 连接超时: %v", clientID, err)
				return
			}
		}(i)
	}
	wg.Wait()

	// 检查活跃会话数量
	sessions := manager.ListSessions()
	if len(sessions) < config.ClientCount {
		log.Printf("警告: 只有 %d/%d 客户端连接成功", len(sessions), config.ClientCount)
	} else {
		log.Printf("所有 %d 个客户端已连接", len(sessions))
	}

	// 设置订阅
	log.Println("正在设置订阅...")
	for _, name := range sessions {
		// 为每个客户端创建一个专有主题，避免消息重叠
		subTopic := fmt.Sprintf("%s/%s", config.Topic, name)

		err := manager.SubscribeTo(name, subTopic, func(topic string, payload []byte) {
			// 记录接收时间并计算延迟
			receiveTime := time.Now()

			// 从payload中提取发送时间戳(假设前8字节是int64时间戳)
			if len(payload) >= 8 {
				var sendTimeNano int64
				for i := 0; i < 8; i++ {
					sendTimeNano = (sendTimeNano << 8) | int64(payload[i])
				}
				sendTime := time.Unix(0, sendTimeNano)
				latency := receiveTime.Sub(sendTime)

				// 记录延迟
				stats.mu.Lock()
				stats.Latencies = append(stats.Latencies, latency)
				stats.mu.Unlock()
			}

			atomic.AddInt64(&stats.MessagesReceived, 1)
		}, config.QoS)
		if err != nil {
			log.Printf("客户端 %s 订阅失败: %v", name, err)
		}
	}

	// 等待一秒确保订阅完成
	time.Sleep(1 * time.Second)

	// 开始发布消息
	log.Println("开始发布消息...")
	stats.StartTime = time.Now()

	// 创建工作池限制并行度
	workerPool := make(chan struct{}, config.Parallel)

	wg = sync.WaitGroup{}
	for _, name := range sessions {
		clientName := name // 捕获循环变量

		for msgIdx := 0; msgIdx < config.MessageCount; msgIdx++ {
			wg.Add(1)
			workerPool <- struct{}{} // 获取工作槽

			go func(clientName string, msgIdx int) {
				defer wg.Done()
				defer func() { <-workerPool }() // 释放工作槽

				// 创建消息负载，前8字节为时间戳
				messagePayload := make([]byte, config.MessageSize)
				copy(messagePayload, payload)

				// 在消息前8字节中嵌入当前时间纳秒，用于计算延迟
				sendTime := time.Now().UnixNano()
				for i := 0; i < 8; i++ {
					messagePayload[7-i] = byte(sendTime)
					sendTime >>= 8
				}

				// 发布到这个客户端的专有主题
				pubTopic := fmt.Sprintf("%s/%s", config.Topic, clientName)
				err := manager.PublishTo(clientName, pubTopic, messagePayload, config.QoS)

				if err != nil {
					atomic.AddInt64(&stats.PublishErrors, 1)
				} else {
					atomic.AddInt64(&stats.MessagesSent, 1)
				}

				// 按照配置的间隔休眠
				if config.Interval > 0 {
					time.Sleep(config.Interval)
				}
			}(clientName, msgIdx)
		}
	}

	// 定期显示进度
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		expectedTotal := int64(len(sessions) * config.MessageCount)

		for range ticker.C {
			sent := atomic.LoadInt64(&stats.MessagesSent)
			received := atomic.LoadInt64(&stats.MessagesReceived)
			errors := atomic.LoadInt64(&stats.PublishErrors)

			if sent > 0 {
				log.Printf("进度: 已发送 %d/%d (%.1f%%) 已接收 %d (%.1f%%) 错误 %d",
					sent, expectedTotal, float64(sent)/float64(expectedTotal)*100,
					received, float64(received)/float64(expectedTotal)*100, errors)
			}

			// 如果所有消息都已发送和接收，则退出
			if sent >= expectedTotal && received >= expectedTotal {
				return
			}

			// 如果已经结束但还没有达到目标，也退出
			if sent == atomic.LoadInt64(&stats.MessagesSent) &&
				received == atomic.LoadInt64(&stats.MessagesReceived) &&
				time.Since(stats.StartTime) > time.Duration(30*time.Second) {
				return
			}
		}
	}()

	// 等待所有发布完成
	wg.Wait()

	// 等待几秒钟接收剩余消息
	deadline := time.After(10 * time.Second)
	expectedTotal := int64(len(sessions) * config.MessageCount)
	checkInterval := time.NewTicker(100 * time.Millisecond)
	defer checkInterval.Stop()

checkLoop:
	for {
		received := atomic.LoadInt64(&stats.MessagesReceived)
		if received >= expectedTotal {
			break
		}

		select {
		case <-deadline:
			log.Printf("等待接收超时，部分消息可能未到达")
			break checkLoop
		case <-checkInterval.C:
			continue
		}
	}

	// 记录结束时间
	stats.EndTime = time.Now()

	// 断开所有连接
	log.Println("测试完成，断开连接...")
	manager.DisconnectAll()

	// 计算并打印结果
	PrintResults(stats, config)
}

// 打印测试结果
func PrintResults(stats *Stats, config *BenchmarkConfig) {
	duration := stats.EndTime.Sub(stats.StartTime)
	messagesSent := atomic.LoadInt64(&stats.MessagesSent)
	messagesReceived := atomic.LoadInt64(&stats.MessagesReceived)
	publishErrors := atomic.LoadInt64(&stats.PublishErrors)

	log.Printf("\n==== 基准测试结果 ====")
	log.Printf("测试持续时间: %v", duration)
	log.Printf("已发送消息: %d", messagesSent)
	log.Printf("已接收消息: %d", messagesReceived)
	log.Printf("发布错误: %d", publishErrors)

	if messagesSent > 0 {
		log.Printf("发送速率: %.2f 消息/秒", float64(messagesSent)/duration.Seconds())
		log.Printf("发送吞吐量: %.2f KB/s", float64(messagesSent*int64(config.MessageSize))/(1024*duration.Seconds()))
	}

	if messagesReceived > 0 {
		log.Printf("接收速率: %.2f 消息/秒", float64(messagesReceived)/duration.Seconds())
		log.Printf("接收吞吐量: %.2f KB/s", float64(messagesReceived*int64(config.MessageSize))/(1024*duration.Seconds()))
	}

	// 计算延迟统计
	if len(stats.Latencies) > 0 {
		// 计算平均延迟
		var totalLatency time.Duration
		for _, lat := range stats.Latencies {
			totalLatency += lat
		}
		avgLatency := totalLatency / time.Duration(len(stats.Latencies))

		// 计算最小和最大延迟
		minLatency := stats.Latencies[0]
		maxLatency := stats.Latencies[0]
		for _, lat := range stats.Latencies {
			if lat < minLatency {
				minLatency = lat
			}
			if lat > maxLatency {
				maxLatency = lat
			}
		}

		log.Printf("平均延迟: %v", avgLatency)
		log.Printf("最小延迟: %v", minLatency)
		log.Printf("最大延迟: %v", maxLatency)
	}

	// 显示内存使用情况
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	log.Printf("内存使用 - Alloc: %v MB, Sys: %v MB",
		m.Alloc/1024/1024, m.Sys/1024/1024)

	log.Printf("==== 测试完成 ====\n")

	// 将结果保存到文件
	saveResultsToFile(stats, config)
}

// 将结果保存到文件
func saveResultsToFile(stats *Stats, config *BenchmarkConfig) {
	// 创建结果文件
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("benchmark-%s.txt", timestamp)
	f, err := os.Create(filename)
	if err != nil {
		log.Printf("无法创建结果文件: %v", err)
		return
	}
	defer f.Close()

	// 写入配置信息
	fmt.Fprintf(f, "==== MQTTX 基准测试结果 ====\n")
	fmt.Fprintf(f, "时间: %s\n", timestamp)
	fmt.Fprintf(f, "\n==== 配置 ====\n")
	fmt.Fprintf(f, "代理地址: %s\n", config.BrokerURL)
	fmt.Fprintf(f, "客户端数量: %d\n", config.ClientCount)
	fmt.Fprintf(f, "每客户端消息数: %d\n", config.MessageCount)
	fmt.Fprintf(f, "消息大小: %d 字节\n", config.MessageSize)
	fmt.Fprintf(f, "QoS: %d\n", config.QoS)
	fmt.Fprintf(f, "发布间隔: %v\n", config.Interval)
	fmt.Fprintf(f, "并行度: %d\n", config.Parallel)

	// 写入结果
	duration := stats.EndTime.Sub(stats.StartTime)
	messagesSent := atomic.LoadInt64(&stats.MessagesSent)
	messagesReceived := atomic.LoadInt64(&stats.MessagesReceived)
	publishErrors := atomic.LoadInt64(&stats.PublishErrors)

	fmt.Fprintf(f, "\n==== 结果 ====\n")
	fmt.Fprintf(f, "测试持续时间: %v\n", duration)
	fmt.Fprintf(f, "已发送消息: %d\n", messagesSent)
	fmt.Fprintf(f, "已接收消息: %d\n", messagesReceived)
	fmt.Fprintf(f, "发布错误: %d\n", publishErrors)

	if messagesSent > 0 {
		fmt.Fprintf(f, "发送速率: %.2f 消息/秒\n", float64(messagesSent)/duration.Seconds())
		fmt.Fprintf(f, "发送吞吐量: %.2f KB/s\n", float64(messagesSent*int64(config.MessageSize))/(1024*duration.Seconds()))
	}

	if messagesReceived > 0 {
		fmt.Fprintf(f, "接收速率: %.2f 消息/秒\n", float64(messagesReceived)/duration.Seconds())
		fmt.Fprintf(f, "接收吞吐量: %.2f KB/s\n", float64(messagesReceived*int64(config.MessageSize))/(1024*duration.Seconds()))
	}

	// 延迟统计
	if len(stats.Latencies) > 0 {
		var totalLatency time.Duration
		for _, lat := range stats.Latencies {
			totalLatency += lat
		}
		avgLatency := totalLatency / time.Duration(len(stats.Latencies))

		minLatency := stats.Latencies[0]
		maxLatency := stats.Latencies[0]
		for _, lat := range stats.Latencies {
			if lat < minLatency {
				minLatency = lat
			}
			if lat > maxLatency {
				maxLatency = lat
			}
		}

		fmt.Fprintf(f, "平均延迟: %v\n", avgLatency)
		fmt.Fprintf(f, "最小延迟: %v\n", minLatency)
		fmt.Fprintf(f, "最大延迟: %v\n", maxLatency)
	}

	// 内存统计
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Fprintf(f, "内存使用 - Alloc: %v MB, Sys: %v MB\n",
		m.Alloc/1024/1024, m.Sys/1024/1024)

	log.Printf("结果已保存到文件: %s", filename)
}
