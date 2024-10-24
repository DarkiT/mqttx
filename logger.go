package mqtt

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// DefaultLogger 默认日志记录器
type DefaultLogger struct {
	logger *slog.Logger
}

// newDefaultLogger 创建一个新的默认日志记录器
func newDefaultLogger() *DefaultLogger {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
		// 添加调用者信息到日志
		AddSource: true,
	}

	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)

	return &DefaultLogger{
		logger: logger,
	}
}

func (l *DefaultLogger) Debug(msg string, args ...interface{}) {
	l.logger.Debug(msg, slogArgs(args)...)
}

func (l *DefaultLogger) Debugf(format string, args ...interface{}) {
	l.logger.Debug(sprintf(format, args...))
}

func (l *DefaultLogger) Info(msg string, args ...interface{}) {
	l.logger.Info(msg, slogArgs(args)...)
}

func (l *DefaultLogger) Infof(format string, args ...interface{}) {
	l.logger.Info(sprintf(format, args...))
}

func (l *DefaultLogger) Warn(msg string, args ...interface{}) {
	l.logger.Warn(msg, slogArgs(args)...)
}

func (l *DefaultLogger) Warnf(format string, args ...interface{}) {
	l.logger.Warn(sprintf(format, args...))
}

func (l *DefaultLogger) Error(msg string, args ...interface{}) {
	l.logger.Error(msg, slogArgs(args)...)
}

func (l *DefaultLogger) Errorf(format string, args ...interface{}) {
	l.logger.Error(sprintf(format, args...))
}

func (l *DefaultLogger) logWithFields(level LogLevel, msg string, fields map[string]interface{}) {
	if fields == nil {
		fields = make(map[string]interface{})
	}
	fields["timestamp"] = time.Now() // 直接使用 time.Time，让 slog 处理格式化
	fields["level"] = level.String()

	// 使用 slog 记录结构化日志
	l.logger.Log(context.Background(), slog.Level(level), msg, formatFields(fields)...)
}

// sprintf 格式化字符串
func sprintf(format string, args ...interface{}) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

// slogArgs 转换可变参数为 slog.Attr 切片
func slogArgs(args []interface{}) []any {
	if len(args) == 0 {
		return nil
	}

	slogArgs := make([]any, len(args))
	for i, arg := range args {
		switch v := arg.(type) {
		case error:
			slogArgs[i] = slog.String("error", v.Error())
		default:
			slogArgs[i] = slog.Any(fmt.Sprintf("arg%d", i), v)
		}
	}
	return slogArgs
}

// formatFields converts a map of fields to a slice of slog.Attr
func formatFields(fields map[string]interface{}) []any {
	attrs := make([]any, 0, len(fields))
	for k, v := range fields {
		// 处理特殊类型
		switch val := v.(type) {
		case error:
			attrs = append(attrs, slog.String(k, val.Error()))
		case time.Time:
			attrs = append(attrs, slog.Time(k, val))
		case time.Duration:
			attrs = append(attrs, slog.Duration(k, val))
		default:
			attrs = append(attrs, slog.Any(k, v))
		}
	}
	return attrs
}
