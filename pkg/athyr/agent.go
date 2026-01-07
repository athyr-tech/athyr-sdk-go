package athyr

import (
	"context"
	"fmt"
	"sync"
	"time"

	athyr "github.com/athyr-tech/athyr-sdk-go/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Agent provides access to the Athyr platform.
type Agent interface {
	// Lifecycle
	Connect(ctx context.Context) error
	Close() error
	AgentID() string
	Connected() bool

	// Messaging
	Publish(ctx context.Context, subject string, data []byte) error
	Subscribe(ctx context.Context, subject string, handler MessageHandler) (Subscription, error)
	QueueSubscribe(ctx context.Context, subject, queue string, handler MessageHandler) (Subscription, error)
	Request(ctx context.Context, subject string, data []byte) ([]byte, error)

	// LLM
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
	CompleteStream(ctx context.Context, req CompletionRequest, handler StreamHandler) error
	Models(ctx context.Context) ([]Model, error)

	// Memory
	CreateSession(ctx context.Context, profile SessionProfile, systemPrompt string) (*Session, error)
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	DeleteSession(ctx context.Context, sessionID string) error
	AddHint(ctx context.Context, sessionID, hint string) error

	// KV
	KV(bucket string) KVBucket
}

// agent implements the Agent interface.
type agent struct {
	addr    string
	opts    agentOptions
	conn    *grpc.ClientConn
	athyr   athyr.AthyrClient
	agentID string

	heartbeatCancel context.CancelFunc
	mu              sync.RWMutex
	connected       bool
}

// NewAgent creates a new Athyr agent.
func NewAgent(addr string, opts ...AgentOption) (Agent, error) {
	options := defaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	return &agent{
		addr: addr,
		opts: options,
	}, nil
}

// MustConnect creates and connects an agent, panicking on error.
// Use this for initialization code where connection failure is unrecoverable.
//
// Example:
//
//	agent := athyr.MustConnect("localhost:9090",
//	    athyr.WithAgentCard(athyr.AgentCard{Name: "my-agent"}),
//	)
//	defer agent.Close()
func MustConnect(addr string, opts ...AgentOption) Agent {
	agent, err := NewAgent(addr, opts...)
	if err != nil {
		panic(fmt.Sprintf("athyr: failed to create agent: %v", err))
	}

	if err := agent.Connect(context.Background()); err != nil {
		panic(fmt.Sprintf("athyr: failed to connect to %s: %v", addr, err))
	}

	return agent
}

// Connect establishes a connection and registers the agent.
func (c *agent) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return ErrAlreadyConnected
	}

	// Build transport credentials based on TLS options
	creds, err := c.buildTransportCredentials()
	if err != nil {
		return fmt.Errorf("failed to configure TLS: %w", err)
	}

	// Establish gRPC connection
	conn, err := grpc.NewClient(c.addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	c.conn = conn
	c.athyr = athyr.NewAthyrClient(conn)

	// Register agent
	resp, err := c.athyr.Connect(ctx, &athyr.ConnectRequest{
		AgentCard: &athyr.AgentCard{
			Name:         c.opts.agentCard.Name,
			Description:  c.opts.agentCard.Description,
			Version:      c.opts.agentCard.Version,
			Capabilities: c.opts.agentCard.Capabilities,
			Metadata:     c.opts.agentCard.Metadata,
		},
	})
	if err != nil {
		c.conn.Close()
		return fmt.Errorf("failed to register: %w", err)
	}

	c.agentID = resp.AgentId
	c.connected = true

	c.opts.logger.Info("connected to athyr", "addr", c.addr, "agent_id", c.agentID)

	// Start heartbeat loop
	hbCtx, cancel := context.WithCancel(context.Background())
	c.heartbeatCancel = cancel
	go c.heartbeatLoop(hbCtx)

	return nil
}

// Close disconnects from the platform.
func (c *agent) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	// Stop heartbeat
	if c.heartbeatCancel != nil {
		c.heartbeatCancel()
	}

	// Gracefully disconnect
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.athyr.Disconnect(ctx, &athyr.DisconnectRequest{AgentId: c.agentID})

	c.opts.logger.Info("disconnected from athyr", "agent_id", c.agentID)

	c.connected = false
	return c.conn.Close()
}

// AgentID returns the assigned agent ID.
func (c *agent) AgentID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.agentID
}

// Connected returns whether the agent is connected.
func (c *agent) Connected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

func (c *agent) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(c.opts.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.mu.RLock()
			agentID := c.agentID
			c.mu.RUnlock()

			hbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			_, err := c.athyr.Heartbeat(hbCtx, &athyr.HeartbeatRequest{AgentId: agentID})
			cancel()

			if err != nil {
				c.opts.logger.Error("heartbeat failed", "error", err)
			}
		}
	}
}

// buildTransportCredentials returns the appropriate gRPC transport credentials
// based on the agent's TLS configuration.
func (c *agent) buildTransportCredentials() (credentials.TransportCredentials, error) {
	switch {
	case c.opts.insecure:
		// Explicit insecure mode for development
		return insecure.NewCredentials(), nil

	case c.opts.tlsConfig != nil:
		// Custom TLS config provided
		return credentials.NewTLS(c.opts.tlsConfig), nil

	case c.opts.tlsCertFile != "":
		// Load CA cert from file
		creds, err := credentials.NewClientTLSFromFile(c.opts.tlsCertFile, "")
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS cert %q: %w", c.opts.tlsCertFile, err)
		}
		return creds, nil

	default:
		// No TLS options specified - backwards compatibility mode
		// Log warning and use insecure (will be removed in future version)
		c.opts.logger.Warn("no TLS configured, connection is insecure",
			"hint", "use WithInsecure() to silence this warning, or WithSystemTLS()/WithTLS() for production")
		return insecure.NewCredentials(), nil
	}
}

func (c *agent) checkConnected() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.connected {
		return ErrNotConnected
	}
	return nil
}
