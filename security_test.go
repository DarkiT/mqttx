package mqttx

import (
	"crypto/tls"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnhancedTLSConfig(t *testing.T) {
	// 创建临时测试目录
	tempDir := t.TempDir()

	// 创建测试证书文件
	caContent := []byte(`-----BEGIN CERTIFICATE-----
MIIDazCCAlOgAwIBAgIUJFUP8QhHY6R/iYWFsv3zIcGRhX8wDQYJKoZIhvcNAQEL
BQAwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoM
GEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yMzA1MTUwMDAwMDBaFw0yNDA1
MTUwMDAwMDBaMEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEw
HwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwggEiMA0GCSqGSIb3DQEB
AQUAA4IBDwAwggEKAoIBAQC6iUQTbLAWCKA9lQMxqRmNBlmUGLa5QEZgX0cN1R4p
0QRNmhqzZ+vgUHmzBGl8IauLSFqf8PYYvs3YYzAJjnA2ygY6oJtNGJUQQXg1pGBh
V6T9/EEz/9Wm+j/2Pvnl3u7UyxFIoBQ2NnLI8m8CpJyLQr1jHPYZVdpWWvMLLOWS
2kcvCb3qrJOFQW3QA9IPNsALaPKj+XHNHmLLXRbKXoJUYZ/5RYwpYdAYxjkBEYKV
wSKzSgzeH7YALczCKNvO4JbYOL4QwYKIPUjQKqgEgdXoMY7UxpOiFxKWKzMtLdM+
qaAjkZ1QHjQnlkYJkXMQPSKptOW8KwxGC4aXH7+MK4JbAgMBAAGjUzBRMB0GA1Ud
DgQWBBQOHuGC3AmBMJ/EQ7AV6FhGcGYHUjAfBgNVHSMEGDAWgBQOHuGC3AmBMJ/E
Q7AV6FhGcGYHUjAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQCY
FcX4HQbT4TzYvFuuQ0XHGXGGFqQbU1tVBmUUFXDzTCKGSWKX4KrLLJvjDDkSj6ZR
JY1gpJL6JA3UX3MFgJ5TCLnhGfzWG9zNnWZJHIVKHBJZFPJ+oKK+1RwKfHE/Ry8P
Z9yGMn2yD0M2mX0wkYP5RuKKBpj58J9XsLOYRxKNgBgbWzQjDwFl7/K8jUkIHFNo
HNZJ+HwFUXFR6xsGR7z0MO/VGRGUOvXJIJUfCYJaC1Iy5JkwXHOQxdwUCGQJUe0Z
DGVo+2DJDfnQnQNVGkHXUU50c7MlIzt7XSxJQ4RhW8LP4mNGcODH0zXVMbEYuPQq
+9DZpvXLTJD3mZShQnJf
-----END CERTIFICATE-----`)

	certContent := []byte(`-----BEGIN CERTIFICATE-----
MIIDazCCAlOgAwIBAgIUJFUP8QhHY6R/iYWFsv3zIcGRhX8wDQYJKoZIhvcNAQEL
BQAwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoM
GEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yMzA1MTUwMDAwMDBaFw0yNDA1
MTUwMDAwMDBaMEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEw
HwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwggEiMA0GCSqGSIb3DQEB
AQUAA4IBDwAwggEKAoIBAQC6iUQTbLAWCKA9lQMxqRmNBlmUGLa5QEZgX0cN1R4p
0QRNmhqzZ+vgUHmzBGl8IauLSFqf8PYYvs3YYzAJjnA2ygY6oJtNGJUQQXg1pGBh
V6T9/EEz/9Wm+j/2Pvnl3u7UyxFIoBQ2NnLI8m8CpJyLQr1jHPYZVdpWWvMLLOWS
2kcvCb3qrJOFQW3QA9IPNsALaPKj+XHNHmLLXRbKXoJUYZ/5RYwpYdAYxjkBEYKV
wSKzSgzeH7YALczCKNvO4JbYOL4QwYKIPUjQKqgEgdXoMY7UxpOiFxKWKzMtLdM+
qaAjkZ1QHjQnlkYJkXMQPSKptOW8KwxGC4aXH7+MK4JbAgMBAAGjUzBRMB0GA1Ud
DgQWBBQOHuGC3AmBMJ/EQ7AV6FhGcGYHUjAfBgNVHSMEGDAWgBQOHuGC3AmBMJ/E
Q7AV6FhGcGYHUjAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQCY
FcX4HQbT4TzYvFuuQ0XHGXGGFqQbU1tVBmUUFXDzTCKGSWKX4KrLLJvjDDkSj6ZR
JY1gpJL6JA3UX3MFgJ5TCLnhGfzWG9zNnWZJHIVKHBJZFPJ+oKK+1RwKfHE/Ry8P
Z9yGMn2yD0M2mX0wkYP5RuKKBpj58J9XsLOYRxKNgBgbWzQjDwFl7/K8jUkIHFNo
HNZJ+HwFUXFR6xsGR7z0MO/VGRGUOvXJIJUfCYJaC1Iy5JkwXHOQxdwUCGQJUe0Z
DGVo+2DJDfnQnQNVGkHXUU50c7MlIzt7XSxJQ4RhW8LP4mNGcODH0zXVMbEYuPQq
+9DZpvXLTJD3mZShQnJf
-----END CERTIFICATE-----`)

	keyContent := []byte(`-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQC6iUQTbLAWCKA9
lQMxqRmNBlmUGLa5QEZgX0cN1R4p0QRNmhqzZ+vgUHmzBGl8IauLSFqf8PYYvs3Y
YzAJjnA2ygY6oJtNGJUQQXg1pGBhV6T9/EEz/9Wm+j/2Pvnl3u7UyxFIoBQ2NnLI
8m8CpJyLQr1jHPYZVdpWWvMLLOWS2kcvCb3qrJOFQW3QA9IPNsALaPKj+XHNHmLL
XRbKXoJUYZ/5RYwpYdAYxjkBEYKVwSKzSgzeH7YALczCKNvO4JbYOL4QwYKIPUjQ
KqgEgdXoMY7UxpOiFxKWKzMtLdM+qaAjkZ1QHjQnlkYJkXMQPSKptOW8KwxGC4aX
H7+MK4JbAgMBAAECggEBAKqQ9F5w1tLOQbLQbYUuQpJmHl/jQTJNFJgqCBcqAQpY
MjZoW3WUHhIgFXvNZBcTaseY5aaO+MVoiGJKADnNz6/K7Mvtz0NhxuZQJc5h+KIV
P4w4ULy/PuWMYQrGcDSxX0PX3VL3YYhEjGQmLQHQGkRZUOA/LVJgGmfwqJZfNQWU
JJIi+yZPJRsm6XJfKKQNYIYPMSIgBjgBGzIX0oQIDBDsZmxSKlbc6VFGF9jYqxnN
y1RdEZRRMU8rYPVdJP3yUGHgbF7HpODVGYnwYLn+0mBfxYMKicunpqdMBKcOCMOA
C9RNKRmLk+SXmTQKyB7WBiF+5XMqLkbwrKqxJzxFTAECgYEA8YEPuvnfYMwXUEVn
ekIDFMcZQryUjMU4o+OmZGJepeJJBGFgQl5O9xqDFgGPzYpN3cAX3/fGCjHHZLEb
nHKi5AaEMbHNrLWqgYs/QHpYlNKFrCOBQPBBhXfVl5QbpA0VnDmQQC7H5JEFxTXj
hgF/ZUBXnKQvQxXlmQBauWNqbAECgYEAxcEIqfqDAOKoqOjPG1jV9BpGYXqAQI5v
wGzPZe5aKZ1JCTDq+3YkAQQzYI6LbOgYN9qJgcU4a/2GYYpQIXEKleqEYJ9a4uV0
WXpGQUDmYXWXKvH6xHPXMHQEpIUG5ZqfR8Uv9pCRjvYQhP3QK3L6tZmD0CJXpjUQ
oULgLHW4GtsCgYA3Mq2hW0JCgUi2UJaHH0yQHYJTNQhNpYS7CSBJsJV5XCYVUECm
XnFfgfGbLPpJ05hLyL3K4qRZlTMqcVEGLdJh8VPGX1KeJ4DgPexHJKTWBPHfYBCe
XPMPJyG9gJKqJ21G2AMzOWJhDGI6Eieie7oYUeZRXqCIq8RGHZYZGYGAAQKBgHJd
yjRlXhFYV9LKHxI7k6FKDWBsVEzCkGU4bNgMPGJyJHZYKKmS4XXXjkHDAsvLNAQl
g7+qsN5nELmKPYhJx5YpHxYZGMqGvx9CzTThE3vYAGG0XlFCXJcBZxNgGfkxP3Gy
KzrLLnQTfKW5hXHl/I4bUF9v6HQS7APKWw/LS1bvAoGBAJiRX8YK/9aV21QKGpP+
/Q6+7+vQ7+6XE4Fu8/FVMfBEYPTSUCCX0d8qvPxCML1S+qdZiuKdIVbHbqB/vB7S
Pv0s3cjJggDy74xRHLcpVWInJZ0Z3C5F5+Zn0HKEYjZmGiAOc/V+xyOKOOXgUGRL
EJUPYQzeLtBQmwQF/nMWxGmj
-----END PRIVATE KEY-----`)

	// 写入测试证书文件
	caFile := filepath.Join(tempDir, "ca.crt")
	certFile := filepath.Join(tempDir, "client.crt")
	keyFile := filepath.Join(tempDir, "client.key")

	if err := os.WriteFile(caFile, caContent, 0o644); err != nil {
		t.Fatalf("Failed to write CA file: %v", err)
	}
	if err := os.WriteFile(certFile, certContent, 0o644); err != nil {
		t.Fatalf("Failed to write cert file: %v", err)
	}
	if err := os.WriteFile(keyFile, keyContent, 0o644); err != nil {
		t.Fatalf("Failed to write key file: %v", err)
	}

	// 测试创建基本的增强TLS配置
	t.Run("CreateEnhancedTLSConfig", func(t *testing.T) {
		config := NewEnhancedTLSConfig()
		if config == nil {
			t.Fatal("Failed to create EnhancedTLSConfig")
		}
		if config.CAFormat != CertFormatPEM {
			t.Errorf("Expected default CAFormat to be %s, got %s", CertFormatPEM, config.CAFormat)
		}
		if config.MinVersion != tls.VersionTLS12 {
			t.Errorf("Expected default MinVersion to be %d, got %d", tls.VersionTLS12, config.MinVersion)
		}
	})

	// 测试链式方法
	t.Run("ChainedMethods", func(t *testing.T) {
		config := NewEnhancedTLSConfig().
			WithCACert(caFile, CertFormatPEM).
			WithClientCert(certFile, keyFile, CertFormatPEM, CertFormatPEM).
			WithTLSVersion(tls.VersionTLS12, tls.VersionTLS13).
			WithAutoReload(true, 5*time.Minute)

		if config.CAFile != caFile {
			t.Errorf("Expected CAFile to be %s, got %s", caFile, config.CAFile)
		}
		if config.CertFile != certFile {
			t.Errorf("Expected CertFile to be %s, got %s", certFile, config.CertFile)
		}
		if config.KeyFile != keyFile {
			t.Errorf("Expected KeyFile to be %s, got %s", keyFile, config.KeyFile)
		}
		if !config.AutoReload {
			t.Error("Expected AutoReload to be true")
		}
		if config.ReloadInterval != 5*time.Minute {
			t.Errorf("Expected ReloadInterval to be %s, got %s", 5*time.Minute, config.ReloadInterval)
		}
	})

	// 测试构建TLS配置
	t.Run("BuildTLSConfig", func(t *testing.T) {
		config := NewEnhancedTLSConfig().
			WithCACert(caFile, CertFormatPEM).
			WithTLSVersion(tls.VersionTLS12, tls.VersionTLS13).
			WithCipherSuites([]uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256})

		tlsConfig, err := config.BuildTLSConfig()
		if err != nil {
			t.Fatalf("Failed to build TLS config: %v", err)
		}
		if tlsConfig == nil {
			t.Fatal("Expected non-nil TLS config")
		}
		if tlsConfig.RootCAs == nil {
			t.Error("Expected non-nil RootCAs")
		}
		if tlsConfig.MinVersion != tls.VersionTLS12 {
			t.Errorf("Expected MinVersion to be %d, got %d", tls.VersionTLS12, tlsConfig.MinVersion)
		}
		if len(tlsConfig.CipherSuites) != 1 {
			t.Errorf("Expected 1 cipher suite, got %d", len(tlsConfig.CipherSuites))
		}
	})

	// 测试证书过期检查
	t.Run("CertificateExpiry", func(t *testing.T) {
		config := NewEnhancedTLSConfig().
			WithCACert(caFile, CertFormatPEM).
			WithCertExpireWarning(365 * 24 * time.Hour) // 1年

		// 手动设置过期时间进行测试
		config.certExpiry = time.Now().Add(180 * 24 * time.Hour) // 180天后过期

		// 证书应该在一年内过期
		if !config.IsExpiringSoon() {
			t.Error("Expected certificate to be expiring soon with 1 year warning")
		}

		// 使用较短的警告期
		config.WithCertExpireWarning(1 * time.Hour) // 1小时
		if config.IsExpiringSoon() {
			t.Error("Expected certificate not to be expiring soon with 1 hour warning")
		}
	})

	// 测试与Options集成
	t.Run("IntegrationWithOptions", func(t *testing.T) {
		enhancedConfig := NewEnhancedTLSConfig().
			WithCACert(caFile, CertFormatPEM).
			WithClientCert(certFile, keyFile, CertFormatPEM, CertFormatPEM)

		opts := DefaultOptions().WithEnhancedTLS(enhancedConfig)

		if opts.EnhancedTLS == nil {
			t.Fatal("Expected EnhancedTLS to be set in Options")
		}

		if opts.TLS == nil {
			t.Fatal("Expected TLS to be set in Options")
		}

		if opts.TLS.CAFile != caFile {
			t.Errorf("Expected TLS.CAFile to be %s, got %s", caFile, opts.TLS.CAFile)
		}
	})
}

func TestSecurityError(t *testing.T) {
	// 测试安全错误
	t.Run("SecurityError", func(t *testing.T) {
		baseErr := errors.New("base error")
		secErr := newSecurityError("test error", baseErr)

		if secErr.Error() != "test error: base error" {
			t.Errorf("Expected error message 'test error: base error', got '%s'", secErr.Error())
		}

		if errors.Unwrap(secErr) != baseErr {
			t.Error("Expected Unwrap to return base error")
		}

		// 测试没有基础错误的情况
		secErr = newSecurityError("only message", nil)
		if secErr.Error() != "only message" {
			t.Errorf("Expected error message 'only message', got '%s'", secErr.Error())
		}
	})
}
