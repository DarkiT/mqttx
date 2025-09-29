package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/darkit/mqttx"
)

func main() {
	// 创建会话管理器（启用分片锁优化）
	manager := mqttx.NewSessionManager()
	defer manager.Close()

	// 配置持久化会话
	persistentOpts := mqttx.DefaultOptions()
	persistentOpts.Name = "persistent-session"
	persistentOpts.ClientID = "persistent-client-123" // 固定的客户端ID很重要
	persistentOpts.Brokers = []string{"tcp://broker.emqx.io:1883"}

	// 配置持久会话参数
	persistentOpts.ConnectProps.CleanSession = false // 不清除之前的会话
	persistentOpts.ConnectProps.PersistentSession = true
	persistentOpts.ConnectProps.ResumeSubs = true    // 恢复之前的订阅
	persistentOpts.ConnectProps.AutoReconnect = true // 自动重连
	persistentOpts.ConnectProps.MaxReconnectInterval = 30 * time.Second

	// 配置存储选项
	persistentOpts.Storage = &mqttx.StorageOptions{
		Type: mqttx.StoreTypeFile, // 文件存储
		Path: "./mqttx_storage",   // 存储路径
	}

	// 配置性能选项
	persistentOpts.Performance = &mqttx.PerformanceOptions{
		WriteBufferSize:    8192, // 8KB
		ReadBufferSize:     8192, // 8KB
		MessageChanSize:    1000, // 消息通道容量
		MaxPendingMessages: 5000, // 最大待处理消息数
	}

	// 添加持久会话
	err := manager.AddSession(persistentOpts)
	if err != nil {
		log.Printf("添加持久会话失败: %v", err)
	}

	// 配置TLS安全会话
	secureOpts := mqttx.DefaultOptions()
	secureOpts.Name = "secure-session"
	secureOpts.ClientID = "secure-client-123"
	secureOpts.Brokers = []string{"ssl://broker.emqx.io:8883"} // SSL端口

	// 基本TLS配置
	// 注意：在实际使用中，替换为你自己的证书路径
	secureOpts.TLS = &mqttx.TLSConfig{
		// CAFile:   "./ca.pem",     // CA证书
		// CertFile: "./client.crt", // 客户端证书
		// KeyFile:  "./client.key", // 客户端密钥
		SkipVerify: true, // 仅用于示例！生产环境应设为false
	}

	// 增强的TLS配置（可选）
	secureOpts.EnhancedTLS = &mqttx.EnhancedTLSConfig{
		// CAFile:   "./ca.pem",
		// CertFile: "./client.crt",
		// KeyFile:  "./client.key",
		SkipVerify: true, // 仅用于示例！
		MinVersion: tls.VersionTLS12,
		MaxVersion: tls.VersionTLS13,
		// CipherSuites可以指定支持的加密套件
		// AutoReload: true, // 自动重载证书
	}

	// 添加安全会话
	err = manager.AddSession(secureOpts)
	if err != nil {
		log.Printf("添加安全会话失败: %v", err)
	}

	// 等待会话连接
	log.Println("等待会话连接...")
	manager.WaitForAllSessions(10 * time.Second)

	// 列出所有会话
	sessions := manager.ListSessions()
	log.Printf("活跃会话列表: %v", sessions)

	// 获取所有会话状态
	statusMap := manager.GetAllSessionsStatus()
	for name, status := range statusMap {
		log.Printf("会话 %s 状态: %s", name, status)
	}

	// 在所有会话上订阅同一个主题
	errs := manager.SubscribeAll("mqttx/broadcast", func(topic string, payload []byte) {
		log.Printf("[广播] 收到消息: %s, 主题: %s", string(payload), topic)
	}, 1)

	if len(errs) > 0 {
		log.Printf("部分会话订阅失败: %v", errs)
	} else {
		log.Println("所有会话已订阅广播主题")
	}

	// 向所有会话发布消息
	broadcastMsg := fmt.Sprintf("这是一条广播消息 - %s", time.Now().Format(time.RFC3339))
	errs = manager.PublishToAll("mqttx/broadcast", []byte(broadcastMsg), 1)
	if len(errs) > 0 {
		log.Printf("部分会话发布失败: %v", errs)
	}

	// 使用消息通道接收消息
	log.Println("设置消息监听...")
	msgChan, route := manager.Listen("mqttx/stream/#")

	// 处理接收到的消息
	go func() {
		for msg := range msgChan {
			log.Printf("从通道接收消息: %s, 主题: %s", msg.PayloadString(), msg.Topic)
		}
	}()
	defer route.Stop() // 停止路由

	// 演示并发发布消息
	log.Println("开始并发消息发布测试...")
	var wg sync.WaitGroup
	msgCount := 10

	for i := 0; i < msgCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			topic := fmt.Sprintf("mqttx/stream/msg-%d", idx)
			payload := fmt.Sprintf("并发消息 #%d", idx)

			// 随机选择一个会话发布消息
			sessionName := sessions[idx%len(sessions)]
			err := manager.PublishTo(sessionName, topic, []byte(payload), 0)
			if err != nil {
				log.Printf("消息 #%d 发布失败: %v", idx, err)
			}
		}(i)
	}

	// 等待所有消息发布完成
	wg.Wait()
	log.Println("并发消息发布测试完成")

	// 监控会话事件
	manager.OnEvent(mqttx.EventSessionDisconnected, func(event mqttx.Event) {
		log.Printf("事件: 会话 %s 断开连接", event.Session)
	})

	manager.OnEvent(mqttx.EventSessionReconnecting, func(event mqttx.Event) {
		log.Printf("事件: 会话 %s 正在尝试重连", event.Session)
	})

	// 监控性能指标
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// 管理器级别指标
			metrics := manager.GetMetrics()
			log.Printf("性能指标 - 活跃会话: %v, 总消息: %v",
				metrics["active_sessions"], metrics["total_messages"])

			// 会话级别指标
			for _, name := range sessions {
				session, err := manager.GetSession(name)
				if err != nil {
					continue
				}

				sessionMetrics := session.GetMetrics()
				log.Printf("会话 %s 指标 - 已发送: %v, 已接收: %v",
					name, sessionMetrics["messages_sent"],
					sessionMetrics["messages_received"])
			}

			// 内存使用情况
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			log.Printf("内存使用 - Alloc: %v MB, Sys: %v MB",
				m.Alloc/1024/1024, m.Sys/1024/1024)
		}
	}()

	// 等待中断信号退出
	log.Println("高级客户端运行中，按 Ctrl+C 退出...")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("正在优雅关闭...")
	// 取消所有会话的订阅
	manager.UnsubscribeAll("mqttx/broadcast")

	// 断开所有会话连接
	manager.DisconnectAll()

	log.Println("程序退出")
}
