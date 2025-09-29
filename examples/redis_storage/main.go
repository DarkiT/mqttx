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

// 定义日志适配器
type StdLogger struct {
	logger *log.Logger
}

func NewStdLogger(logger *log.Logger) *StdLogger {
	return &StdLogger{logger: logger}
}

func (l *StdLogger) Debug(msg string, args ...any) {
	l.logger.Printf("[DEBUG] %s %v", msg, args)
}

func (l *StdLogger) Debugf(format string, args ...any) {
	l.logger.Printf("[DEBUG] "+format, args...)
}

func (l *StdLogger) Info(msg string, args ...any) {
	l.logger.Printf("[INFO] %s %v", msg, args)
}

func (l *StdLogger) Infof(format string, args ...any) {
	l.logger.Printf("[INFO] "+format, args...)
}

func (l *StdLogger) Warn(msg string, args ...any) {
	l.logger.Printf("[WARN] %s %v", msg, args)
}

func (l *StdLogger) Warnf(format string, args ...any) {
	l.logger.Printf("[WARN] "+format, args...)
}

func (l *StdLogger) Error(msg string, args ...any) {
	l.logger.Printf("[ERROR] %s %v", msg, args)
}

func (l *StdLogger) Errorf(format string, args ...any) {
	l.logger.Printf("[ERROR] "+format, args...)
}

func main() {
	// 创建会话管理器
	m := mqttx.NewSessionManager()

	// 设置日志
	logger := log.New(os.Stdout, "[MQTTX] ", log.LstdFlags)
	m.SetLogger(NewStdLogger(logger))

	// 配置Redis存储选项
	opts := mqttx.DefaultOptions()
	opts.Name = "redis-test"
	opts.Brokers = []string{"tcp://localhost:1883"} // 使用本地MQTT broker
	opts.ClientID = "redis-storage-client"

	// 启用持久会话
	opts.ConnectProps.PersistentSession = true

	// 配置Redis存储
	opts.WithRedisStorage(
		"localhost:6379", // Redis地址
		"",               // 用户名
		"",               // 密码
		0,                // 数据库
		"mqttx:test:",    // 键前缀
		3600,             // TTL（秒）
	)

	// 添加会话
	if err := m.AddSession(opts); err != nil {
		log.Fatalf("Failed to add session: %v", err)
	}

	// 等待会话连接
	if err := m.WaitForSession("redis-test", 10*time.Second); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	log.Println("Connected successfully")

	// 订阅主题
	m.HandleTo("redis-test", "test/topic", func(msg *mqttx.Message) {
		log.Printf("Received message on %s: %s", msg.Topic, string(msg.Payload))
	})

	// 发布消息
	go func() {
		count := 0
		for range time.Tick(2 * time.Second) {
			count++
			msg := fmt.Sprintf("Hello World %d", count)
			if err := m.PublishTo("redis-test", "test/topic", []byte(msg), 1); err != nil {
				log.Printf("Failed to publish: %v", err)
			} else {
				log.Printf("Published: %s", msg)
			}

			// 发送10条消息后退出
			if count >= 10 {
				break
			}
		}
	}()

	// 监听会话事件
	m.OnEvent("session_disconnected", func(event mqttx.Event) {
		log.Printf("Session disconnected: %s", event.Session)
	})

	m.OnEvent("session_reconnecting", func(event mqttx.Event) {
		log.Printf("Session reconnecting: %s", event.Session)
	})

	m.OnEvent("session_connected", func(event mqttx.Event) {
		log.Printf("Session reconnected: %s", event.Session)
	})

	// 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	// 正常关闭
	m.RemoveSession("redis-test")
	log.Println("Session removed, exiting")
}
