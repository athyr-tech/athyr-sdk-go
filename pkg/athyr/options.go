package athyr

import (
	"crypto/tls"
	"time"
)

// AgentOption configures a Agent.
type AgentOption func(*agentOptions)

type agentOptions struct {
	agentCard         AgentCard
	heartbeatInterval time.Duration
	requestTimeout    time.Duration

	// TLS configuration
	tlsCertFile string      // Path to CA cert file (PEM)
	tlsConfig   *tls.Config // Custom TLS config (advanced)
	insecure    bool        // Explicit opt-in for insecure connections

	// Observability
	logger Logger
}

func defaultOptions() agentOptions {
	return agentOptions{
		heartbeatInterval: 30 * time.Second,
		requestTimeout:    60 * time.Second,
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
// If certFile is empty, the system certificate pool is used.
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
