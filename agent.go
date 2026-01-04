package sdk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	athyr "github.com/athyr-tech/athyr-sdk-go/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Common errors.
var (
	ErrNotConnected     = errors.New("agent not connected")
	ErrAlreadyConnected = errors.New("agent already connected")
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
	CreateSession(ctx context.Context, profile SessionProfile) (*Session, error)
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	DeleteSession(ctx context.Context, sessionID string) error
	AddHint(ctx context.Context, sessionID, hint string) error

	// KV
	KV(bucket string) KVBucket
}

// KVBucket provides key-value operations on a specific bucket.
type KVBucket interface {
	Get(ctx context.Context, key string) (*KVEntry, error)
	Put(ctx context.Context, key string, value []byte) (uint64, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]string, error)
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

// Connect establishes a connection and registers the agent.
func (c *agent) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return ErrAlreadyConnected
	}

	// Establish gRPC connection
	conn, err := grpc.NewClient(c.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
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
			c.athyr.Heartbeat(hbCtx, &athyr.HeartbeatRequest{AgentId: agentID})
			cancel()
		}
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

// ============ Messaging ============

func (c *agent) Publish(ctx context.Context, subject string, data []byte) error {
	if err := c.checkConnected(); err != nil {
		return err
	}

	_, err := c.athyr.Publish(ctx, &athyr.PublishRequest{
		AgentId: c.agentID,
		Subject: subject,
		Data:    data,
	})
	return err
}

func (c *agent) Subscribe(ctx context.Context, subject string, handler MessageHandler) (Subscription, error) {
	return c.subscribe(ctx, subject, "", handler)
}

func (c *agent) QueueSubscribe(ctx context.Context, subject, queue string, handler MessageHandler) (Subscription, error) {
	return c.subscribe(ctx, subject, queue, handler)
}

func (c *agent) subscribe(ctx context.Context, subject, queue string, handler MessageHandler) (Subscription, error) {
	if err := c.checkConnected(); err != nil {
		return nil, err
	}

	stream, err := c.athyr.Subscribe(ctx, &athyr.SubscribeRequest{
		AgentId:    c.agentID,
		Subject:    subject,
		QueueGroup: queue,
	})
	if err != nil {
		return nil, err
	}

	sub := &subscription{
		cancel: func() { stream.CloseSend() },
	}

	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				return
			}
			handler(SubscribeMessage{
				Subject: msg.Subject,
				Data:    msg.Data,
				Reply:   msg.Reply,
			})
		}
	}()

	return sub, nil
}

func (c *agent) Request(ctx context.Context, subject string, data []byte) ([]byte, error) {
	if err := c.checkConnected(); err != nil {
		return nil, err
	}

	resp, err := c.athyr.Request(ctx, &athyr.RequestMessage{
		AgentId:   c.agentID,
		Subject:   subject,
		Data:      data,
		TimeoutMs: c.opts.requestTimeout.Milliseconds(),
	})
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// ============ LLM ============

func (c *agent) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	if err := c.checkConnected(); err != nil {
		return nil, err
	}

	protoReq := &athyr.CompletionRequest{
		AgentId:       c.agentID,
		Model:         req.Model,
		SessionId:     req.SessionID,
		IncludeMemory: req.IncludeMemory,
		Config: &athyr.CompletionConfig{
			Temperature: req.Config.Temperature,
			MaxTokens:   int32(req.Config.MaxTokens),
			TopP:        req.Config.TopP,
			Stop:        req.Config.Stop,
		},
	}

	for _, msg := range req.Messages {
		protoReq.Messages = append(protoReq.Messages, &athyr.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	resp, err := c.athyr.Complete(ctx, protoReq)
	if err != nil {
		return nil, err
	}

	return &CompletionResponse{
		Content:      resp.Content,
		Model:        resp.Model,
		Backend:      resp.Backend,
		FinishReason: resp.FinishReason,
		Latency:      time.Duration(resp.LatencyMs) * time.Millisecond,
		Usage: TokenUsage{
			PromptTokens:     int(resp.Usage.PromptTokens),
			CompletionTokens: int(resp.Usage.CompletionTokens),
			TotalTokens:      int(resp.Usage.TotalTokens),
		},
	}, nil
}

func (c *agent) CompleteStream(ctx context.Context, req CompletionRequest, handler StreamHandler) error {
	if err := c.checkConnected(); err != nil {
		return err
	}

	protoReq := &athyr.CompletionRequest{
		AgentId:       c.agentID,
		Model:         req.Model,
		SessionId:     req.SessionID,
		IncludeMemory: req.IncludeMemory,
		Config: &athyr.CompletionConfig{
			Temperature: req.Config.Temperature,
			MaxTokens:   int32(req.Config.MaxTokens),
			TopP:        req.Config.TopP,
			Stop:        req.Config.Stop,
		},
	}

	for _, msg := range req.Messages {
		protoReq.Messages = append(protoReq.Messages, &athyr.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	stream, err := c.athyr.CompleteStream(ctx, protoReq)
	if err != nil {
		return err
	}

	// Track last error chunk for StreamError context
	var lastErrorChunk *athyr.StreamChunk

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			// If we received an error chunk before the gRPC error,
			// wrap it in a StreamError for agent retry decisions
			if lastErrorChunk != nil && lastErrorChunk.PartialResponse {
				return &StreamError{
					Err:                errors.New(lastErrorChunk.Error),
					Backend:            lastErrorChunk.Backend,
					AccumulatedContent: lastErrorChunk.AccumulatedContent,
					PartialResponse:    lastErrorChunk.PartialResponse,
				}
			}
			return err
		}

		// Check for error chunk (sent before stream closes on failure)
		if chunk.Error != "" && chunk.Done {
			lastErrorChunk = chunk
			// Don't call handler for error chunks - the error will be returned
			continue
		}

		sdkChunk := StreamChunk{
			Content: chunk.Content,
			Done:    chunk.Done,
			Model:   chunk.Model,
			Backend: chunk.Backend,
			Error:   chunk.Error,
		}
		if chunk.Usage != nil {
			sdkChunk.Usage = &TokenUsage{
				PromptTokens:     int(chunk.Usage.PromptTokens),
				CompletionTokens: int(chunk.Usage.CompletionTokens),
				TotalTokens:      int(chunk.Usage.TotalTokens),
			}
		}

		if err := handler(sdkChunk); err != nil {
			return err
		}
	}
}

func (c *agent) Models(ctx context.Context) ([]Model, error) {
	if err := c.checkConnected(); err != nil {
		return nil, err
	}

	resp, err := c.athyr.ListModels(ctx, &athyr.ListModelsRequest{})
	if err != nil {
		return nil, err
	}

	models := make([]Model, len(resp.Models))
	for i, m := range resp.Models {
		models[i] = Model{
			ID:        m.Id,
			Name:      m.Name,
			Backend:   m.Backend,
			Available: m.Available,
		}
	}

	return models, nil
}

// ============ Memory ============

func (c *agent) CreateSession(ctx context.Context, profile SessionProfile) (*Session, error) {
	if err := c.checkConnected(); err != nil {
		return nil, err
	}

	resp, err := c.athyr.CreateSession(ctx, &athyr.CreateSessionRequest{
		AgentId: c.agentID,
		Profile: &athyr.SessionProfile{
			Type:                   profile.Type,
			MaxTokens:              int32(profile.MaxTokens),
			SummarizationThreshold: int32(profile.SummarizationThreshold),
		},
	})
	if err != nil {
		return nil, err
	}

	return protoToSession(resp), nil
}

func (c *agent) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	if err := c.checkConnected(); err != nil {
		return nil, err
	}

	resp, err := c.athyr.GetSession(ctx, &athyr.GetSessionRequest{
		AgentId:   c.agentID,
		SessionId: sessionID,
	})
	if err != nil {
		return nil, err
	}

	return protoToSession(resp), nil
}

func (c *agent) DeleteSession(ctx context.Context, sessionID string) error {
	if err := c.checkConnected(); err != nil {
		return err
	}

	_, err := c.athyr.DeleteSession(ctx, &athyr.DeleteSessionRequest{
		AgentId:   c.agentID,
		SessionId: sessionID,
	})
	return err
}

func (c *agent) AddHint(ctx context.Context, sessionID, hint string) error {
	if err := c.checkConnected(); err != nil {
		return err
	}

	_, err := c.athyr.AddHint(ctx, &athyr.AddHintRequest{
		AgentId:   c.agentID,
		SessionId: sessionID,
		Hint:      hint,
	})
	return err
}

// ============ KV ============

func (c *agent) KV(bucket string) KVBucket {
	return &kvBucket{
		client: c,
		bucket: bucket,
	}
}

// ============ Helpers ============

type subscription struct {
	cancel func()
}

func (s *subscription) Unsubscribe() error {
	s.cancel()
	return nil
}

type kvBucket struct {
	client *agent
	bucket string
}

func (k *kvBucket) Get(ctx context.Context, key string) (*KVEntry, error) {
	if err := k.client.checkConnected(); err != nil {
		return nil, err
	}

	resp, err := k.client.athyr.KVGet(ctx, &athyr.KVGetRequest{
		AgentId: k.client.agentID,
		Bucket:  k.bucket,
		Key:     key,
	})
	if err != nil {
		return nil, err
	}

	return &KVEntry{
		Value:    resp.Value,
		Revision: resp.Revision,
	}, nil
}

func (k *kvBucket) Put(ctx context.Context, key string, value []byte) (uint64, error) {
	if err := k.client.checkConnected(); err != nil {
		return 0, err
	}

	resp, err := k.client.athyr.KVPut(ctx, &athyr.KVPutRequest{
		AgentId: k.client.agentID,
		Bucket:  k.bucket,
		Key:     key,
		Value:   value,
	})
	if err != nil {
		return 0, err
	}

	return resp.Revision, nil
}

func (k *kvBucket) Delete(ctx context.Context, key string) error {
	if err := k.client.checkConnected(); err != nil {
		return err
	}

	_, err := k.client.athyr.KVDelete(ctx, &athyr.KVDeleteRequest{
		AgentId: k.client.agentID,
		Bucket:  k.bucket,
		Key:     key,
	})
	return err
}

func (k *kvBucket) List(ctx context.Context, prefix string) ([]string, error) {
	if err := k.client.checkConnected(); err != nil {
		return nil, err
	}

	resp, err := k.client.athyr.KVList(ctx, &athyr.KVListRequest{
		AgentId: k.client.agentID,
		Bucket:  k.bucket,
		Prefix:  prefix,
	})
	if err != nil {
		return nil, err
	}

	return resp.Keys, nil
}

func protoToSession(s *athyr.Session) *Session {
	session := &Session{
		ID:      s.Id,
		AgentID: s.AgentId,
		Summary: s.Summary,
		Hints:   s.Hints,
	}

	if s.CreatedAt != nil {
		session.CreatedAt = s.CreatedAt.AsTime()
	}
	if s.UpdatedAt != nil {
		session.UpdatedAt = s.UpdatedAt.AsTime()
	}
	if s.Profile != nil {
		session.Profile = SessionProfile{
			Type:                   s.Profile.Type,
			MaxTokens:              int(s.Profile.MaxTokens),
			SummarizationThreshold: int(s.Profile.SummarizationThreshold),
		}
	}

	for _, msg := range s.Messages {
		m := SessionMessage{
			Role:    msg.Role,
			Content: msg.Content,
			Tokens:  int(msg.Tokens),
		}
		if msg.Timestamp != nil {
			m.Timestamp = msg.Timestamp.AsTime()
		}
		session.Messages = append(session.Messages, m)
	}

	return session
}
