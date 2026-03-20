package errs

import (
	"errors"
	"fmt"
)

// Severity indicates how an error should be handled by the caller.
type Severity int

const (
	// SeverityUser means the error message is safe to show to the end user.
	SeverityUser Severity = iota
	// SeveritySilent means the error should be logged but not shown to the user.
	SeveritySilent
	// SeverityInternal means something unexpected happened; alert developers.
	SeverityInternal
)

// String returns a human-readable severity label.
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

// ErrorCode is a machine-readable error identifier.
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

// AppError is the standard error type used throughout the application.
type AppError struct {
	Code     ErrorCode
	Severity Severity
	Message  string
	Cause    error
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause so errors.Is / errors.As work.
func (e *AppError) Unwrap() error {
	return e.Cause
}

// Is reports whether target matches this AppError by code.
func (e *AppError) Is(target error) bool {
	var appErr *AppError
	if errors.As(target, &appErr) {
		return e.Code == appErr.Code
	}
	return false
}

// NewUserError creates an error whose message is safe to display to the user.
func NewUserError(code ErrorCode, message string, cause ...error) *AppError {
	return &AppError{
		Code:     code,
		Severity: SeverityUser,
		Message:  message,
		Cause:    firstOrNil(cause),
	}
}

// NewSilentError creates an error that should be logged but not shown to the user.
func NewSilentError(code ErrorCode, message string, cause ...error) *AppError {
	return &AppError{
		Code:     code,
		Severity: SeveritySilent,
		Message:  message,
		Cause:    firstOrNil(cause),
	}
}

// NewInternalError creates an error indicating an unexpected internal failure.
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
