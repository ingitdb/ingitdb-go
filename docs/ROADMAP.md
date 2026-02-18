# inGitDB CLI Roadmap

## Vision

A CLI tool that turns a Git repository into a fully-featured, AI-friendly database: schema-validated, queryable, with precomputed views, event-driven integrations, and native MCP support for AI agents.

## Phases

| Phase | Feature | Status |
|---|---|---|
| 1 | Validator + Materialized Views | WIP |
| 2 | Query | Pending |
| 3 | Subscribers | Pending |
| 4 | MCP Server | Pending |
| 5 | HTTP API Server | Pending |
| 6 | GraphQL | Pending |
| 7 | Migration Script Generator | Pending |

---

## Phase 1: Validator + Materialized Views

**Goal:** Reliable, fast schema and data validation with materialized views rebuilt as part of the same pass.

**Why together:** The validator already walks every collection and record. Materialized views are computed from that same data, so rebuilding them during validation avoids a second full scan.

**Deliverables:**
- `ingitdb validate [--path=PATH] [--from-commit=SHA] [--to-commit=SHA]` validates schema and all records, then rebuilds materialized views for affected collections
- Clear, actionable error messages: collection, file path, field, and violation description
- Change validation mode: validate and rematerialize only files changed between two commits
- CLI subcommand interface per `docs/CLI.md`

---

## Phase 2: Query

**Goal:** Read and filter records from collections via the CLI.

**Deliverables:**
- `ingitdb query --collection=<key> [--path=PATH] [--format=CSV|JSON|YAML]` returns records from a collection
- Default format is JSON

---

## Phase 3: Subscribers

**Goal:** Event-driven notifications when data changes, usable in CI or as standalone hooks.

**Deliverables:**
- Subscriber configuration at collection or DB level in YAML
- Three subscriber types: webhook (HTTP POST), email, and shell command
- Triggered after successful validation and materialization

---

## Phase 4: MCP Server

**Goal:** Expose inGitDB to AI agents via the Model Context Protocol (MCP).

**Deliverables:**
- MCP server mode: `ingitdb mcp [--path=PATH]`
- Tools: list known DBs, list collections, get collection metadata, CRUD on records
- Transaction support: begin / commit / rollback

---

## Phase 5: HTTP API Server

**Goal:** REST access to inGitDB data for external tooling and integrations.

**Deliverables:**
- OpenAPI-compatible HTTP server
- CRUD endpoints per collection
- Schema-driven request and response validation

---

## Phase 6: GraphQL

**Goal:** GraphQL interface auto-generated from collection schemas.

**Deliverables:**
- Schema derived from `.ingitdb-collection.yaml` definitions
- Query and mutation support

---

## Phase 7: Migration Script Generator

**Goal:** Generate forward and rollback migration scripts to sync a target database with a desired inGitDB version.

**Deliverables:**
- `ingitdb migrate --from=<sha> --to=<sha> --target=<connection-string> [--collections=...] [--output-dir=...]`
- Diffs records and schemas between two git SHAs; produces INSERT/UPDATE/DELETE and ALTER TABLE statements
- Rollback script generated alongside the forward migration
- Initial format: SQL; target introspected via connection string to validate applicability and infer dialect
- See [Migration Generator component doc](components/migration-generator.md) for implementation details
