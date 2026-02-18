# inGitDB Coding Standards

## Generic and common sense

- Follow standard Go idioms and formatting (`go fmt`).
- Ensure that tests cover new code for at least 90% of lines.
- Keep commit focused on a single change.
- If possible keep pull requests focused on a single change.

## Code Style

- Strictly follow standard Go idioms and formatting (`go fmt`).
- always check or explicitly ignore returned errors
- avoid calling functions inside calls of other functions (no nested calls)
- at the end always verify changes with `golangci-lint run` - should report no errors or warnings.
- Use `fmt.Fprintf` to `os.Stderr` or specific buffers instead of
  `fmt.Println` or `fmt.Printf` to avoid interfering with the TUI output on `stdout`.
- Prefer explicit error handling and avoid `panic` in production code.

## Readability / Debuggability rules

- No nested calls: don’t write `f2(f1())`; assign the intermediate result in a variable first.

## Standard for commit messages:

All commits MUST follow the Conventional Commits specification.

```text
<type>(<scope>): <short summary>
```

- The type MUST be one of: `feat, fix, docs, refactor, test, chore, ci, perf`
- The summary MUST be descriptive, concise, imperative, and written in lowercase
- The summary MUST NOT end with a period
- Commits that introduce breaking changes MUST include ! after the type or scope


## Testability

- Code should avoid the usage of package level variables
    - Dependencies should be passed to type or a func

## Tests

- We aim for 100% test coverage.
- Call `t.Parallel()` as the first statement of every top-level test.
- use `-timeout=10s` when running tests

- **Adding New Tests**:
    - Place tests in the same package as the code being tested, using the `_test.go` suffix.
    - The project uses the standard `testing` package.


### Unused arguments – explicitly mark function parameters as intentionally unused

- Assigns the parameters to the blank identifier(s) `_`
    - Prevents the Go compiler from complaining about unused variables
    - Documents that the parameters are currently unused by design

```go
package some

func foo(a1, a2 string) {
	_, _ = a1, a2
}
```

