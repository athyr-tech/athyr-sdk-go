package athyr

import (
	"errors"
	"fmt"
)

// Sentinel errors for agent operations.
var (
	ErrNotConnected     = errors.New("agent not connected")
	ErrAlreadyConnected = errors.New("agent already connected")
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
