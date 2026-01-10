# Contributing to Athyr SDK for Go

Thank you for your interest in contributing! This document provides guidelines for contributing to the project.

## Development Setup

```bash
# Clone the repository
git clone https://github.com/athyr-tech/athyr-sdk-go.git
cd athyr-sdk-go

# Verify everything builds
go build ./...

# Run tests
go test ./...
```

## Code Style

- Follow standard Go conventions ([Effective Go](https://go.dev/doc/effective_go))
- Run `go fmt` before committing
- Run `go vet` to catch common issues
- All exported types and functions must have GoDoc comments

## Testing

### Unit Tests
```bash
go test ./pkg/athyr/...
```

### Integration Tests
Integration tests require a running Athyr server:
```bash
go test -tags=integration ./pkg/athyr/...
```

### Benchmarks
```bash
go test -bench=. -benchmem ./pkg/athyr/...
```

## Pull Request Process

1. **Fork** the repository and create your branch from `main`
2. **Write tests** for any new functionality
3. **Update documentation** if you're changing public APIs
4. **Run the full test suite** to ensure nothing is broken
5. **Keep commits focused** - one logical change per commit
6. **Write clear commit messages** following conventional commits:
   - `feat(athyr): add new feature`
   - `fix(athyr): resolve issue with X`
   - `docs: update README`
   - `test: add integration tests`
   - `chore: update dependencies`

## Reporting Issues

When reporting issues, please include:
- Go version (`go version`)
- SDK version
- Minimal reproduction steps
- Expected vs actual behavior

## Questions?

Open a [GitHub Issue](https://github.com/athyr-tech/athyr-sdk-go/issues) for questions or discussions.
