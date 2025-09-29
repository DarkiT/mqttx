package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/darkit/mqttx"
)

func main() {
	// 创建MQTTX会话管理器
	manager := mqttx.NewSessionManager()
	defer manager.Close()

	// 获取证书文件路径
	certDir := getCertDir()
	caFile := filepath.Join(certDir, "ca.crt")
	certFile := filepath.Join(certDir, "client.crt")
	keyFile := filepath.Join(certDir, "client.key")

	// 创建增强的TLS配置
	tlsConfig := mqttx.NewEnhancedTLSConfig().
		WithCACert(caFile, mqttx.CertFormatPEM).
		WithClientCert(certFile, keyFile, mqttx.CertFormatPEM, mqttx.CertFormatPEM).
		WithPassword("", "client-key-password"). // 如果私钥有密码
		WithTLSVersion(tls.VersionTLS12, tls.VersionTLS13).
		WithAutoReload(true, 1*time.Minute).
		WithCertExpireWarning(30 * 24 * time.Hour) // 30天过期警告

	// 设置MQTT会话选项
	opts := mqttx.DefaultOptions()
	opts.Name = "tls-demo"
	opts.Brokers = []string{"ssl://localhost:8883"} // 使用SSL端口
	opts.ClientID = "mqttx-tls-demo"
	opts.WithEnhancedTLS(tlsConfig) // 使用增强的TLS配置

	// 添加会话
	sessionID := opts.Name
	err := manager.AddSession(opts)
	if err != nil {
		fmt.Printf("Failed to add session: %v\n", err)
		os.Exit(1)
	}

	// 等待连接建立
	fmt.Println("Connecting to MQTT broker with TLS...")
	// 等待连接就绪
	connected := false
	err = manager.WaitForSession(sessionID, 5*time.Second)
	if err == nil {
		connected = true
	}

	if !connected {
		fmt.Println("Failed to connect to MQTT broker")
		os.Exit(1)
	}
	fmt.Println("Connected to MQTT broker with TLS")

	// 订阅主题
	err = manager.SubscribeTo(sessionID, "test/tls", func(topic string, payload []byte) {
		fmt.Printf("Received message on %s: %s\n", topic, string(payload))
	}, 1)
	if err != nil {
		fmt.Printf("Failed to subscribe: %v\n", err)
	}

	// 发布消息
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		count := 0
		for range ticker.C {
			count++
			message := fmt.Sprintf("Secure message %d", count)
			err := manager.PublishTo(sessionID, "test/tls", []byte(message), 1)
			if err != nil {
				fmt.Printf("Failed to publish: %v\n", err)
			} else {
				fmt.Printf("Published: %s\n", message)
			}

			if count >= 10 {
				break
			}
		}
	}()

	// 等待信号退出
	fmt.Println("Running... Press Ctrl+C to exit")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	fmt.Println("Exiting...")
}

// getCertDir 获取证书目录
func getCertDir() string {
	// 首先检查环境变量
	certDir := os.Getenv("MQTTX_CERT_DIR")
	if certDir != "" {
		return certDir
	}

	// 然后检查当前目录下的certs目录
	if _, err := os.Stat("certs"); err == nil {
		return "certs"
	}

	// 最后使用相对路径
	return "./example/enhanced_tls/certs"
}
