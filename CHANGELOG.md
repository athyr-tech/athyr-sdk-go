# Changelog

All notable changes to the Athyr Go SDK will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/athyr-tech/athyr-sdk-go/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/athyr-tech/athyr-sdk-go/releases/tag/v0.1.0
