package mqttx

import (
	"context"
	"errors"
	"time"
)

// MockRedisClient 是Redis客户端的模拟实现
type MockRedisClient struct {
	data     map[string]string
	getError error
	setError error
	delError error
}

// NewMockRedisClient 创建一个新的模拟Redis客户端
func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{
		data: make(map[string]string),
	}
}

// Set 设置键值对
func (m *MockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	if m.setError != nil {
		return m.setError
	}
	m.data[key] = value.(string)
	return nil
}

// Get 获取键对应的值
func (m *MockRedisClient) Get(ctx context.Context, key string) (string, error) {
	if m.getError != nil {
		return "", m.getError
	}
	value, ok := m.data[key]
	if !ok {
		return "", errors.New("redis: nil")
	}
	return value, nil
}

// Del 删除键
func (m *MockRedisClient) Del(ctx context.Context, keys ...string) error {
	if m.delError != nil {
		return m.delError
	}
	for _, key := range keys {
		delete(m.data, key)
	}
	return nil
}

// Close 关闭连接
func (m *MockRedisClient) Close() error {
	return nil
}

// Ping 测试连接
func (m *MockRedisClient) Ping(ctx context.Context) error {
	return nil
}

// WithGetError 设置Get操作的错误
func (m *MockRedisClient) WithGetError(err error) *MockRedisClient {
	m.getError = err
	return m
}

// WithSetError 设置Set操作的错误
func (m *MockRedisClient) WithSetError(err error) *MockRedisClient {
	m.setError = err
	return m
}

// WithDelError 设置Del操作的错误
func (m *MockRedisClient) WithDelError(err error) *MockRedisClient {
	m.delError = err
	return m
}
