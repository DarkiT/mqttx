package mqttx

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// defaultLogger 默认日志记录器
type defaultLogger struct {
	logger *slog.Logger
}

// newLogger 创建一个新的默认日志记录器
func newLogger() *defaultLogger {
	return &defaultLogger{
		logger: slog.Default(),
	}
}

func (l *defaultLogger) Debug(msg string, args ...interface{}) {
	l.logger.Debug(msg, slogArgs(args)...)
}

func (l *defaultLogger) Debugf(format string, args ...interface{}) {
	l.logger.Debug(sprintf(format, args...))
}

func (l *defaultLogger) Info(msg string, args ...interface{}) {
	l.logger.Info(msg, slogArgs(args)...)
}

func (l *defaultLogger) Infof(format string, args ...interface{}) {
	l.logger.Info(sprintf(format, args...))
}

func (l *defaultLogger) Warn(msg string, args ...interface{}) {
	l.logger.Warn(msg, slogArgs(args)...)
}

func (l *defaultLogger) Warnf(format string, args ...interface{}) {
	l.logger.Warn(sprintf(format, args...))
}

func (l *defaultLogger) Error(msg string, args ...interface{}) {
	l.logger.Error(msg, slogArgs(args)...)
}

func (l *defaultLogger) Errorf(format string, args ...interface{}) {
	l.logger.Error(sprintf(format, args...))
}

func (l *defaultLogger) logWithFields(level LogLevel, msg string, fields map[string]interface{}) {
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

	// 确保参数是成对的
	if len(args)%2 != 0 {
		return []any{slog.String("error", "odd number of log arguments")}
	}

	slogArgs := make([]any, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			key = fmt.Sprintf("%v", i)
		}
		value := args[i+1]
		switch v := value.(type) {
		case error:
			slogArgs[i/2] = slog.String(key, v.Error())
		default:
			slogArgs[i/2] = slog.Any(key, v)
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
