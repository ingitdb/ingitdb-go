# inGitDB Development Guidelines for AI Agents & Humans

This document provides project-specific information for developers and AI agents working on inGitDB.

Refer to [documentation](.) to learn about architecture, features, components, etc. of the `ingitdb` CLI.

## 1. Build and Configuration

- **Go Version**: Ensure you are using the Go version specified in `go.mod` (or later).
- **Dependencies**: Managed via Go modules. Run `go mod download` to fetch them.
- **Main Entry Point**: The CLI entry point is `cmd/ingitdb/main.go`.
- **Build Command**:
  ```shell
  go build -o ingitdb ./cmd/ingitdb
  ```
- **Running Locally**:
  ```shell
  go run ./cmd/ingitdb
  ```

## 2. Development Information

- All code must follow our [coding standards](CODING_STANDARDS.md).
- See [Architecture](ARCHITECTURE.md) for the package map and design decisions.

## 3. Testing Information

inGitDB aims for 100% test coverage.

Our [coding standards](CODING_STANDARDS.md) have a dedicated "Tests" section.

- **Running All Tests**:
  ```shell
  go test -timeout=10s ./...
  ```
- **Lint** (must report no errors before committing):
  ```shell
  golangci-lint run
  ```
- **Coverage Analysis**:
  ```shell
  go test -coverprofile=coverage.out ./...
  go tool cover -html=coverage.out
  ```
