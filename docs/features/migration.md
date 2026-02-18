# Data Migration Script Generator

Generates migration scripts to bring a target database in sync with a desired version of an inGitDB database. Also generates a rollback script.

## Use case

When inGitDB data is the source of truth and a target database (e.g. PostgreSQL, MySQL) must be kept in sync, the migration generator computes the diff between two inGitDB versions and produces the scripts needed to apply or revert that change.

## CLI

```
ingitdb migrate [--path=PATH] --from=<git-sha> --to=<git-sha> --target=<connection-string> [--format=SQL] [--collections=<col1,col2>] [--output-dir=./migrations/]
```

| Flag | Default | Description |
|---|---|---|
| `--path` | `.` | Path to the inGitDB database root |
| `--from` | required | Git SHA of the source version |
| `--to` | required | Git SHA of the target version |
| `--target` | required | Connection string to the target database |
| `--format` | `SQL` | Output script format (SQL, …) |
| `--collections` | all | Comma-separated list of collection IDs to include |
| `--output-dir` | `./migrations/` | Directory to write generated scripts |

## Output

Two files are written to `--output-dir`:

- `migration.<from>_<to>.<format>` — applies changes from `--from` to `--to`
- `rollback.<from>_<to>.<format>` — reverts changes from `--to` back to `--from`

## What is compared

The generator diffs the two inGitDB versions at the given git SHAs:

- **New records** → `INSERT` statements in the migration; `DELETE` in the rollback
- **Modified records** → `UPDATE` statements in both directions
- **Deleted records** → `DELETE` in the migration; `INSERT` in the rollback
- **Schema changes** (columns added/removed) → `ALTER TABLE` in migration; inverse `ALTER TABLE` in rollback
