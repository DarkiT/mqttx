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

	// 配置源会话(边缘设备)
	sourceOpts := &mqttx.Options{
		Name:     "edge-device",
		Brokers:  []string{"tcp://broker.emqx.io:1883"},
		ClientID: "edge-device-001",
		ConnectProps: &mqttx.ConnectProps{
			KeepAlive:     60,
			CleanSession:  true,
			AutoReconnect: true,
		},
	}

	// 添加源会话
	if err := manager.AddSession(sourceOpts); err != nil {
		log.Fatalf("添加源会话失败: %v", err)
	}

	// 配置目标会话(云服务)
	targetOpts := &mqttx.Options{
		Name:     "cloud-service",
		Brokers:  []string{"tcp://broker.emqx.io:1883"}, // 实际应用中使用云服务地址
		ClientID: "cloud-client-001",
		ConnectProps: &mqttx.ConnectProps{
			KeepAlive:     60,
			CleanSession:  true,
			AutoReconnect: true,
		},
	}

	// 添加目标会话
	if err := manager.AddSession(targetOpts); err != nil {
		log.Fatalf("添加目标会话失败: %v", err)
	}

	// 等待所有会话连接（30秒超时）
	if err := manager.WaitForAllSessions(30 * time.Second); err != nil {
		log.Fatalf("连接会话失败: %v", err)
	}

	// 创建转发器注册表
	registry := mqttx.NewForwarderRegistry(manager)

	// 配置主题映射
	topicMap := map[string]string{
		"sensors/+/temperature": "cloud/data/temperature",
		"sensors/+/humidity":    "cloud/data/humidity",
		"sensors/+/status":      "cloud/status",
	}

	// 创建转发器配置
	config := mqttx.ForwarderConfig{
		Name:          "edge-to-cloud",
		SourceSession: "edge-device",
		SourceTopics: []string{
			"sensors/+/temperature",
			"sensors/+/humidity",
			"sensors/+/status",
		},
		TargetSession: "cloud-service",
		TopicMap:      topicMap,
		QoS:           1,
		BufferSize:    100,
		Enabled:       true,
	}

	// 注册转发器
	_, err := registry.Register(config)
	if err != nil {
		log.Fatalf("注册转发器失败: %v", err)
	}

	// 创建反向控制转发器
	controlConfig := mqttx.ForwarderConfig{
		Name:          "cloud-to-edge",
		SourceSession: "cloud-service",
		SourceTopics:  []string{"cloud/control/#"},
		TargetSession: "edge-device",
		TopicMap: map[string]string{
			"cloud/control/command": "sensors/control",
		},
		QoS:        1,
		BufferSize: 50,
		Enabled:    true,
	}

	// 注册控制转发器
	_, err = registry.Register(controlConfig)
	if err != nil {
		log.Fatalf("注册控制转发器失败: %v", err)
	}

	// 每30秒打印转发器指标
	go func() {
		for {
			time.Sleep(30 * time.Second)
			metrics := registry.GetAllMetrics()

			fmt.Println("转发器指标:")
			for name, data := range metrics {
				fmt.Printf("  %s:\n", name)
				fmt.Printf("    - 接收消息: %d\n", data["received"])
				fmt.Printf("    - 发送消息: %d\n", data["sent"])
				fmt.Printf("    - 丢弃消息: %d\n", data["dropped"])
				fmt.Printf("    - 运行状态: %v\n", data["running"])
			}
			fmt.Println()
		}
	}()

	// 模拟数据发布
	go func() {
		// 等待连接建立
		time.Sleep(2 * time.Second)

		// 每1秒发布一次传感器数据
		for i := 0; i < 10; i++ {
			// 发布温度数据
			manager.PublishTo("edge-device", "sensors/room1/temperature", []byte("24.5"), 1)

			// 发布湿度数据
			manager.PublishTo("edge-device", "sensors/room1/humidity", []byte("68%"), 1)

			// 发布状态数据
			manager.PublishTo("edge-device", "sensors/room1/status", []byte(`{"status":"active"}`), 1)

			fmt.Printf("已发布第%d轮传感器数据\n", i+1)
			time.Sleep(1 * time.Second)

			// 第5轮后，从云端发送控制命令
			if i == 4 {
				manager.PublishTo("cloud-service", "cloud/control/command", []byte(`{"action":"adjust_temperature","value":22}`), 1)
				fmt.Println("已发送云端控制命令")
			}
		}
	}()

	// 等待中断信号
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	<-stop
	fmt.Println("正在关闭应用...")

	// 停止所有转发器
	registry.StopAll()

	// 断开所有会话连接
	manager.DisconnectAll()

	fmt.Println("应用已安全关闭")
}
