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
