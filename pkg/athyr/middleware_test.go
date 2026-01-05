package athyr

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	handler := func(ctx Context, data []byte) ([]byte, error) {
		return []byte("response"), nil
	}

	wrapped := Logger(logger)(handler)
	ctx := newMockContext()
	ctx.subject = "test.log"

	resp, err := wrapped(ctx, []byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp) != "response" {
		t.Errorf("expected 'response', got '%s'", resp)
	}

	logged := buf.String()
	if !strings.Contains(logged, "[INFO]") {
		t.Errorf("expected [INFO] in log, got: %s", logged)
	}
	if !strings.Contains(logged, "test.log") {
		t.Errorf("expected subject in log, got: %s", logged)
	}
	if !strings.Contains(logged, "8 bytes") {
		t.Errorf("expected response size in log, got: %s", logged)
	}
}

func TestLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	handler := func(ctx Context, data []byte) ([]byte, error) {
		return nil, BadRequest("invalid")
	}

	wrapped := Logger(logger)(handler)
	ctx := newMockContext()

	_, err := wrapped(ctx, nil)
	if err == nil {
		t.Fatal("expected error")
	}

	logged := buf.String()
	if !strings.Contains(logged, "[ERROR]") {
		t.Errorf("expected [ERROR] in log, got: %s", logged)
	}
}

func TestLogger_NilLogger(t *testing.T) {
	// Should not panic with nil logger
	handler := func(ctx Context, data []byte) ([]byte, error) {
		return []byte("ok"), nil
	}

	wrapped := Logger(nil)(handler)
	ctx := newMockContext()

	_, err := wrapped(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecover(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	handler := func(ctx Context, data []byte) ([]byte, error) {
		panic("something bad happened")
	}

	wrapped := Recover(logger)(handler)
	ctx := newMockContext()

	resp, err := wrapped(ctx, nil)
	if err == nil {
		t.Fatal("expected error from panic")
	}

	var se *ServiceError
	if !errors.As(err, &se) {
		t.Fatalf("expected ServiceError, got %T", err)
	}
	if se.Code != "internal" {
		t.Errorf("expected 'internal' code, got '%s'", se.Code)
	}

	if resp != nil {
		t.Errorf("expected nil response, got %v", resp)
	}

	logged := buf.String()
	if !strings.Contains(logged, "[PANIC]") {
		t.Errorf("expected [PANIC] in log, got: %s", logged)
	}
	if !strings.Contains(logged, "something bad happened") {
		t.Errorf("expected panic message in log, got: %s", logged)
	}
}

func TestRecover_NoPanic(t *testing.T) {
	handler := func(ctx Context, data []byte) ([]byte, error) {
		return []byte("success"), nil
	}

	wrapped := Recover(nil)(handler)
	ctx := newMockContext()

	resp, err := wrapped(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp) != "success" {
		t.Errorf("expected 'success', got '%s'", resp)
	}
}

func TestTimeout(t *testing.T) {
	handler := func(ctx Context, data []byte) ([]byte, error) {
		return []byte("fast"), nil
	}

	wrapped := Timeout(100 * time.Millisecond)(handler)
	ctx := newMockContext()

	resp, err := wrapped(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp) != "fast" {
		t.Errorf("expected 'fast', got '%s'", resp)
	}
}

func TestTimeout_Exceeded(t *testing.T) {
	handler := func(ctx Context, data []byte) ([]byte, error) {
		time.Sleep(200 * time.Millisecond)
		return []byte("slow"), nil
	}

	wrapped := Timeout(50 * time.Millisecond)(handler)
	ctx := newMockContext()

	_, err := wrapped(ctx, nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}

	var se *ServiceError
	if !errors.As(err, &se) {
		t.Fatalf("expected ServiceError, got %T", err)
	}
	if se.Code != "unavailable" {
		t.Errorf("expected 'unavailable' code, got '%s'", se.Code)
	}
}

func TestTimeout_PreservesContext(t *testing.T) {
	handler := func(ctx Context, data []byte) ([]byte, error) {
		// Verify context methods work
		if ctx.Subject() != "test.timeout" {
			t.Errorf("expected subject 'test.timeout', got '%s'", ctx.Subject())
		}
		return []byte("ok"), nil
	}

	wrapped := Timeout(100 * time.Millisecond)(handler)
	ctx := newMockContext()
	ctx.subject = "test.timeout"

	_, err := wrapped(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRateLimit(t *testing.T) {
	var concurrent int32

	handler := func(ctx Context, data []byte) ([]byte, error) {
		c := atomic.AddInt32(&concurrent, 1)
		defer atomic.AddInt32(&concurrent, -1)

		if c > 2 {
			t.Errorf("concurrent requests exceeded limit: %d", c)
		}

		time.Sleep(50 * time.Millisecond)
		return []byte("ok"), nil
	}

	wrapped := RateLimit(2)(handler)
	ctx := newMockContext()

	// Start 5 concurrent requests
	results := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			_, err := wrapped(ctx, nil)
			results <- err
		}()
	}

	// Collect results
	var rateLimited int
	for i := 0; i < 5; i++ {
		err := <-results
		if err != nil {
			var se *ServiceError
			if errors.As(err, &se) && se.Code == "unavailable" {
				rateLimited++
			}
		}
	}

	// At least some should be rate limited
	if rateLimited == 0 {
		t.Error("expected some requests to be rate limited")
	}
}

func TestMetrics(t *testing.T) {
	var metricsSubject string
	var metricsDuration time.Duration
	var metricsErr error

	callback := func(subject string, duration time.Duration, err error) {
		metricsSubject = subject
		metricsDuration = duration
		metricsErr = err
	}

	handler := func(ctx Context, data []byte) ([]byte, error) {
		time.Sleep(10 * time.Millisecond)
		return []byte("ok"), nil
	}

	wrapped := Metrics(callback)(handler)
	ctx := newMockContext()
	ctx.subject = "metrics.test"

	_, _ = wrapped(ctx, nil)

	if metricsSubject != "metrics.test" {
		t.Errorf("expected subject 'metrics.test', got '%s'", metricsSubject)
	}
	if metricsDuration < 10*time.Millisecond {
		t.Errorf("expected duration >= 10ms, got %v", metricsDuration)
	}
	if metricsErr != nil {
		t.Errorf("expected nil error, got %v", metricsErr)
	}
}

func TestChain(t *testing.T) {
	calls := []string{}

	mw1 := func(next RawHandler) RawHandler {
		return func(ctx Context, data []byte) ([]byte, error) {
			calls = append(calls, "mw1-before")
			resp, err := next(ctx, data)
			calls = append(calls, "mw1-after")
			return resp, err
		}
	}

	mw2 := func(next RawHandler) RawHandler {
		return func(ctx Context, data []byte) ([]byte, error) {
			calls = append(calls, "mw2-before")
			resp, err := next(ctx, data)
			calls = append(calls, "mw2-after")
			return resp, err
		}
	}

	handler := func(ctx Context, data []byte) ([]byte, error) {
		calls = append(calls, "handler")
		return []byte("done"), nil
	}

	chained := Chain(mw1, mw2)(handler)
	ctx := newMockContext()

	_, _ = chained(ctx, nil)

	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if len(calls) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(calls), calls)
	}
	for i, exp := range expected {
		if calls[i] != exp {
			t.Errorf("call %d: expected '%s', got '%s'", i, exp, calls[i])
		}
	}
}

func TestValidate(t *testing.T) {
	validator := func(data []byte) error {
		if len(data) < 5 {
			return errors.New("too short")
		}
		return nil
	}

	handler := func(ctx Context, data []byte) ([]byte, error) {
		return data, nil
	}

	wrapped := Validate(validator)(handler)
	ctx := newMockContext()

	// Valid input
	resp, err := wrapped(ctx, []byte("valid input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp) != "valid input" {
		t.Errorf("expected 'valid input', got '%s'", resp)
	}

	// Invalid input
	_, err = wrapped(ctx, []byte("bad"))
	if err == nil {
		t.Fatal("expected validation error")
	}

	var se *ServiceError
	if !errors.As(err, &se) {
		t.Fatalf("expected ServiceError, got %T", err)
	}
	if se.Code != "bad_request" {
		t.Errorf("expected 'bad_request', got '%s'", se.Code)
	}
}

func TestRetry(t *testing.T) {
	attempts := 0

	handler := func(ctx Context, data []byte) ([]byte, error) {
		attempts++
		if attempts < 3 {
			return nil, Unavailable("temporary failure")
		}
		return []byte("success"), nil
	}

	wrapped := Retry(5, 10*time.Millisecond)(handler)
	ctx := newMockContext()

	resp, err := wrapped(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp) != "success" {
		t.Errorf("expected 'success', got '%s'", resp)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetry_MaxExceeded(t *testing.T) {
	attempts := 0

	handler := func(ctx Context, data []byte) ([]byte, error) {
		attempts++
		return nil, Unavailable("always fails")
	}

	wrapped := Retry(3, 5*time.Millisecond)(handler)
	ctx := newMockContext()

	_, err := wrapped(ctx, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
	if !strings.Contains(err.Error(), "max retries exceeded") {
		t.Errorf("expected 'max retries exceeded' in error, got: %v", err)
	}
}

func TestRetry_NonRetryableError(t *testing.T) {
	attempts := 0

	handler := func(ctx Context, data []byte) ([]byte, error) {
		attempts++
		return nil, BadRequest("not retryable")
	}

	wrapped := Retry(3, 5*time.Millisecond)(handler)
	ctx := newMockContext()

	_, err := wrapped(ctx, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	// Should not retry non-retryable errors
	if attempts != 1 {
		t.Errorf("expected 1 attempt for non-retryable error, got %d", attempts)
	}
}

func TestRetry_ContextCancellation(t *testing.T) {
	attempts := 0

	handler := func(ctx Context, data []byte) ([]byte, error) {
		attempts++
		return nil, Unavailable("temporary")
	}

	wrapped := Retry(10, 50*time.Millisecond)(handler)

	baseCtx, cancel := context.WithCancel(context.Background())
	ctx := &mockContext{Context: baseCtx, subject: "test"}

	// Cancel after a short delay
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	_, err := wrapped(ctx, nil)
	if err == nil {
		t.Fatal("expected error")
	}

	// Should have stopped retrying due to context cancellation
	if attempts > 3 {
		t.Errorf("expected fewer attempts due to cancellation, got %d", attempts)
	}
}
