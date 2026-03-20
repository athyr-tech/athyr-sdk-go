package athyr

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// StreamError represents a streaming failure with context about what was
// already streamed. This allows agents to make informed retry decisions.
type StreamError struct {
	Err                error  // Original error
	Backend            string // Backend that failed
	AccumulatedContent string // Content streamed before failure
	PartialResponse    bool   // True if some content was streamed
}

func (e *StreamError) Error() string {
	if e.PartialResponse {
		return fmt.Sprintf("stream failed after partial response (%d chars) from %s: %v",
			len(e.AccumulatedContent), e.Backend, e.Err)
	}
	return fmt.Sprintf("stream failed on %s: %v", e.Backend, e.Err)
}

func (e *StreamError) Unwrap() error {
	return e.Err
}

// ErrorCode represents a category of error from SDK operations.
type ErrorCode string

const (
	ErrCodeNotFound         ErrorCode = "not_found"
	ErrCodeUnavailable      ErrorCode = "unavailable"
	ErrCodeInvalidArgument  ErrorCode = "invalid_argument"
	ErrCodeInternal         ErrorCode = "internal"
	ErrCodeUnauthenticated  ErrorCode = "unauthenticated"
	ErrCodePermissionDenied ErrorCode = "permission_denied"
	ErrCodeAlreadyExists    ErrorCode = "already_exists"
	ErrCodeDeadlineExceeded ErrorCode = "deadline_exceeded"
	ErrCodeNotConnected     ErrorCode = "not_connected"
	ErrCodeAlreadyConnected ErrorCode = "already_connected"
	ErrCodeUnknown          ErrorCode = "unknown"
)

// AthyrError represents an error from SDK operations.
// It wraps underlying errors (typically gRPC errors) with additional context.
type AthyrError struct {
	Code    ErrorCode // Error category
	Message string    // Human-readable message
	Op      string    // Operation that failed (e.g., "Complete", "KV.Get")
	Cause   error     // Underlying error (e.g., gRPC error)
}

func (e *AthyrError) Error() string {
	if e.Op != "" {
		return fmt.Sprintf("%s: %s", e.Op, e.Message)
	}
	return e.Message
}

// Unwrap returns the underlying error for errors.Unwrap support.
func (e *AthyrError) Unwrap() error {
	return e.Cause
}

// Is supports errors.Is by matching on error code.
func (e *AthyrError) Is(target error) bool {
	if t, ok := target.(*AthyrError); ok {
		return e.Code == t.Code
	}
	return false
}

// Sentinel errors for use with errors.Is.
var (
	ErrNotFound         = &AthyrError{Code: ErrCodeNotFound}
	ErrUnavailable      = &AthyrError{Code: ErrCodeUnavailable}
	ErrInvalidArgument  = &AthyrError{Code: ErrCodeInvalidArgument}
	ErrInternal         = &AthyrError{Code: ErrCodeInternal}
	ErrUnauthenticated  = &AthyrError{Code: ErrCodeUnauthenticated}
	ErrPermissionDenied = &AthyrError{Code: ErrCodePermissionDenied}
	ErrAlreadyExists    = &AthyrError{Code: ErrCodeAlreadyExists}
	ErrDeadlineExceeded = &AthyrError{Code: ErrCodeDeadlineExceeded}
	ErrNotConnected     = &AthyrError{Code: ErrCodeNotConnected, Message: "agent not connected"}
	ErrAlreadyConnected = &AthyrError{Code: ErrCodeAlreadyConnected, Message: "agent already connected"}
)

// IsNotFound reports whether the error indicates a not found condition.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsUnavailable reports whether the error indicates the service is unavailable.
func IsUnavailable(err error) bool {
	return errors.Is(err, ErrUnavailable)
}

// IsBadRequest reports whether the error indicates an invalid argument.
func IsBadRequest(err error) bool {
	return errors.Is(err, ErrInvalidArgument)
}

// IsInternal reports whether the error indicates an internal server error.
func IsInternal(err error) bool {
	return errors.Is(err, ErrInternal)
}

// IsUnauthenticated reports whether the error indicates missing authentication.
func IsUnauthenticated(err error) bool {
	return errors.Is(err, ErrUnauthenticated)
}

// IsPermissionDenied reports whether the error indicates insufficient permissions.
func IsPermissionDenied(err error) bool {
	return errors.Is(err, ErrPermissionDenied)
}

// IsAlreadyExists reports whether the error indicates a resource already exists.
func IsAlreadyExists(err error) bool {
	return errors.Is(err, ErrAlreadyExists)
}

// IsDeadlineExceeded reports whether the error indicates a timeout.
func IsDeadlineExceeded(err error) bool {
	return errors.Is(err, ErrDeadlineExceeded)
}

// IsNotConnected reports whether the error indicates the agent is not connected.
func IsNotConnected(err error) bool {
	return errors.Is(err, ErrNotConnected)
}

// IsAlreadyConnected reports whether the error indicates the agent is already connected.
func IsAlreadyConnected(err error) bool {
	return errors.Is(err, ErrAlreadyConnected)
}

// wrapError wraps an error with operation context.
// If the error is a gRPC error, it extracts the code and message.
// Returns nil if err is nil.
func wrapError(op string, err error) error {
	if err == nil {
		return nil
	}

	// Extract gRPC status if available
	st, ok := status.FromError(err)
	if !ok {
		// Not a gRPC error, wrap as unknown
		return &AthyrError{
			Code:    ErrCodeUnknown,
			Message: err.Error(),
			Op:      op,
			Cause:   err,
		}
	}

	return &AthyrError{
		Code:    codeFromGRPC(st.Code()),
		Message: st.Message(),
		Op:      op,
		Cause:   err,
	}
}

// codeFromGRPC maps gRPC status codes to ErrorCode.
func codeFromGRPC(code codes.Code) ErrorCode {
	switch code {
	case codes.NotFound:
		return ErrCodeNotFound
	case codes.Unavailable:
		return ErrCodeUnavailable
	case codes.InvalidArgument:
		return ErrCodeInvalidArgument
	case codes.Internal:
		return ErrCodeInternal
	case codes.Unauthenticated:
		return ErrCodeUnauthenticated
	case codes.PermissionDenied:
		return ErrCodePermissionDenied
	case codes.AlreadyExists:
		return ErrCodeAlreadyExists
	case codes.DeadlineExceeded:
		return ErrCodeDeadlineExceeded
	default:
		return ErrCodeUnknown
	}
}
