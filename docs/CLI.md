# ingitdb Command Line Interface

`--path` defaults to the current working directory when omitted. `~` in paths is expanded to the user's home directory.

## Global flags

| Flag | Description |
|------|-------------|
| `--help`, `-h` | Print usage information and exit. |

## Commands

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

---

### `setup` — initialise a new database directory *(not yet implemented)*

```
ingitdb setup [--path=PATH]
```

| Flag | Description |
|------|-------------|
| `--path=PATH` | Path to the directory to initialise. Defaults to the current working directory. |

---

### `resolve` — resolve merge conflicts in database files *(not yet implemented)*

```
ingitdb resolve [--path=PATH] [--file=FILE]
```

| Flag | Description |
|------|-------------|
| `--path=PATH` | Path to the database directory. Defaults to the current working directory. |
| `--file=FILE` | Specific conflict file to resolve. Without this flag, all conflicted files are processed. |

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

Lists all collection paths defined in the database. Example:

```
ingitdb list collections --in=countries/(provinces|counties) --filter-name=*ire*
```

#### `list view`

```
ingitdb list view [--path=PATH] [--in=REGEXP] [--filter-name=PATTERN]
```

Lists all view definitions in the database.

#### `list subscribers`

```
ingitdb list subscribers [--path=PATH] [--in=REGEXP] [--filter-name=PATTERN]
```

Lists all subscriber definitions in the database.

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

#### `delete view`

```
ingitdb delete view --view=ID [--path=PATH]
```

Deletes a view definition and removes its materialised output files.

| Flag | Required | Description |
|------|----------|-------------|
| `--view=ID` | yes | View id to delete. |
| `--path=PATH` | no | Path to the database directory. Defaults to the current working directory. |

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
