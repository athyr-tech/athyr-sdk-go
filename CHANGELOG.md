# Changelog

All notable changes to the Athyr Go SDK will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-03-21

### Added

- Agent discovery: `ListAgents` and `GetAgent` methods with `AgentInfo` type
- Durable streaming: `ResumeStream` on Agent interface, `Durable` field on `CompletionRequest`, `Sequence`/`StreamInfo` on `StreamChunk`, `DurableStreamInfo` type
- `pkg/athyrtest` package with exported `MockAgent` (function-fields pattern) so users don't need to reimplement the Agent interface for testing
- `MaxConcurrency` middleware (replaces `RateLimit` which was a misnomer)
- Typed generic orchestration wrappers: `ExecutePipeline`, `ExecuteFanOut`, `HandleHandoff`, `DiscussGroupChat`
- `WithServerSystemTLS` server option for parity with agent-side `WithSystemTLS`
- `IsNotConnected` and `IsAlreadyConnected` error helper functions

### Fixed

- `WithSystemTLS` silently falling back to insecure credentials
- `marshalResults` producing invalid JSON when agent output contains quotes or backslashes
- `isRetryable` only checking `ServiceError`, missing `AthyrError` from gRPC-originated transient failures
- `Request()` not enforcing `requestTimeout` client-side (only sent as server metadata, calls could hang indefinitely)
- Misleading `WithTLS` doc about empty certFile behavior

### Changed

- **BREAKING:** Agent interface adds `ResumeStream` method — implementations must add this method
- **BREAKING:** `LogRequests()` and `Recover()` now accept `Logger` interface instead of `*log.Logger`
- **BREAKING:** `Run()` and `RunRaw()` now take `[]ServerOption` before variadic `ServiceOption`
- **BREAKING:** `HandleRaw` is now a package-level function for consistency with `Handle`
- `ErrNotConnected`/`ErrAlreadyConnected` are now `*AthyrError` sentinels (were plain errors)
- KV `List` parameter renamed from `prefix` to `pattern` with updated godoc

### Deprecated

- `RateLimit` middleware — use `MaxConcurrency` instead (it's a semaphore, not a rate limiter)

## [0.1.0] - 2025-01-10

### Added

- Agent lifecycle management (Connect, Disconnect, Heartbeat)
- LLM completions (blocking and streaming)
- Memory sessions with auto-summarization and hints
- Key-value storage operations
- Pub/sub messaging (Publish, Subscribe, QueueSubscribe)
- Request/response patterns with timeouts
- Tool calling support for LLM function calls
- Orchestration patterns: Pipeline, FanOut, Handoff, GroupChat
- Middleware chain: Logger, Recover, Timeout, Retry, RateLimit, Metrics, Validate
- Server pattern for building agent services
- Auto-reconnect with exponential backoff
- Structured error types with gRPC wrapping
- Example implementations for all orchestration patterns

[0.2.0]: https://github.com/athyr-tech/athyr-sdk-go/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/athyr-tech/athyr-sdk-go/releases/tag/v0.1.0
