package athyr

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAthyrError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *AthyrError
		expected string
	}{
		{
			name: "with op",
			err: &AthyrError{
				Code:    ErrCodeNotFound,
				Message: "key not found",
				Op:      "KV.Get",
			},
			expected: "KV.Get: key not found",
		},
		{
			name: "without op",
			err: &AthyrError{
				Code:    ErrCodeNotFound,
				Message: "key not found",
			},
			expected: "key not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestAthyrError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := &AthyrError{
		Code:    ErrCodeInternal,
		Message: "something failed",
		Cause:   cause,
	}

	if err.Unwrap() != cause {
		t.Error("Unwrap() should return the cause")
	}
}

func TestAthyrError_Is(t *testing.T) {
	err := &AthyrError{
		Code:    ErrCodeNotFound,
		Message: "key not found",
		Op:      "KV.Get",
	}

	// Should match sentinel with same code
	if !errors.Is(err, ErrNotFound) {
		t.Error("errors.Is should match ErrNotFound")
	}

	// Should not match sentinel with different code
	if errors.Is(err, ErrUnavailable) {
		t.Error("errors.Is should not match ErrUnavailable")
	}
}

func TestAthyrError_As(t *testing.T) {
	cause := status.Error(codes.NotFound, "not found")
	wrapped := wrapError("KV.Get", cause)

	var ae *AthyrError
	if !errors.As(wrapped, &ae) {
		t.Fatal("errors.As should extract AthyrError")
	}

	if ae.Code != ErrCodeNotFound {
		t.Errorf("Code = %q, want %q", ae.Code, ErrCodeNotFound)
	}
	if ae.Op != "KV.Get" {
		t.Errorf("Op = %q, want %q", ae.Op, "KV.Get")
	}
	if ae.Cause != cause {
		t.Error("Cause should be preserved")
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "AthyrError with NotFound code",
			err:      &AthyrError{Code: ErrCodeNotFound, Message: "not found"},
			expected: true,
		},
		{
			name:     "AthyrError with different code",
			err:      &AthyrError{Code: ErrCodeInternal, Message: "internal"},
			expected: false,
		},
		{
			name:     "wrapped gRPC NotFound",
			err:      wrapError("Test", status.Error(codes.NotFound, "not found")),
			expected: true,
		},
		{
			name:     "plain error",
			err:      errors.New("not found"),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFound(tt.err); got != tt.expected {
				t.Errorf("IsNotFound() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsUnavailable(t *testing.T) {
	err := wrapError("Test", status.Error(codes.Unavailable, "service unavailable"))
	if !IsUnavailable(err) {
		t.Error("IsUnavailable should return true for Unavailable gRPC error")
	}
}

func TestIsBadRequest(t *testing.T) {
	err := wrapError("Test", status.Error(codes.InvalidArgument, "invalid"))
	if !IsBadRequest(err) {
		t.Error("IsBadRequest should return true for InvalidArgument gRPC error")
	}
}

func TestIsInternal(t *testing.T) {
	err := wrapError("Test", status.Error(codes.Internal, "internal"))
	if !IsInternal(err) {
		t.Error("IsInternal should return true for Internal gRPC error")
	}
}

func TestIsDeadlineExceeded(t *testing.T) {
	err := wrapError("Test", status.Error(codes.DeadlineExceeded, "timeout"))
	if !IsDeadlineExceeded(err) {
		t.Error("IsDeadlineExceeded should return true for DeadlineExceeded gRPC error")
	}
}

func TestWrapError_NilError(t *testing.T) {
	if wrapError("Test", nil) != nil {
		t.Error("wrapError should return nil for nil error")
	}
}

func TestWrapError_NonGRPCError(t *testing.T) {
	plainErr := errors.New("plain error")
	wrapped := wrapError("Test", plainErr)

	var ae *AthyrError
	if !errors.As(wrapped, &ae) {
		t.Fatal("should wrap as AthyrError")
	}

	if ae.Code != ErrCodeUnknown {
		t.Errorf("Code = %q, want %q for non-gRPC error", ae.Code, ErrCodeUnknown)
	}
	if ae.Cause != plainErr {
		t.Error("Cause should be preserved")
	}
}

func TestErrNotConnected_IsConsistent(t *testing.T) {
	// ErrNotConnected is now an *AthyrError, so errors.Is should work
	// with both the sentinel and new instances with the same code.
	err := &AthyrError{Code: ErrCodeNotConnected, Message: "custom message"}
	if !errors.Is(err, ErrNotConnected) {
		t.Error("errors.Is should match ErrNotConnected by code")
	}
}

func TestErrAlreadyConnected_IsConsistent(t *testing.T) {
	err := &AthyrError{Code: ErrCodeAlreadyConnected, Message: "custom message"}
	if !errors.Is(err, ErrAlreadyConnected) {
		t.Error("errors.Is should match ErrAlreadyConnected by code")
	}
}

func TestAllSentinels_ErrorsIs(t *testing.T) {
	sentinels := map[string]*AthyrError{
		"ErrNotFound":         ErrNotFound,
		"ErrUnavailable":      ErrUnavailable,
		"ErrInvalidArgument":  ErrInvalidArgument,
		"ErrInternal":         ErrInternal,
		"ErrUnauthenticated":  ErrUnauthenticated,
		"ErrPermissionDenied": ErrPermissionDenied,
		"ErrAlreadyExists":    ErrAlreadyExists,
		"ErrDeadlineExceeded": ErrDeadlineExceeded,
		"ErrNotConnected":     ErrNotConnected,
		"ErrAlreadyConnected": ErrAlreadyConnected,
	}

	for name, sentinel := range sentinels {
		t.Run(name, func(t *testing.T) {
			// Create a new error with the same code
			err := &AthyrError{
				Code:    sentinel.Code,
				Message: "test error",
				Op:      "TestOp",
			}
			if !errors.Is(err, sentinel) {
				t.Errorf("errors.Is(%s) should match sentinel", name)
			}
		})
	}
}

func TestCodeFromGRPC(t *testing.T) {
	tests := []struct {
		grpcCode codes.Code
		expected ErrorCode
	}{
		{codes.NotFound, ErrCodeNotFound},
		{codes.Unavailable, ErrCodeUnavailable},
		{codes.InvalidArgument, ErrCodeInvalidArgument},
		{codes.Internal, ErrCodeInternal},
		{codes.Unauthenticated, ErrCodeUnauthenticated},
		{codes.PermissionDenied, ErrCodePermissionDenied},
		{codes.AlreadyExists, ErrCodeAlreadyExists},
		{codes.DeadlineExceeded, ErrCodeDeadlineExceeded},
		{codes.Canceled, ErrCodeUnknown}, // unmapped code
	}

	for _, tt := range tests {
		t.Run(tt.grpcCode.String(), func(t *testing.T) {
			if got := codeFromGRPC(tt.grpcCode); got != tt.expected {
				t.Errorf("codeFromGRPC(%v) = %q, want %q", tt.grpcCode, got, tt.expected)
			}
		})
	}
}
