package mqttx

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileStore(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "mqttx-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建文件存储
	store, err := NewFileStore(tempDir)
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}

	// 准备测试数据
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
	err = store.SaveState(sessionName, state)
	if err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	// 验证文件是否创建
	expectedFile := filepath.Join(tempDir, sessionName+".json")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("State file not created: %s", expectedFile)
	}

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

	// 测试删除状态
	err = store.DeleteState(sessionName)
	if err != nil {
		t.Fatalf("DeleteState failed: %v", err)
	}

	// 验证文件是否删除
	if _, err := os.Stat(expectedFile); !os.IsNotExist(err) {
		t.Errorf("State file not deleted: %s", expectedFile)
	}

	// 测试不存在的会话
	nonExistentState, err := store.LoadState("non-existent")
	if err != nil {
		t.Errorf("LoadState for non-existent session should not return error: %v", err)
	}
	if nonExistentState != nil {
		t.Errorf("LoadState for non-existent session should return nil state, got: %+v", nonExistentState)
	}
}

func TestMemoryStore(t *testing.T) {
	// 创建内存存储
	store := NewMemoryStore()

	// 准备测试数据
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

	// 测试删除状态
	err = store.DeleteState(sessionName)
	if err != nil {
		t.Fatalf("DeleteState failed: %v", err)
	}

	// 验证状态是否删除
	loadedState, err = store.LoadState(sessionName)
	if err != nil {
		t.Errorf("LoadState after delete should not return error: %v", err)
	}
	if loadedState != nil {
		t.Errorf("LoadState after delete should return nil state, got: %+v", loadedState)
	}

	// 测试不存在的会话
	nonExistentState, err := store.LoadState("non-existent")
	if err != nil {
		t.Errorf("LoadState for non-existent session should not return error: %v", err)
	}
	if nonExistentState != nil {
		t.Errorf("LoadState for non-existent session should return nil state, got: %+v", nonExistentState)
	}
}
