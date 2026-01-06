package athyr

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// Server manages multiple services and their lifecycle.
type Server struct {
	addr       string
	agentName  string
	agentDesc  string
	version    string
	services   []*Service
	middleware []Middleware

	// TLS configuration
	tlsCertFile string
	tlsConfig   *tls.Config
	insecure    bool

	agent Agent
	subs  []Subscription
	mu    sync.Mutex
}

// ServerOption configures a Server.
type ServerOption func(*Server)

// WithAgentName sets the agent name for registration.
func WithAgentName(name string) ServerOption {
	return func(s *Server) {
		s.agentName = name
	}
}

// WithAgentDescription sets the agent description.
func WithAgentDescription(desc string) ServerOption {
	return func(s *Server) {
		s.agentDesc = desc
	}
}

// WithVersion sets the agent version.
func WithVersion(version string) ServerOption {
	return func(s *Server) {
		s.version = version
	}
}

// WithMiddleware adds global middleware applied to all services.
func WithMiddleware(mw ...Middleware) ServerOption {
	return func(s *Server) {
		s.middleware = append(s.middleware, mw...)
	}
}

// WithServerTLS configures TLS using a CA certificate file.
func WithServerTLS(certFile string) ServerOption {
	return func(s *Server) {
		s.tlsCertFile = certFile
		s.insecure = false
	}
}

// WithServerTLSConfig configures TLS with a custom tls.Config.
func WithServerTLSConfig(cfg *tls.Config) ServerOption {
	return func(s *Server) {
		s.tlsConfig = cfg
		s.insecure = false
	}
}

// WithServerInsecure disables TLS for development and testing.
func WithServerInsecure() ServerOption {
	return func(s *Server) {
		s.insecure = true
	}
}

// NewServer creates a new server that will connect to the given address.
func NewServer(addr string, opts ...ServerOption) *Server {
	s := &Server{
		addr:      addr,
		agentName: "service-agent",
		version:   "1.0.0",
		services:  make([]*Service, 0),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Add adds a pre-built service to the server.
func (s *Server) Add(svc *Service) *Server {
	s.services = append(s.services, svc)
	return s
}

// Handle adds a typed handler for a subject.
// This is a convenience method that creates and adds a Service.
func Handle[Req, Resp any](s *Server, subject string, handler Handler[Req, Resp], opts ...ServiceOption) *Server {
	svc := NewService(subject, handler, opts...)
	s.services = append(s.services, svc)
	return s
}

// HandleRaw adds a raw handler for a subject.
func (s *Server) HandleRaw(subject string, handler RawHandler, opts ...ServiceOption) *Server {
	svc := NewRawService(subject, handler, opts...)
	s.services = append(s.services, svc)
	return s
}

// Run starts the server and blocks until context is cancelled or signal received.
// It connects to Athyr, subscribes all services, and handles graceful shutdown.
func (s *Server) Run(ctx context.Context) error {
	if len(s.services) == 0 {
		return fmt.Errorf("no services registered")
	}

	// Build agent options
	agentOpts := []AgentOption{
		WithAgentCard(AgentCard{
			Name:        s.agentName,
			Description: s.agentDesc,
			Version:     s.version,
		}),
	}

	// Add TLS options
	switch {
	case s.insecure:
		agentOpts = append(agentOpts, WithInsecure())
	case s.tlsConfig != nil:
		agentOpts = append(agentOpts, WithTLSConfig(s.tlsConfig))
	case s.tlsCertFile != "":
		agentOpts = append(agentOpts, WithTLS(s.tlsCertFile))
	}

	// Create agent
	agent, err := NewAgent(s.addr, agentOpts...)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}
	s.agent = agent

	// Connect
	if err := agent.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Subscribe all services
	for _, svc := range s.services {
		if err := s.subscribeService(ctx, svc); err != nil {
			s.cleanup()
			return fmt.Errorf("failed to subscribe %s: %w", svc.subject, err)
		}
	}

	// Wait for shutdown signal or context cancellation
	return s.waitForShutdown(ctx)
}

// subscribeService sets up a subscription for a service.
func (s *Server) subscribeService(ctx context.Context, svc *Service) error {
	handler := svc.BuildHandler(s.middleware)

	// Create the message handler
	msgHandler := func(msg SubscribeMessage) {
		svcCtx := &serviceContext{
			Context:      ctx,
			agent:        s.agent,
			subject:      msg.Subject,
			replySubject: msg.Reply,
		}

		// Call the handler
		resp, err := handler(svcCtx, msg.Data)

		// Send response if there's a reply subject
		if msg.Reply != "" {
			var respData []byte
			if err != nil {
				respData = formatError(err)
			} else {
				respData = resp
			}
			_ = s.agent.Publish(ctx, msg.Reply, respData)
		}
	}

	// Subscribe with or without queue group
	var sub Subscription
	var err error
	if svc.queueGroup != "" {
		sub, err = s.agent.QueueSubscribe(ctx, svc.subject, svc.queueGroup, msgHandler)
	} else {
		sub, err = s.agent.Subscribe(ctx, svc.subject, msgHandler)
	}

	if err != nil {
		return err
	}

	s.mu.Lock()
	s.subs = append(s.subs, sub)
	s.mu.Unlock()

	return nil
}

// waitForShutdown blocks until shutdown signal or context done.
func (s *Server) waitForShutdown(ctx context.Context) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-ctx.Done():
		s.cleanup()
		return ctx.Err()
	case sig := <-sigCh:
		s.cleanup()
		return fmt.Errorf("received signal: %v", sig)
	}
}

// cleanup unsubscribes and disconnects.
func (s *Server) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sub := range s.subs {
		_ = sub.Unsubscribe()
	}
	s.subs = nil

	if s.agent != nil {
		_ = s.agent.Close()
	}
}

// Run is a convenience function to run a single service.
func Run[Req, Resp any](ctx context.Context, addr, subject string, handler Handler[Req, Resp], opts ...ServiceOption) error {
	server := NewServer(addr, WithAgentName(subject))
	Handle(server, subject, handler, opts...)
	return server.Run(ctx)
}

// RunRaw runs a single raw service.
func RunRaw(ctx context.Context, addr, subject string, handler RawHandler, opts ...ServiceOption) error {
	server := NewServer(addr, WithAgentName(subject))
	server.HandleRaw(subject, handler, opts...)
	return server.Run(ctx)
}
