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
	systemTLS   bool

	agent Agent
	subs  []Subscription
	mu    sync.Mutex
}

// ServerOption configures a Server created via NewServer, Run, or RunRaw.
// These options control server-level concerns: agent identity, TLS, and
// global middleware. For per-service configuration, see ServiceOption.
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

// WithServerSystemTLS configures TLS using the system certificate pool.
// Use this for production when connecting to servers with publicly trusted certificates.
func WithServerSystemTLS() ServerOption {
	return func(s *Server) {
		s.systemTLS = true
		s.tlsCertFile = ""
		s.tlsConfig = nil
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
// This is a package-level function for consistency with Handle.
func HandleRaw(s *Server, subject string, handler RawHandler, opts ...ServiceOption) *Server {
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
	case s.systemTLS:
		agentOpts = append(agentOpts, WithSystemTLS())
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
// serverOpts configures the server (TLS, middleware, agent name, etc.).
// If no WithAgentName option is provided, the subject is used as the agent name.
func Run[Req, Resp any](ctx context.Context, addr, subject string,
	handler Handler[Req, Resp], serverOpts []ServerOption, serviceOpts ...ServiceOption) error {
	serverOpts = ensureAgentName(serverOpts, subject)
	server := NewServer(addr, serverOpts...)
	Handle(server, subject, handler, serviceOpts...)
	return server.Run(ctx)
}

// RunRaw runs a single raw service.
// serverOpts configures the server (TLS, middleware, agent name, etc.).
// If no WithAgentName option is provided, the subject is used as the agent name.
func RunRaw(ctx context.Context, addr, subject string,
	handler RawHandler, serverOpts []ServerOption, serviceOpts ...ServiceOption) error {
	serverOpts = ensureAgentName(serverOpts, subject)
	server := NewServer(addr, serverOpts...)
	HandleRaw(server, subject, handler, serviceOpts...)
	return server.Run(ctx)
}

// ensureAgentName prepends WithAgentName(subject) so user-provided
// WithAgentName options (applied later) take precedence.
func ensureAgentName(opts []ServerOption, subject string) []ServerOption {
	return append([]ServerOption{WithAgentName(subject)}, opts...)
}
