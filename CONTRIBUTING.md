# Contributing to Cilo

Thank you for your interest in contributing to Cilo! We welcome contributions from the community to help make Cilo the best tool for isolated workspace environments.

## Development Environment Setup

### Prerequisites

- **Go:** 1.24 or newer
- **Docker:** Engine and Compose plugin installed
- **dnsmasq:** Required for local DNS resolution
- **Operating System:** Linux (systemd-resolved supported) or macOS

### Building from Source

```bash
git clone https://github.com/sharedco/cilo.git
cd cilo/cilo
go build -o cilo main.go
```

## Running Tests

Cilo's test suite includes unit tests and integration tests. Some integration tests require `sudo` to configure system DNS.

### Unit Tests

```bash
go test ./pkg/...
```

### Integration Tests

```bash
# Some tests may require sudo for DNS setup
go test ./tests/e2e/...
```

## Contribution Workflow

1.  **Fork the repository** on GitHub.
2.  **Create a feature branch** (`git checkout -b feature/my-new-feature`).
3.  **Make your changes** and ensure code adheres to the project's style.
4.  **Add tests** for any new functionality or bug fixes.
5.  **Run the full test suite** to ensure no regressions.
6.  **Commit your changes** with clear, descriptive commit messages.
7.  **Push to your fork** and **submit a Pull Request**.

## Coding Standards

- Follow idiomatic Go patterns.
- Ensure `go fmt`, `go vet`, and `golangci-lint` pass.
- Maintain the "Absolute Paths Only" principle for file operations.
- Use the `internal/` package for logic that should not be exposed to external users.

## Documentation

- Update the `docs/` directory for any architectural changes.
- Ensure the `README.md` reflects current features and usage.
- Add examples to the `examples/` directory for new use cases.

## Community

- Join our discussions on [GitHub Discussions](https://github.com/sharedco/cilo/discussions).
- Adhere to our [Code of Conduct](CODE_OF_CONDUCT.md).
