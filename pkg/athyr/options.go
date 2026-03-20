package athyr

import (
	"crypto/tls"
	"time"
)

// ConnectionState represents the agent's connection status.
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateReconnecting
)

func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateReconnecting:
		return "reconnecting"
	default:
		return "unknown"
	}
}

// ConnectionCallback is called when connection state changes.
// The error is non-nil when transitioning to StateDisconnected or StateReconnecting.
type ConnectionCallback func(state ConnectionState, err error)

// AgentOption configures an Agent created via NewAgent or MustConnect.
// These options control agent-level concerns: TLS, heartbeats, reconnection,
// and observability. For server-level configuration, see ServerOption.
type AgentOption func(*agentOptions)

type agentOptions struct {
	agentCard         AgentCard
	heartbeatInterval time.Duration
	requestTimeout    time.Duration

	// TLS configuration
	tlsCertFile string      // Path to CA cert file (PEM)
	tlsConfig   *tls.Config // Custom TLS config (advanced)
	insecure    bool        // Explicit opt-in for insecure connections
	systemTLS   bool        // Use system certificate pool

	// Auto-reconnect
	autoReconnect bool
	maxRetries    int           // 0 = infinite retries
	baseBackoff   time.Duration // Initial backoff duration
	maxBackoff    time.Duration // Maximum backoff duration
	onStateChange ConnectionCallback

	// Observability
	logger Logger
}

func defaultOptions() agentOptions {
	return agentOptions{
		heartbeatInterval: 30 * time.Second,
		requestTimeout:    60 * time.Second,
		baseBackoff:       1 * time.Second,
		maxBackoff:        30 * time.Second,
		logger:            nopLogger{},
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

// WithTLS configures TLS using a CA certificate file.
// The cert file should be a PEM-encoded CA certificate.
// To use the system certificate pool instead, use WithSystemTLS().
//
// Example:
//
//	agent, _ := athyr.NewAgent("athyr.example.com:9090",
//	    athyr.WithTLS("/etc/ssl/certs/athyr-ca.pem"),
//	)
func WithTLS(certFile string) AgentOption {
	return func(o *agentOptions) {
		o.tlsCertFile = certFile
		o.insecure = false
	}
}

// WithSystemTLS configures TLS using the system certificate pool.
// Use this for production when connecting to servers with publicly trusted certificates.
//
// Example:
//
//	agent, _ := athyr.NewAgent("athyr.example.com:9090",
//	    athyr.WithSystemTLS(),
//	)
func WithSystemTLS() AgentOption {
	return func(o *agentOptions) {
		o.systemTLS = true
		o.tlsCertFile = ""
		o.tlsConfig = nil
		o.insecure = false
	}
}

// WithTLSConfig configures TLS with a custom tls.Config.
// Use this for advanced scenarios like mutual TLS or custom verification.
//
// Example:
//
//	agent, _ := athyr.NewAgent("athyr.example.com:9090",
//	    athyr.WithTLSConfig(&tls.Config{
//	        MinVersion: tls.VersionTLS13,
//	    }),
//	)
func WithTLSConfig(cfg *tls.Config) AgentOption {
	return func(o *agentOptions) {
		o.tlsConfig = cfg
		o.insecure = false
	}
}

// WithInsecure disables TLS for development and testing.
// WARNING: Do not use in production. Traffic will be unencrypted.
//
// Example:
//
//	agent, _ := athyr.NewAgent("localhost:9090",
//	    athyr.WithInsecure(),
//	)
func WithInsecure() AgentOption {
	return func(o *agentOptions) {
		o.insecure = true
	}
}

// WithLogger sets a logger for observability.
// The logger interface is compatible with slog.Logger, zap.SugaredLogger, and similar.
// By default, no logging is performed.
//
// Example with slog:
//
//	agent, _ := athyr.NewAgent("localhost:9090",
//	    athyr.WithLogger(slog.Default()),
//	)
func WithLogger(logger Logger) AgentOption {
	return func(o *agentOptions) {
		if logger != nil {
			o.logger = logger
		}
	}
}

// WithAutoReconnect enables automatic reconnection on connection loss.
// maxRetries specifies the maximum number of reconnection attempts (0 = infinite).
// baseBackoff specifies the initial backoff duration between attempts.
// Backoff increases exponentially with jitter, capped at maxBackoff (default: 30s).
//
// Example:
//
//	agent, _ := athyr.NewAgent("athyr.example.com:9090",
//	    athyr.WithAutoReconnect(10, time.Second), // 10 retries, 1s base backoff
//	)
func WithAutoReconnect(maxRetries int, baseBackoff time.Duration) AgentOption {
	return func(o *agentOptions) {
		o.autoReconnect = true
		o.maxRetries = maxRetries
		if baseBackoff > 0 {
			o.baseBackoff = baseBackoff
		}
	}
}

// WithMaxBackoff sets the maximum backoff duration for reconnection attempts.
// Default is 30 seconds.
//
// Example:
//
//	agent, _ := athyr.NewAgent("athyr.example.com:9090",
//	    athyr.WithAutoReconnect(0, time.Second),
//	    athyr.WithMaxBackoff(time.Minute),
//	)
func WithMaxBackoff(d time.Duration) AgentOption {
	return func(o *agentOptions) {
		if d > 0 {
			o.maxBackoff = d
		}
	}
}

// WithConnectionCallback sets a callback invoked when connection state changes.
// The callback receives the new state and any associated error.
// Use this to monitor connection health and react to disconnections.
//
// Example:
//
//	agent, _ := athyr.NewAgent("athyr.example.com:9090",
//	    athyr.WithConnectionCallback(func(state athyr.ConnectionState, err error) {
//	        if state == athyr.StateReconnecting {
//	            log.Println("Connection lost, reconnecting...")
//	        }
//	    }),
//	)
func WithConnectionCallback(cb ConnectionCallback) AgentOption {
	return func(o *agentOptions) {
		o.onStateChange = cb
	}
}
