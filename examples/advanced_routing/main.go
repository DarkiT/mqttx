package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/darkit/mqttx"
)

// 定义一个简单的消息结构体
type SensorReading struct {
	Device      string  `json:"device"`
	Temperature float64 `json:"temperature"`
	Humidity    float64 `json:"humidity"`
	Timestamp   int64   `json:"timestamp"`
}

func main() {
	// 创建MQTTX会话管理器
	manager := mqttx.NewSessionManager()
	defer manager.Close()

	// 设置MQTT会话选项
	opts := mqttx.DefaultOptions()
	opts.Name = "advanced-routing-demo"
	opts.Brokers = []string{"tcp://localhost:1883"}
	opts.ClientID = "mqttx-demo-advanced-routing"

	// 添加会话
	err := manager.AddSession(opts)
	if err != nil {
		fmt.Printf("Failed to add session: %v\n", err)
		os.Exit(1)
	}

	// 等待连接建立
	fmt.Println("Connecting to MQTT broker...")

	// 等待连接就绪
	err = manager.WaitForSession("advanced-routing-demo", 5*time.Second)
	if err != nil {
		fmt.Println("Failed to connect to MQTT broker:", err)
		os.Exit(1)
	}
	fmt.Println("Connected to MQTT broker")

	// 创建主题过滤器 - 只匹配sensor/+/temperature主题
	topicFilter, err := mqttx.NewTopicFilter("sensor/+/temperature", false)
	if err != nil {
		fmt.Printf("Failed to create topic filter: %v\n", err)
		os.Exit(1)
	}

	// 创建负载过滤器 - 只匹配温度高于25度的消息
	highTempFilter := mqttx.NewPayloadTransformer("High Temperature Filter", func(payload []byte) ([]byte, error) {
		var reading SensorReading
		if err := json.Unmarshal(payload, &reading); err != nil {
			return nil, err
		}

		// 如果温度低于25度，返回nil表示过滤掉这条消息
		if reading.Temperature < 25.0 {
			return nil, nil
		}
		return payload, nil
	})

	// 创建主题重写转换器 - 将主题格式从sensor/X/temperature改为high-temp/X
	topicRewriter, err := mqttx.NewTopicRewriteTransformer(
		"sensor/([^/]+)/temperature",
		"high-temp/$1",
		true,
	)
	if err != nil {
		fmt.Printf("Failed to create topic rewriter: %v\n", err)
		os.Exit(1)
	}

	// 创建JSON负载转换器 - 添加警告标记
	addWarningTransformer := mqttx.NewPayloadTransformer("Add Warning Flag", func(payload []byte) ([]byte, error) {
		var reading SensorReading
		if err := json.Unmarshal(payload, &reading); err != nil {
			return nil, err
		}

		// 添加一个警告标记到JSON中
		data := map[string]interface{}{
			"device":      reading.Device,
			"temperature": reading.Temperature,
			"humidity":    reading.Humidity,
			"timestamp":   reading.Timestamp,
			"warning":     "High temperature detected!",
		}

		return json.Marshal(data)
	})

	// 创建高级路由配置
	routeConfig := mqttx.RouteConfig{
		Topic:        "sensor/+/temperature",
		QoS:          1,
		Filters:      []mqttx.MessageFilter{topicFilter},                                               // 使用主题过滤器
		Transformers: []mqttx.MessageTransformer{highTempFilter, topicRewriter, addWarningTransformer}, // 使用转换器链
		BufferSize:   10,
		Description:  "High Temperature Alert Route",
		SessionName:  "advanced-routing-demo", // 指定会话名称
	}

	// 处理高温消息
	manager.HandleWithConfig(routeConfig, func(msg *mqttx.Message) {
		fmt.Printf("高温警告 - 主题: %s\n", msg.Topic)

		var data map[string]interface{}
		if err := json.Unmarshal(msg.Payload, &data); err != nil {
			fmt.Printf("解析JSON失败: %v\n", err)
			return
		}

		fmt.Printf("  设备: %s\n", data["device"])
		fmt.Printf("  温度: %.1f\n", data["temperature"])
		fmt.Printf("  湿度: %.1f\n", data["humidity"])
		fmt.Printf("  警告: %s\n", data["warning"])
		fmt.Println()
	})

	// 同时监听所有温度消息以便比较
	manager.SubscribeTo("advanced-routing-demo", "sensor/#", func(topic string, payload []byte) {
		var reading SensorReading
		if err := json.Unmarshal(payload, &reading); err != nil {
			return
		}
		fmt.Printf("收到消息 - 主题: %s, 设备: %s, 温度: %.1f\n",
			topic, reading.Device, reading.Temperature)
	}, 1)

	// 发布一些测试消息
	go func() {
		devices := []string{"livingroom", "kitchen", "bedroom"}
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			for _, device := range devices {
				// 随机温度，有些高有些低
				temp := 20.0 + float64(time.Now().UnixNano()%10)
				humid := 50.0 + float64(time.Now().UnixNano()%20)

				reading := SensorReading{
					Device:      device,
					Temperature: temp,
					Humidity:    humid,
					Timestamp:   time.Now().Unix(),
				}

				payload, _ := json.Marshal(reading)
				topic := fmt.Sprintf("sensor/%s/temperature", device)

				manager.PublishTo("advanced-routing-demo", topic, payload, 1)
			}
		}
	}()

	// 等待信号退出
	fmt.Println("运行中... 按Ctrl+C退出")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	fmt.Println("正在退出...")
}
