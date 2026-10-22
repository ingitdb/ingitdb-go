---
format: https://specscore.md/feature-specification
status: Draft
---

# Feature: Cache-Control setting

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/cache-control?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/cache-control?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/cache-control?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/cache-control?op=request-change) |
**Status:** Draft
**Date:** 2026-10-23
**Owner:** alex
**Supersedes:** —
**Source Ideas:** —

## Summary

A database, and any collection in it, may declare an optional `cache_control` setting whose value is a standard HTTP `Cache-Control` directive string (RFC 9111 section 5.2). It tells readers how long the database's files may be cached. The collection value overrides the database value. A declared value may tighten what the file's origin already says, and may relax it only when the read is pinned to an immutable commit SHA.

## Problem

An inGitDB database is read straight from a Git host in the browser (for example `raw.githubusercontent.com`, through `@ingitdb/client-github`). The host sends its own `Cache-Control` (`max-age=300`), which is right for a moving branch and far too short for data that never changes at a commit SHA. The database author is the one who knows how volatile the data is. Today there is nowhere to say so, and each client invents its own TTLs.

The founder ruled (2026-10-23): use the existing HTTP cache-control header standard, and put the same setting at both database and collection level. No new vocabulary: directive syntax and meaning come from RFC 9111, so every HTTP-literate tool understands the value, and a server can emit it verbatim.

## Behavior

### REQ: cache-control-declarable

`.ingitdb/settings.yaml` (database level) and `<collection>/.collection/definition.yaml` (collection level) MUST both accept an optional key `cache_control` (snake_case, like every other key in those files) holding a string. Absent or empty means "not declared". Example:

```yaml
cache_control: "public, max-age=31536000, immutable"
```

### REQ: cache-control-syntax

A declared value MUST be a comma-separated list of RFC 9111 directives, each `name` or `name=argument`, where `name` is an RFC 9110 token and `argument` is a token or quoted string. `ingitdb validate` (definition load) MUST reject, with an error naming `cache_control` and the offending directive:

- a whitespace-only value, or an empty element (`a,,b`);
- a name that is not a token, or an unterminated or malformed quoted string;
- a repeated directive;
- `max-age`, `s-maxage`, `stale-while-revalidate` or `stale-if-error` without a non-negative integer number of seconds;
- `no-store`, `must-revalidate`, `proxy-revalidate`, `immutable`, `public`, `no-transform` or `must-understand` given an argument.

Directive names are case-insensitive. A directive the reader does not know MUST be ignored, never rejected, as RFC 9111 requires of every cache, so a newer database stays readable by an older reader.

### REQ: cache-control-precedence

The effective value for a collection is its own `cache_control` when declared, otherwise the database one. The collection value replaces the database value as a whole; directives are never merged across levels. A database-level view (a `$ingitdb` view not owned by one collection) uses the database value.

### REQ: cache-control-scope

A reader MUST apply the effective value to the files it fetches for that collection: record files, and the collection's materialized `.ingr` views under `$ingitdb`. The settings file and collection definition files are the source of the setting, so they are governed by the origin's own header alone.

### REQ: cache-control-origin-combination

Let the origin's own `Cache-Control` be O and the effective declared value be D.

- **Unpinned read** (a branch, a tag, or no ref): D may only tighten O. A reader MUST NOT treat a file as fresh for longer than the smaller of O's and D's `max-age`, and MUST honor `no-store`, `no-cache` and `must-revalidate` from either side. D's `immutable` and any larger `max-age` MUST be ignored.
- **Pinned read** (the ref is a full commit SHA, 40 hex characters, or 64 for SHA-256 repositories): the content cannot change, so D may relax O. D's freshness directives (`max-age`, `immutable`) replace O's. `no-store` and `no-cache` from either side are still honored.

A reader that cannot tell whether a read is pinned MUST treat it as unpinned.

### REQ: cache-control-emission

Any component of inGitDB that serves files over HTTP MUST send the effective value as the `Cache-Control` response header, verbatim, subject to the same pinned rule (a mutable ref must not be served with relaxing directives). The Go packages expose `ParseCacheControl` and `EffectiveCacheControl` for this.

## Acceptance Criteria

### AC: settings-and-definition-decode

**Requirements:** cache-control#req:cache-control-declarable

**Given** a settings file and a collection definition each declaring `cache_control`
**When** they are decoded by the strict `KnownFields(true)` decoders
**Then** decoding succeeds and the value is present on `Settings` and `CollectionDef`

### AC: invalid-value-rejected

**Requirements:** cache-control#req:cache-control-syntax

**Given** a value such as `max-age=soon`, `immutable=1`, `public,,no-store` or `max-age=1, max-age=2`
**When** the definition or settings are validated
**Then** validation fails with an error naming `cache_control` and the offending directive

### AC: unknown-directive-ignored

**Requirements:** cache-control#req:cache-control-syntax

**Given** `max-age=60, future-directive=7`
**When** it is validated
**Then** it is accepted

### AC: collection-overrides-database

**Requirements:** cache-control#req:cache-control-precedence

**Given** database `cache_control: "public, max-age=60"` and a collection declaring `no-store`
**When** the effective value is resolved for that collection
**Then** it is `no-store`, and for a collection declaring nothing it is `public, max-age=60`

### AC: unpinned-read-only-tightens

**Requirements:** cache-control#req:cache-control-origin-combination

**Given** origin `max-age=300` and declared `public, max-age=31536000, immutable`
**When** a file is read from a branch
**Then** the reader treats it as fresh for at most 300 seconds

### AC: pinned-read-may-relax

**Requirements:** cache-control#req:cache-control-origin-combination

**Given** the same origin and declared values
**When** a file is read at a full commit SHA
**Then** the reader may treat it as fresh for 31536000 seconds

## Not Doing (and Why)

- **A new TTL vocabulary.** The founder ruled for the HTTP standard.
- **Merging directives across levels.** Whole-value override is predictable; partial merges are not.
- **Per-record or per-view values.** Two levels are enough until someone needs a third.
- **Emitting the header from the CLI today.** `ingitdb` has no HTTP serving command; the emission requirement binds the first one that is added.

## Dependent Modules

- `ingitdb-ts`: `@ingitdb/client` and `@ingitdb/client-github` read the setting and apply the origin-combination rule in their fetch cache.
- `openvaultdb`: the OVDB descriptor carries the same `cacheControl` field with the same semantics; keep the two in step.

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/feature-specification*
