package mqttx

import (
	"fmt"
	"log/slog"
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
	l.logger.Debug(msg, args...)
}

func (l *defaultLogger) Debugf(format string, args ...interface{}) {
	l.logger.Debug(sprintf(format, args...))
}

func (l *defaultLogger) Info(msg string, args ...interface{}) {
	l.logger.Info(msg, args...)
}

func (l *defaultLogger) Infof(format string, args ...interface{}) {
	l.logger.Info(sprintf(format, args...))
}

func (l *defaultLogger) Warn(msg string, args ...interface{}) {
	l.logger.Warn(msg, args...)
}

func (l *defaultLogger) Warnf(format string, args ...interface{}) {
	l.logger.Warn(sprintf(format, args...))
}

func (l *defaultLogger) Error(msg string, args ...interface{}) {
	l.logger.Error(msg, args...)
}

func (l *defaultLogger) Errorf(format string, args ...interface{}) {
	l.logger.Error(sprintf(format, args...))
}

// sprintf 格式化字符串
func sprintf(format string, args ...interface{}) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}
