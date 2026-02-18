# Features of the inGitDB

| Feature                                              | Status  | Description                                                                                                                                        |
|------------------------------------------------------|---------|----------------------------------------------------------------------------------------------------------------------------------------------------|
| [Validator](validator.md)                   | WIP     | Validates schema & data; rebuilds materialized views in the same pass                                                                             |
| [Materialized views](materialized-views.md) | WIP     | Generate a precomputed output dataset from multiple source files using defined filters, joins, and transformation logic for fast reuse and access. |
| [Subscribers](subscribers.md)               | pending | Receive notifications about inGitDB changes                                                                                                        |
| [MCP server](mcp-server.md)                 | pending | MCP server for AI agents to read & modify data                                                                                                     |
| [HTTP API server](http-api-server.md)                | pending | Provides API to read & edit inGitDB data                                                                                                           |
| [GraphQL](graph-ql.md)                               | pending |                                                                                                                                                    |
| [Migration Script Generator](migration.md)           | pending | Generates forward and rollback migration scripts to sync a target database with a desired inGitDB version                                          |