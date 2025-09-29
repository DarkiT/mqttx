package mqttx

import (
	"testing"
)

func TestDefaultStorageOptions(t *testing.T) {
	opts := DefaultStorageOptions()

	// 验证默认值
	if opts.Type != StoreTypeMemory {
		t.Errorf("DefaultStorageOptions.Type = %s; want memory", opts.Type)
	}

	if opts.Path != "" {
		t.Errorf("DefaultStorageOptions.Path = %s; want empty string", opts.Path)
	}

	if opts.Redis == nil {
		t.Errorf("DefaultStorageOptions.Redis is nil; want non-nil")
	}
}

func TestDefaultRedisOptions(t *testing.T) {
	opts := DefaultRedisOptions()

	// 验证默认值
	if opts.Addr != "localhost:6379" {
		t.Errorf("DefaultRedisOptions.Addr = %s; want localhost:6379", opts.Addr)
	}

	if opts.Username != "" {
		t.Errorf("DefaultRedisOptions.Username = %s; want empty string", opts.Username)
	}

	if opts.Password != "" {
		t.Errorf("DefaultRedisOptions.Password = %s; want empty string", opts.Password)
	}

	if opts.DB != 0 {
		t.Errorf("DefaultRedisOptions.DB = %d; want 0", opts.DB)
	}

	if opts.KeyPrefix != "mqttx:session:" {
		t.Errorf("DefaultRedisOptions.KeyPrefix = %s; want mqttx:session:", opts.KeyPrefix)
	}

	if opts.TTL != 86400 {
		t.Errorf("DefaultRedisOptions.TTL = %d; want 86400", opts.TTL)
	}

	if opts.PoolSize != 10 {
		t.Errorf("DefaultRedisOptions.PoolSize = %d; want 10", opts.PoolSize)
	}
}

func TestWithStorage(t *testing.T) {
	opts := DefaultOptions()

	// 测试文件存储
	opts.WithStorage(StoreTypeFile, "/tmp/mqttx")

	if opts.Storage.Type != StoreTypeFile {
		t.Errorf("WithStorage did not set Type correctly: got %s, want %s", opts.Storage.Type, StoreTypeFile)
	}

	if opts.Storage.Path != "/tmp/mqttx" {
		t.Errorf("WithStorage did not set Path correctly: got %s, want /tmp/mqttx", opts.Storage.Path)
	}

	// 测试内存存储
	opts.WithStorage(StoreTypeMemory, "")

	if opts.Storage.Type != StoreTypeMemory {
		t.Errorf("WithStorage did not set Type correctly: got %s, want %s", opts.Storage.Type, StoreTypeMemory)
	}
}

func TestWithRedisStorage(t *testing.T) {
	opts := DefaultOptions()

	// 测试Redis存储
	opts.WithRedisStorage(
		"redis.example.com:6379",
		"user",
		"password",
		1,
		"custom:prefix:",
		3600,
	)

	if opts.Storage.Type != StoreTypeRedis {
		t.Errorf("WithRedisStorage did not set Type correctly: got %s, want %s", opts.Storage.Type, StoreTypeRedis)
	}

	if opts.Storage.Redis == nil {
		t.Fatalf("WithRedisStorage did not set Redis options")
	}

	redis := opts.Storage.Redis

	if redis.Addr != "redis.example.com:6379" {
		t.Errorf("WithRedisStorage did not set Addr correctly: got %s, want redis.example.com:6379", redis.Addr)
	}

	if redis.Username != "user" {
		t.Errorf("WithRedisStorage did not set Username correctly: got %s, want user", redis.Username)
	}

	if redis.Password != "password" {
		t.Errorf("WithRedisStorage did not set Password correctly: got %s, want password", redis.Password)
	}

	if redis.DB != 1 {
		t.Errorf("WithRedisStorage did not set DB correctly: got %d, want 1", redis.DB)
	}

	if redis.KeyPrefix != "custom:prefix:" {
		t.Errorf("WithRedisStorage did not set KeyPrefix correctly: got %s, want custom:prefix:", redis.KeyPrefix)
	}

	if redis.TTL != 3600 {
		t.Errorf("WithRedisStorage did not set TTL correctly: got %d, want 3600", redis.TTL)
	}

	// 测试默认值
	opts = DefaultOptions()
	opts.WithRedisStorage(
		"redis.example.com:6379",
		"",
		"",
		0,
		"",
		0,
	)

	redis = opts.Storage.Redis

	// KeyPrefix应该保持默认值
	if redis.KeyPrefix != "mqttx:session:" {
		t.Errorf("WithRedisStorage did not keep default KeyPrefix: got %s, want mqttx:session:", redis.KeyPrefix)
	}

	// TTL应该保持默认值
	if redis.TTL != 86400 {
		t.Errorf("WithRedisStorage did not keep default TTL: got %d, want 86400", redis.TTL)
	}
}

func TestOptions_Validate_Storage(t *testing.T) {
	// 测试文件存储验证
	opts := DefaultOptions()
	opts.WithStorage(StoreTypeFile, "")

	err := opts.Validate()
	if err == nil {
		t.Errorf("Validate should fail with empty path for file store")
	}

	// 测试Redis存储验证
	opts = DefaultOptions()
	opts.WithStorage(StoreTypeRedis, "")
	opts.Storage.Redis.Addr = ""

	err = opts.Validate()
	if err == nil {
		t.Errorf("Validate should fail with empty addr for redis store")
	}

	// 测试Redis TTL验证
	opts = DefaultOptions()
	opts.WithStorage(StoreTypeRedis, "")
	opts.Storage.Redis.TTL = 0

	err = opts.Validate()
	if err != nil {
		t.Errorf("Validate should not fail with zero TTL for redis store: %v", err)
	}

	// TTL应该被设置为默认值
	if opts.Storage.Redis.TTL != 86400 {
		t.Errorf("Validate did not set default TTL: got %d, want 86400", opts.Storage.Redis.TTL)
	}

	// 测试默认存储类型
	opts = DefaultOptions()
	opts.Storage.Type = "invalid"

	err = opts.Validate()
	if err != nil {
		t.Errorf("Validate should not fail with invalid store type: %v", err)
	}

	// 类型应该被设置为默认值
	if opts.Storage.Type != StoreTypeMemory {
		t.Errorf("Validate did not set default store type: got %s, want %s", opts.Storage.Type, StoreTypeMemory)
	}
}
