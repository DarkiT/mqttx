package mqttx

import (
	"testing"
	"time"
)

// TestSessionBuilder 测试会话构建器
func TestSessionBuilder(t *testing.T) {
	t.Run("QuickConnect", func(t *testing.T) {
		opts, err := QuickConnect("test-session", "localhost:1883").Build()
		if err != nil {
			t.Fatalf("QuickConnect failed: %v", err)
		}

		if opts.Name != "test-session" {
			t.Errorf("Expected name 'test-session', got '%s'", opts.Name)
		}
		if len(opts.Brokers) != 1 || opts.Brokers[0] != "tcp://localhost:1883" {
			t.Errorf("Expected broker 'tcp://localhost:1883', got %v", opts.Brokers)
		}
	})

	t.Run("SecureConnect", func(t *testing.T) {
		opts, err := SecureConnect("secure-session", "ssl://broker.example.com:8883", "/path/to/ca.crt").
			Auth("user123", "pass456").
			KeepAlive(30).
			Build()
		if err != nil {
			t.Fatalf("SecureConnect failed: %v", err)
		}

		if opts.TLS == nil {
			t.Error("Expected TLS config to be set")
		} else if opts.TLS.CAFile != "/path/to/ca.crt" {
			t.Errorf("Expected CA file '/path/to/ca.crt', got '%s'", opts.TLS.CAFile)
		}

		if opts.Username != "user123" {
			t.Errorf("Expected username 'user123', got '%s'", opts.Username)
		}
		if opts.ConnectProps.KeepAlive != 30 {
			t.Errorf("Expected KeepAlive 30, got %d", opts.ConnectProps.KeepAlive)
		}
	})

	t.Run("ComplexConfiguration", func(t *testing.T) {
		handler := func(topic string, payload []byte) {
			// Mock handler
		}

		opts, err := NewSessionBuilder("complex-session").
			Brokers("tcp://broker1.example.com:1883", "tcp://broker2.example.com:1883").
			ClientID("custom-client-001").
			Auth("admin", "secret123").
			TLS("/etc/ssl/ca.crt", "/etc/ssl/client.crt", "/etc/ssl/client.key", false).
			KeepAlive(60).
			AutoReconnect().
			ReconnectConfig(5, 300, 2.0).
			Timeouts(45, 30).
			Performance(8, 2000).
			MessageChannelSize(500).
			Persistent().
			RedisStorage("localhost:6379").
			RedisAuth("redisuser", "redispass", 1).
			Subscribe("sensors/+/temperature", 1, handler).
			Subscribe("alerts/#", 2, handler).
			Build()
		if err != nil {
			t.Fatalf("Complex configuration failed: %v", err)
		}

		// 验证配置
		if len(opts.Brokers) != 2 {
			t.Errorf("Expected 2 brokers, got %d", len(opts.Brokers))
		}
		if opts.ClientID != "custom-client-001" {
			t.Errorf("Expected client ID 'custom-client-001', got '%s'", opts.ClientID)
		}
		if opts.ConnectProps.AutoReconnect != true {
			t.Error("Expected AutoReconnect to be true")
		}
		if opts.ConnectProps.InitialReconnectInterval != 5*time.Second {
			t.Errorf("Expected initial reconnect interval 5s, got %v", opts.ConnectProps.InitialReconnectInterval)
		}
		if opts.Performance.WriteBufferSize != 8*1024 {
			t.Errorf("Expected write buffer size 8KB, got %d", opts.Performance.WriteBufferSize)
		}
		if opts.Storage.Type != StoreTypeRedis {
			t.Errorf("Expected Redis storage, got %v", opts.Storage.Type)
		}
		if len(opts.Topics) != 2 {
			t.Errorf("Expected 2 subscriptions, got %d", len(opts.Topics))
		}
	})

	t.Run("ValidationErrors", func(t *testing.T) {
		// 测试空broker
		_, err := NewSessionBuilder("test").Broker("").Build()
		if err == nil {
			t.Error("Expected error for empty broker")
		}

		// 测试无效QoS
		handler := func(topic string, payload []byte) {}
		_, err = NewSessionBuilder("test").
			Subscribe("test/topic", 5, handler). // 无效QoS
			Build()
		if err == nil {
			t.Error("Expected error for invalid QoS")
		}

		// 测试空Redis地址
		_, err = NewSessionBuilder("test").RedisStorage("").Build()
		if err == nil {
			t.Error("Expected error for empty Redis address")
		}
	})

	t.Run("BuilderReuse", func(t *testing.T) {
		builder := NewSessionBuilder("session1")

		// 第一次构建
		opts1, err := builder.
			Broker("localhost:1883").
			KeepAlive(30).
			Build()
		if err != nil {
			t.Fatalf("First build failed: %v", err)
		}

		// 重置并第二次构建
		opts2, err := builder.Reset("session2").
			Broker("localhost:1884").
			KeepAlive(60).
			Build()
		if err != nil {
			t.Fatalf("Second build failed: %v", err)
		}

		if opts1.Name == opts2.Name {
			t.Error("Expected different session names after reset")
		}
		if opts1.ConnectProps.KeepAlive == opts2.ConnectProps.KeepAlive {
			t.Error("Expected different KeepAlive values after reset")
		}
	})
}

// TestForwarderBuilder 测试转发器构建器
func TestForwarderBuilder(t *testing.T) {
	t.Run("BasicForwarder", func(t *testing.T) {
		config, err := NewForwarderBuilder("temp-forwarder").
			Source("sensor-session", "sensors/+/temperature").
			Target("storage-session").
			QoS(1).
			BufferSize(200).
			Build()
		if err != nil {
			t.Fatalf("Basic forwarder build failed: %v", err)
		}

		if config.Name != "temp-forwarder" {
			t.Errorf("Expected name 'temp-forwarder', got '%s'", config.Name)
		}
		if config.SourceSession != "sensor-session" {
			t.Errorf("Expected source session 'sensor-session', got '%s'", config.SourceSession)
		}
		if len(config.SourceTopics) != 1 {
			t.Errorf("Expected 1 source topic, got %d", len(config.SourceTopics))
		}
		if config.QoS != 1 {
			t.Errorf("Expected QoS 1, got %d", config.QoS)
		}
	})

	t.Run("ForwarderWithMapping", func(t *testing.T) {
		config, err := NewForwarderBuilder("mapping-forwarder").
			Source("input-session", "raw/+/data", "raw/+/status").
			Target("processed-session").
			MapTopic("raw/sensor1/data", "processed/sensor1/readings").
			MapTopic("raw/sensor1/status", "processed/sensor1/health").
			TopicMapping(map[string]string{
				"raw/sensor2/data":   "processed/sensor2/readings",
				"raw/sensor2/status": "processed/sensor2/health",
			}).
			Build()
		if err != nil {
			t.Fatalf("Forwarder with mapping build failed: %v", err)
		}

		if len(config.SourceTopics) != 2 {
			t.Errorf("Expected 2 source topics, got %d", len(config.SourceTopics))
		}
		if len(config.TopicMap) != 4 {
			t.Errorf("Expected 4 topic mappings, got %d", len(config.TopicMap))
		}

		// 验证映射
		if config.TopicMap["raw/sensor1/data"] != "processed/sensor1/readings" {
			t.Error("Topic mapping not set correctly")
		}
	})

	t.Run("ForwarderValidationErrors", func(t *testing.T) {
		// 测试空名称
		_, err := NewForwarderBuilder("").
			Source("src", "topic").
			Target("dst").
			Build()
		if err == nil {
			t.Error("Expected error for empty forwarder name")
		}

		// 测试空源会话
		_, err = NewForwarderBuilder("test").
			Source("", "topic").
			Target("dst").
			Build()
		if err == nil {
			t.Error("Expected error for empty source session")
		}

		// 测试无效QoS
		_, err = NewForwarderBuilder("test").
			Source("src", "topic").
			Target("dst").
			QoS(5).
			Build()
		if err == nil {
			t.Error("Expected error for invalid QoS")
		}
	})

	t.Run("DisabledForwarder", func(t *testing.T) {
		config, err := NewForwarderBuilder("disabled-forwarder").
			Source("src-session", "test/topic").
			Target("dst-session").
			Disable().
			Build()
		if err != nil {
			t.Fatalf("Disabled forwarder build failed: %v", err)
		}

		if config.Enabled {
			t.Error("Expected forwarder to be disabled")
		}

		// 重新启用
		config2, err := NewForwarderBuilder("enabled-forwarder").
			Source("src-session", "test/topic").
			Target("dst-session").
			Disable().
			Enable().
			Build()
		if err != nil {
			t.Fatalf("Re-enabled forwarder build failed: %v", err)
		}

		if !config2.Enabled {
			t.Error("Expected forwarder to be enabled after Enable() call")
		}
	})
}

// TestErrorTypes 测试新的错误类型
func TestErrorTypes(t *testing.T) {
	t.Run("MQTTXError", func(t *testing.T) {
		err := NewConnectionError("connection timeout", ErrTimeout).
			WithSession("test-session").
			WithTopic("test/topic").
			WithContext("retry_count", 3)

		if err.Type != TypeConnection {
			t.Errorf("Expected connection error type, got %v", err.Type)
		}
		if err.Severity != SeverityError {
			t.Errorf("Expected error severity, got %v", err.Severity)
		}
		if err.Session != "test-session" {
			t.Errorf("Expected session 'test-session', got '%s'", err.Session)
		}
		if err.Topic != "test/topic" {
			t.Errorf("Expected topic 'test/topic', got '%s'", err.Topic)
		}

		retryCount, ok := err.Context["retry_count"]
		if !ok || retryCount != 3 {
			t.Errorf("Expected retry_count 3 in context, got %v", retryCount)
		}

		// 测试错误消息格式
		errMsg := err.Error()
		if errMsg == "" {
			t.Error("Error message should not be empty")
		}
		t.Logf("Error message: %s", errMsg)
	})

	t.Run("ValidationErrors", func(t *testing.T) {
		validationErrs := NewValidationErrors()
		validationErrs.Add(ErrInvalidBroker)
		validationErrs.Add(ErrInvalidClientID)
		validationErrs.Add(ErrQoSNotSupported)

		if !validationErrs.HasErrors() {
			t.Error("Expected validation errors to be present")
		}

		if len(validationErrs.Errors) != 3 {
			t.Errorf("Expected 3 validation errors, got %d", len(validationErrs.Errors))
		}

		errMsg := validationErrs.Error()
		if errMsg == "" {
			t.Error("Validation errors message should not be empty")
		}
		t.Logf("Validation errors: %s", errMsg)
	})

	t.Run("ErrorUtilities", func(t *testing.T) {
		// 测试临时错误判断
		connErr := NewConnectionError("temporary failure", nil)
		if !IsTemporary(connErr) {
			t.Error("Connection error should be temporary")
		}

		configErr := NewConfigError("critical config error", nil)
		if IsTemporary(configErr) {
			t.Error("Config error should not be temporary")
		}

		// 测试错误类型获取
		if GetErrorType(connErr) != TypeConnection {
			t.Error("Should get connection error type")
		}

		if GetErrorSeverity(configErr) != SeverityCritical {
			t.Error("Should get critical severity")
		}
	})
}

// TestAPIUsabilityImprovements 测试API易用性改进
func TestAPIUsabilityImprovements(t *testing.T) {
	t.Run("FluentAPI", func(t *testing.T) {
		// 测试链式调用的可读性
		opts := NewSessionBuilder("fluent-test").
			Broker("localhost:1883").
			Auth("user", "pass").
			KeepAlive(60).
			AutoReconnect().
			Performance(4, 1000).
			MustBuild() // 用于测试代码，实际使用应该处理错误

		if opts.Name != "fluent-test" {
			t.Error("Fluent API failed to set name")
		}

		// 测试错误累积
		builder := NewSessionBuilder("error-test").
			Broker("").  // 这会添加一个错误
			ClientID("") // 这会添加另一个错误

		errors := builder.Validate()
		if len(errors) < 2 {
			t.Errorf("Expected at least 2 validation errors, got %d", len(errors))
		}
	})

	t.Run("SimplifiedConfigs", func(t *testing.T) {
		// 快速连接应该只需要最少的参数
		simpleOpts, err := QuickConnect("simple", "broker.example.com:1883").Build()
		if err != nil {
			t.Fatalf("Simple connection failed: %v", err)
		}

		// 验证默认值是否合理
		if simpleOpts.ConnectProps.KeepAlive <= 0 {
			t.Error("Default KeepAlive should be positive")
		}
		if simpleOpts.ConnectProps.ConnectTimeout <= 0 {
			t.Error("Default ConnectTimeout should be positive")
		}
		if !simpleOpts.ConnectProps.AutoReconnect {
			t.Error("AutoReconnect should be enabled by default")
		}
	})

	t.Run("AutoProtocolDetection", func(t *testing.T) {
		// 测试自动协议前缀添加
		opts1, _ := NewSessionBuilder("test1").Broker("localhost:1883").Build()
		if opts1.Brokers[0] != "tcp://localhost:1883" {
			t.Error("Should auto-add tcp:// prefix")
		}

		// 不应该重复添加协议前缀
		opts2, _ := NewSessionBuilder("test2").Broker("ssl://localhost:8883").Build()
		if opts2.Brokers[0] != "ssl://localhost:8883" {
			t.Error("Should not modify existing protocol prefix")
		}
	})
}
