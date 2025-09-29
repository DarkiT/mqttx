package mqttx

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"
)

// TestDefaultErrorHandler 测试默认错误处理器
func TestDefaultErrorHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	handler := NewDefaultErrorHandler(logger)
	ctx := context.Background()

	t.Run("HandleMQTTXError", func(t *testing.T) {
		// 测试不同严重程度的错误处理
		testCases := []struct {
			name         string
			err          *MQTTXError
			shouldHandle bool
		}{
			{
				name: "Info Error",
				err: NewError(TypeConnection, SeverityInfo, "connection info").
					WithSession("test-session"),
				shouldHandle: true,
			},
			{
				name: "Warning Error",
				err: NewError(TypeSubscription, SeverityWarning, "subscription warning").
					WithTopic("test/topic"),
				shouldHandle: true,
			},
			{
				name: "Regular Error",
				err: NewError(TypePublish, SeverityError, "publish failed").
					WithSession("test-session").
					WithTopic("test/topic"),
				shouldHandle: false, // 非临时错误
			},
			{
				name:         "Critical Error",
				err:          NewError(TypeConfiguration, SeverityCritical, "critical config error"),
				shouldHandle: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				handled := handler.HandleError(ctx, tc.err)
				if handled != tc.shouldHandle {
					t.Errorf("Expected handled=%v, got %v for %s",
						tc.shouldHandle, handled, tc.name)
				}
			})
		}
	})

	t.Run("HandleRegularError", func(t *testing.T) {
		regularErr := errors.New("regular error")
		handled := handler.HandleError(ctx, regularErr)
		if handled {
			t.Error("Regular errors should not be handled by default")
		}
	})

	t.Run("HandleNilError", func(t *testing.T) {
		handled := handler.HandleError(ctx, nil)
		if !handled {
			t.Error("Nil errors should be considered handled")
		}
	})
}

// TestInMemoryErrorReporter 测试内存错误报告器
func TestInMemoryErrorReporter(t *testing.T) {
	reporter := NewInMemoryErrorReporter()

	t.Run("ReportError", func(t *testing.T) {
		// 报告不同类型的错误
		err1 := NewConnectionError("connection failed", nil)
		err2 := NewPublishError("test/topic", errors.New("publish failed"))
		err3 := errors.New("regular error")

		reporter.ReportError(err1)
		reporter.ReportError(err2)
		reporter.ReportError(err3)

		stats := reporter.GetErrorStats()
		totalErrors := stats["total_errors"].(int)
		if totalErrors != 3 {
			t.Errorf("Expected 3 total errors, got %d", totalErrors)
		}

		// 检查类型统计
		if stats["type_connection"] != 1 {
			t.Error("Expected 1 connection error")
		}
		if stats["type_publish"] != 1 {
			t.Error("Expected 1 publish error")
		}
		if stats["unknown_type"] != 1 {
			t.Error("Expected 1 unknown type error")
		}
	})

	t.Run("GetRecentErrors", func(t *testing.T) {
		reporter := NewInMemoryErrorReporter()

		// 添加多个错误
		for i := 0; i < 10; i++ {
			err := NewError(TypeConnection, SeverityError, "error %d")
			reporter.ReportError(err)
		}

		// 获取最近5个错误
		recentErrors := reporter.GetRecentErrors(5)
		if len(recentErrors) != 5 {
			t.Errorf("Expected 5 recent errors, got %d", len(recentErrors))
		}

		// 获取所有错误（限制大于实际数量）
		allErrors := reporter.GetRecentErrors(20)
		if len(allErrors) != 10 {
			t.Errorf("Expected 10 errors, got %d", len(allErrors))
		}
	})
}

// TestWrapError 测试错误包装
func TestWrapError(t *testing.T) {
	t.Run("WrapRegularError", func(t *testing.T) {
		originalErr := errors.New("original error")
		wrappedErr := WrapError(originalErr, TypeConnection, "connection failed")

		if wrappedErr == nil {
			t.Fatal("Expected wrapped error, got nil")
		}

		if wrappedErr.Type != TypeConnection {
			t.Errorf("Expected type %s, got %s", TypeConnection, wrappedErr.Type)
		}

		if wrappedErr.Message != "connection failed" {
			t.Errorf("Expected message 'connection failed', got '%s'", wrappedErr.Message)
		}

		// 检查原始错误是否保存在上下文中
		if wrappedErr.Context["original_error"] != "original error" {
			t.Error("Original error not preserved in context")
		}
	})

	t.Run("WrapMQTTXError", func(t *testing.T) {
		mqttErr := NewConnectionError("original message", nil)
		wrappedErr := WrapError(mqttErr, TypeConnection, "enhanced")

		if wrappedErr.Message != "enhanced: original message" {
			t.Errorf("Expected enhanced message, got '%s'", wrappedErr.Message)
		}
	})

	t.Run("WrapNilError", func(t *testing.T) {
		wrappedErr := WrapError(nil, TypeConnection, "test")
		if wrappedErr != nil {
			t.Error("Wrapping nil error should return nil")
		}
	})
}

// TestChainErrors 测试错误链接
func TestChainErrors(t *testing.T) {
	t.Run("ChainMultipleErrors", func(t *testing.T) {
		err1 := NewConnectionError("connection failed", nil)
		err2 := NewPublishError("test/topic", errors.New("publish failed"))
		err3 := errors.New("regular error")

		chainedErr := ChainErrors(err1, err2, err3)
		if chainedErr == nil {
			t.Fatal("Expected chained error, got nil")
		}

		validationErrs, ok := chainedErr.(*ValidationErrors)
		if !ok {
			t.Fatal("Expected ValidationErrors type")
		}

		if len(validationErrs.Errors) != 3 {
			t.Errorf("Expected 3 chained errors, got %d", len(validationErrs.Errors))
		}
	})

	t.Run("ChainWithNilErrors", func(t *testing.T) {
		err1 := NewConnectionError("connection failed", nil)

		chainedErr := ChainErrors(err1, nil, nil)
		if chainedErr == nil {
			t.Fatal("Expected chained error, got nil")
		}

		validationErrs, ok := chainedErr.(*ValidationErrors)
		if !ok {
			t.Fatal("Expected ValidationErrors type")
		}

		if len(validationErrs.Errors) != 1 {
			t.Errorf("Expected 1 chained error, got %d", len(validationErrs.Errors))
		}
	})

	t.Run("ChainAllNilErrors", func(t *testing.T) {
		chainedErr := ChainErrors(nil, nil, nil)
		if chainedErr != nil {
			t.Error("Chaining all nil errors should return nil")
		}
	})
}

// TestFilterErrors 测试错误过滤
func TestFilterErrors(t *testing.T) {
	errors := []error{
		NewConnectionError("connection error", nil),
		NewPublishError("test/topic", errors.New("publish error")),
		NewConnectionError("another connection error", nil),
		errors.New("regular error"),
	}

	connectionErrors := FilterErrors(errors, TypeConnection)
	if len(connectionErrors) != 2 {
		t.Errorf("Expected 2 connection errors, got %d", len(connectionErrors))
	}

	publishErrors := FilterErrors(errors, TypePublish)
	if len(publishErrors) != 1 {
		t.Errorf("Expected 1 publish error, got %d", len(publishErrors))
	}

	configErrors := FilterErrors(errors, TypeConfiguration)
	if len(configErrors) != 0 {
		t.Errorf("Expected 0 config errors, got %d", len(configErrors))
	}
}

// TestErrorMetrics 测试错误度量
func TestErrorMetrics(t *testing.T) {
	metrics := NewErrorMetrics()

	t.Run("RecordErrors", func(t *testing.T) {
		// 记录不同类型的错误
		err1 := NewConnectionError("connection failed", nil)
		err2 := NewPublishError("test/topic", errors.New("publish failed"))
		err3 := NewConnectionError("another connection error", nil)

		metrics.RecordError(err1)
		metrics.RecordError(err2)
		metrics.RecordError(err3)

		if metrics.TotalErrors != 3 {
			t.Errorf("Expected 3 total errors, got %d", metrics.TotalErrors)
		}

		if metrics.ErrorsByType["connection"] != 2 {
			t.Errorf("Expected 2 connection errors, got %d",
				metrics.ErrorsByType["connection"])
		}

		if metrics.ErrorsByType["publish"] != 1 {
			t.Errorf("Expected 1 publish error, got %d",
				metrics.ErrorsByType["publish"])
		}

		if metrics.ErrorsBySeverity["error"] != 3 {
			t.Errorf("Expected 3 error-severity errors, got %d",
				metrics.ErrorsBySeverity["error"])
		}
	})

	t.Run("GetSnapshot", func(t *testing.T) {
		snapshot := metrics.GetSnapshot()

		if snapshot["total_errors"] != uint64(3) {
			t.Error("Snapshot total_errors mismatch")
		}

		errorsByType, ok := snapshot["errors_by_type"].(map[string]uint64)
		if !ok {
			t.Fatal("errors_by_type not found in snapshot")
		}

		if errorsByType["connection"] != 2 {
			t.Error("Snapshot connection errors mismatch")
		}
	})

	t.Run("Reset", func(t *testing.T) {
		metrics.Reset()

		if metrics.TotalErrors != 0 {
			t.Error("Reset failed - total errors not zero")
		}

		if len(metrics.ErrorsByType) != 0 {
			t.Error("Reset failed - errors by type not empty")
		}

		if len(metrics.ErrorsBySeverity) != 0 {
			t.Error("Reset failed - errors by severity not empty")
		}
	})
}

// TestRetryableError 测试可重试错误
func TestRetryableError(t *testing.T) {
	baseErr := NewConnectionError("connection failed", ErrTimeout)
	retryableErr := NewRetryableError(baseErr, 3, int64(time.Millisecond*100))

	t.Run("InitialState", func(t *testing.T) {
		if !retryableErr.CanRetry() {
			t.Error("Should be able to retry initially")
		}

		if retryableErr.CurrentRetry != 0 {
			t.Errorf("Expected 0 current retry, got %d", retryableErr.CurrentRetry)
		}
	})

	t.Run("Retry", func(t *testing.T) {
		// 第一次重试
		retryableErr.NextRetry()
		if retryableErr.CurrentRetry != 1 {
			t.Errorf("Expected 1 current retry, got %d", retryableErr.CurrentRetry)
		}

		delay1 := retryableErr.GetRetryDelay()
		expectedDelay1 := int64(time.Millisecond * 200) // 100 * 2^1
		if delay1 != expectedDelay1 {
			t.Errorf("Expected delay %d, got %d", expectedDelay1, delay1)
		}

		// 第二次重试
		retryableErr.NextRetry()
		delay2 := retryableErr.GetRetryDelay()
		expectedDelay2 := int64(time.Millisecond * 400) // 100 * 2^2
		if delay2 != expectedDelay2 {
			t.Errorf("Expected delay %d, got %d", expectedDelay2, delay2)
		}
	})

	t.Run("MaxRetriesReached", func(t *testing.T) {
		// 第三次重试
		retryableErr.NextRetry()
		if retryableErr.CurrentRetry != 3 {
			t.Errorf("Expected 3 current retry, got %d", retryableErr.CurrentRetry)
		}

		// 已达到最大重试次数
		if retryableErr.CanRetry() {
			t.Error("Should not be able to retry after max retries")
		}
	})
}

// TestErrorUtilityFunctions 测试错误工具函数
func TestErrorUtilityFunctions(t *testing.T) {
	t.Run("HasCriticalErrors", func(t *testing.T) {
		errors := []error{
			NewConnectionError("connection error", nil),
			NewPublishError("test/topic", errors.New("publish error")),
		}

		if HasCriticalErrors(errors) {
			t.Error("Should not have critical errors")
		}

		// 添加严重错误
		errors = append(errors, NewConfigError("critical config error", nil))
		if !HasCriticalErrors(errors) {
			t.Error("Should have critical errors")
		}
	})

	t.Run("CountErrorsBySeverity", func(t *testing.T) {
		errors := []error{
			NewError(TypeConnection, SeverityWarning, "warning"),
			NewError(TypePublish, SeverityError, "error"),
			NewError(TypeConfiguration, SeverityCritical, "critical"),
			errors.New("regular error"), // 应该被计为 SeverityError
		}

		counts := CountErrorsBySeverity(errors)

		if counts[SeverityWarning] != 1 {
			t.Errorf("Expected 1 warning, got %d", counts[SeverityWarning])
		}

		if counts[SeverityError] != 2 { // 一个显式error + 一个regular error
			t.Errorf("Expected 2 errors, got %d", counts[SeverityError])
		}

		if counts[SeverityCritical] != 1 {
			t.Errorf("Expected 1 critical, got %d", counts[SeverityCritical])
		}
	})

	t.Run("FormatErrorSummary", func(t *testing.T) {
		errors := []error{
			NewError(TypeConnection, SeverityWarning, "warning"),
			NewError(TypePublish, SeverityError, "error"),
			NewError(TypeConfiguration, SeverityCritical, "critical"),
		}

		summary := FormatErrorSummary(errors)
		expectedSummary := "Total: 3 errors, Critical: 1, Error: 1, Warning: 1"

		if summary != expectedSummary {
			t.Errorf("Expected summary '%s', got '%s'", expectedSummary, summary)
		}

		// 测试空错误列表
		emptySummary := FormatErrorSummary([]error{})
		if emptySummary != "No errors" {
			t.Errorf("Expected 'No errors', got '%s'", emptySummary)
		}
	})
}
