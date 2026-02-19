# Testing Seams for Commands Package

This document explains the testing seams pattern used in the `cmd/ingitdb/commands` package.

## Overview

The commands package uses **package-level variables** as seams to external dependencies. This allows tests to inject mock implementations without requiring dependency injection through function parameters.

## Key Components

### 1. Interfaces (seams.go)

Three interfaces define the boundaries to external packages:

- **`GitHubDBFactory`**: Creates GitHub-backed DAL databases
- **`GitHubFileReaderFactory`**: Creates GitHub file readers
- **`ViewBuilderFactory`**: Creates view builders for collections

### 2. Package-Level Variables (seams.go)

These variables hold the factory implementations:

```go
var (
    gitHubDBFactory         GitHubDBFactory
    gitHubFileReaderFactory GitHubFileReaderFactory
    viewBuilderFactory      ViewBuilderFactory
)
```

By default, these point to real implementations. Tests can replace them with mocks.

### 3. Mock Implementations (mocks_test.go)

Generated using `go.uber.org/mock/mockgen`:

```bash
mockgen -source=cmd/ingitdb/commands/seams.go \
        -destination=cmd/ingitdb/commands/mocks_test.go \
        -package=commands
```

## Usage in Tests

### Important Rules

**Tests that replace package-level variables MUST NOT run in parallel.**

When a test modifies package-level variables:
1. **DO NOT** call `t.Parallel()`
2. Save the original value
3. Replace with mock
4. Defer restoration of original value

### Example: Testing with Mocks

```go
func TestSomething_WithMock(t *testing.T) {
    // NOTE: No t.Parallel() call here!
    
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    // Save and restore original
    original := gitHubDBFactory
    defer func() {
        gitHubDBFactory = original
    }()

    // Install mock
    mockFactory := NewMockGitHubDBFactory(ctrl)
    gitHubDBFactory = mockFactory

    // Set expectations
    mockFactory.EXPECT().
        NewGitHubDBWithDef(gomock.Any(), gomock.Any()).
        Return(nil, errors.New("test error"))

    // Run test code that uses gitHubDBFactory
    // ...
}
```

### Example: Testing without Mocks

Tests that don't modify package-level variables CAN run in parallel:

```go
func TestSomething_WithRealImplementation(t *testing.T) {
    t.Parallel() // OK - not modifying package variables

    // Test code using real implementations
    // ...
}
```

## Regenerating Mocks

When interfaces in `seams.go` change, regenerate mocks:

```bash
go install go.uber.org/mock/mockgen@latest
mockgen -source=cmd/ingitdb/commands/seams.go \
        -destination=cmd/ingitdb/commands/mocks_test.go \
        -package=commands
```

## Design Rationale

### Why Package-Level Variables?

1. **Minimal API Surface**: Commands already accept many function parameters (homeDir, getWd, readDefinition, newDB, logf). Adding more would make the API unwieldy.

2. **Focused Injection Points**: Only GitHub and view-related operations need mocking. Most tests use real local filesystem operations.

3. **Non-Invasive**: Production code doesn't need to know about test infrastructure.

### Why Not Constructor Injection?

The command factory functions (`Create()`, `Read()`, etc.) already have 5-6 parameters. Adding factory parameters for rarely-mocked dependencies would:
- Make the API harder to use
- Require changes throughout the codebase
- Not improve testability for the common case

### When to Use This Pattern

Use package-level variable seams when:
- The dependency is external (network, GitHub API)
- The dependency is rarely mocked (most tests use real implementations)
- Adding it as a parameter would clutter the API
- The code follows the "no package-level variables" convention elsewhere

Do NOT use for:
- Core dependencies that every test needs to mock
- Dependencies that change frequently
- Code that benefits from constructor injection

## See Also

- `seams.go` - Interface definitions and default implementations
- `mocks_test.go` - Generated mock implementations (auto-generated, DO NOT EDIT)
- `seams_test.go` - Example tests demonstrating the pattern
