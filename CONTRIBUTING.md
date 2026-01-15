# Contributing to Ravenforge

Thank you for your interest in contributing to Ravenforge! This document provides guidelines and information for contributors.

## Code of Conduct

By participating in this project, you agree to maintain a respectful and inclusive environment for everyone.

## Getting Started

### Prerequisites

- Go 1.22 or later
- Docker 24.0 or later
- Make
- Linux environment (Ubuntu 22.04+ recommended)

### Development Setup

```bash
# Clone the repository
git clone https://github.com/ravenforge/ravenforge.git
cd ravenforge

# Build everything
make build

# Run tests
make test

# Start development environment
make up
```

## Architecture Overview

Ravenforge follows a strict tool-based architecture:

- **Core**: Minimal runtime for orchestration, sandboxing, policy enforcement, and auditing
- **Tools**: All SOC capabilities are implemented as separate, pluggable tools
- **SDK**: Libraries and templates for tool development

### Key Principles

1. **No monoliths**: Features belong in tools, not the core
2. **Security by default**: Tools are sandboxed, network-disabled, and policy-gated
3. **Full auditability**: Every action is logged immutably
4. **Linux-first**: Primary support is for Linux environments

## Making Contributions

### Types of Contributions

1. **Core improvements**: Enhancements to the runtime, API, or CLI
2. **New tools**: Adding SOC capabilities as tools
3. **SDK enhancements**: Improving developer experience
4. **Documentation**: Improving docs, examples, tutorials
5. **Tests**: Adding test coverage and scenarios

### Contribution Workflow

1. **Fork the repository** and create a feature branch
2. **Make your changes** following our coding standards
3. **Write tests** for new functionality
4. **Update documentation** if needed
5. **Submit a pull request** with a clear description

### Commit Messages

Follow conventional commits:

```
type(scope): description

[optional body]

[optional footer]
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`

### Code Standards

#### Go Code

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt` and `golint`
- Add structured logging with zap
- Write table-driven tests
- Document exported functions

#### Tool Development

- Follow the tool manifest specification
- Request minimal capabilities
- Handle errors gracefully
- Emit structured JSON logs
- Include comprehensive tests

### Pull Request Process

1. Ensure all tests pass (`make test`)
2. Ensure linting passes (`make lint`)
3. Update CHANGELOG.md if applicable
4. Request review from maintainers
5. Address review feedback
6. Squash commits before merge

## Tool Development Guide

### Creating a New Tool

1. Use the scaffold:
   ```bash
   ravenforge tool scaffold --name my-tool --runtime oci
   ```

2. Implement your tool following the SDK contract

3. Create a valid `tool.yaml` manifest

4. Build and test locally:
   ```bash
   cd tools/category/my-tool
   make build
   make test
   ```

5. Register with the daemon:
   ```bash
   ravenforge tool register ./tools/category/my-tool
   ```

### Tool Requirements

- **Manifest compliance**: Valid `tool.yaml` with all required fields
- **Input/output schemas**: JSON Schema definitions for all artifacts
- **Structured logging**: JSON lines to stdout
- **Deterministic behavior**: Same inputs should produce same outputs
- **Tests**: Unit and integration tests required

### Security Review

Tools requesting sensitive capabilities undergo additional review:

- `network: true` - Network access justification required
- `uses_ai: true` - AI/ML usage documentation required
- `response_action: true` - Action safety analysis required
- `secrets` - Secret usage justification required

## Testing

### Running Tests

```bash
# All tests
make test

# Unit tests only
make test-unit

# Integration tests (requires Docker)
make test-integration

# Specific scenario
make scenario-ssh
```

### Writing Tests

- Unit tests go in `*_test.go` files
- Integration tests go in `tests/integration/`
- Scenario fixtures go in `examples/datasets/`

## Documentation

### Updating Documentation

- API changes: Update OpenAPI spec in `sdk/spec/openapi.yaml`
- Architecture changes: Update `docs/architecture.md`
- New features: Add to relevant docs
- Tools: Include README.md with each tool

### Documentation Standards

- Use clear, concise language
- Include code examples
- Provide diagrams for complex concepts
- Keep examples working and tested

## Release Process

1. Version follows semantic versioning
2. Releases are tagged in git
3. Binaries are built for Linux (amd64, arm64)
4. Container images are pushed to registry
5. Changelog is updated

## Getting Help

- **Issues**: GitHub Issues for bugs and feature requests
- **Discussions**: GitHub Discussions for questions
- **Security**: security@ravenforge.dev for vulnerabilities

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.
