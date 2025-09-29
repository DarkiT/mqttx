package mqttx

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// RedisStore 基于Redis的会话状态存储
type RedisStore struct {
	client RedisClient
	prefix string
	ttl    time.Duration
}

// RedisClient Redis客户端接口
// 这是一个接口，允许使用不同的Redis客户端实现
type RedisClient interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) error
	Close() error
}

// RedisStoreOptions Redis存储选项
type RedisStoreOptions struct {
	Prefix string        // 键前缀
	TTL    time.Duration // 过期时间
}

// NewRedisStore 创建新的Redis存储
func NewRedisStore(client RedisClient, opts *RedisStoreOptions) *RedisStore {
	if opts == nil {
		opts = &RedisStoreOptions{
			Prefix: "mqttx:session:",
			TTL:    24 * time.Hour, // 默认24小时过期
		}
	}

	if opts.Prefix == "" {
		opts.Prefix = "mqttx:session:"
	}

	if opts.TTL <= 0 {
		opts.TTL = 24 * time.Hour
	}

	return &RedisStore{
		client: client,
		prefix: opts.Prefix,
		ttl:    opts.TTL,
	}
}

// SaveState 保存会话状态
func (s *RedisStore) SaveState(sessionName string, state *SessionState) error {
	if state == nil {
		return fmt.Errorf("state cannot be nil")
	}

	data, err := json.Marshal(state)
	if err != nil {
		return wrapError(err, "failed to marshal session state")
	}

	key := s.prefix + sessionName
	ctx := context.Background()
	if err := s.client.Set(ctx, key, string(data), s.ttl); err != nil {
		return wrapError(err, "failed to save session state to redis")
	}

	return nil
}

// LoadState 加载会话状态
func (s *RedisStore) LoadState(sessionName string) (*SessionState, error) {
	key := s.prefix + sessionName
	ctx := context.Background()

	data, err := s.client.Get(ctx, key)
	if err != nil {
		// 如果是键不存在错误，返回nil,nil
		if err.Error() == "redis: nil" || err.Error() == "key not found" {
			return nil, nil
		}
		return nil, wrapError(err, "failed to get session state from redis")
	}

	if data == "" {
		return nil, nil
	}

	var state SessionState
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		return nil, wrapError(err, "failed to unmarshal session state")
	}

	return &state, nil
}

// DeleteState 删除会话状态
func (s *RedisStore) DeleteState(sessionName string) error {
	key := s.prefix + sessionName
	ctx := context.Background()

	if err := s.client.Del(ctx, key); err != nil {
		return wrapError(err, "failed to delete session state from redis")
	}

	return nil
}

// Close 关闭Redis连接
func (s *RedisStore) Close() error {
	return s.client.Close()
}
