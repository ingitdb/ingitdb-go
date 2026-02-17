# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build -o ingitdb ./cmd/ingitdb

# Run all tests
go test -timeout=10s ./...

# Run a single test
go test -timeout=10s -run TestName ./path/to/package

# Test coverage
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out

# Lint (must report no errors before committing)
golangci-lint run
```

## Architecture

**inGitDB** stores database records as YAML/JSON files in a Git repository. Collections, schemas, views, and materialized views are defined declaratively in `.ingitdb.yaml` configuration.

The codebase has two main packages:

- **`pkg/ingitdb/`** — Core schema definitions (`Definition`, `CollectionDef`, `ColumnDef`, views) and the `validator/` sub-package that reads and validates a database directory against its schema.
- **`pkg/dalgo2ingitdb/`** — DALgo (Database Abstraction Layer) integration, implementing `dal.DB`, read-only and read-write transactions for CRUD access.
- **`cmd/ingitdb/`** — CLI entry point using `github.com/urfave/cli/v3` for subcommand and flag parsing. The `run()` function is dependency-injected for testability (accepts `homeDir`, `readDefinition`, `fatal`, `logf` as parameters).
- **`cmd/watcher/`** — Obsolete file watcher, to be folded into `ingitdb watch`.

Test data lives in `test-ingitdb/` and `.ingitdb.yaml` at the repo root points to it.

## Code Conventions

- **No nested calls**: never write `f2(f1())`; assign the intermediate result first.
- **Errors**: always check or explicitly ignore returned errors. Avoid `panic` in production code.
- **Output**: use `fmt.Fprintf(os.Stderr, ...)` — never `fmt.Println`/`fmt.Printf` — to avoid interfering with TUI stdout.
- **Unused params**: mark intentionally unused function parameters with `_, _ = a1, a2`.
- **No package-level variables**: pass dependencies via struct fields or function parameters.
- **Tests**: call `t.Parallel()` as the first statement in every top-level test.

## Commit Messages

All commits must follow Conventional Commits:

```
<type>(<scope>): <short summary>
```

Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`, `perf`. Summary must be lowercase, imperative, and not end with a period. Use `!` after type/scope for breaking changes.
