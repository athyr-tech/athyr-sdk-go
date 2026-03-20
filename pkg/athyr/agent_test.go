package athyr

import (
	"testing"
)

func TestBuildTransportCredentials_SystemTLS(t *testing.T) {
	a := &agent{
		opts: agentOptions{
			systemTLS: true,
			logger:    nopLogger{},
		},
	}

	creds, err := a.buildTransportCredentials()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// System TLS should return real TLS credentials
	info := creds.Info()
	if info.SecurityProtocol != "tls" {
		t.Errorf("expected security protocol 'tls', got %q", info.SecurityProtocol)
	}
}

func TestBuildTransportCredentials_Insecure(t *testing.T) {
	a := &agent{
		opts: agentOptions{
			insecure: true,
			logger:   nopLogger{},
		},
	}

	creds, err := a.buildTransportCredentials()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info := creds.Info()
	if info.SecurityProtocol == "tls" {
		t.Error("WithInsecure should not return TLS credentials")
	}
}

func TestBuildTransportCredentials_DefaultFallsBackToInsecure(t *testing.T) {
	a := &agent{
		opts: agentOptions{
			logger: nopLogger{},
		},
	}

	creds, err := a.buildTransportCredentials()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info := creds.Info()
	if info.SecurityProtocol == "tls" {
		t.Error("default (no TLS options) should not return TLS credentials")
	}
}

func TestBuildTransportCredentials_InsecureTakesPriorityOverSystemTLS(t *testing.T) {
	a := &agent{
		opts: agentOptions{
			insecure:  true,
			systemTLS: true,
			logger:    nopLogger{},
		},
	}

	creds, err := a.buildTransportCredentials()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info := creds.Info()
	if info.SecurityProtocol == "tls" {
		t.Error("insecure should take priority over systemTLS")
	}
}

func TestWithSystemTLS_SetsFlag(t *testing.T) {
	opts := defaultOptions()
	opts.insecure = true // simulate prior state

	WithSystemTLS()(&opts)

	if !opts.systemTLS {
		t.Error("WithSystemTLS should set systemTLS to true")
	}
	if opts.insecure {
		t.Error("WithSystemTLS should clear insecure")
	}
	if opts.tlsCertFile != "" {
		t.Error("WithSystemTLS should clear tlsCertFile")
	}
	if opts.tlsConfig != nil {
		t.Error("WithSystemTLS should clear tlsConfig")
	}
}
