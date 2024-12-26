package mqttx

import (
	"fmt"
	"log/slog"
)

// Logger 日志接口
type Logger interface {
	Debug(msg string, args ...any)
	Debugf(format string, args ...any)
	Info(msg string, args ...any)
	Infof(format string, args ...any)
	Warn(msg string, args ...any)
	Warnf(format string, args ...any)
	Error(msg string, args ...any)
	Errorf(format string, args ...any)
}

// defaultLogger 默认日志记录器
type defaultLogger struct {
	*slog.Logger
}

// newLogger 创建一个新的默认日志记录器
func newLogger() *defaultLogger {
	logger := &defaultLogger{}
	logger.Logger = slog.Default()
	return logger
}

func (l *defaultLogger) Debug(msg string, args ...any) {
	l.Logger.Debug(msg, args...)
}

func (l *defaultLogger) Debugf(format string, args ...any) {
	l.Logger.Debug(sprintf(format, args...))
}

func (l *defaultLogger) Info(msg string, args ...any) {
	l.Logger.Info(msg, args...)
}

func (l *defaultLogger) Infof(format string, args ...any) {
	l.Logger.Info(sprintf(format, args...))
}

func (l *defaultLogger) Warn(msg string, args ...any) {
	l.Logger.Warn(msg, args...)
}

func (l *defaultLogger) Warnf(format string, args ...any) {
	l.Logger.Warn(sprintf(format, args...))
}

func (l *defaultLogger) Error(msg string, args ...any) {
	l.Logger.Error(msg, args...)
}

func (l *defaultLogger) Errorf(format string, args ...any) {
	l.Logger.Error(sprintf(format, args...))
}

// sprintf 格式化字符串
func sprintf(format string, args ...any) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}
