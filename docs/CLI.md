# ingitdb Command Line Interface

`--path` defaults to the current working directory when omitted. `~` in paths is expanded to the user's home directory.

## Global flags

| Flag | Description |
|------|-------------|
| `--help`, `-h` | Print usage information and exit. |

## Commands

Each command is implemented in its own file under `cmd/ingitdb/commands/`
(e.g. `validate.go`, `list.go`). `cmd/ingitdb/main.go` assembles them and
injects process-level dependencies. See [Architecture](ARCHITECTURE.md) for details.

### `version` — print build information

```
ingitdb version
```

Prints build version, commit hash, and build date.

---

### `validate` — validate database schema and data

```
ingitdb validate [--path=PATH] [--from-commit=SHA] [--to-commit=SHA]
```

| Flag | Description |
|------|-------------|
| `--path=PATH` | Path to the database directory. Defaults to the current working directory. |
| `--from-commit=SHA` | Validate only records changed since this commit. |
| `--to-commit=SHA` | Validate only records up to this commit. |

Reads `.ingitdb.yaml`, checks that every record file matches its collection schema, and reports any violations to stderr. With `--from-commit` / `--to-commit`, only files changed in that commit range are checked (see [Validator docs](components/validator/README.md)).

Exit code is `0` on success, non-zero on any validation error.

**Examples:**

```shell
# Validate the current directory
ingitdb validate

# Validate a specific path
ingitdb validate --path=/path/to/your/db

# Fast CI mode: validate only records changed between two commits
ingitdb validate --from-commit=abc1234 --to-commit=def5678
```

---

### `query` — query records from a collection *(not yet implemented)*

```
ingitdb query --collection=KEY [--path=PATH] [--format=json|yaml]
```

| Flag | Required | Description |
|------|----------|-------------|
| `--collection=KEY` | yes | Key of the collection to query. |
| `--path=PATH` | no | Path to the database directory. Defaults to the current working directory. |
| `--format=FORMAT` | no | Output format: `json` (default) or `yaml`. |

**Examples:**

```shell
# Query all records from a collection (JSON output)
ingitdb query --collection=countries/ie/counties

# Query with YAML output
ingitdb query --collection=tasks --format=yaml

# Query from a specific database path
ingitdb query --collection=users --path=/var/db/myapp
```

---

### `materialize` — build materialized views *(not yet implemented)*

```
ingitdb materialize [--path=PATH] [--views=VIEW1,VIEW2,...]
```

| Flag | Description |
|------|-------------|
| `--path=PATH` | Path to the database directory. Defaults to the current working directory. |
| `--views=LIST` | Comma-separated list of view names to materialize. Without this flag, all views are materialized. |

Output is written into the `$views/` directory defined in `.ingitdb.yaml`.

**Examples:**

```shell
# Rebuild all views
ingitdb materialize

# Rebuild specific views only
ingitdb materialize --views=by_status,by_assignee

# Rebuild views for a database at a specific path
ingitdb materialize --path=/var/db/myapp
```

---

### `pull` — pull latest changes, resolve conflicts, and rebuild views *(not yet implemented)*

```
ingitdb pull [--path=PATH] [--strategy=rebase|merge] [--remote=REMOTE] [--branch=BRANCH]
```

| Flag | Description |
|------|-------------|
| `--path=PATH` | Path to the database directory. Defaults to the current working directory. |
| `--strategy=rebase\|merge` | Git pull strategy. Default: `rebase`. |
| `--remote=REMOTE` | Remote to pull from. Default: `origin`. |
| `--branch=BRANCH` | Branch to pull. Default: the current branch's tracking branch. |

Performs a complete pull cycle in one command:

1. `git pull --rebase` (or `--merge`) from the specified remote and branch.
2. Auto-resolves any conflicts in generated files (materialized views, `README.md`) by regenerating them.
3. Opens an interactive TUI for any conflicts in source data files that require a human decision.
4. Rebuilds materialized views and `README.md` if new changes require it.
5. Prints a summary of records added, updated, and deleted by the pull.

Exits `0` if all conflicts were resolved and views rebuilt successfully. Exits `1` if unresolved conflicts remain after interactive resolution. Exits `2` on infrastructure errors (git not found, network failure, bad flags).

**Examples:**

```shell
# Pull from origin using the default rebase strategy
ingitdb pull

# Pull using merge instead of rebase
ingitdb pull --strategy=merge

# Pull from a specific remote and branch
ingitdb pull --remote=upstream --branch=main
```

---

### `setup` — initialise a new database directory *(not yet implemented)*

```
ingitdb setup [--path=PATH]
```

| Flag | Description |
|------|-------------|
| `--path=PATH` | Path to the directory to initialise. Defaults to the current working directory. |

**Examples:**

```shell
# Initialise a database in the current directory
ingitdb setup

# Initialise a database at a specific path
ingitdb setup --path=/var/db/myapp
```

---

### `resolve` — resolve merge conflicts in database files *(not yet implemented)*

```
ingitdb resolve [--path=PATH] [--file=FILE]
```

| Flag | Description |
|------|-------------|
| `--path=PATH` | Path to the database directory. Defaults to the current working directory. |
| `--file=FILE` | Specific conflict file to resolve. Without this flag, all conflicted files are processed. |

**Examples:**

```shell
# Interactively resolve all conflicted files
ingitdb resolve

# Resolve a single conflicted file
ingitdb resolve --file=countries/ie/counties/dublin.yaml
```

---

### `watch` — watch database for changes *(not yet implemented)*

```
ingitdb watch [--path=PATH] [--format=text|json]
```

| Flag | Description |
|------|-------------|
| `--path=PATH` | Path to the database directory. Defaults to the current working directory. |
| `--format=FORMAT` | Output format: `text` (default) or `json`. |

Watches the database directory for file-system changes and writes a structured event to **stdout** for every record that is added, updated, or deleted. Runs in the foreground until interrupted.

**Examples:**

```shell
# Watch the current directory, text output
ingitdb watch

# Watch a specific database path with JSON output (pipe-friendly)
ingitdb watch --path=/var/db/myapp --format=json
```

**Text output example:**

```
Record /countries/gb/cities/london: added
Record /countries/gb/cities/london: 2 fields updated: {population: 9000000, area: 1572}
Record /countries/gb/cities/london: deleted
```

**JSON output example (`--format=json`):**

```json
{"type":"added","record":"/countries/gb/cities/london"}
{"type":"updated","record":"/countries/gb/cities/london","fields":{"population":9000000,"area":1572}}
{"type":"deleted","record":"/countries/gb/cities/london"}
```

---

### `serve` — start one or more servers *(not yet implemented)*

```
ingitdb serve [--path=PATH] [--mcp] [--http] [--watcher]
```

| Flag | Description |
|------|-------------|
| `--path=PATH` | Path to the database directory. Defaults to the current working directory. |
| `--mcp` | Enable the MCP (Model Context Protocol) server. |
| `--http` | Enable the HTTP API server. |
| `--watcher` | Enable the file watcher. |

At least one service flag must be provided. Multiple flags may be combined to run services together in a single process.

**Examples:**

```shell
# Start the MCP server for AI agent access
ingitdb serve --mcp

# Start the HTTP API server
ingitdb serve --http

# Start MCP and the file watcher together in one process
ingitdb serve --mcp --watcher

# Start all services for a specific database path
ingitdb serve --mcp --http --watcher --path=/var/db/myapp
```

---

### `list` — list database objects *(not yet implemented)*

Top-level command with three subcommands. Shared flags on every subcommand:

| Flag | Description |
|------|-------------|
| `--path=PATH` | Path to the database directory. Defaults to the current working directory. |
| `--in=REGEXP` | Regular expression that matches the starting-point path. Only objects under matching paths are listed. |
| `--filter-name=PATTERN` | Glob-style pattern to filter by name (e.g. `*substr*`). |

#### `list collections`

```
ingitdb list collections [--path=PATH] [--in=REGEXP] [--filter-name=PATTERN]
```

Lists all collection paths defined in the database.

**Examples:**

```shell
# List all collections
ingitdb list collections

# List collections nested under a matching path
ingitdb list collections --in='countries/(ie|gb)'

# List collections whose name contains "city"
ingitdb list collections --filter-name='*city*'

# Combined: scope and filter
ingitdb list collections --in='countries/(provinces|counties)' --filter-name='*ire*'
```

#### `list view`

```
ingitdb list view [--path=PATH] [--in=REGEXP] [--filter-name=PATTERN]
```

Lists all view definitions in the database.

**Examples:**

```shell
# List all views
ingitdb list view

# List views under a specific path
ingitdb list view --in='countries/.*'
```

#### `list subscribers`

```
ingitdb list subscribers [--path=PATH] [--in=REGEXP] [--filter-name=PATTERN]
```

Lists all subscriber definitions in the database.

**Examples:**

```shell
# List all subscribers
ingitdb list subscribers

# List subscribers filtered by name
ingitdb list subscribers --filter-name='*webhook*'
```

---

### `find` — search records by value *(not yet implemented)*

```
ingitdb find [--path=PATH] [--in=REGEXP] [--substr=TEXT] [--re=REGEXP] [--exact=VALUE] [--fields=FIELDS] [--limit=N]
```

Searches record files for fields matching the given pattern. At least one of `--substr`, `--re`, or `--exact` must
be provided. When multiple search flags are given they are combined with OR.

| Flag | Required | Description |
|------|----------|-------------|
| `--path=PATH` | no | Path to the database directory. Defaults to the current working directory. |
| `--substr=TEXT` | one of three | Match records where any field contains TEXT as a substring. |
| `--re=REGEXP` | one of three | Match records where any field value matches REGEXP. |
| `--exact=VALUE` | one of three | Match records where any field value equals VALUE exactly. |
| `--in=REGEXP` | no | Regular expression scoping the search to a sub-path. |
| `--fields=LIST` | no | Comma-separated list of field names to search. Without this flag all fields are searched. |
| `--limit=N` | no | Maximum number of matching records to return. |

**Examples:**

```shell
# Search all fields for a substring
ingitdb find --substr=Dublin

# Regex search with a result cap
ingitdb find --re='pop.*[0-9]{6,}' --limit=10

# Search specific fields only
ingitdb find --substr=Dublin --fields=name,capital

# Scope search to a sub-path and match a specific field value exactly
ingitdb find --exact=Ireland --in='countries/.*' --fields=country
```

---

### `delete` — delete database objects *(not yet implemented)*

Top-level command with three subcommands.

#### `delete collection`

```
ingitdb delete collection --collection=ID [--path=PATH]
```

Deletes a collection definition and all of its record files.

| Flag | Required | Description |
|------|----------|-------------|
| `--collection=ID` | yes | Collection id to delete (e.g. `countries/ie/counties`). |
| `--path=PATH` | no | Path to the database directory. Defaults to the current working directory. |

**Example:**

```shell
ingitdb delete collection --collection=countries/ie/counties/dublin
```

#### `delete view`

```
ingitdb delete view --view=ID [--path=PATH]
```

Deletes a view definition and removes its materialised output files.

| Flag | Required | Description |
|------|----------|-------------|
| `--view=ID` | yes | View id to delete. |
| `--path=PATH` | no | Path to the database directory. Defaults to the current working directory. |

**Example:**

```shell
ingitdb delete view --view=by_status
```

#### `delete records`

```
ingitdb delete records --collection=ID [--path=PATH] [--in=REGEXP] [--filter-name=PATTERN]
```

Deletes individual records from a collection. Use `--in` and `--filter-name` to scope which records are removed.

| Flag | Required | Description |
|------|----------|-------------|
| `--collection=ID` | yes | Collection to delete records from. |
| `--path=PATH` | no | Path to the database directory. Defaults to the current working directory. |
| `--in=REGEXP` | no | Regular expression scoping deletion to a sub-path. |
| `--filter-name=PATTERN` | no | Glob-style pattern to match record names to delete. |

**Example:**

```shell
ingitdb delete records --collection=countries/ie/counties --filter-name='*old*'
```

---

### `truncate` — remove all records from a collection *(not yet implemented)*

```
ingitdb truncate --collection=ID [--path=PATH]
```

Removes every record file from the specified collection, leaving the collection definition intact.

| Flag | Required | Description |
|------|----------|-------------|
| `--collection=ID` | yes | Collection id to truncate (e.g. `countries/ie/counties/dublin`). Nested paths are supported. |
| `--path=PATH` | no | Path to the database directory. Defaults to the current working directory. |

**Example:**

```shell
ingitdb truncate --collection=countries/ie/counties
```

---

### `migrate` — migrate data between schema versions *(not yet implemented)*

```
ingitdb migrate --from=VERSION --to=VERSION --target=TARGET \
    [--path=PATH] [--format=FORMAT] [--collections=LIST] [--output-dir=DIR]
```

| Flag | Required | Description |
|------|----------|-------------|
| `--from=VERSION` | yes | Source schema version. |
| `--to=VERSION` | yes | Target schema version. |
| `--target=TARGET` | yes | Migration target identifier. |
| `--path=PATH` | no | Path to the database directory. Defaults to the current working directory. |
| `--format=FORMAT` | no | Output format for migrated records. |
| `--collections=LIST` | no | Comma-separated list of collections to migrate. Without this flag, all collections are migrated. |
| `--output-dir=DIR` | no | Directory to write migrated records into. |

**Examples:**

```shell
# Migrate all collections from v1 to v2
ingitdb migrate --from=v1 --to=v2 --target=production

# Migrate specific collections only
ingitdb migrate --from=v1 --to=v2 --target=production --collections=tasks,users

# Write migrated records to a staging directory
ingitdb migrate --from=v1 --to=v2 --target=production --output-dir=/tmp/migration
```

---

### `import` — import data from external databases *(not yet implemented)*

```
ingitdb import --connection=CONN [--path=PATH] [--global-filter=CONDITION] [--table=TABLE1] [--table=TABLE2:CONDITION]
```

| Flag | Required | Description |
|------|----------|-------------|
| `--connection=CONN` | yes | Connection string for the external database (e.g., PostgreSQL, MySQL, GraphQL endpoint). |
| `--path=PATH` | no | Path to the inGitDB database directory. Defaults to the current working directory. |
| `--global-filter=CONDITION` | no | Generic condition applied to all tables that have the specified fields (e.g., `status == "active"`). |
| `--table=TABLE` | no | Table to import. Can be specified multiple times. Format: `table_name` or `table_name:condition` for table-specific conditions. |

The import command (also known as the "Data ingesting" feature or "Ingester component") loads data from external databases such as SQL databases, GraphQL APIs, or other data sources. It applies the global filter to all tables that have the relevant fields, and optionally applies per-table conditions.

**Examples:**

```shell
# Import all tables from a PostgreSQL database
ingitdb import --connection="postgres://user:pass@localhost:5432/mydb"

# Import with a global filter applied to all tables with a 'status' field
ingitdb import --connection="postgres://user:pass@localhost:5432/mydb" --global-filter='status == "active"'

# Import specific tables only
ingitdb import --connection="postgres://user:pass@localhost:5432/mydb" --table=users --table=orders

# Import specific tables with per-table conditions
ingitdb import --connection="postgres://user:pass@localhost:5432/mydb" --table=users:role=="admin" --table=orders:total>100

# Import to a specific database path
ingitdb import --connection="postgres://user:pass@localhost:5432/mydb" --path=/var/db/myapp
```
