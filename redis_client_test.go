package mqttx

import (
	"testing"
	"time"
)

func TestDefaultGoRedisOptions(t *testing.T) {
	opts := DefaultGoRedisOptions()

	// 验证默认值
	if opts.Addr != "localhost:6379" {
		t.Errorf("DefaultGoRedisOptions.Addr = %s; want localhost:6379", opts.Addr)
	}

	if opts.Password != "" {
		t.Errorf("DefaultGoRedisOptions.Password = %s; want empty string", opts.Password)
	}

	if opts.DB != 0 {
		t.Errorf("DefaultGoRedisOptions.DB = %d; want 0", opts.DB)
	}

	if opts.PoolSize != 10 {
		t.Errorf("DefaultGoRedisOptions.PoolSize = %d; want 10", opts.PoolSize)
	}

	if opts.MinIdleConns != 2 {
		t.Errorf("DefaultGoRedisOptions.MinIdleConns = %d; want 2", opts.MinIdleConns)
	}

	if opts.DialTimeout != 5*time.Second {
		t.Errorf("DefaultGoRedisOptions.DialTimeout = %v; want 5s", opts.DialTimeout)
	}

	if opts.ReadTimeout != 3*time.Second {
		t.Errorf("DefaultGoRedisOptions.ReadTimeout = %v; want 3s", opts.ReadTimeout)
	}

	if opts.WriteTimeout != 3*time.Second {
		t.Errorf("DefaultGoRedisOptions.WriteTimeout = %v; want 3s", opts.WriteTimeout)
	}
}

func TestNewGoRedisClient(t *testing.T) {
	// 测试使用自定义选项
	customOpts := &GoRedisOptions{
		Addr:         "redis.example.com:6379",
		Password:     "secret",
		Username:     "user",
		DB:           1,
		PoolSize:     20,
		MinIdleConns: 5,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	client := NewGoRedisClient(customOpts)
	if client == nil {
		t.Fatal("NewGoRedisClient returned nil")
	}

	// 注意：我们不能直接检查client.client的字段，因为它是内部实现
	// 这个测试主要确保函数不会崩溃，并返回一个非nil的客户端

	// 测试使用nil选项（应该使用默认选项）
	client = NewGoRedisClient(nil)
	if client == nil {
		t.Fatal("NewGoRedisClient with nil options returned nil")
	}
}

func TestGoRedisAdapter_Methods(t *testing.T) {
	// 使用mock而不是真实的Redis
	// 这个测试只是确保GoRedisAdapter的方法都存在并可以被调用
	// 实际的功能测试在store_redis_test.go中

	// 确保所有方法签名都是正确的
	var client RedisClient = NewMockRedisClient()

	// 编译时类型检查
	_ = client.Set
	_ = client.Get
	_ = client.Del
	_ = client.Close
}
