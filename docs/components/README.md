# inGitDB components

- [Validator](validator/README.md) - validated inGitDB schema and data
    - [**Schema Validator**](validator/schema-validator.md) – Validates inGitDB schema.
    - [**Data validator**](validator/data-validator.md) - Validate data stored in collections.
- [**Views Builder**](views-builder.md) – creates and
  updates [materialized views](features/materialized-views.md).
- [MCP server](mcp-server.md) - provides MCP server for AI agents.
- [Merge Conflict Resolver](merge-conflict-resolver.md) - resolves git merge conflicts; auto-regenerates generated files and provides a TUI for source data file conflicts.
- [Migration Generator](migration-generator.md) - diffs two inGitDB versions and generates forward and rollback migration scripts for a target database.