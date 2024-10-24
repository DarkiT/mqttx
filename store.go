package mqtt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileStore 基于文件的会话状态存储
type FileStore struct {
	directory string
	mu        sync.RWMutex
}

// NewFileStore 创建新的文件存储
func NewFileStore(directory string) (*FileStore, error) {
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	return &FileStore{
		directory: directory,
	}, nil
}

// SaveState 保存会话状态
func (s *FileStore) SaveState(sessionName string, state *SessionState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(state)
	if err != nil {
		return wrapError(err, "failed to marshal session state")
	}

	filename := filepath.Join(s.directory, fmt.Sprintf("%s.json", sessionName))
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return wrapError(err, "failed to write session state")
	}

	return nil
}

// LoadState 加载会话状态
func (s *FileStore) LoadState(sessionName string) (*SessionState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filename := filepath.Join(s.directory, fmt.Sprintf("%s.json", sessionName))
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, wrapError(err, "failed to read session state")
	}

	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, wrapError(err, "failed to unmarshal session state")
	}

	return &state, nil
}

// DeleteState 删除会话状态
func (s *FileStore) DeleteState(sessionName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := filepath.Join(s.directory, fmt.Sprintf("%s.json", sessionName))
	if err := os.Remove(filename); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return wrapError(err, "failed to delete session state")
	}

	return nil
}

// MemoryStore 基于内存的会话状态存储
type MemoryStore struct {
	states map[string]*SessionState
	mu     sync.RWMutex
}

// NewMemoryStore 创建新的内存存储
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		states: make(map[string]*SessionState),
	}
}

// SaveState 保存会话状态
func (s *MemoryStore) SaveState(sessionName string, state *SessionState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.states[sessionName] = state
	return nil
}

// LoadState 加载会话状态
func (s *MemoryStore) LoadState(sessionName string) (*SessionState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.states[sessionName]
	if !ok {
		return nil, nil
	}
	return state, nil
}

// DeleteState 删除会话状态
func (s *MemoryStore) DeleteState(sessionName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.states, sessionName)
	return nil
}
