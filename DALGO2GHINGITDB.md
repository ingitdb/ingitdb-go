# dalgo2ghingitdb - GitHub DALgo Adapter

## Overview

`dalgo2ghingitdb` is a DALgo database adapter that provides read-only access to inGitDB repositories stored on GitHub. It uses the GitHub REST API to read files directly from repositories without requiring local clones.

## Features

### Current (Phase 1)
- ✅ Read-only access to public repositories
- ✅ No authentication required
- ✅ Support for both SingleRecord and MapOfIDRecords record types
- ✅ Support for YAML and JSON record formats
- ✅ Locale-aware field reading
- ✅ Proper error handling for GitHub API errors (rate limits, 404s, etc.)
- ✅ Comprehensive test coverage

### Future (Phase 2)
- 🔄 Authentication support (GitHub tokens)
- 🔄 Write operations (create, update, delete)
- 🔄 Private repository access
- 🔄 Transaction support for atomic operations

## Architecture

### Components

#### `Config` (file_reader.go)
Configuration struct for GitHub repository access:
```go
type Config struct {
    Owner      string           // GitHub organization/user
    Repo       string           // Repository name
    Ref        string           // Git reference (branch, tag, commit)
    APIBaseURL string           // Optional custom API endpoint
    HTTPClient *http.Client     // Optional custom HTTP client
}
```

#### `FileReader` (file_reader.go)
Interface for reading files from GitHub:
```go
type FileReader interface {
    ReadFile(ctx context.Context, path string) (content []byte, found bool, err error)
}
```

Implementation uses the GitHub API v72 (`go-github`) to fetch file contents.

#### `githubDB` (db_github.go)
Main DALgo database adapter implementing the `dal.DB` interface.

Constructors:
- `NewGitHubDB(cfg Config)` - Basic constructor without schema
- `NewGitHubDBWithDef(cfg Config, def *ingitdb.Definition)` - Constructor with schema (recommended)

#### `readonlyTx` (tx_readonly.go)
Read-only transaction implementation supporting:
- `Get()` - Fetch single records
- `Options()` - Transaction options

#### Helper Functions (record_content.go)
- `resolveRecordPath()` - Resolve record file paths from collection definitions
- `parseRecordContent()` - Parse YAML/JSON record content
- `parseMapOfIDRecordsContent()` - Parse map-of-records format
- `applyLocaleToRead()` - Apply locale-specific field transformations

## Usage Example

```go
package main

import (
    "context"
    "github.com/dal-go/dalgo/dal"
    "github.com/ingitdb/ingitdb-cli/pkg/dalgo2ghingitdb"
    "github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func main() {
    // Create configuration for ingitdb-cli test repository
    cfg := dalgo2ghingitdb.Config{
        Owner: "ingitdb",
        Repo:  "ingitdb-cli",
        Ref:   "main",
    }
    
    // Load schema definition
    def := &ingitdb.Definition{
        Collections: map[string]*ingitdb.CollectionDef{
            "todo.tags": {
                ID:      "todo.tags",
                DirPath: "test-ingitdb/todo/tags",
                RecordFile: &ingitdb.RecordFileDef{
                    Name:       "{key}.yaml",
                    Format:     "yaml",
                    RecordType: ingitdb.SingleRecord,
                },
            },
        },
    }
    
    // Create database adapter
    db, err := dalgo2ghingitdb.NewGitHubDBWithDef(cfg, def)
    if err != nil {
        panic(err)
    }
    
    // Read a record
    ctx := context.Background()
    key := dal.NewKeyWithID("todo.tags", "active")
    record := dal.NewRecordWithData(key, map[string]any{})
    
    err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
        return tx.Get(ctx, record)
    })
    
    if err != nil {
        panic(err)
    }
    
    // Use record data
    data := record.Data().(map[string]any)
    println(data["title"]) // Example output
}
```

## Testing

The package includes comprehensive tests using a mock HTTP server:

```bash
cd pkg/dalgo2ghingitdb
go test -v
```

Tests verify:
- Single record retrieval
- Map-of-records retrieval
- Record not found handling
- Locale-aware field reading
- GitHub API error handling

## Dependencies

- `github.com/google/go-github/v72` - GitHub REST API client
- `github.com/dal-go/dalgo` - DALgo database abstraction
- `gopkg.in/yaml.v3` - YAML parsing
- Standard library: `encoding/json`, `context`, etc.

## Error Handling

The adapter gracefully handles:
- **404 Not Found** - Returns `false` for `found` without error
- **Rate Limiting** - Returns descriptive error message
- **API Errors** - Wraps GitHub errors with context
- **Invalid Configuration** - Validates Owner and Repo at initialization

## Future Considerations for Phase 2

1. **Authentication**: Implement TokenAuth with GitHub tokens
   - Support for personal access tokens
   - Support for GitHub App tokens
   - Support for OAuth flows

2. **Write Operations**: 
   - Implement `RunReadwriteTransaction()`
   - Support for creating new records
   - Support for updating existing records
   - Support for deleting records
   - Commit message generation

3. **Advanced Features**:
   - Batch operations for efficiency
   - Caching layer for repeated reads
   - Webhook support for change notifications
   - Conflict resolution for concurrent writes

## Design Principles

1. **Consistency with dalgo2ingitdb**: Follows the same patterns and interfaces for easy switching between local and remote implementations
2. **Readonly First**: Conservative approach with clear phase separation
3. **Error Clarity**: All errors include context about what operation failed
4. **No Local State**: Stateless operations for scalability
5. **Context Awareness**: Proper context propagation for cancellation and timeouts
