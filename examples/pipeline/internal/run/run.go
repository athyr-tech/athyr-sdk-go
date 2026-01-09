// Package run provides helpers for running pipeline agents.
package run

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// UntilSignal runs fn until SIGINT/SIGTERM is received.
func UntilSignal(fn func(ctx context.Context) error) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("shutting down...")
		cancel()
	}()

	return fn(ctx)
}

// Log prints a formatted log message with the agent name prefix.
func Log(agent, format string, args ...any) {
	fmt.Printf(agent+": "+format+"\n", args...)
}

// Truncate shortens text to maxLen characters.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}