package athyr

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
	"time"
)

// LogRequests returns middleware that logs requests and responses.
// It logs the subject, duration, and any errors.
func LogRequests(logger *log.Logger) Middleware {
	if logger == nil {
		logger = log.Default()
	}
	return func(next RawHandler) RawHandler {
		return func(ctx Context, data []byte) ([]byte, error) {
			start := time.Now()
			subject := ctx.Subject()

			resp, err := next(ctx, data)
			duration := time.Since(start)

			if err != nil {
				logger.Printf("[ERROR] %s (%v): %v", subject, duration, err)
			} else {
				logger.Printf("[INFO] %s (%v): %d bytes", subject, duration, len(resp))
			}

			return resp, err
		}
	}
}

// Recover returns middleware that recovers from panics.
// It converts panics to Internal errors and optionally logs them.
func Recover(logger *log.Logger) Middleware {
	return func(next RawHandler) RawHandler {
		return func(ctx Context, data []byte) (resp []byte, err error) {
			defer func() {
				if r := recover(); r != nil {
					stack := debug.Stack()
					if logger != nil {
						logger.Printf("[PANIC] %s: %v\n%s", ctx.Subject(), r, stack)
					}
					err = Internal("internal error: panic recovered")
				}
			}()
			return next(ctx, data)
		}
	}
}

// Timeout returns middleware that enforces a request timeout.
// If the handler doesn't complete within the duration, it returns Unavailable.
func Timeout(d time.Duration) Middleware {
	return func(next RawHandler) RawHandler {
		return func(ctx Context, data []byte) ([]byte, error) {
			// Create a new context with timeout
			timeoutCtx, cancel := context.WithTimeout(ctx, d)
			defer cancel()

			// Create a wrapped context that carries the timeout
			wrappedCtx := &timeoutContext{
				Context:      timeoutCtx,
				serviceCtx:   ctx,
			}

			// Run the handler in a goroutine
			type result struct {
				resp []byte
				err  error
			}
			done := make(chan result, 1)

			go func() {
				resp, err := next(wrappedCtx, data)
				done <- result{resp, err}
			}()

			select {
			case <-timeoutCtx.Done():
				if timeoutCtx.Err() == context.DeadlineExceeded {
					return nil, Unavailable("request timeout after %v", d)
				}
				return nil, timeoutCtx.Err()
			case r := <-done:
				return r.resp, r.err
			}
		}
	}
}

// timeoutContext wraps a timeout context while preserving Context methods.
type timeoutContext struct {
	context.Context
	serviceCtx Context
}

func (c *timeoutContext) Agent() Agent         { return c.serviceCtx.Agent() }
func (c *timeoutContext) Subject() string      { return c.serviceCtx.Subject() }
func (c *timeoutContext) ReplySubject() string { return c.serviceCtx.ReplySubject() }

// RateLimit returns middleware that limits concurrent requests.
// Requests beyond the limit receive Unavailable error immediately.
func RateLimit(maxConcurrent int) Middleware {
	sem := make(chan struct{}, maxConcurrent)
	return func(next RawHandler) RawHandler {
		return func(ctx Context, data []byte) ([]byte, error) {
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				return next(ctx, data)
			default:
				return nil, Unavailable("rate limit exceeded: max %d concurrent requests", maxConcurrent)
			}
		}
	}
}

// MetricsCallback receives timing and error info for each request.
// Used with the Metrics middleware to collect observability data.
type MetricsCallback func(subject string, duration time.Duration, err error)

// Metrics returns middleware that tracks request metrics.
// The callback is invoked after each request with the subject, duration, and any error.
func Metrics(callback MetricsCallback) Middleware {
	return func(next RawHandler) RawHandler {
		return func(ctx Context, data []byte) ([]byte, error) {
			start := time.Now()
			resp, err := next(ctx, data)
			callback(ctx.Subject(), time.Since(start), err)
			return resp, err
		}
	}
}

// Chain combines multiple middleware into one.
// Middleware are applied in order: first middleware is outermost.
func Chain(mw ...Middleware) Middleware {
	return func(next RawHandler) RawHandler {
		for i := len(mw) - 1; i >= 0; i-- {
			next = mw[i](next)
		}
		return next
	}
}

// Validate returns middleware that validates requests before processing.
// If the validator returns an error, the request is rejected with BadRequest.
func Validate(validator func(data []byte) error) Middleware {
	return func(next RawHandler) RawHandler {
		return func(ctx Context, data []byte) ([]byte, error) {
			if err := validator(data); err != nil {
				return nil, BadRequest("validation failed: %v", err)
			}
			return next(ctx, data)
		}
	}
}

// Retry returns middleware that retries failed requests.
// Only retries if the error is retryable (Unavailable or temporary).
func Retry(maxAttempts int, backoff time.Duration) Middleware {
	return func(next RawHandler) RawHandler {
		return func(ctx Context, data []byte) ([]byte, error) {
			var lastErr error
			for attempt := 0; attempt < maxAttempts; attempt++ {
				resp, err := next(ctx, data)
				if err == nil {
					return resp, nil
				}

				// Check if error is retryable
				if !isRetryable(err) {
					return nil, err
				}

				lastErr = err

				// Wait before retry (with exponential backoff)
				wait := backoff * time.Duration(1<<attempt)
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(wait):
					// Continue to next attempt
				}
			}
			return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
		}
	}
}

// isRetryable checks if an error should be retried.
func isRetryable(err error) bool {
	if se, ok := err.(*ServiceError); ok {
		return se.Code == "unavailable"
	}
	return false
}
