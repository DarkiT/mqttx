package mqttx

import (
	"fmt"
	"log"
	"os"
)

// Logger 是日志接口定义
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// DefaultLogger 是默认的日志实现
type DefaultLogger struct {
	logger *log.Logger
}

// NewDefaultLogger 创建一个默认的日志实现
func NewDefaultLogger() *DefaultLogger {
	return &DefaultLogger{
		logger: log.New(os.Stdout, "", log.LstdFlags),
	}
}

// Debug 记录Debug级别日志
func (l *DefaultLogger) Debug(msg string, args ...interface{}) {
	l.logger.Printf("[DEBUG] %s %v", msg, formatArgs(args...))
}

// Info 记录Info级别日志
func (l *DefaultLogger) Info(msg string, args ...interface{}) {
	l.logger.Printf("[INFO] %s %v", msg, formatArgs(args...))
}

// Warn 记录Warn级别日志
func (l *DefaultLogger) Warn(msg string, args ...interface{}) {
	l.logger.Printf("[WARN] %s %v", msg, formatArgs(args...))
}

// Error 记录Error级别日志
func (l *DefaultLogger) Error(msg string, args ...interface{}) {
	l.logger.Printf("[ERROR] %s %v", msg, formatArgs(args...))
}

// Debugf 记录Debug级别格式化日志
func (l *DefaultLogger) Debugf(format string, args ...interface{}) {
	l.logger.Printf("[DEBUG] "+format, args...)
}

// Infof 记录Info级别格式化日志
func (l *DefaultLogger) Infof(format string, args ...interface{}) {
	l.logger.Printf("[INFO] "+format, args...)
}

// Warnf 记录Warn级别格式化日志
func (l *DefaultLogger) Warnf(format string, args ...interface{}) {
	l.logger.Printf("[WARN] "+format, args...)
}

// Errorf 记录Error级别格式化日志
func (l *DefaultLogger) Errorf(format string, args ...interface{}) {
	l.logger.Printf("[ERROR] "+format, args...)
}

// 格式化参数列表为字符串
func formatArgs(args ...interface{}) string {
	if len(args) == 0 {
		return ""
	}

	formatted := ""
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			formatted += fmt.Sprintf("%v=%v ", args[i], args[i+1])
		} else {
			formatted += fmt.Sprintf("%v ", args[i])
		}
	}
	return formatted
}
