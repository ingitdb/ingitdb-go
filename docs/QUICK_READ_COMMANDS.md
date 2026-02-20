# 🖥️ Two CLI Commands to Read the Same Record

## ⚙️ Reading the "active" tag from todo.tags collection

### 🔹 1. From Local Repository (using dalgo2ingitdb)
```bash
ingitdb read record --id todo.tags/active
```

### 🐙 2. From GitHub Repository (using dalgo2ghingitdb)
```bash
ingitdb read record --github ingitdb/ingitdb-cli --id todo.tags/active
```

## 🖥️ Both Commands Output
```yaml
title: Active
```

## 📂 Breakdown

### 🖥️ Local Command
- `read record` - subcommand to read a single record
- `--id todo.tags/active` - record ID (collection.id/key)
- Implicitly reads from current directory (or use `--path <dir>`)
- Uses `dalgo2ingitdb` adapter for local filesystem access

### 🖥️ GitHub Command
- `read record` - subcommand to read a single record
- `--github ingitdb/ingitdb-cli` - GitHub repo as owner/repo
- `--id todo.tags/active` - same record ID format
- Optionally add `@branch` or `@tag`: `--github ingitdb/ingitdb-cli@main`
- Uses `dalgo2ghingitdb` adapter for GitHub REST API access

## 📂 Output Format Options

Add `--format json` to both commands for JSON output:

```bash
# 📘 Local
ingitdb read record --id todo.tags/active --format json

# 🐙 GitHub
ingitdb read record --github ingitdb/ingitdb-cli --id todo.tags/active --format json
```

Both output:
```json
{
  "title": "Active"
}
```
