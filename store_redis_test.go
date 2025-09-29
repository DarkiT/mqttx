package mqttx

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestRedisStore_SaveState(t *testing.T) {
	// 准备测试数据
	mockClient := NewMockRedisClient()
	store := NewRedisStore(mockClient, nil)
	sessionName := "test-session"
	state := &SessionState{
		Topics: []TopicSubscription{
			{Topic: "test/topic", QoS: 1},
		},
		Messages: []*Message{
			{Topic: "test/topic", Payload: []byte("test message"), QoS: 1},
		},
		LastSequence:     123,
		LastConnected:    time.Now(),
		LastDisconnected: time.Now().Add(-1 * time.Hour),
		QoSMessages:      make(map[uint16]*Message),
		RetainedMessages: make(map[string]*Message),
		ClientID:         "test-client",
		Version:          1,
	}

	// 测试保存状态
	err := store.SaveState(sessionName, state)
	if err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	// 验证数据是否正确保存
	key := "mqttx:session:test-session"
	value, exists := mockClient.data[key]
	if !exists {
		t.Fatalf("State not saved to Redis")
	}

	// 解析保存的数据
	var savedState SessionState
	if err := json.Unmarshal([]byte(value), &savedState); err != nil {
		t.Fatalf("Failed to unmarshal saved state: %v", err)
	}

	// 验证数据字段
	if savedState.LastSequence != state.LastSequence {
		t.Errorf("LastSequence mismatch: got %d, want %d", savedState.LastSequence, state.LastSequence)
	}
	if savedState.ClientID != state.ClientID {
		t.Errorf("ClientID mismatch: got %s, want %s", savedState.ClientID, state.ClientID)
	}
	if len(savedState.Topics) != len(state.Topics) {
		t.Errorf("Topics length mismatch: got %d, want %d", len(savedState.Topics), len(state.Topics))
	}
	if len(savedState.Messages) != len(state.Messages) {
		t.Errorf("Messages length mismatch: got %d, want %d", len(savedState.Messages), len(state.Messages))
	}

	// 测试保存错误
	mockClient.WithSetError(errors.New("set error"))
	err = store.SaveState(sessionName, state)
	if err == nil {
		t.Errorf("Expected error when SaveState fails, got nil")
	}
}

func TestRedisStore_LoadState(t *testing.T) {
	// 准备测试数据
	mockClient := NewMockRedisClient()
	store := NewRedisStore(mockClient, nil)
	sessionName := "test-session"
	state := &SessionState{
		Topics: []TopicSubscription{
			{Topic: "test/topic", QoS: 1},
		},
		Messages: []*Message{
			{Topic: "test/topic", Payload: []byte("test message"), QoS: 1},
		},
		LastSequence:     123,
		LastConnected:    time.Now(),
		LastDisconnected: time.Now().Add(-1 * time.Hour),
		QoSMessages:      make(map[uint16]*Message),
		RetainedMessages: make(map[string]*Message),
		ClientID:         "test-client",
		Version:          1,
	}

	// 首先保存状态
	data, _ := json.Marshal(state)
	mockClient.Set(context.Background(), "mqttx:session:test-session", string(data), 0)

	// 测试加载状态
	loadedState, err := store.LoadState(sessionName)
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if loadedState == nil {
		t.Fatalf("LoadState returned nil state")
	}

	// 验证加载的数据
	if loadedState.LastSequence != state.LastSequence {
		t.Errorf("LastSequence mismatch: got %d, want %d", loadedState.LastSequence, state.LastSequence)
	}
	if loadedState.ClientID != state.ClientID {
		t.Errorf("ClientID mismatch: got %s, want %s", loadedState.ClientID, state.ClientID)
	}
	if len(loadedState.Topics) != len(state.Topics) {
		t.Errorf("Topics length mismatch: got %d, want %d", len(loadedState.Topics), len(state.Topics))
	}
	if len(loadedState.Messages) != len(state.Messages) {
		t.Errorf("Messages length mismatch: got %d, want %d", len(loadedState.Messages), len(state.Messages))
	}

	// 测试不存在的会话
	nonExistentState, err := store.LoadState("non-existent")
	if err != nil {
		t.Errorf("LoadState for non-existent session should not return error: %v", err)
	}
	if nonExistentState != nil {
		t.Errorf("LoadState for non-existent session should return nil state, got: %+v", nonExistentState)
	}

	// 测试加载错误
	mockClient.WithGetError(errors.New("get error"))
	_, err = store.LoadState(sessionName)
	if err == nil {
		t.Errorf("Expected error when LoadState fails, got nil")
	}

	// 测试解析错误
	mockClient.WithGetError(nil)
	mockClient.Set(context.Background(), "mqttx:session:test-session", "invalid json", 0)
	_, err = store.LoadState(sessionName)
	if err == nil {
		t.Errorf("Expected error when unmarshaling invalid JSON, got nil")
	}
}

func TestRedisStore_DeleteState(t *testing.T) {
	// 准备测试数据
	mockClient := NewMockRedisClient()
	store := NewRedisStore(mockClient, nil)
	sessionName := "test-session"

	// 首先保存一些数据
	mockClient.Set(context.Background(), "mqttx:session:test-session", "{}", 0)

	// 测试删除状态
	err := store.DeleteState(sessionName)
	if err != nil {
		t.Fatalf("DeleteState failed: %v", err)
	}

	// 验证数据是否已删除
	_, exists := mockClient.data["mqttx:session:test-session"]
	if exists {
		t.Errorf("State not deleted from Redis")
	}

	// 测试删除错误
	mockClient.WithDelError(errors.New("del error"))
	err = store.DeleteState(sessionName)
	if err == nil {
		t.Errorf("Expected error when DeleteState fails, got nil")
	}
}

func TestRedisStore_WithOptions(t *testing.T) {
	// 测试自定义选项
	mockClient := NewMockRedisClient()
	customOpts := &RedisStoreOptions{
		Prefix: "custom:prefix:",
		TTL:    1 * time.Hour,
	}
	store := NewRedisStore(mockClient, customOpts)

	// 保存数据
	sessionName := "test-session"
	state := &SessionState{ClientID: "test-client"}
	err := store.SaveState(sessionName, state)
	if err != nil {
		t.Fatalf("SaveState with custom options failed: %v", err)
	}

	// 验证自定义前缀是否生效
	key := "custom:prefix:test-session"
	_, exists := mockClient.data[key]
	if !exists {
		t.Errorf("Custom prefix not applied: expected key %s not found", key)
	}
}

func TestRedisStore_NilOptions(t *testing.T) {
	// 测试nil选项
	mockClient := NewMockRedisClient()
	store := NewRedisStore(mockClient, nil)

	// 保存数据
	sessionName := "test-session"
	state := &SessionState{ClientID: "test-client"}
	err := store.SaveState(sessionName, state)
	if err != nil {
		t.Fatalf("SaveState with nil options failed: %v", err)
	}

	// 验证默认前缀是否生效
	key := "mqttx:session:test-session"
	_, exists := mockClient.data[key]
	if !exists {
		t.Errorf("Default prefix not applied: expected key %s not found", key)
	}
}

func TestRedisStore_NilState(t *testing.T) {
	// 测试空状态
	mockClient := NewMockRedisClient()
	store := NewRedisStore(mockClient, nil)

	// 尝试保存nil状态
	err := store.SaveState("test-session", nil)
	if err == nil {
		t.Errorf("Expected error when saving nil state, got nil")
	}
}
