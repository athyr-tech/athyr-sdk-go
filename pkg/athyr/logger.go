package athyr

// Logger defines a minimal logging interface compatible with slog.Logger.
// This allows plugging in any logging framework (slog, zap, zerolog, etc.).
//
// Example with slog:
//
//	agent, _ := athyr.NewAgent("localhost:9090",
//	    athyr.WithLogger(slog.Default()),
//	)
//
// Example with zap:
//
//	zapLogger, _ := zap.NewProduction()
//	agent, _ := athyr.NewAgent("localhost:9090",
//	    athyr.WithLogger(zapLogger.Sugar()),
//	)
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// nopLogger is the default logger that discards all output.
type nopLogger struct{}

func (nopLogger) Debug(string, ...any) {}
func (nopLogger) Info(string, ...any)  {}
func (nopLogger) Warn(string, ...any)  {}
func (nopLogger) Error(string, ...any) {}
