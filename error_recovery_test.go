package mqttx

import (
	"errors"
	"fmt"
	"testing"
)

// TestErrorRecovery 测试错误恢复机制
func TestErrorRecovery(t *testing.T) {
	m := NewSessionManager()

	// 添加测试会话
	opts := &Options{
		Name:     "test1",
		Brokers:  []string{"tcp://broker.emqx.io:1883"},
		ClientID: "client1",
	}
	if err := m.AddSession(opts); err != nil {
		t.Fatalf("Failed to add session: %v", err)
	}

	// 测试错误注册和恢复
	testErr := errors.New("test error")
	info := m.RegisterError("test1", testErr, CategoryConnection)

	if info == nil {
		t.Fatal("RegisterError should return error info")
	}

	if info.Error != testErr {
		t.Errorf("Error mismatch, got %v, want %v", info.Error, testErr)
	}

	if info.Category != CategoryConnection {
		t.Errorf("Category mismatch, got %v, want %v", info.Category, CategoryConnection)
	}

	// 验证错误严重程度
	if info.Severity != SeverityWarning {
		t.Errorf("Expected SeverityWarning for connection error, got %v", info.Severity)
	}

	// 验证恢复策略
	if info.Strategy != StrategyReconnect {
		t.Errorf("Expected StrategyReconnect for connection error, got %v", info.Strategy)
	}

	// 测试获取活跃错误
	activeErrors := m.GetActiveErrors()
	if len(activeErrors) != 1 {
		t.Errorf("Expected 1 active error, got %d", len(activeErrors))
	}

	// 测试标记错误已恢复
	if m.recovery.MarkErrorRecovered("test1", CategoryConnection, testErr) != true {
		t.Error("MarkErrorRecovered should return true")
	}

	// 测试清理已恢复的错误
	count := m.ClearRecoveredErrors()
	if count != 1 {
		t.Errorf("Expected 1 cleared error, got %d", count)
	}

	// 测试获取错误统计
	stats := m.GetErrorStats()
	if stats["total"].(int) != 0 {
		t.Errorf("Expected 0 total errors after clearing, got %d", stats["total"].(int))
	}
}

// TestErrorCategories 测试不同类别的错误处理
func TestErrorCategories(t *testing.T) {
	m := NewSessionManager()

	// 添加测试会话
	opts := &Options{
		Name:     "test_cat",
		Brokers:  []string{"tcp://broker.emqx.io:1883"},
		ClientID: "client_cat",
	}
	if err := m.AddSession(opts); err != nil {
		t.Fatalf("Failed to add session: %v", err)
	}

	errorCases := []struct {
		category         ErrorCategory
		expectedSeverity ErrorSeverity
		expectedStrategy RecoveryStrategy
	}{
		{CategoryConnection, SeverityWarning, StrategyReconnect},
		{CategorySubscription, SeverityWarning, StrategyRetry},
		{CategoryPublish, SeverityWarning, StrategyRetry},
		{CategoryAuthentication, SeverityCritical, StrategyNotify},
		{CategoryInternal, SeverityCritical, StrategyNotify},
	}

	for _, tc := range errorCases {
		testErr := fmt.Errorf("test error for %s", tc.category)
		info := m.RegisterError("test_cat", testErr, tc.category)

		if info.Severity != tc.expectedSeverity {
			t.Errorf("Category %s: expected severity %v, got %v",
				tc.category, tc.expectedSeverity, info.Severity)
		}

		if info.Strategy != tc.expectedStrategy {
			t.Errorf("Category %s: expected strategy %v, got %v",
				tc.category, tc.expectedStrategy, info.Strategy)
		}
	}
}

// TestErrorLimit 测试错误数量限制功能
func TestErrorLimit(t *testing.T) {
	m := NewSessionManager()

	// 设置低错误限制以便测试
	m.recovery.errorLimit = 5

	// 添加测试会话
	opts := &Options{
		Name:     "test_limit",
		Brokers:  []string{"tcp://broker.emqx.io:1883"},
		ClientID: "client_limit",
	}
	if err := m.AddSession(opts); err != nil {
		t.Fatalf("Failed to add session: %v", err)
	}

	// 生成多个错误，超过限制
	for i := 0; i < 10; i++ {
		testErr := fmt.Errorf("test error %d", i)
		m.RegisterError("test_limit", testErr, CategoryConnection)
	}

	// 验证错误数量不超过限制
	activeErrors := m.GetActiveErrors()
	if len(activeErrors) > m.recovery.errorLimit {
		t.Errorf("Error count %d exceeded limit %d",
			len(activeErrors), m.recovery.errorLimit)
	}

	// 验证最后几个错误被正确记录
	lastErrorFound := false
	for _, err := range activeErrors {
		if err.Error.Error() == "test error 9" {
			lastErrorFound = true
			break
		}
	}

	if !lastErrorFound {
		t.Error("Last registered error not found, error rotation failed")
	}
}
