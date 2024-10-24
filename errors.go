package mqtt

import (
	"errors"
	"fmt"
)

var (
	ErrSessionNotFound   = errors.New("session not found")
	ErrSessionExists     = errors.New("session already exists")
	ErrNotConnected      = errors.New("not connected")
	ErrSessionClosed     = errors.New("session closed")
	ErrInvalidOptions    = errors.New("invalid options")
	ErrInvalidBroker     = errors.New("invalid broker address")
	ErrInvalidClientID   = errors.New("invalid client ID")
	ErrInvalidTopic      = errors.New("invalid topic")
	ErrTimeout           = errors.New("operation timeout")
	ErrInvalidPayload    = errors.New("invalid payload")
	ErrSubscribeFailed   = errors.New("subscribe failed")
	ErrUnsubscribeFailed = errors.New("unsubscribe failed")
	ErrPublishFailed     = errors.New("publish failed")
	ErrQoSNotSupported   = errors.New("QoS level not supported")
	ErrStorageFailure    = errors.New("storage operation failed")
)

// SessionError 会话相关错误
type SessionError struct {
	SessionName string
	Err         error
}

func (e *SessionError) Error() string {
	return fmt.Sprintf("session %s: %v", e.SessionName, e.Err)
}

func (e *SessionError) Unwrap() error {
	return e.Err
}

// newSessionError 创建新的会话错误
func newSessionError(name string, err error) *SessionError {
	return &SessionError{
		SessionName: name,
		Err:         err,
	}
}

// wrapError 包装错误信息
func wrapError(err error, msg string) error {
	return fmt.Errorf("%s: %w", msg, err)
}
