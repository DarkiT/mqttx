package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/darkit/mqttx"
)

func main() {
	// 创建会话管理器
	manager := mqttx.NewSessionManager()
	defer manager.Close()

	// 创建会话选项
	opts := mqttx.DefaultOptions()
	opts.Name = "example-session"
	opts.Brokers = []string{"tcp://broker.emqx.io:1883"} // 公共MQTT代理
	opts.ClientID = "mqttx-example-client"
	opts.Username = "" // 设置如果需要认证
	opts.Password = "" // 设置如果需要认证

	// 添加会话
	err := manager.AddSession(opts)
	if err != nil {
		log.Fatalf("添加会话失败: %v", err)
	}

	// 等待会话连接
	err = manager.WaitForSession("example-session", 5*time.Second)
	if err != nil {
		log.Fatalf("会话连接失败: %v", err)
	}
	log.Println("会话已连接")

	// 订阅主题
	topic := "mqttx/example"
	err = manager.SubscribeTo("example-session", topic, func(t string, payload []byte) {
		log.Printf("收到消息: %s, 主题: %s", string(payload), t)
	}, 1) // QoS 1

	if err != nil {
		log.Printf("订阅失败: %v", err)
	} else {
		log.Printf("已订阅主题: %s", topic)
	}

	// 发布消息
	message := fmt.Sprintf("Hello from MQTTX! Time: %s", time.Now().Format(time.RFC3339))
	err = manager.PublishTo("example-session", topic, []byte(message), 1) // QoS 1

	if err != nil {
		log.Printf("发布失败: %v", err)
	} else {
		log.Printf("已发布消息到主题 %s: %s", topic, message)
	}

	// 使用路由处理消息
	route := manager.Handle("mqttx/events/#", func(msg *mqttx.Message) {
		log.Printf("通过路由处理消息: %s, 主题: %s", msg.PayloadString(), msg.Topic)
	})
	defer route.Stop() // 重要：停止路由

	// 监听事件
	manager.OnEvent(mqttx.EventSessionConnected, func(event mqttx.Event) {
		log.Printf("事件: 会话 %s 已连接", event.Session)
	})

	manager.OnEvent(mqttx.EventSessionDisconnected, func(event mqttx.Event) {
		log.Printf("事件: 会话 %s 已断开连接", event.Session)
	})

	// 定期发布消息
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			message := fmt.Sprintf("定时消息 - %s", time.Now().Format(time.RFC3339))
			err := manager.PublishTo("example-session", topic, []byte(message), 0)
			if err != nil {
				log.Printf("发布定时消息失败: %v", err)
			} else {
				log.Printf("已发布定时消息: %s", message)
			}
		}
	}()
	defer ticker.Stop()

	// 获取会话状态
	session, _ := manager.GetSession("example-session")
	log.Printf("会话状态: %s", session.GetStatus())
	log.Printf("会话客户端ID: %s", session.GetClientID())
	log.Printf("会话连接状态: %v", session.IsConnected())

	// 等待中断信号退出
	log.Println("客户端运行中，按 Ctrl+C 退出...")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("正在断开连接...")
	session.Disconnect()
	log.Println("程序退出")
}
