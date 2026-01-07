package athyr

import (
	"context"
	"fmt"
	"math/rand"
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
	State() ConnectionState

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

// subRecord tracks subscription info for re-subscription after reconnect.
type subRecord struct {
	subject string
	queue   string
	handler MessageHandler
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

	// Connection state management
	state         ConnectionState
	reconnectMu   sync.Mutex   // Prevents concurrent reconnection attempts
	reconnectCtx  context.Context
	reconnectStop context.CancelFunc
	subscriptions []subRecord // Track subscriptions for re-subscribe
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
	if c.connected {
		c.mu.Unlock()
		return ErrAlreadyConnected
	}
	c.mu.Unlock()

	c.setState(StateConnecting, nil)

	// Build transport credentials based on TLS options
	creds, err := c.buildTransportCredentials()
	if err != nil {
		c.setState(StateDisconnected, err)
		return fmt.Errorf("failed to configure TLS: %w", err)
	}

	// Establish gRPC connection
	conn, err := grpc.NewClient(c.addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		c.setState(StateDisconnected, err)
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.athyr = athyr.NewAthyrClient(conn)
	c.mu.Unlock()

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
		c.setState(StateDisconnected, err)
		return fmt.Errorf("failed to register: %w", err)
	}

	c.mu.Lock()
	c.agentID = resp.AgentId
	c.mu.Unlock()

	c.setState(StateConnected, nil)
	c.opts.logger.Info("connected to athyr", "addr", c.addr, "agent_id", c.agentID)

	// Start heartbeat loop
	hbCtx, cancel := context.WithCancel(context.Background())
	c.mu.Lock()
	c.heartbeatCancel = cancel
	c.mu.Unlock()
	go c.heartbeatLoop(hbCtx)

	return nil
}

// Close disconnects from the platform.
func (c *agent) Close() error {
	c.mu.Lock()

	// Cancel any in-progress reconnection
	if c.reconnectStop != nil {
		c.reconnectStop()
		c.reconnectStop = nil
	}

	if !c.connected && c.state != StateReconnecting {
		c.mu.Unlock()
		return nil
	}

	// Stop heartbeat
	if c.heartbeatCancel != nil {
		c.heartbeatCancel()
	}

	// Clear subscription tracking
	c.subscriptions = nil

	agentID := c.agentID
	conn := c.conn
	athyrClient := c.athyr
	c.mu.Unlock()

	// Gracefully disconnect if we have a connection
	if athyrClient != nil && agentID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		athyrClient.Disconnect(ctx, &athyr.DisconnectRequest{AgentId: agentID})
	}

	c.setState(StateDisconnected, nil)
	c.opts.logger.Info("disconnected from athyr", "agent_id", agentID)

	if conn != nil {
		return conn.Close()
	}
	return nil
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

// State returns the current connection state.
func (c *agent) State() ConnectionState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// setState updates the connection state and invokes the callback if set.
func (c *agent) setState(state ConnectionState, err error) {
	c.mu.Lock()
	c.state = state
	c.connected = (state == StateConnected)
	callback := c.opts.onStateChange
	c.mu.Unlock()

	if callback != nil {
		callback(state, err)
	}

	// Log state transitions
	switch state {
	case StateConnecting:
		c.opts.logger.Debug("connection state changed", "state", state.String())
	case StateConnected:
		c.opts.logger.Info("connection state changed", "state", state.String())
	case StateReconnecting:
		c.opts.logger.Warn("connection state changed", "state", state.String(), "error", err)
	case StateDisconnected:
		if err != nil {
			c.opts.logger.Error("connection state changed", "state", state.String(), "error", err)
		} else {
			c.opts.logger.Info("connection state changed", "state", state.String())
		}
	}
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

				// Trigger reconnection if auto-reconnect is enabled
				if c.opts.autoReconnect {
					c.triggerReconnect(err)
					return // Stop this heartbeat loop, reconnection will start a new one
				}
			}
		}
	}
}

// triggerReconnect starts the reconnection process if not already reconnecting.
func (c *agent) triggerReconnect(err error) {
	c.reconnectMu.Lock()
	defer c.reconnectMu.Unlock()

	// Check if already reconnecting
	c.mu.RLock()
	state := c.state
	c.mu.RUnlock()

	if state == StateReconnecting {
		return
	}

	c.setState(StateReconnecting, err)

	// Create cancellable context for reconnection
	ctx, cancel := context.WithCancel(context.Background())
	c.mu.Lock()
	c.reconnectCtx = ctx
	c.reconnectStop = cancel
	c.mu.Unlock()

	go c.reconnectLoop(ctx, err)
}

// reconnectLoop attempts to reconnect with exponential backoff.
func (c *agent) reconnectLoop(ctx context.Context, initialErr error) {
	attempt := 0
	lastErr := initialErr

	for {
		select {
		case <-ctx.Done():
			c.opts.logger.Info("reconnection cancelled")
			return
		default:
		}

		// Check max retries
		if c.opts.maxRetries > 0 && attempt >= c.opts.maxRetries {
			c.opts.logger.Error("max reconnection attempts reached",
				"attempts", attempt,
				"last_error", lastErr)
			c.setState(StateDisconnected, lastErr)
			return
		}

		// Calculate backoff with jitter
		backoff := c.backoffDuration(attempt)
		c.opts.logger.Info("reconnecting",
			"attempt", attempt+1,
			"backoff", backoff.String())

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		// Attempt reconnection
		if err := c.doReconnect(ctx); err != nil {
			lastErr = err
			attempt++
			continue
		}

		// Reconnection successful
		c.opts.logger.Info("reconnected successfully", "attempts", attempt+1)
		return
	}
}

// backoffDuration calculates exponential backoff with jitter.
func (c *agent) backoffDuration(attempt int) time.Duration {
	// Exponential backoff: baseBackoff * 2^attempt
	backoff := c.opts.baseBackoff * time.Duration(1<<uint(attempt))

	// Cap at maxBackoff
	if backoff > c.opts.maxBackoff {
		backoff = c.opts.maxBackoff
	}

	// Add jitter: 0.5x to 1.5x
	jitter := 0.5 + rand.Float64()
	return time.Duration(float64(backoff) * jitter)
}

// doReconnect performs the actual reconnection attempt.
func (c *agent) doReconnect(ctx context.Context) error {
	// Close existing connection
	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
		c.athyr = nil
	}
	c.mu.Unlock()

	// Build transport credentials
	creds, err := c.buildTransportCredentials()
	if err != nil {
		return fmt.Errorf("failed to configure TLS: %w", err)
	}

	// Establish new gRPC connection
	conn, err := grpc.NewClient(c.addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.athyr = athyr.NewAthyrClient(conn)
	c.mu.Unlock()

	// Re-register agent
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
		c.mu.Lock()
		c.conn = nil
		c.athyr = nil
		c.mu.Unlock()
		return fmt.Errorf("failed to register: %w", err)
	}

	c.mu.Lock()
	c.agentID = resp.AgentId
	subs := make([]subRecord, len(c.subscriptions))
	copy(subs, c.subscriptions)
	c.mu.Unlock()

	// Re-subscribe to all tracked subscriptions
	if err := c.resubscribeAll(ctx, subs); err != nil {
		c.opts.logger.Warn("some subscriptions failed to restore", "error", err)
	}

	c.setState(StateConnected, nil)

	// Start new heartbeat loop
	hbCtx, cancel := context.WithCancel(context.Background())
	c.mu.Lock()
	c.heartbeatCancel = cancel
	c.reconnectCtx = nil
	c.reconnectStop = nil
	c.mu.Unlock()
	go c.heartbeatLoop(hbCtx)

	return nil
}

// resubscribeAll re-subscribes to all tracked subscriptions.
func (c *agent) resubscribeAll(ctx context.Context, subs []subRecord) error {
	var lastErr error

	for _, sub := range subs {
		var err error
		if sub.queue != "" {
			_, err = c.QueueSubscribe(ctx, sub.subject, sub.queue, sub.handler)
		} else {
			_, err = c.Subscribe(ctx, sub.subject, sub.handler)
		}
		if err != nil {
			c.opts.logger.Error("failed to re-subscribe",
				"subject", sub.subject,
				"queue", sub.queue,
				"error", err)
			lastErr = err
		}
	}

	return lastErr
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
