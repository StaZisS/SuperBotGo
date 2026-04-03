package errs

import (
	"errors"
	"fmt"
	"testing"
)

func TestNewUserError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		code         ErrorCode
		message      string
		cause        []error
		wantSeverity Severity
	}{
		{
			name:         "without cause",
			code:         ErrInvalidInput,
			message:      "bad input",
			wantSeverity: SeverityUser,
		},
		{
			name:         "with cause",
			code:         ErrUserNotFound,
			message:      "user missing",
			cause:        []error{errors.New("db error")},
			wantSeverity: SeverityUser,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			appErr := NewUserError(tt.code, tt.message, tt.cause...)
			if appErr.Severity != tt.wantSeverity {
				t.Errorf("Severity = %v, want %v", appErr.Severity, tt.wantSeverity)
			}
			if appErr.Code != tt.code {
				t.Errorf("Code = %v, want %v", appErr.Code, tt.code)
			}
		})
	}
}

func TestNewSilentError(t *testing.T) {
	t.Parallel()

	appErr := NewSilentError(ErrStateExpired, "session expired")
	if appErr.Severity != SeveritySilent {
		t.Errorf("Severity = %v, want %v", appErr.Severity, SeveritySilent)
	}
}

func TestNewInternalError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		message      string
		cause        []error
		wantCode     ErrorCode
		wantSeverity Severity
	}{
		{
			name:         "without cause",
			message:      "something broke",
			wantCode:     ErrInternal,
			wantSeverity: SeverityInternal,
		},
		{
			name:         "with cause",
			message:      "db crash",
			cause:        []error{errors.New("connection refused")},
			wantCode:     ErrInternal,
			wantSeverity: SeverityInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			appErr := NewInternalError(tt.message, tt.cause...)
			if appErr.Code != tt.wantCode {
				t.Errorf("Code = %v, want %v", appErr.Code, tt.wantCode)
			}
			if appErr.Severity != tt.wantSeverity {
				t.Errorf("Severity = %v, want %v", appErr.Severity, tt.wantSeverity)
			}
		})
	}
}

func TestAppError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		appErr  *AppError
		wantMsg string
	}{
		{
			name: "without cause",
			appErr: &AppError{
				Code:    ErrInvalidInput,
				Message: "bad field",
			},
			wantMsg: "[INVALID_INPUT] bad field",
		},
		{
			name: "with cause",
			appErr: &AppError{
				Code:    ErrInternal,
				Message: "oops",
				Cause:   errors.New("disk full"),
			},
			wantMsg: "[INTERNAL_ERROR] oops: disk full",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.appErr.Error()
			if got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

func TestAppError_Unwrap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cause     error
		wantCause error
	}{
		{
			name:      "with cause",
			cause:     errors.New("root cause"),
			wantCause: nil, // compared by message below
		},
		{
			name:      "without cause",
			cause:     nil,
			wantCause: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			appErr := &AppError{Code: ErrInternal, Message: "test", Cause: tt.cause}
			unwrapped := appErr.Unwrap()

			if tt.cause == nil {
				if unwrapped != nil {
					t.Errorf("Unwrap() = %v, want nil", unwrapped)
				}
			} else {
				if unwrapped == nil {
					t.Fatal("Unwrap() = nil, want non-nil")
				}
				if unwrapped.Error() != tt.cause.Error() {
					t.Errorf("Unwrap().Error() = %q, want %q", unwrapped.Error(), tt.cause.Error())
				}
			}
		})
	}
}

func TestAppError_Is(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		err    *AppError
		target *AppError
		want   bool
	}{
		{
			name:   "same code matches regardless of message",
			err:    &AppError{Code: ErrInvalidInput, Message: "foo"},
			target: &AppError{Code: ErrInvalidInput, Message: "bar"},
			want:   true,
		},
		{
			name:   "different code does not match",
			err:    &AppError{Code: ErrInvalidInput, Message: "foo"},
			target: &AppError{Code: ErrInternal, Message: "foo"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.err.Is(tt.target)
			if got != tt.want {
				t.Errorf("Is() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorsIs_WithWrappedAppError(t *testing.T) {
	t.Parallel()

	inner := NewUserError(ErrPermissionDenied, "denied")
	wrapped := fmt.Errorf("outer: %w", inner)

	target := &AppError{Code: ErrPermissionDenied}
	if !errors.Is(wrapped, target) {
		t.Error("errors.Is() = false for wrapped AppError with same code, want true")
	}

	differentTarget := &AppError{Code: ErrInternal}
	if errors.Is(wrapped, differentTarget) {
		t.Error("errors.Is() = true for wrapped AppError with different code, want false")
	}
}

func TestErrorsAs_ExtractsAppError(t *testing.T) {
	t.Parallel()

	inner := NewInternalError("kaboom", errors.New("root"))
	wrapped := fmt.Errorf("layer1: %w", fmt.Errorf("layer2: %w", inner))

	var appErr *AppError
	if !errors.As(wrapped, &appErr) {
		t.Fatal("errors.As() = false, want true")
	}
	if appErr.Code != ErrInternal {
		t.Errorf("extracted Code = %v, want %v", appErr.Code, ErrInternal)
	}
	if appErr.Severity != SeverityInternal {
		t.Errorf("extracted Severity = %v, want %v", appErr.Severity, SeverityInternal)
	}
}

func TestSeverity_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		severity Severity
		want     string
	}{
		{SeverityUser, "USER"},
		{SeveritySilent, "SILENT"},
		{SeverityInternal, "INTERNAL"},
		{Severity(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := tt.severity.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
