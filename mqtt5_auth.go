package mqttx

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"
)

// MQTT 5.0 认证方法常量
const (
	// 标准认证方法
	AuthMethodPlain     = "PLAIN"      // 明文认证
	AuthMethodSHA256    = "SHA-256"    // SHA-256哈希认证
	AuthMethodScramSHA1 = "SCRAM-SHA1" // SCRAM-SHA1认证
	// OAuth 2.0认证
	AuthMethodOAuth2 = "oauth2" // OAuth 2.0认证
	// JWT认证
	AuthMethodJWT = "jwt" // JWT认证
)

// MQTT 5.0 认证状态常量
const (
	AuthStateNone       = iota // 未认证
	AuthStateInProgress        // 认证进行中
	AuthStateSuccess           // 认证成功
	AuthStateFailed            // 认证失败
)

// AuthError 认证错误
type AuthError struct {
	Method  string
	Message string
	Reason  error
}

// Error 实现error接口
func (e *AuthError) Error() string {
	if e.Reason != nil {
		return fmt.Sprintf("authentication error [%s]: %s: %v", e.Method, e.Message, e.Reason)
	}
	return fmt.Sprintf("authentication error [%s]: %s", e.Method, e.Message)
}

// Unwrap 返回底层错误
func (e *AuthError) Unwrap() error {
	return e.Reason
}

// AuthProvider 认证提供者接口
// 用于实现不同的认证机制
type AuthProvider interface {
	// 认证方法名称
	Method() string

	// 初始化认证过程
	// 返回初始认证数据和可能的错误
	Initialize() ([]byte, error)

	// 处理认证挑战
	// data为服务器返回的认证数据
	// 返回响应数据和可能的错误
	HandleChallenge(data []byte) ([]byte, error)

	// 完成认证过程
	// data为服务器返回的最终认证数据
	// 返回是否认证成功和可能的错误
	Complete(data []byte) (bool, error)
}

// PlainAuthProvider 明文认证提供者
type PlainAuthProvider struct {
	username string
	password string
	state    int
}

// NewPlainAuthProvider 创建明文认证提供者
func NewPlainAuthProvider(username, password string) *PlainAuthProvider {
	return &PlainAuthProvider{
		username: username,
		password: password,
		state:    AuthStateNone,
	}
}

// Method 返回认证方法名称
func (p *PlainAuthProvider) Method() string {
	return AuthMethodPlain
}

// Initialize 初始化认证过程
func (p *PlainAuthProvider) Initialize() ([]byte, error) {
	// 明文认证直接发送用户名和密码
	// 格式：username\0password
	data := []byte(fmt.Sprintf("%s\x00%s", p.username, p.password))
	p.state = AuthStateInProgress
	return data, nil
}

// HandleChallenge 处理认证挑战
func (p *PlainAuthProvider) HandleChallenge(data []byte) ([]byte, error) {
	// 明文认证通常不需要多次交互
	return nil, &AuthError{
		Method:  p.Method(),
		Message: "plain authentication does not support challenge-response",
	}
}

// Complete 完成认证过程
func (p *PlainAuthProvider) Complete(data []byte) (bool, error) {
	// 如果服务器返回空数据，表示认证成功
	if len(data) == 0 {
		p.state = AuthStateSuccess
		return true, nil
	}

	// 否则认证失败
	p.state = AuthStateFailed
	return false, &AuthError{
		Method:  p.Method(),
		Message: "authentication failed",
	}
}

// SHA256AuthProvider SHA-256哈希认证提供者
type SHA256AuthProvider struct {
	username string
	password string
	salt     []byte
	state    int
}

// NewSHA256AuthProvider 创建SHA-256哈希认证提供者
func NewSHA256AuthProvider(username, password string) *SHA256AuthProvider {
	return &SHA256AuthProvider{
		username: username,
		password: password,
		state:    AuthStateNone,
	}
}

// Method 返回认证方法名称
func (p *SHA256AuthProvider) Method() string {
	return AuthMethodSHA256
}

// Initialize 初始化认证过程
func (p *SHA256AuthProvider) Initialize() ([]byte, error) {
	// 发送用户名
	p.state = AuthStateInProgress
	return []byte(p.username), nil
}

// HandleChallenge 处理认证挑战
func (p *SHA256AuthProvider) HandleChallenge(data []byte) ([]byte, error) {
	// 服务器应该返回salt
	if len(data) == 0 {
		return nil, &AuthError{
			Method:  p.Method(),
			Message: "empty challenge data",
		}
	}

	// 保存salt
	p.salt = data

	// 计算哈希
	// HMAC-SHA256(password, salt)
	h := hmac.New(sha256.New, []byte(p.password))
	h.Write(p.salt)
	hash := h.Sum(nil)

	return hash, nil
}

// Complete 完成认证过程
func (p *SHA256AuthProvider) Complete(data []byte) (bool, error) {
	// 如果服务器返回空数据，表示认证成功
	if len(data) == 0 {
		p.state = AuthStateSuccess
		return true, nil
	}

	// 否则认证失败
	p.state = AuthStateFailed
	return false, &AuthError{
		Method:  p.Method(),
		Message: "authentication failed",
	}
}

// JWTAuthProvider JWT认证提供者
type JWTAuthProvider struct {
	token string
	state int
}

// NewJWTAuthProvider 创建JWT认证提供者
func NewJWTAuthProvider(token string) *JWTAuthProvider {
	return &JWTAuthProvider{
		token: token,
		state: AuthStateNone,
	}
}

// Method 返回认证方法名称
func (p *JWTAuthProvider) Method() string {
	return AuthMethodJWT
}

// Initialize 初始化认证过程
func (p *JWTAuthProvider) Initialize() ([]byte, error) {
	// 直接发送JWT令牌
	p.state = AuthStateInProgress
	return []byte(p.token), nil
}

// HandleChallenge 处理认证挑战
func (p *JWTAuthProvider) HandleChallenge(data []byte) ([]byte, error) {
	// JWT认证通常不需要多次交互
	return nil, &AuthError{
		Method:  p.Method(),
		Message: "JWT authentication does not support challenge-response",
	}
}

// Complete 完成认证过程
func (p *JWTAuthProvider) Complete(data []byte) (bool, error) {
	// 如果服务器返回空数据，表示认证成功
	if len(data) == 0 {
		p.state = AuthStateSuccess
		return true, nil
	}

	// 否则认证失败
	p.state = AuthStateFailed
	return false, &AuthError{
		Method:  p.Method(),
		Message: "authentication failed",
	}
}

// OAuth2AuthProvider OAuth 2.0认证提供者
type OAuth2AuthProvider struct {
	accessToken  string
	refreshToken string
	expiresAt    time.Time
	state        int
}

// NewOAuth2AuthProvider 创建OAuth 2.0认证提供者
func NewOAuth2AuthProvider(accessToken, refreshToken string, expiresIn time.Duration) *OAuth2AuthProvider {
	return &OAuth2AuthProvider{
		accessToken:  accessToken,
		refreshToken: refreshToken,
		expiresAt:    time.Now().Add(expiresIn),
		state:        AuthStateNone,
	}
}

// Method 返回认证方法名称
func (p *OAuth2AuthProvider) Method() string {
	return AuthMethodOAuth2
}

// Initialize 初始化认证过程
func (p *OAuth2AuthProvider) Initialize() ([]byte, error) {
	// 检查令牌是否过期
	if !p.expiresAt.IsZero() && time.Now().After(p.expiresAt) {
		return nil, &AuthError{
			Method:  p.Method(),
			Message: "access token expired",
		}
	}

	// 发送访问令牌
	p.state = AuthStateInProgress
	return []byte(p.accessToken), nil
}

// HandleChallenge 处理认证挑战
func (p *OAuth2AuthProvider) HandleChallenge(data []byte) ([]byte, error) {
	// OAuth 2.0认证通常不需要多次交互
	return nil, &AuthError{
		Method:  p.Method(),
		Message: "OAuth 2.0 authentication does not support challenge-response",
	}
}

// Complete 完成认证过程
func (p *OAuth2AuthProvider) Complete(data []byte) (bool, error) {
	// 如果服务器返回空数据，表示认证成功
	if len(data) == 0 {
		p.state = AuthStateSuccess
		return true, nil
	}

	// 否则认证失败
	p.state = AuthStateFailed
	return false, &AuthError{
		Method:  p.Method(),
		Message: "authentication failed",
	}
}

// 生成随机字节
func generateRandomBytes(length int) ([]byte, error) {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

// MQTT5AuthManager MQTT 5.0认证管理器
type MQTT5AuthManager struct {
	provider AuthProvider
	state    int
}

// NewMQTT5AuthManager 创建MQTT 5.0认证管理器
func NewMQTT5AuthManager(provider AuthProvider) *MQTT5AuthManager {
	return &MQTT5AuthManager{
		provider: provider,
		state:    AuthStateNone,
	}
}

// StartAuth 开始认证过程
func (m *MQTT5AuthManager) StartAuth() (string, []byte, error) {
	if m.provider == nil {
		return "", nil, errors.New("no authentication provider configured")
	}

	data, err := m.provider.Initialize()
	if err != nil {
		return "", nil, err
	}

	m.state = AuthStateInProgress
	return m.provider.Method(), data, nil
}

// HandleAuth 处理认证响应
func (m *MQTT5AuthManager) HandleAuth(data []byte) ([]byte, error) {
	if m.provider == nil {
		return nil, errors.New("no authentication provider configured")
	}

	if m.state != AuthStateInProgress {
		return nil, errors.New("authentication not in progress")
	}

	return m.provider.HandleChallenge(data)
}

// CompleteAuth 完成认证过程
func (m *MQTT5AuthManager) CompleteAuth(data []byte) (bool, error) {
	if m.provider == nil {
		return false, errors.New("no authentication provider configured")
	}

	if m.state != AuthStateInProgress {
		return false, errors.New("authentication not in progress")
	}

	success, err := m.provider.Complete(data)
	if err != nil {
		m.state = AuthStateFailed
		return false, err
	}

	if success {
		m.state = AuthStateSuccess
	} else {
		m.state = AuthStateFailed
	}

	return success, nil
}

// GetState 获取认证状态
func (m *MQTT5AuthManager) GetState() int {
	return m.state
}

// GetMethod 获取认证方法
func (m *MQTT5AuthManager) GetMethod() string {
	if m.provider == nil {
		return ""
	}
	return m.provider.Method()
}

// WithEnhancedAuth 为会话配置增强认证
func (s *Session) WithEnhancedAuth(provider AuthProvider) *Session {
	if s.opts.MQTT5 == nil {
		s.opts.WithMQTT5(DefaultMQTT5Config())
	}

	// 创建认证管理器
	authManager := NewMQTT5AuthManager(provider)

	// 获取认证方法和初始数据
	method, data, err := authManager.StartAuth()
	if err != nil {
		s.manager.logger.Error("Failed to start enhanced authentication",
			"session", s.name,
			"method", method,
			"error", err)
		return s
	}

	// 配置认证信息
	auth := NewMQTT5Auth(method, data)
	s.opts.WithEnhancedAuth(auth)

	return s
}
