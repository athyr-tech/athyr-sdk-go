package agent

import (
	"context"
	"encoding/json"
	"fmt"
)

// Context is passed to service handlers, providing access to the agent
// and request metadata.
type Context interface {
	context.Context

	// Agent returns the underlying agent connection.
	Agent() Agent

	// Subject returns the subject this request arrived on.
	Subject() string

	// ReplySubject returns where to send the response (used internally).
	ReplySubject() string
}

// serviceContext implements Context.
type serviceContext struct {
	context.Context
	agent        Agent
	subject      string
	replySubject string
}

func (c *serviceContext) Agent() Agent         { return c.agent }
func (c *serviceContext) Subject() string      { return c.subject }
func (c *serviceContext) ReplySubject() string { return c.replySubject }

// Handler is a typed request handler function.
// The SDK automatically handles JSON marshaling/unmarshaling.
type Handler[Req, Resp any] func(ctx Context, req Req) (Resp, error)

// RawHandler handles requests with raw bytes for full control.
type RawHandler func(ctx Context, data []byte) ([]byte, error)

// Middleware wraps a RawHandler to add cross-cutting concerns.
type Middleware func(RawHandler) RawHandler

// Service represents a single endpoint that an agent exposes.
type Service struct {
	name       string
	subject    string
	queueGroup string
	handler    RawHandler
	middleware []Middleware
}

// ServiceOption configures a Service.
type ServiceOption func(*Service)

// WithQueueGroup sets the queue group for load balancing.
// Multiple instances with the same queue group share the load.
func WithQueueGroup(group string) ServiceOption {
	return func(s *Service) {
		s.queueGroup = group
	}
}

// WithName sets a display name for the service.
func WithName(name string) ServiceOption {
	return func(s *Service) {
		s.name = name
	}
}

// WithServiceMiddleware adds middleware to this specific service.
func WithServiceMiddleware(mw ...Middleware) ServiceOption {
	return func(s *Service) {
		s.middleware = append(s.middleware, mw...)
	}
}

// NewService creates a service with a typed handler.
// Request and response types are automatically JSON marshaled.
func NewService[Req, Resp any](subject string, handler Handler[Req, Resp], opts ...ServiceOption) *Service {
	s := &Service{
		name:    subject, // Default name is the subject
		subject: subject,
		handler: wrapTypedHandler(handler),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// NewRawService creates a service with a raw byte handler.
// Use this when you need full control over serialization.
func NewRawService(subject string, handler RawHandler, opts ...ServiceOption) *Service {
	s := &Service{
		name:    subject,
		subject: subject,
		handler: handler,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// wrapTypedHandler converts a typed handler to a raw handler.
func wrapTypedHandler[Req, Resp any](handler Handler[Req, Resp]) RawHandler {
	return func(ctx Context, data []byte) ([]byte, error) {
		var req Req
		if len(data) > 0 {
			if err := json.Unmarshal(data, &req); err != nil {
				return nil, BadRequest("invalid request: %v", err)
			}
		}

		resp, err := handler(ctx, req)
		if err != nil {
			return nil, err
		}

		respBytes, err := json.Marshal(resp)
		if err != nil {
			return nil, Internal("failed to marshal response: %v", err)
		}

		return respBytes, nil
	}
}

// BuildHandler applies middleware and returns the final handler.
func (s *Service) BuildHandler(globalMiddleware []Middleware) RawHandler {
	handler := s.handler

	// Apply service-specific middleware (inner)
	for i := len(s.middleware) - 1; i >= 0; i-- {
		handler = s.middleware[i](handler)
	}

	// Apply global middleware (outer)
	for i := len(globalMiddleware) - 1; i >= 0; i-- {
		handler = globalMiddleware[i](handler)
	}

	return handler
}

// errorResponse is the wire format for error responses.
type errorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details any    `json:"details,omitempty"`
}

// formatError converts an error to JSON response bytes.
func formatError(err error) []byte {
	resp := errorResponse{
		Error: err.Error(),
	}

	// Check for typed errors
	if se, ok := err.(*ServiceError); ok {
		resp.Code = se.Code
		resp.Details = se.Details
	}

	data, _ := json.Marshal(resp)
	return data
}

// ServiceError is a structured error with code and details.
type ServiceError struct {
	Code    string
	Message string
	Details any
}

func (e *ServiceError) Error() string {
	return e.Message
}

// BadRequest creates a client error (invalid input).
func BadRequest(format string, args ...any) error {
	return &ServiceError{
		Code:    "bad_request",
		Message: fmt.Sprintf(format, args...),
	}
}

// Internal creates a server error.
func Internal(format string, args ...any) error {
	return &ServiceError{
		Code:    "internal",
		Message: fmt.Sprintf(format, args...),
	}
}

// Unavailable creates a temporary failure error (retry later).
func Unavailable(format string, args ...any) error {
	return &ServiceError{
		Code:    "unavailable",
		Message: fmt.Sprintf(format, args...),
	}
}

// NotFound creates a not found error.
func NotFound(format string, args ...any) error {
	return &ServiceError{
		Code:    "not_found",
		Message: fmt.Sprintf(format, args...),
	}
}
