package athyr

import "time"

// AgentOption configures a Agent.
type AgentOption func(*agentOptions)

type agentOptions struct {
	agentCard         AgentCard
	heartbeatInterval time.Duration
	requestTimeout    time.Duration
}

func defaultOptions() agentOptions {
	return agentOptions{
		heartbeatInterval: 30 * time.Second,
		requestTimeout:    60 * time.Second,
	}
}

// WithAgentCard sets the agent card for registration.
func WithAgentCard(card AgentCard) AgentOption {
	return func(o *agentOptions) {
		o.agentCard = card
	}
}

// WithHeartbeatInterval sets how often heartbeats are sent.
func WithHeartbeatInterval(d time.Duration) AgentOption {
	return func(o *agentOptions) {
		o.heartbeatInterval = d
	}
}

// WithRequestTimeout sets the default timeout for requests.
func WithRequestTimeout(d time.Duration) AgentOption {
	return func(o *agentOptions) {
		o.requestTimeout = d
	}
}

// WithCapabilities sets the agent's capabilities.
// Capabilities describe what the agent can do (e.g., "chat", "analysis").
func WithCapabilities(caps ...string) AgentOption {
	return func(o *agentOptions) {
		o.agentCard.Capabilities = caps
	}
}
