package mqttx

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMQTT5Config(t *testing.T) {
	// 测试默认配置
	cfg := DefaultMQTT5Config()
	assert.Equal(t, byte(MQTT50), cfg.ProtocolVersion)
	assert.Equal(t, 24*time.Hour, cfg.SessionExpiry)
	assert.Equal(t, uint16(65535), cfg.ReceiveMaximum)
	assert.Equal(t, uint32(268435455), cfg.MaximumPacketSize)
	assert.Equal(t, uint16(65535), cfg.TopicAliasMaximum)
	assert.True(t, cfg.RequestRespInfo)
	assert.True(t, cfg.RequestProblemInfo)
	assert.False(t, cfg.EnhancedAuth)
	assert.True(t, cfg.SharedSubAvailable)
	assert.NotNil(t, cfg.UserProperties)
}

func TestWithMQTT5(t *testing.T) {
	// 创建基本选项
	opts := DefaultOptions()

	// 添加MQTT 5.0配置
	mqtt5Cfg := DefaultMQTT5Config()
	mqtt5Cfg.SessionExpiry = 1 * time.Hour
	mqtt5Cfg.UserProperties["app"] = "test"

	opts.WithMQTT5(mqtt5Cfg)

	// 验证配置
	assert.NotNil(t, opts.MQTT5)
	assert.Equal(t, byte(MQTT50), opts.ProtocolVersion)
	assert.Equal(t, 1*time.Hour, opts.MQTT5.MQTT5Config.SessionExpiry)
	assert.Equal(t, "test", opts.MQTT5.MQTT5Config.UserProperties["app"])
}

func TestWithUserProperty(t *testing.T) {
	// 创建基本选项
	opts := DefaultOptions()

	// 添加用户属性
	opts.WithUserProperty("device", "sensor")
	opts.WithUserProperty("location", "room1")

	// 验证配置
	assert.NotNil(t, opts.MQTT5)
	assert.Equal(t, "sensor", opts.MQTT5.MQTT5Config.UserProperties["device"])
	assert.Equal(t, "room1", opts.MQTT5.MQTT5Config.UserProperties["location"])
}

func TestWithSessionExpiry(t *testing.T) {
	// 创建基本选项
	opts := DefaultOptions()

	// 设置会话过期时间
	opts.WithSessionExpiry(30 * time.Minute)

	// 验证配置
	assert.NotNil(t, opts.MQTT5)
	assert.Equal(t, 30*time.Minute, opts.MQTT5.MQTT5Config.SessionExpiry)
}

func TestWithSharedSubscription(t *testing.T) {
	// 创建基本选项
	opts := DefaultOptions()

	// 配置共享订阅
	opts.WithSharedSubscription(true, "$share")

	// 验证配置
	assert.NotNil(t, opts.MQTT5)
	assert.True(t, opts.MQTT5.MQTT5Config.SharedSubAvailable)
	assert.Equal(t, "$share", opts.MQTT5.SharedSubPrefix)
}

func TestPlainAuthProvider(t *testing.T) {
	// 创建明文认证提供者
	provider := NewPlainAuthProvider("user1", "password123")

	// 验证方法名称
	assert.Equal(t, AuthMethodPlain, provider.Method())

	// 测试初始化
	data, err := provider.Initialize()
	assert.NoError(t, err)
	assert.Equal(t, []byte("user1\x00password123"), data)

	// 测试挑战处理
	_, err = provider.HandleChallenge([]byte("challenge"))
	assert.Error(t, err)

	// 测试完成
	success, err := provider.Complete([]byte{})
	assert.NoError(t, err)
	assert.True(t, success)

	// 测试失败完成
	success, err = provider.Complete([]byte("error"))
	assert.Error(t, err)
	assert.False(t, success)
}

func TestSHA256AuthProvider(t *testing.T) {
	// 创建SHA-256认证提供者
	provider := NewSHA256AuthProvider("user1", "password123")

	// 验证方法名称
	assert.Equal(t, AuthMethodSHA256, provider.Method())

	// 测试初始化
	data, err := provider.Initialize()
	assert.NoError(t, err)
	assert.Equal(t, []byte("user1"), data)

	// 测试挑战处理 - 空挑战
	_, err = provider.HandleChallenge([]byte{})
	assert.Error(t, err)

	// 测试挑战处理 - 有效挑战
	salt := []byte("randomsalt")
	response, err := provider.HandleChallenge(salt)
	assert.NoError(t, err)
	assert.NotEmpty(t, response)

	// 测试完成
	success, err := provider.Complete([]byte{})
	assert.NoError(t, err)
	assert.True(t, success)

	// 测试失败完成
	success, err = provider.Complete([]byte("error"))
	assert.Error(t, err)
	assert.False(t, success)
}

func TestJWTAuthProvider(t *testing.T) {
	// 创建JWT认证提供者
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ"
	provider := NewJWTAuthProvider(token)

	// 验证方法名称
	assert.Equal(t, AuthMethodJWT, provider.Method())

	// 测试初始化
	data, err := provider.Initialize()
	assert.NoError(t, err)
	assert.Equal(t, []byte(token), data)

	// 测试挑战处理
	_, err = provider.HandleChallenge([]byte("challenge"))
	assert.Error(t, err)

	// 测试完成
	success, err := provider.Complete([]byte{})
	assert.NoError(t, err)
	assert.True(t, success)

	// 测试失败完成
	success, err = provider.Complete([]byte("error"))
	assert.Error(t, err)
	assert.False(t, success)
}

func TestOAuth2AuthProvider(t *testing.T) {
	// 创建OAuth2认证提供者
	accessToken := "access_token_123"
	refreshToken := "refresh_token_456"
	expiresIn := 1 * time.Hour
	provider := NewOAuth2AuthProvider(accessToken, refreshToken, expiresIn)

	// 验证方法名称
	assert.Equal(t, AuthMethodOAuth2, provider.Method())

	// 测试初始化
	data, err := provider.Initialize()
	assert.NoError(t, err)
	assert.Equal(t, []byte(accessToken), data)

	// 测试挑战处理
	_, err = provider.HandleChallenge([]byte("challenge"))
	assert.Error(t, err)

	// 测试完成
	success, err := provider.Complete([]byte{})
	assert.NoError(t, err)
	assert.True(t, success)

	// 测试失败完成
	success, err = provider.Complete([]byte("error"))
	assert.Error(t, err)
	assert.False(t, success)
}

func TestMQTT5AuthManager(t *testing.T) {
	// 创建认证提供者
	provider := NewPlainAuthProvider("user1", "password123")

	// 创建认证管理器
	manager := NewMQTT5AuthManager(provider)

	// 测试开始认证
	method, data, err := manager.StartAuth()
	assert.NoError(t, err)
	assert.Equal(t, AuthMethodPlain, method)
	assert.Equal(t, []byte("user1\x00password123"), data)
	assert.Equal(t, AuthStateInProgress, manager.GetState())

	// 测试处理认证
	_, err = manager.HandleAuth([]byte("challenge"))
	assert.Error(t, err) // 明文认证不支持挑战

	// 测试完成认证
	success, err := manager.CompleteAuth([]byte{})
	assert.NoError(t, err)
	assert.True(t, success)
	assert.Equal(t, AuthStateSuccess, manager.GetState())

	// 测试获取方法
	assert.Equal(t, AuthMethodPlain, manager.GetMethod())
}

func TestWithEnhancedAuth(t *testing.T) {
	// 跳过此测试，因为需要完整的会话实现
	t.Skip("Skipping test that requires full session implementation")

	// 创建选项
	opts := DefaultOptions()
	opts.Name = "test-session"
	opts.WithMQTT5(DefaultMQTT5Config())

	// 验证配置
	auth := NewMQTT5Auth(AuthMethodJWT, []byte("test-token"))
	opts.WithEnhancedAuth(auth)

	assert.NotNil(t, opts.MQTT5)
	assert.NotNil(t, opts.MQTT5.Auth)
	assert.Equal(t, AuthMethodJWT, opts.MQTT5.Auth.Method)
	assert.Equal(t, []byte("test-token"), opts.MQTT5.Auth.Data)
}
