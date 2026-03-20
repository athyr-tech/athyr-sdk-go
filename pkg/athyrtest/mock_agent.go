// Package athyrtest provides test utilities for the Athyr SDK.
package athyrtest

import (
	"context"

	"github.com/athyr-tech/athyr-sdk-go/pkg/athyr"
)

// MockAgent is a test double for athyr.Agent with overridable method implementations.
// Each method delegates to a corresponding function field. If the function field is nil,
// the method returns zero values.
//
// Example:
//
//	mock := &athyrtest.MockAgent{
//	    RequestFunc: func(ctx context.Context, subject string, data []byte) ([]byte, error) {
//	        return []byte(`{"result": "ok"}`), nil
//	    },
//	}
//	// Use mock wherever athyr.Agent is expected
type MockAgent struct {
	ConnectFunc        func(ctx context.Context) error
	CloseFunc          func() error
	AgentIDFunc        func() string
	ConnectedFunc      func() bool
	StateFunc          func() athyr.ConnectionState
	PublishFunc        func(ctx context.Context, subject string, data []byte) error
	SubscribeFunc      func(ctx context.Context, subject string, handler athyr.MessageHandler) (athyr.Subscription, error)
	QueueSubscribeFunc func(ctx context.Context, subject, queue string, handler athyr.MessageHandler) (athyr.Subscription, error)
	RequestFunc        func(ctx context.Context, subject string, data []byte) ([]byte, error)
	CompleteFunc       func(ctx context.Context, req athyr.CompletionRequest) (*athyr.CompletionResponse, error)
	CompleteStreamFunc func(ctx context.Context, req athyr.CompletionRequest, handler athyr.StreamHandler) error
	ResumeStreamFunc   func(ctx context.Context, requestID string, lastSequence uint64, handler athyr.StreamHandler) error
	ModelsFunc         func(ctx context.Context) ([]athyr.Model, error)
	CreateSessionFunc  func(ctx context.Context, profile athyr.SessionProfile, systemPrompt string) (*athyr.Session, error)
	GetSessionFunc     func(ctx context.Context, sessionID string) (*athyr.Session, error)
	DeleteSessionFunc  func(ctx context.Context, sessionID string) error
	AddHintFunc        func(ctx context.Context, sessionID, hint string) error
	ListAgentsFunc     func(ctx context.Context, skillFilter string) ([]athyr.AgentInfo, error)
	GetAgentFunc       func(ctx context.Context, agentID string) (*athyr.AgentInfo, error)
	KVFunc             func(bucket string) athyr.KVBucket
}

func (m *MockAgent) Connect(ctx context.Context) error {
	if m.ConnectFunc != nil {
		return m.ConnectFunc(ctx)
	}
	return nil
}

func (m *MockAgent) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func (m *MockAgent) AgentID() string {
	if m.AgentIDFunc != nil {
		return m.AgentIDFunc()
	}
	return ""
}

func (m *MockAgent) Connected() bool {
	if m.ConnectedFunc != nil {
		return m.ConnectedFunc()
	}
	return false
}

func (m *MockAgent) State() athyr.ConnectionState {
	if m.StateFunc != nil {
		return m.StateFunc()
	}
	return athyr.StateDisconnected
}

func (m *MockAgent) Publish(ctx context.Context, subject string, data []byte) error {
	if m.PublishFunc != nil {
		return m.PublishFunc(ctx, subject, data)
	}
	return nil
}

func (m *MockAgent) Subscribe(ctx context.Context, subject string, handler athyr.MessageHandler) (athyr.Subscription, error) {
	if m.SubscribeFunc != nil {
		return m.SubscribeFunc(ctx, subject, handler)
	}
	return nil, nil
}

func (m *MockAgent) QueueSubscribe(ctx context.Context, subject, queue string, handler athyr.MessageHandler) (athyr.Subscription, error) {
	if m.QueueSubscribeFunc != nil {
		return m.QueueSubscribeFunc(ctx, subject, queue, handler)
	}
	return nil, nil
}

func (m *MockAgent) Request(ctx context.Context, subject string, data []byte) ([]byte, error) {
	if m.RequestFunc != nil {
		return m.RequestFunc(ctx, subject, data)
	}
	return nil, nil
}

func (m *MockAgent) Complete(ctx context.Context, req athyr.CompletionRequest) (*athyr.CompletionResponse, error) {
	if m.CompleteFunc != nil {
		return m.CompleteFunc(ctx, req)
	}
	return nil, nil
}

func (m *MockAgent) CompleteStream(ctx context.Context, req athyr.CompletionRequest, handler athyr.StreamHandler) error {
	if m.CompleteStreamFunc != nil {
		return m.CompleteStreamFunc(ctx, req, handler)
	}
	return nil
}

func (m *MockAgent) ResumeStream(ctx context.Context, requestID string, lastSequence uint64, handler athyr.StreamHandler) error {
	if m.ResumeStreamFunc != nil {
		return m.ResumeStreamFunc(ctx, requestID, lastSequence, handler)
	}
	return nil
}

func (m *MockAgent) Models(ctx context.Context) ([]athyr.Model, error) {
	if m.ModelsFunc != nil {
		return m.ModelsFunc(ctx)
	}
	return nil, nil
}

func (m *MockAgent) CreateSession(ctx context.Context, profile athyr.SessionProfile, systemPrompt string) (*athyr.Session, error) {
	if m.CreateSessionFunc != nil {
		return m.CreateSessionFunc(ctx, profile, systemPrompt)
	}
	return nil, nil
}

func (m *MockAgent) GetSession(ctx context.Context, sessionID string) (*athyr.Session, error) {
	if m.GetSessionFunc != nil {
		return m.GetSessionFunc(ctx, sessionID)
	}
	return nil, nil
}

func (m *MockAgent) DeleteSession(ctx context.Context, sessionID string) error {
	if m.DeleteSessionFunc != nil {
		return m.DeleteSessionFunc(ctx, sessionID)
	}
	return nil
}

func (m *MockAgent) AddHint(ctx context.Context, sessionID, hint string) error {
	if m.AddHintFunc != nil {
		return m.AddHintFunc(ctx, sessionID, hint)
	}
	return nil
}

func (m *MockAgent) ListAgents(ctx context.Context, skillFilter string) ([]athyr.AgentInfo, error) {
	if m.ListAgentsFunc != nil {
		return m.ListAgentsFunc(ctx, skillFilter)
	}
	return nil, nil
}

func (m *MockAgent) GetAgent(ctx context.Context, agentID string) (*athyr.AgentInfo, error) {
	if m.GetAgentFunc != nil {
		return m.GetAgentFunc(ctx, agentID)
	}
	return nil, nil
}

func (m *MockAgent) KV(bucket string) athyr.KVBucket {
	if m.KVFunc != nil {
		return m.KVFunc(bucket)
	}
	return nil
}
