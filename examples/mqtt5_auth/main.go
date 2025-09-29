package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/darkit/mqttx"
)

// 创建一个简单的日志记录器
type simpleLogger struct{}

func (l *simpleLogger) Debug(msg string, keyvals ...interface{}) {
	fmt.Printf("[DEBUG] %s %v\n", msg, keyvals)
}

func (l *simpleLogger) Debugf(format string, args ...interface{}) {
	fmt.Printf("[DEBUG] "+format+"\n", args...)
}

func (l *simpleLogger) Info(msg string, keyvals ...interface{}) {
	fmt.Printf("[INFO] %s %v\n", msg, keyvals)
}

func (l *simpleLogger) Infof(format string, args ...interface{}) {
	fmt.Printf("[INFO] "+format+"\n", args...)
}

func (l *simpleLogger) Warn(msg string, keyvals ...interface{}) {
	fmt.Printf("[WARN] %s %v\n", msg, keyvals)
}

func (l *simpleLogger) Warnf(format string, args ...interface{}) {
	fmt.Printf("[WARN] "+format+"\n", args...)
}

func (l *simpleLogger) Error(msg string, keyvals ...interface{}) {
	fmt.Printf("[ERROR] %s %v\n", msg, keyvals)
}

func (l *simpleLogger) Errorf(format string, args ...interface{}) {
	fmt.Printf("[ERROR] "+format+"\n", args...)
}

func main() {
	// 创建一个会话管理器
	manager := mqttx.NewSessionManager()
	manager.SetLogger(&simpleLogger{})

	// 创建JWT认证提供者
	jwtToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ"
	jwtProvider := mqttx.NewJWTAuthProvider(jwtToken)

	// 创建OAuth 2.0认证提供者
	oauthProvider := mqttx.NewOAuth2AuthProvider(
		"access_token_123",
		"refresh_token_456",
		1*time.Hour,
	)

	// 创建MQTT 5.0配置
	mqtt5Config := mqttx.DefaultMQTT5Config()
	mqtt5Config.SessionExpiry = 30 * time.Minute
	mqtt5Config.UserProperties["app"] = "mqtt5-auth-example"
	mqtt5Config.UserProperties["version"] = "1.0.0"

	// 创建JWT认证会话选项
	jwtOpts := mqttx.DefaultOptions()
	jwtOpts.Name = "jwt-session"
	jwtOpts.Brokers = []string{"tcp://localhost:1883"}
	jwtOpts.ClientID = "mqtt5-jwt-client"
	jwtOpts.WithMQTT5(mqtt5Config)

	// 创建认证管理器
	jwtAuthManager := mqttx.NewMQTT5AuthManager(jwtProvider)

	// 开始认证过程
	method, data, err := jwtAuthManager.StartAuth()
	if err != nil {
		fmt.Printf("JWT认证初始化失败: %v\n", err)
		return
	}

	// 创建认证数据
	jwtAuth := mqttx.NewMQTT5Auth(method, data)
	jwtOpts.WithEnhancedAuth(jwtAuth)

	// 添加JWT认证会话
	err = manager.AddSession(jwtOpts)
	if err != nil {
		fmt.Printf("添加JWT认证会话失败: %v\n", err)
		return
	}

	fmt.Println("已添加JWT认证会话")

	// 创建OAuth 2.0认证会话选项
	oauthOpts := mqttx.DefaultOptions()
	oauthOpts.Name = "oauth-session"
	oauthOpts.Brokers = []string{"tcp://localhost:1883"}
	oauthOpts.ClientID = "mqtt5-oauth-client"
	oauthOpts.WithMQTT5(mqtt5Config)

	// 创建认证管理器
	oauthAuthManager := mqttx.NewMQTT5AuthManager(oauthProvider)

	// 开始认证过程
	method, data, err = oauthAuthManager.StartAuth()
	if err != nil {
		fmt.Printf("OAuth认证初始化失败: %v\n", err)
		return
	}

	// 创建认证数据
	oauthAuth := mqttx.NewMQTT5Auth(method, data)
	oauthOpts.WithEnhancedAuth(oauthAuth)

	// 添加OAuth认证会话
	err = manager.AddSession(oauthOpts)
	if err != nil {
		fmt.Printf("添加OAuth认证会话失败: %v\n", err)
		return
	}

	fmt.Println("已添加OAuth认证会话")

	// 获取JWT会话
	jwtSession, err := manager.GetSession("jwt-session")
	if err != nil {
		fmt.Printf("获取JWT会话失败: %v\n", err)
		return
	}

	// 订阅主题
	err = jwtSession.Subscribe("mqtt5/auth/jwt", func(topic string, payload []byte) {
		fmt.Printf("JWT会话收到消息: %s - %s\n", topic, string(payload))
	}, 1)

	if err != nil {
		fmt.Printf("JWT会话订阅主题失败: %v\n", err)
	} else {
		fmt.Println("JWT会话已订阅主题: mqtt5/auth/jwt")
	}

	// 获取OAuth会话
	oauthSession, err := manager.GetSession("oauth-session")
	if err != nil {
		fmt.Printf("获取OAuth会话失败: %v\n", err)
		return
	}

	// 订阅主题
	err = oauthSession.Subscribe("mqtt5/auth/oauth", func(topic string, payload []byte) {
		fmt.Printf("OAuth会话收到消息: %s - %s\n", topic, string(payload))
	}, 1)

	if err != nil {
		fmt.Printf("OAuth会话订阅主题失败: %v\n", err)
	} else {
		fmt.Println("OAuth会话已订阅主题: mqtt5/auth/oauth")
	}

	// 使用共享订阅
	err = jwtSession.SubscribeShared("auth-group", "mqtt5/auth/shared", func(topic string, payload []byte) {
		fmt.Printf("JWT会话收到共享订阅消息: %s - %s\n", topic, string(payload))
	}, 1)

	if err != nil {
		fmt.Printf("JWT会话订阅共享主题失败: %v\n", err)
	} else {
		fmt.Println("JWT会话已订阅共享主题: mqtt5/auth/shared")
	}

	// 使用共享订阅
	err = oauthSession.SubscribeShared("auth-group", "mqtt5/auth/shared", func(topic string, payload []byte) {
		fmt.Printf("OAuth会话收到共享订阅消息: %s - %s\n", topic, string(payload))
	}, 1)

	if err != nil {
		fmt.Printf("OAuth会话订阅共享主题失败: %v\n", err)
	} else {
		fmt.Println("OAuth会话已订阅共享主题: mqtt5/auth/shared")
	}

	// 发布消息
	err = jwtSession.Publish("mqtt5/auth/jwt", []byte("来自JWT会话的消息"), 1)
	if err != nil {
		fmt.Printf("JWT会话发布消息失败: %v\n", err)
	}

	err = oauthSession.Publish("mqtt5/auth/oauth", []byte("来自OAuth会话的消息"), 1)
	if err != nil {
		fmt.Printf("OAuth会话发布消息失败: %v\n", err)
	}

	// 发布共享订阅消息
	for i := 0; i < 5; i++ {
		err = jwtSession.Publish("mqtt5/auth/shared", []byte(fmt.Sprintf("共享订阅消息 %d", i)), 1)
		if err != nil {
			fmt.Printf("发布共享订阅消息失败: %v\n", err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// 等待信号
	fmt.Println("按 Ctrl+C 退出")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	// 断开连接
	manager.DisconnectAll()
	fmt.Println("已断开所有连接")
}
