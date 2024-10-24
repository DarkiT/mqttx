package main

import (
	"encoding/json"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	manager "github.com/darkit/mqtt"
	"github.com/darkit/slog"
)

// SensorData 传感器数据结构
type SensorData struct {
	DeviceID    string    `json:"device_id"`
	Temperature float64   `json:"temperature"`
	Humidity    float64   `json:"humidity"`
	Timestamp   time.Time `json:"timestamp"`
}

func init() {
	slog.SetLevelDebug()
	slog.NewLogger(os.Stdout, true, false)
}

func main() {
	// 创建会话管理器
	m := manager.NewSessionManager()

	// 设置自定义日志记录器
	m.SetLogger(slog.Default("mqtt"))

	// 创建信号通道用于优雅退出
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 创建存储目录
	storageDir := filepath.Join(os.TempDir(), "mqtt-sessions")
	os.MkdirAll(storageDir, 0755)

	// 1. 添加持久化会话
	persistentOpts := &manager.Options{
		Name:        "persistent-device",
		Brokers:     []string{"tcp://broker.emqx.io:1883"},
		ClientID:    "device-001",
		Username:    "user",
		Password:    "pass",
		StoragePath: storageDir,
		ConnectProps: &manager.ConnectProps{
			KeepAlive:         60,
			CleanSession:      false,
			AutoReconnect:     true,
			PersistentSession: true,
			ResumeSubs:        true,
		},
		Performance: &manager.PerformanceOptions{
			MessageChanSize:    1000,
			MaxPendingMessages: 5000,
			WriteTimeout:       time.Second * 30,
			ReadTimeout:        time.Second * 30,
		},
	}

	if err := m.AddSession(persistentOpts); err != nil {
		slog.Fatalf("Failed to add persistent session: %v", err)
	}

	// 2. 添加临时会话
	tempOpts := &manager.Options{
		Name:     "temp-device",
		Brokers:  []string{"tcp://broker.emqx.io:1883"},
		ClientID: "device-002",
		ConnectProps: &manager.ConnectProps{
			CleanSession:  true,
			AutoReconnect: true,
		},
	}

	if err := m.AddSession(tempOpts); err != nil {
		slog.Fatalf("Failed to add temporary session: %v", err)
	}

	// 等待特定会话连接
	//if err := m.WaitForSession("test-session", 30*time.Second); err != nil {
	//	log.Fatal(err)
	//}

	// 或者等待所有会话连接
	slog.Println("Waiting for all sessions to connect...")
	if err := m.WaitForAllSessions(30 * time.Second); err != nil {
		slog.Fatalf("Failed while waiting for sessions: %v", err)
	}
	slog.Println("All sessions connected successfully")

	// 3. 注册事件处理
	m.OnEvent("session_ready", func(event manager.Event) {
		data := event.Data.(map[string]interface{})
		slog.Printf("Session %s is ready:", event.Session)
		slog.Printf("  Connected to: %v", data["connected_broker"])
		slog.Printf("  Client ID: %v", data["client_id"])
		slog.Printf("  Subscriptions: %v", data["subscriptions"])
	})

	m.OnEvent("session_connected", func(event manager.Event) {
		slog.Printf("Session connected: %s", event.Session)
	})

	m.OnEvent("session_disconnected", func(event manager.Event) {
		slog.Printf("Session disconnected: %s, reason: %v", event.Session, event.Data)
	})

	m.OnEvent("session_reconnecting", func(event manager.Event) {
		slog.Printf("Session reconnecting: %s", event.Session)
	})

	// 4. 使用Handle模式处理消息
	handleRoute := m.Handle("sensors/+/temperature", func(msg *manager.Message) {
		var data SensorData
		if err := msg.PayloadJSON(&data); err != nil {
			slog.Printf("Failed to parse message: %v", err)
			return
		}
		slog.Printf("Received temperature from %s: %.1f°C", data.DeviceID, data.Temperature)
	})
	defer handleRoute.Stop()

	// 5. 使用HandleTo模式处理特定会话的消息
	handleToRoute, err := m.HandleTo("persistent-device", "sensors/+/humidity", func(msg *manager.Message) {
		var data SensorData
		if err := msg.PayloadJSON(&data); err != nil {
			slog.Printf("Failed to parse message: %v", err)
			return
		}
		slog.Printf("Received humidity from %s: %.1f%%", data.DeviceID, data.Humidity)
	})
	if err != nil {
		slog.Printf("Failed to setup HandleTo: %v", err)
	}
	defer handleToRoute.Stop()

	// 6. 使用Listen模式接收消息
	messages, listenRoute := m.Listen("sensors/#")
	go func() {
		for msg := range messages {
			slog.Printf("Listen received message on topic %s: %s", msg.Topic, msg.PayloadString())
		}
	}()
	defer listenRoute.Stop()

	// 7. 使用ListenTo模式接收特定会话的消息
	sessionMessages, listenToRoute, err := m.ListenTo("temp-device", "control/#")
	if err != nil {
		slog.Printf("Failed to setup ListenTo: %v", err)
	} else {
		go func() {
			for msg := range sessionMessages {
				slog.Printf("ListenTo received message on topic %s: %s", msg.Topic, msg.PayloadString())
			}
		}()
		defer listenToRoute.Stop()
	}

	// 8. 定期发布消息的示例
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// 创建示例数据
				data := SensorData{
					DeviceID:    "sensor-001",
					Temperature: 25.5,
					Humidity:    60.0,
					Timestamp:   time.Now(),
				}
				payload, _ := json.Marshal(data)

				// 发布到特定会话
				if err := m.PublishTo("persistent-device", "sensors/001/temperature", payload, 1); err != nil {
					slog.Printf("Failed to publish to session: %v", err)
				}

				// 发布到所有会话
				if errors := m.PublishToAll("sensors/broadcast", payload, 0); len(errors) > 0 {
					slog.Printf("Failed to publish to all sessions: %v", errors)
				}

				// 打印当前状态
				status := m.GetAllSessionsStatus()
				slog.Printf("Current session status: %v", status)

				// 获取指标
				metrics := m.GetMetrics()
				slog.Warn("Manager Metrics:")
				slog.Warnf("  Active Sessions: %d", metrics["active_sessions"])
				slog.Warnf("  Total Messages: %d", metrics["total_messages"])
				slog.Warnf("  Total Data: %s", metrics["total_bytes"])
				slog.Warnf("  Message Rate: %s", metrics["message_rate"])
				slog.Warnf("  Data Rate: %s", metrics["bytes_rate"])
				slog.Warnf("  Errors: %d", metrics["error_count"])
				slog.Warnf("  Reconnects: %d", metrics["reconnect_count"])
				slog.Warnf("  Last Update: %s", metrics["last_update"])
				slog.Warnf("  Uptime: %s", metrics["uptime"])

				// 打印每个会话的指标
				sessions := m.ListSessions()
				for _, name := range sessions {
					if session, err := m.GetSession(name); err == nil {
						sessionMetrics := session.GetMetrics()
						slog.Warnf("Session Metrics for %s:", name)
						slog.Warnf("  Messages Sent: %d", sessionMetrics["messages_sent"])
						slog.Warnf("  Messages Received: %d", sessionMetrics["messages_received"])
						slog.Warnf("  Data Sent: %d", sessionMetrics["bytes_sent"])
						slog.Warnf("  Data Received: %d", sessionMetrics["bytes_received"])
						slog.Warnf("  Errors: %d", sessionMetrics["errors"])
						slog.Warnf("  Reconnects: %d", sessionMetrics["reconnects"])
						slog.Warnf("  Last Message: %s", sessionMetrics["last_message"].(time.Time).Format(time.DateTime))
						if lastError, ok := sessionMetrics["last_error"].(time.Time); ok && !lastError.IsZero() {
							slog.Warnf("  Last Error: %s", lastError.Format(time.DateTime))
						}
					}
				}

			case <-sigChan:
				slog.Println("Shutting down publisher...")
				return
			}
		}
	}()

	// 9. 演示其他管理功能
	go func() {
		// 列出所有会话
		sessions := m.ListSessions()
		slog.Printf("Active sessions: %v", sessions)

		// 获取特定会话
		if session, err := m.GetSession("persistent-device"); err == nil {
			metrics := session.GetMetrics()
			slog.Warnf("Session Metrics for %s:", "persistent-device")
			slog.Warnf("  Messages Sent: %d", metrics["messages_sent"])
			slog.Warnf("  Messages Received: %d", metrics["messages_received"])
			slog.Warnf("  Data Sent: %d", metrics["bytes_sent"])
			slog.Warnf("  Data Received: %d", metrics["bytes_received"])
			slog.Warnf("  Errors: %d", metrics["errors"])
			slog.Warnf("  Reconnects: %d", metrics["reconnects"])
			slog.Warnf("  Last Message: %s", metrics["last_message"].(time.Time).Format(time.DateTime))
			if lastError, ok := metrics["last_error"].(time.Time); ok && !lastError.IsZero() {
				slog.Warnf("  Last Error: %s", lastError.Format(time.DateTime))
			}
		}

		// 订阅示例
		topic := "example/topic"
		qos := byte(1)

		// 为所有会话订阅
		if errors := m.SubscribeAll(topic, func(t string, payload []byte) {
			slog.Printf("SubscribeAll received: %s", string(payload))
		}, qos); len(errors) > 0 {
			slog.Printf("SubscribeAll errors: %v", errors)
		}

		// 为特定会话订阅
		if err := m.SubscribeTo("persistent-device", topic, func(t string, payload []byte) {
			slog.Printf("SubscribeTo received: %s", string(payload))
		}, qos); err != nil {
			slog.Printf("SubscribeTo error: %v", err)
		}
	}()

	// 等待中断信号
	<-sigChan
	slog.Println("Shutting down...")

	// 10. 清理资源
	if err := m.RemoveSession("temp-device"); err != nil {
		slog.Printf("Failed to remove temp session: %v", err)
	}

	// 断开所有连接
	m.DisconnectAll()

	// 关闭管理器
	m.Close()

	// 等待发布器关闭
	wg.Wait()
}
