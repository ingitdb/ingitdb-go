# inGitDB

[![Build, Test, Vet, Lint](https://github.com/ingitdb/ingitdb-cli/actions/workflows/golangci.yml/badge.svg)](https://github.com/ingitdb/ingitdb-cli/actions/workflows/golangci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ingitdb/ingitdb-cli)](https://goreportcard.com/report/github.com/ingitdb/ingitdb-cli)
[![Coverage Status](https://coveralls.io/repos/github/ingitdb/ingitdb-cli/badge.svg?branch=main&kill-cache=3)](https://coveralls.io/github/ingitdb/ingitdb-cli?branch=main)
[![GoDoc](https://godoc.org/github.com/ingitdb/ingitdb-cli?status.svg)](https://godoc.org/github.com/ingitdb/ingitdb-cli)
[![Version](https://img.shields.io/github/v/tag/ingitdb/ingitdb-cli?filter=v*.*.*&logo=Go)](https://github.com/ingitdb/ingitdb-cli/tags)

<img src="https://github.com/ingitdb/.github/raw/main/inGitDB-full4.png" alt="inGitDB Logo" />

inGitDB is a **developer-grade, schema-validated, AI-native database whose storage engine is a Git
repository**. Every record is a plain YAML or JSON file, every change is a commit, and every team
workflow — branching, code review, pull requests — extends naturally to data. This makes inGitDB
simultaneously a database, a version-control system, an event bus, and a data layer for AI agents,
all with zero server infrastructure for reads.

## Why inGitDB?

- **Plain files, real Git.** Records are YAML or JSON files you can read in any editor, diff in any
  pull request, and clone with a single `git clone`. No binary blobs, no proprietary format.
- **Full Git history for free.** Branching, merging, bisect, and revert work on your data exactly
  as they do on your code — because the data _is_ in your code repository.
- **Schema validation built in.** Collections are defined with typed column schemas in YAML. The
  `ingitdb validate` command checks every record and reports violations with the collection,
  file path, and field name.
- **Zero server infrastructure for reads.** There is no daemon to run. Reading data is a
  file-system operation on a git clone.
- **AI-native via MCP.** The planned MCP server (`ingitdb serve --mcp`) will expose CRUD operations
  to AI agents through the Model Context Protocol — no custom integration required (Phase 6).
- **Go library via DALgo.** `pkg/dalgo2ingitdb` implements the [DALgo](https://github.com/dal-go/dalgo)
  `dal.DB` interface, so any Go program can use inGitDB as a standard database abstraction.

## How it works

```mermaid
flowchart LR
    A([YAML / JSON\nrecord files]) --> B[(Git repository\non disk)]
    B --> C[ingitdb validate\nschema + data check]
    C --> D[Views Builder\ngenerates $views/]
    D --> E([CLI output\ningitdb query])
    D --> F([Go programs\nDALgo API])
    D --> G([AI agents\nMCP server — Phase 6])
```

The `ingitdb validate` command reads `.ingitdb.yaml`, checks every record against its collection
schema, and rebuilds materialized views in the same pass. Validation can be scoped to a commit
range (`--from-commit` / `--to-commit`) so CI stays fast on large databases.

## Quick start

```shell
# Build the CLI
go build -o ingitdb ./cmd/ingitdb

# Validate a database directory (defaults to current working directory)
ingitdb validate

# Validate a specific path
ingitdb validate --path=/path/to/your/db

# Validate only records changed between two commits (fast CI mode)
ingitdb validate --from-commit=abc1234 --to-commit=def5678

# List all collections
ingitdb list collections

# List collections nested under a path (regular expression)
ingitdb list collections --in='countries/(ie|gb)'

# List collections whose name contains "city"
ingitdb list collections --filter-name='*city*'

# Search records for a substring across all fields
ingitdb find --substr=Dublin

# Search records using a regular expression, limited to 10 results
ingitdb find --re='pop.*[0-9]{6,}' --limit=10

# Search only specific fields
ingitdb find --substr=Dublin --fields=name,capital

# Scope a search to a sub-path
ingitdb find --exact=Ireland --in='countries/.*' --fields=country

# Delete all records from a collection (keeps the schema)
ingitdb truncate --collection=countries/ie/counties

# Delete a specific collection and all its records
ingitdb delete collection --collection=countries/ie/counties/dublin

# Delete records matching a pattern within a collection
ingitdb delete records --collection=countries/ie/counties --filter-name='*old*'

# --- Record CRUD (requires record_file.type: "map[string]any" collections) ---

# Create a new record: the --id format is <collection-id-with-slashes>/<record-key>
# (collection IDs use "." in the definition, but "/" in --id flags)
ingitdb create --path=. --id=geo/nations/ie --data='{title: "Ireland"}'

# Read a record (output format: yaml or json)
ingitdb read   --path=. --id=geo/nations/ie
ingitdb read   --path=. --id=geo/nations/ie --format=json

# Update fields of an existing record (patch semantics: only listed fields change)
ingitdb update --path=. --id=geo/nations/ie --set='{title: "Ireland, Republic of"}'

# Delete a single record
ingitdb delete record --path=. --id=geo/nations/ie
```

A minimal `.ingitdb.yaml` at the root of your DB git repository:

```yaml
rootCollections:
  tasks: data/tasks/*   # each subdirectory becomes a collection
languages:
  - required: en
```

## Commands

| Command | Status | Description |
|---|---|---|
| `version` | implemented | Print build version, commit hash, and date |
| `validate` | implemented | Check every record against its collection schema |
| `list collections\|view\|subscribers` | planned | List schema objects, scoped with `--in` and `--filter-name` |
| `find` | planned | Search records by substring, regex, or exact value |
| `create` | implemented | Create a new record (`map[string]any` collections only) |
| `read` | implemented | Read a single record by ID |
| `update` | implemented | Update fields of an existing record |
| `delete collection\|view\|records` | planned | Remove a collection, view definition, or individual records |
| `delete record` | implemented | Delete a single record by ID |
| `truncate` | planned | Remove all records from a collection, keeping its schema |
| `query` | planned | Query and format records from a collection |
| `materialize` | planned | Build materialized views into `$views/` |
| `pull` | planned | Pull remote changes and rebuild views |
| `watch` | planned | Stream record change events to stdout |
| `serve` | planned | Start MCP, HTTP API, or file-watcher server |
| `resolve` | planned | Interactive TUI for resolving data-file merge conflicts |
| `setup` | planned | Initialise a new database directory |
| `migrate` | planned | Migrate records between schema versions |

### --id format

The `--id` flag uses `/` as separator for both the collection path and the record key:

```
<collection-path-with-slashes>/<record-key>
```

Collection IDs in the definition use `.` as separator (e.g. `geo.nations`), but the `--id` flag uses
`/` (e.g. `--id=geo/nations/ie`). The longest matching collection prefix wins when ambiguous.

Only collections with `record_file.type: "map[string]any"` support CRUD. Collections using
`[]map[string]any` (list) or `map[string]map[string]any` (dictionary) are not yet implemented.

See the [CLI reference](docs/CLI.md) for flags and examples.

## Documentation

| Document | Description |
|---|---|
| [Documentation](docs/README.md) | Full docs index — start here |
| [CLI reference](docs/CLI.md) | Every subcommand, flag, and exit code |
| [Features](docs/features/README.md) | What inGitDB can do today and what is coming |
| [Architecture](docs/ARCHITECTURE.md) | Data model, package map, and key design decisions |
| [Roadmap](docs/ROADMAP.md) | Nine delivery phases from Validator to GraphQL |
| [Contributing](docs/CONTRIBUTING.md) | How to open issues and submit pull requests |
| [Competitors](docs/COMPETITORS.md) | Honest feature comparison with related tools |

## Get involved

inGitDB is small enough that every contribution makes a visible difference. The best way to start
is to point the CLI at a directory of YAML files and run `ingitdb validate`, then check the
[Roadmap](docs/ROADMAP.md) to see what is being built next.

To contribute:

1. Read [CONTRIBUTING.md](docs/CONTRIBUTING.md) for the pull-request workflow.
2. Browse [open issues](https://github.com/ingitdb/ingitdb-cli/issues) to find something to work on.
3. Open or comment on an issue before investing time in a large change.

Bug reports, documentation improvements, and questions are all welcome.

## Dependencies

- [DALgo](https://github.com/dal-go/dalgo) — Database Abstraction Layer for Go

## License

This project is free, open source and licensed under the MIT License. See the [LICENSE](LICENSE)
file for details.
