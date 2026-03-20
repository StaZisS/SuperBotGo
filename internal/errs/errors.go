package errs

import (
	"errors"
	"fmt"
)

type Severity int

const (
	SeverityUser Severity = iota
	SeveritySilent
	SeverityInternal
)

func (s Severity) String() string {
	switch s {
	case SeverityUser:
		return "USER"
	case SeveritySilent:
		return "SILENT"
	case SeverityInternal:
		return "INTERNAL"
	default:
		return "UNKNOWN"
	}
}

type ErrorCode string

const (
	ErrUserNotFound     ErrorCode = "USER_NOT_FOUND"
	ErrChatNotFound     ErrorCode = "CHAT_NOT_FOUND"
	ErrProjectNotFound  ErrorCode = "PROJECT_NOT_FOUND"
	ErrPermissionDenied ErrorCode = "PERMISSION_DENIED"
	ErrInvalidInput     ErrorCode = "INVALID_INPUT"
	ErrStateExpired     ErrorCode = "STATE_EXPIRED"
	ErrCommandNotFound  ErrorCode = "COMMAND_NOT_FOUND"
	ErrInternal         ErrorCode = "INTERNAL_ERROR"
	ErrChannelError     ErrorCode = "CHANNEL_ERROR"
)

type AppError struct {
	Code     ErrorCode
	Severity Severity
	Message  string
	Cause    error
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

func (e *AppError) Is(target error) bool {
	var appErr *AppError
	if errors.As(target, &appErr) {
		return e.Code == appErr.Code
	}
	return false
}

func NewUserError(code ErrorCode, message string, cause ...error) *AppError {
	return &AppError{
		Code:     code,
		Severity: SeverityUser,
		Message:  message,
		Cause:    firstOrNil(cause),
	}
}

func NewSilentError(code ErrorCode, message string, cause ...error) *AppError {
	return &AppError{
		Code:     code,
		Severity: SeveritySilent,
		Message:  message,
		Cause:    firstOrNil(cause),
	}
}

func NewInternalError(message string, cause ...error) *AppError {
	return &AppError{
		Code:     ErrInternal,
		Severity: SeverityInternal,
		Message:  message,
		Cause:    firstOrNil(cause),
	}
}

func firstOrNil(errs []error) error {
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
