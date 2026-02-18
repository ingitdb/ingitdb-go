# inGitDB Components

- [Validator](validator/README.md) — validates inGitDB schema and data
    - [**Schema Validator**](validator/schema-validator.md) — validates inGitDB schema.
    - [**Data Validator**](validator/data-validator.md) — validates data stored in collections.
- [**Scanner**](scanner.md) — walks the file system and orchestrates the validator and views builder.
- [**Views Builder**](views-builder.md) — creates and
  updates [materialized views](../features/materialized-views.md).
- [**README Builder**](readme-builders/README.md) — generates `README.md` files for collections and records.
- [**Watcher**](watcher.md) — watches the DB directory for file-system changes and emits structured record events.
- [**Triggers**](triggers.md) — pluggable scripts and webhooks called on data change.
- [MCP Server](mcp-server.md) — provides MCP server for AI agents.
- [Merge Conflict Resolver](merge-conflict-resolver.md) — resolves git merge conflicts; auto-regenerates generated files and provides a TUI for source data file conflicts.
- [Migration Generator](migration-generator.md) — diffs two inGitDB versions and generates forward and rollback migration scripts for a target database.