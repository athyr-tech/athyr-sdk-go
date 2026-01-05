package athyr

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// mockContext implements Context for testing.
type mockContext struct {
	context.Context
	agent        Agent
	subject      string
	replySubject string
}

func (c *mockContext) Agent() Agent         { return c.agent }
func (c *mockContext) Subject() string      { return c.subject }
func (c *mockContext) ReplySubject() string { return c.replySubject }

func newMockContext() *mockContext {
	return &mockContext{
		Context: context.Background(),
		subject: "test.subject",
	}
}

// Test types for typed handlers
type EchoRequest struct {
	Message string `json:"message"`
}

type EchoResponse struct {
	Echo string `json:"echo"`
}

func TestHandler_TypedHandler(t *testing.T) {
	handler := func(ctx Context, req EchoRequest) (EchoResponse, error) {
		return EchoResponse{Echo: req.Message}, nil
	}

	svc := NewService("echo", handler)
	if svc.subject != "echo" {
		t.Errorf("expected subject 'echo', got '%s'", svc.subject)
	}
	if svc.name != "echo" {
		t.Errorf("expected name 'echo', got '%s'", svc.name)
	}
}

func TestHandler_TypedHandler_Marshal(t *testing.T) {
	handler := func(ctx Context, req EchoRequest) (EchoResponse, error) {
		return EchoResponse{Echo: "hello: " + req.Message}, nil
	}

	rawHandler := wrapTypedHandler(handler)
	ctx := newMockContext()

	input := `{"message":"world"}`
	output, err := rawHandler(ctx, []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp EchoResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Echo != "hello: world" {
		t.Errorf("expected 'hello: world', got '%s'", resp.Echo)
	}
}

func TestHandler_TypedHandler_InvalidJSON(t *testing.T) {
	handler := func(ctx Context, req EchoRequest) (EchoResponse, error) {
		return EchoResponse{}, nil
	}

	rawHandler := wrapTypedHandler(handler)
	ctx := newMockContext()

	_, err := rawHandler(ctx, []byte("not valid json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}

	var se *ServiceError
	if !errors.As(err, &se) {
		t.Fatalf("expected ServiceError, got %T", err)
	}
	if se.Code != "bad_request" {
		t.Errorf("expected code 'bad_request', got '%s'", se.Code)
	}
}

func TestHandler_TypedHandler_EmptyInput(t *testing.T) {
	called := false
	handler := func(ctx Context, req EchoRequest) (EchoResponse, error) {
		called = true
		// Empty input should result in zero-value request
		if req.Message != "" {
			t.Errorf("expected empty message, got '%s'", req.Message)
		}
		return EchoResponse{Echo: "empty"}, nil
	}

	rawHandler := wrapTypedHandler(handler)
	ctx := newMockContext()

	_, err := rawHandler(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("handler was not called")
	}
}

func TestRawHandler(t *testing.T) {
	handler := func(ctx Context, data []byte) ([]byte, error) {
		return append([]byte("raw:"), data...), nil
	}

	svc := NewRawService("raw.echo", handler)
	if svc.subject != "raw.echo" {
		t.Errorf("expected subject 'raw.echo', got '%s'", svc.subject)
	}
}

func TestServiceOption_WithQueueGroup(t *testing.T) {
	handler := func(ctx Context, req EchoRequest) (EchoResponse, error) {
		return EchoResponse{}, nil
	}

	svc := NewService("test", handler, WithQueueGroup("workers"))
	if svc.queueGroup != "workers" {
		t.Errorf("expected queue group 'workers', got '%s'", svc.queueGroup)
	}
}

func TestServiceOption_WithName(t *testing.T) {
	handler := func(ctx Context, req EchoRequest) (EchoResponse, error) {
		return EchoResponse{}, nil
	}

	svc := NewService("test.subject", handler, WithName("My Service"))
	if svc.name != "My Service" {
		t.Errorf("expected name 'My Service', got '%s'", svc.name)
	}
}

func TestServiceOption_WithServiceMiddleware(t *testing.T) {
	calls := []string{}
	mw := func(next RawHandler) RawHandler {
		return func(ctx Context, data []byte) ([]byte, error) {
			calls = append(calls, "mw")
			return next(ctx, data)
		}
	}

	handler := func(ctx Context, req EchoRequest) (EchoResponse, error) {
		calls = append(calls, "handler")
		return EchoResponse{}, nil
	}

	svc := NewService("test", handler, WithServiceMiddleware(mw))

	// Build and call the handler
	builtHandler := svc.BuildHandler(nil)
	ctx := newMockContext()
	_, _ = builtHandler(ctx, []byte("{}"))

	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	if calls[0] != "mw" {
		t.Errorf("expected middleware called first, got %s", calls[0])
	}
	if calls[1] != "handler" {
		t.Errorf("expected handler called second, got %s", calls[1])
	}
}

func TestService_BuildHandler_MiddlewareOrder(t *testing.T) {
	calls := []string{}

	globalMW := func(next RawHandler) RawHandler {
		return func(ctx Context, data []byte) ([]byte, error) {
			calls = append(calls, "global")
			return next(ctx, data)
		}
	}

	serviceMW := func(next RawHandler) RawHandler {
		return func(ctx Context, data []byte) ([]byte, error) {
			calls = append(calls, "service")
			return next(ctx, data)
		}
	}

	handler := func(ctx Context, req EchoRequest) (EchoResponse, error) {
		calls = append(calls, "handler")
		return EchoResponse{}, nil
	}

	svc := NewService("test", handler, WithServiceMiddleware(serviceMW))
	builtHandler := svc.BuildHandler([]Middleware{globalMW})

	ctx := newMockContext()
	_, _ = builtHandler(ctx, []byte("{}"))

	// Global middleware should wrap service middleware: global -> service -> handler
	expected := []string{"global", "service", "handler"}
	if len(calls) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(calls), calls)
	}
	for i, exp := range expected {
		if calls[i] != exp {
			t.Errorf("call %d: expected '%s', got '%s'", i, exp, calls[i])
		}
	}
}

func TestServiceError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		code     string
		contains string
	}{
		{"BadRequest", BadRequest("invalid: %s", "test"), "bad_request", "invalid: test"},
		{"Internal", Internal("server error"), "internal", "server error"},
		{"Unavailable", Unavailable("retry later"), "unavailable", "retry later"},
		{"NotFound", NotFound("item %d", 42), "not_found", "item 42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var se *ServiceError
			if !errors.As(tt.err, &se) {
				t.Fatalf("expected ServiceError, got %T", tt.err)
			}
			if se.Code != tt.code {
				t.Errorf("expected code '%s', got '%s'", tt.code, se.Code)
			}
			if se.Message != tt.contains {
				t.Errorf("expected message '%s', got '%s'", tt.contains, se.Message)
			}
			if se.Error() != tt.contains {
				t.Errorf("Error() expected '%s', got '%s'", tt.contains, se.Error())
			}
		})
	}
}

func TestFormatError(t *testing.T) {
	t.Run("plain error", func(t *testing.T) {
		err := errors.New("something went wrong")
		data := formatError(err)

		var resp errorResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if resp.Error != "something went wrong" {
			t.Errorf("expected 'something went wrong', got '%s'", resp.Error)
		}
		if resp.Code != "" {
			t.Errorf("expected empty code, got '%s'", resp.Code)
		}
	})

	t.Run("ServiceError", func(t *testing.T) {
		err := BadRequest("invalid input")
		data := formatError(err)

		var resp errorResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if resp.Error != "invalid input" {
			t.Errorf("expected 'invalid input', got '%s'", resp.Error)
		}
		if resp.Code != "bad_request" {
			t.Errorf("expected 'bad_request', got '%s'", resp.Code)
		}
	})

	t.Run("ServiceError with details", func(t *testing.T) {
		err := &ServiceError{
			Code:    "validation_error",
			Message: "field errors",
			Details: map[string]string{"email": "invalid format"},
		}
		data := formatError(err)

		var resp errorResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if resp.Details == nil {
			t.Error("expected details to be set")
		}
	})
}
