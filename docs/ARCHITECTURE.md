# Architecture of ingitdb CLI

## Overview

inGitDB is a database system where a Git repository is the datastore. Records are YAML or JSON files; schema is declared in YAML config files placed alongside the data. The `ingitdb` CLI reads, validates, queries, and maintains this data.

## Data Model

A database is a directory tree inside a Git repository:

```
<db-root>/
├── .ingitdb.yaml                          # DB-level config (collections, languages)
└── <group>/
    └── <collection>/
        ├── .ingitdb-collection.yaml       # Collection schema
        ├── $records/
        │   └── <record-id>.json           # Individual record files (JSON or YAML)
        ├── .ingitdb-view.<name>.yaml      # View definitions
        └── $views/
            └── <view-name>/
                └── <partition>.md         # Materialized view output files
```

**`.ingitdb.yaml`** — DB-level config: maps collection keys to filesystem paths and declares supported languages.

```yaml
rootCollections:
  todo: test-ingitdb/todo/*   # wildcard: each subdir becomes a collection
  countries: geo/countries    # explicit single collection path
languages:
  - required: en
  - required: fr
  - optional: ru
```

**`.ingitdb-collection.yaml`** — Collection schema: titles (i18n), column definitions, record file format.

```yaml
titles:
  en: Tasks
data_dir: $records
record_file:
  name: "$records/{key}.json"
  type: "[]map[string]any"   # or "map[string]any" for single-record files
  format: json               # or yaml
columns:
  title:
    type: string
    required: true
    max_length: 100
    titles:
      en: Task title
  status:
    type: string
    required: true
    foreign_key: statuses    # value must be a valid record ID in the 'statuses' collection
```

**Column types:** `string`, `int`, `float`, `bool`, `date`, `time`, `datetime`, `map[locale]string`, `any`

**Record files** live in the collection's `data_dir`. A file holds either one record (`map[string]any`) or an array of records (`[]map[string]any`), as declared in `record_file.type`.

**View definitions** (`.ingitdb-view.<name>.yaml`) declare how to partition and render records into materialized view files under `$views/`.

## Component Architecture

```
CLI (cmd/ingitdb)
    │
    ├── validate [--path] [--from-commit] [--to-commit]
    │       └── validator.ReadDefinition()
    │               ├── config.ReadRootConfigFromFile()     reads .ingitdb.yaml
    │               ├── readCollectionDef() × N             reads .ingitdb-collection.yaml per collection
    │               └── colDef.Validate()                   validates schema structure
    │               └── [TODO] DataValidator                walks $records/, validates records against schema
    │
    ├── query --collection [--path] [--format]
    │       └── [TODO] Query engine                         reads and filters records, formats output
    │
    ├── materialize [--path] [--views]
    │       └── [TODO] Views Builder                        reads view defs, generates $views/ output
    │
    └── [TODO] Subscribers/Triggers
            └── dispatches events (webhook, email, shell) on data changes
```

The **Scanner** (see `docs/components/scanner.md`) orchestrates the full pipeline: it walks the filesystem and invokes the Validator and Views Builder in sequence.

## Package Map

| Package | Responsibility |
|---|---|
| `pkg/ingitdb` | Domain types only: `Definition`, `CollectionDef`, `ColumnDef`, `ViewDef`, etc. No I/O. |
| `pkg/ingitdb/config` | Reads `.ingitdb.yaml` (root config) and `~/.ingitdb/.ingitdb-user.yaml` (user config). |
| `pkg/ingitdb/validator` | Reads and validates collection schemas. Entry point: `ReadDefinition()`. |
| `pkg/dalgo2ingitdb` | DALgo integration: implements `dal.DB`, read-only and read-write transactions. |
| `cmd/ingitdb` | CLI entry point. `run()` is fully dependency-injected for testability. |

## Key Design Decisions

**Subcommand-based CLI.** Commands are subcommands (`validate`, `query`, `materialize`) with their own flags, implemented using `github.com/urfave/cli/v3`. `--path` is a per-subcommand flag defaulting to the current working directory. `--version` is handled by the framework. See `docs/CLI.md` for the full interface spec.

**Stdout reserved for data output.** All diagnostic output (logs, errors) goes to `os.Stderr`. Stdout carries only structured data (query results) or TUI output. This allows piping without mixing logs into results.

**Dependency injection in `run()`.** `homeDir`, `readDefinition`, `fatal`, and `logf` are injected as function parameters, making the CLI fully unit-testable without real I/O.

**DALgo abstraction.** `pkg/dalgo2ingitdb` implements the DALgo `dal.DB` interface so consumers can work with inGitDB through a standard Go database abstraction, decoupled from the file-based storage format.

**Two validation modes.** Full validation scans the entire DB. Change validation validates only the files changed between two git commits — essential for keeping CI fast on large databases.

**No package-level variables.** All dependencies are passed via struct fields or function parameters to keep code testable and avoid global state.
