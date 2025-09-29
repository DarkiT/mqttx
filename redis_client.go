package mqttx

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// GoRedisAdapter 是go-redis库的适配器，实现RedisClient接口
type GoRedisAdapter struct {
	client *redis.Client
}

// GoRedisOptions Redis连接选项
type GoRedisOptions struct {
	Addr     string
	Password string
	DB       int
	Username string
	// 连接池配置
	PoolSize     int
	MinIdleConns int
	// 超时设置
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// DefaultGoRedisOptions 返回默认的Redis选项
func DefaultGoRedisOptions() *GoRedisOptions {
	return &GoRedisOptions{
		Addr:         "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 2,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
}

// NewGoRedisClient 创建一个新的Go Redis客户端
func NewGoRedisClient(opts *GoRedisOptions) *GoRedisAdapter {
	if opts == nil {
		opts = DefaultGoRedisOptions()
	}

	client := redis.NewClient(&redis.Options{
		Addr:         opts.Addr,
		Password:     opts.Password,
		Username:     opts.Username,
		DB:           opts.DB,
		PoolSize:     opts.PoolSize,
		MinIdleConns: opts.MinIdleConns,
		DialTimeout:  opts.DialTimeout,
		ReadTimeout:  opts.ReadTimeout,
		WriteTimeout: opts.WriteTimeout,
	})

	return &GoRedisAdapter{
		client: client,
	}
}

// Set 实现RedisClient接口的Set方法
func (a *GoRedisAdapter) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return a.client.Set(ctx, key, value, expiration).Err()
}

// Get 实现RedisClient接口的Get方法
func (a *GoRedisAdapter) Get(ctx context.Context, key string) (string, error) {
	return a.client.Get(ctx, key).Result()
}

// Del 实现RedisClient接口的Del方法
func (a *GoRedisAdapter) Del(ctx context.Context, keys ...string) error {
	return a.client.Del(ctx, keys...).Err()
}

// Close 实现RedisClient接口的Close方法
func (a *GoRedisAdapter) Close() error {
	return a.client.Close()
}

// Ping 测试Redis连接
func (a *GoRedisAdapter) Ping(ctx context.Context) error {
	return a.client.Ping(ctx).Err()
}
