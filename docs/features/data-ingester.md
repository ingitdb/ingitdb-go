# Data Ingester (Import Feature)

**Status:** Planned  
**Related commands:** `ingitdb import`

## Overview

The Data Ingester (also known as the Import feature or Ingester component) enables inGitDB to load data from external databases and data sources such as SQL databases (PostgreSQL, MySQL, etc.), GraphQL APIs, and other structured data sources. This feature transforms external data into inGitDB's file-based format while applying flexible filtering rules.

## Key Features

### 1. Connection String Support

The import command accepts a connection string that specifies how to connect to the external data source:

- **SQL Databases:** Standard connection strings for PostgreSQL, MySQL, SQLite, etc.
- **GraphQL APIs:** HTTP/HTTPS endpoints with optional authentication
- **Other Sources:** Extensible architecture for adding new data source types

### 2. Global Filtering

A generic condition can be applied to all imported tables that have the specified fields. For example:

```shell
ingitdb import --connection="postgres://..." --global-filter='status == "active"'
```

This filter will be applied to every table that has a `status` field, automatically excluding inactive records across the entire import operation.

**Common use cases:**
- Filter by status: `status == "active"`
- Filter by date: `created_at >= "2024-01-01"`
- Filter by flags: `is_deleted == false`
- Complex conditions: `(status == "active" OR priority == "high") AND created_at >= "2024-01-01"`

### 3. Per-Table Filtering

Individual tables can be specified with optional table-specific conditions:

```shell
ingitdb import --connection="postgres://..." \
  --table=users \
  --table=orders:total>100 \
  --table=products:category=="electronics"
```

When both global and per-table filters are specified, both conditions must be satisfied (logical AND).

### 4. Selective Import

Control which tables to import:

- Import all tables (default behavior when no `--table` flags are specified)
- Import specific tables only (specify one or more `--table` flags)
- Mix filtered and unfiltered tables in the same import operation

## Command Syntax

```
ingitdb import --connection=CONN [OPTIONS]
```

### Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--connection=CONN` | yes | Connection string for the external database or API |
| `--path=PATH` | no | Path to the inGitDB database directory (default: current directory) |
| `--global-filter=CONDITION` | no | Generic condition applied to all tables with matching fields |
| `--table=TABLE[:CONDITION]` | no | Table to import, optionally with a table-specific condition. Can be specified multiple times. |

## Examples

### Import All Tables

```shell
ingitdb import --connection="postgres://user:pass@localhost:5432/mydb"
```

### Import with Global Filter

Import only active records from all tables that have a `status` field:

```shell
ingitdb import \
  --connection="postgres://user:pass@localhost:5432/mydb" \
  --global-filter='status == "active"'
```

### Import Specific Tables

Import only the `users` and `orders` tables:

```shell
ingitdb import \
  --connection="postgres://user:pass@localhost:5432/mydb" \
  --table=users \
  --table=orders
```

### Import with Per-Table Conditions

Import users with admin role and high-value orders:

```shell
ingitdb import \
  --connection="postgres://user:pass@localhost:5432/mydb" \
  --table=users:role=="admin" \
  --table=orders:total>1000
```

### Combine Global and Per-Table Filters

Import active records, but only admin users and recent orders:

```shell
ingitdb import \
  --connection="postgres://user:pass@localhost:5432/mydb" \
  --global-filter='status == "active"' \
  --table=users:role=="admin" \
  --table=orders:created_at>="2024-01-01"
```

### Import to Specific Path

Import data into a specific inGitDB database directory:

```shell
ingitdb import \
  --connection="postgres://user:pass@localhost:5432/mydb" \
  --path=/var/db/myapp
```

## Implementation Notes

### Architecture

The Data Ingester will consist of:

1. **Connection Manager:** Handles different connection string formats and establishes connections
2. **Schema Mapper:** Maps external schema to inGitDB collections and column definitions
3. **Query Builder:** Constructs filtered queries based on global and per-table conditions
4. **Data Transformer:** Converts external data formats to YAML/JSON records
5. **Writer:** Creates collection directories and writes record files with proper validation

### Supported Condition Syntax

The condition syntax will support:

- **Comparison operators:** `==`, `!=`, `>`, `<`, `>=`, `<=`
- **Logical operators:** `AND`, `OR`, `NOT`
- **String literals:** Enclosed in double quotes `"active"`
- **Numeric literals:** Integers and decimals `100`, `99.99`
- **Parentheses:** For grouping `(a == 1 OR b == 2) AND c == 3`

### Future Enhancements

- **Incremental imports:** Only import changed records since last import
- **Schema generation:** Automatically generate `.ingitdb-collection.yaml` files from external schema
- **Transform rules:** Apply transformations during import (rename fields, compute values, etc.)
- **Dry-run mode:** Preview what would be imported without writing files
- **Progress reporting:** Show import progress with record counts and timing
- **Validation:** Optionally run `validate` after import completes
- **Conflict resolution:** Handle duplicate records and primary key conflicts

## Related Documentation

- [CLI Reference](../CLI.md#import--import-data-from-external-databases-not-yet-implemented)
- [Architecture](../ARCHITECTURE.md)
- [Roadmap](../ROADMAP.md)

## Status

This feature is currently planned but not yet implemented. The command is stubbed in the CLI and returns "not yet implemented" when executed.
