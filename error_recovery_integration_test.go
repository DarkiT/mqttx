package mqttx

import (
	"fmt"
	"testing"
	"time"
)

// TestErrorRecoveryAndMetricsIntegration 测试错误恢复系统与指标系统的整合
func TestErrorRecoveryAndMetricsIntegration(t *testing.T) {
	m := NewSessionManager()

	// 添加测试会话
	opts := &Options{
		Name:     "integration_test",
		Brokers:  []string{"tcp://broker.emqx.io:1883"},
		ClientID: "client_integration",
	}
	if err := m.AddSession(opts); err != nil {
		t.Fatalf("Failed to add session: %v", err)
	}

	// 初始指标状态
	initialMetrics := m.GetMetrics()
	initialErrorCount := initialMetrics["error_count"]

	// 注册多个错误
	for i := 0; i < 3; i++ {
		testErr := fmt.Errorf("integration test error %d", i)
		m.RegisterError("integration_test", testErr, CategoryConnection)
	}

	// 验证全局错误计数已更新
	updatedMetrics := m.GetMetrics()
	updatedErrorCount := updatedMetrics["error_count"]

	// 错误计数应该增加了3
	expectedErrorCount := initialErrorCount.(uint64) + 3
	if updatedErrorCount.(uint64) != expectedErrorCount {
		t.Errorf("Expected error count %d, got %d",
			expectedErrorCount, updatedErrorCount.(uint64))
	}

	// 验证错误统计正确
	errorStats := m.GetErrorStats()
	if errorStats["total"].(int) != 3 {
		t.Errorf("Expected 3 errors in stats, got %d", errorStats["total"].(int))
	}

	// 模拟连接恢复
	for _, info := range m.GetActiveErrors() {
		m.recovery.MarkErrorRecovered("integration_test", info.Category, info.Error)
	}

	// 清理已恢复的错误
	clearedCount := m.ClearRecoveredErrors()
	if clearedCount != 3 {
		t.Errorf("Expected to clear 3 errors, cleared %d", clearedCount)
	}

	// 验证活跃错误为0
	activeErrors := m.GetActiveErrors()
	if len(activeErrors) != 0 {
		t.Errorf("Expected 0 active errors after clearing, got %d", len(activeErrors))
	}

	// 错误计数不会因为清理而减少
	finalMetrics := m.GetMetrics()
	finalErrorCount := finalMetrics["error_count"]
	if finalErrorCount.(uint64) != expectedErrorCount {
		t.Errorf("Error count should remain at %d after clearing, got %d",
			expectedErrorCount, finalErrorCount.(uint64))
	}
}

// TestReconnectMetricsIntegration 测试重连与指标系统的整合
func TestReconnectMetricsIntegration(t *testing.T) {
	m := NewSessionManager()

	// 添加测试会话
	opts := &Options{
		Name:     "reconnect_test",
		Brokers:  []string{"tcp://broker.emqx.io:1883"},
		ClientID: "client_reconnect",
		ConnectProps: &ConnectProps{
			AutoReconnect:            true,
			InitialReconnectInterval: 100 * time.Millisecond,
			MaxReconnectInterval:     1 * time.Second,
		},
	}
	if err := m.AddSession(opts); err != nil {
		t.Fatalf("Failed to add session: %v", err)
	}

	// 获取会话
	session, err := m.GetSession("reconnect_test")
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	// 初始指标状态
	initialMetrics := m.GetMetrics()
	initialReconnectCount := initialMetrics["reconnect_count"]

	// 模拟重连事件
	session.handleReconnecting()

	// 验证会话和全局重连计数都已更新
	sessionMetrics := session.GetMetrics()
	if sessionMetrics["reconnects"].(uint64) != 1 {
		t.Errorf("Expected session reconnect count 1, got %d",
			sessionMetrics["reconnects"].(uint64))
	}

	updatedMetrics := m.GetMetrics()
	updatedReconnectCount := updatedMetrics["reconnect_count"]
	expectedReconnectCount := initialReconnectCount.(uint64) + 1

	if updatedReconnectCount.(uint64) != expectedReconnectCount {
		t.Errorf("Expected global reconnect count %d, got %d",
			expectedReconnectCount, updatedReconnectCount.(uint64))
	}
}
