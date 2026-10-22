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

The key is additive and optional, so existing databases are unaffected. A database that uses it needs a CLI and library that know the key: the strict decoders reject unknown keys, so releases before this Feature (ingitdb-cli v0.70.2 and earlier) refuse such a database. Unknown *directives* inside the value are the part older readers tolerate, not the key itself.

## Problem

An inGitDB database is read straight from a Git host in the browser (for example `raw.githubusercontent.com`, through `@ingitdb/client-github`). The host sends its own `Cache-Control` (`max-age=300`), which is right for a moving branch and far too short for data that never changes at a commit SHA. The database author is the one who knows how volatile the data is. Today there is nowhere to say so, and each client invents its own TTLs.

The founder ruled (2026-10-23): use the existing HTTP cache-control header standard, and put the same setting at both database and collection level. No new vocabulary: directive syntax and meaning come from RFC 9111, so every HTTP-literate tool understands the value, and a server can emit it verbatim.

## Behavior

### REQ: cache-control-declarable

`.ingitdb/settings.yaml` (database level) and `<collection>/.collection/definition.yaml` (collection level) MUST both accept an optional key `cache_control` (snake_case, like every other key in those files) holding a string. Absent or empty means "not declared". Example:

```yaml
cache_control: "public, max-age=31536000, immutable"
```

### REQ: cache-control-validation

The validator (`ingitdb validate`, definition load) MUST reject, with an error naming `cache_control` and the offending directive:

- a whitespace-only value, or an empty element (`a,,b`);
- a name that is not an RFC 9110 token, or an unterminated or malformed argument;
- a repeated directive;
- `max-age`, `s-maxage`, `stale-while-revalidate` or `stale-if-error` without a non-negative integer number of seconds (the quoted form `max-age="60"` is accepted, as RFC 9111 section 5.2 asks recipients to), or above 2147483648 (2^31, RFC 9111 section 1.2.2);
- `no-store`, `must-revalidate`, `proxy-revalidate`, `immutable`, `public`, `no-transform` or `must-understand` given an argument.

Directive names are case-insensitive. A directive that is not listed here is unknown and MUST be accepted.

### REQ: cache-control-reader-leniency

A reader MUST never fail a read because of `cache_control`. It drops a malformed directive and keeps the rest; the first of a repeated directive wins; a flag directive given an argument still counts as present; a delta-seconds value above 2147483648 is clamped to 2147483648; unknown directives are ignored (RFC 9111 requires this of every cache). The shared conformance vectors fix these outcomes row by row.

### REQ: cache-control-recognized-unused

`s-maxage`, `private`, `public`, `stale-while-revalidate`, `stale-if-error`, `proxy-revalidate`, `no-transform` and `must-understand` are valid and recognized but have no effect on a reader: a reader is a private, single-user cache that never serves a stale entry, which already satisfies `must-revalidate`. They exist so the same value can be emitted verbatim by a server. Do not implement behavior for them in a reader without amending this Feature.

### REQ: cache-control-precedence

The effective value for a collection is its own `cache_control` when declared, otherwise the database one. The collection value replaces the database value as a whole; directives are never merged across levels. A database-level view (a `$ingitdb` view not owned by one collection) uses the database value.

### REQ: cache-control-scope

A reader MUST apply the effective value to the files it fetches for that collection: record files, and the collection's materialized `.ingr` views under `$ingitdb`. The settings file and collection definition files are the source of the setting, so they are governed by the origin's own header alone. Because they are themselves cached, a changed `cache_control` can take up to two settings/definition cache lifetimes to reach a reader.

### REQ: cache-control-pinned-detection

A reader MUST treat a read as pinned only when it has proof the ref is an immutable commit: the caller says so explicitly (it resolved the SHA itself), or the reader verified that the ref resolves to a commit with exactly that id and is not also a branch or tag name. A ref that merely looks like a SHA (40 or 64 hex characters) is not proof, because a branch or tag can be named that way. When a reader cannot tell, the read is unpinned.

### REQ: cache-control-origin-combination

Let the origin's own `Cache-Control` be O (read from the response; when absent or without `max-age`, the reader's default freshness stands in for it) and the effective declared value be D.

- **Unpinned read:** D may only tighten O. A reader MUST NOT keep a file fresh for longer than the smaller of O's and D's `max-age`. D's `immutable` and any larger `max-age` MUST be ignored.
- **Pinned read:** the content cannot change, so D may relax O. D's `max-age` replaces O's; D's `immutable` without `max-age` gives one year (31536000 seconds).
- **Always:** `no-store` and `no-cache`, in D or in O, win. A reader that gets either MUST NOT reuse the file without refetching, and for `no-store` MUST NOT write it to any cache.

### REQ: cache-control-emission

Any component of inGitDB that serves files over HTTP MUST send the effective value as the `Cache-Control` response header, verbatim, but only the tightening-safe form for a mutable ref (a mutable ref must not be served with relaxing directives). The Go packages expose `ParseCacheControl`, `ReadCacheControl`, `EffectiveCacheControl` and `FreshnessSeconds` for this. No component serves files today, so there is no code site yet.

### REQ: cache-control-conformance-vectors

The parse outcomes and the freshness decisions are fixed by the shared vectors in `ingitdb/ingitdb` `conformance/cache-control/vectors.yaml`. Every implementation (ingitdb-go, ingitdb-ts) vendors the file unchanged and runs every row as a test.

## Acceptance Criteria

### AC: settings-and-definition-decode

**Requirements:** cache-control#req:cache-control-declarable

**Given** a settings file and a collection definition, as YAML text, each declaring `cache_control`
**When** they are decoded by the strict `KnownFields(true)` decoders
**Then** decoding succeeds and the value is present on `Settings` and `CollectionDef`

### AC: invalid-value-rejected

**Requirements:** cache-control#req:cache-control-validation

**Given** a value such as `max-age=soon`, `immutable=1`, `public,,no-store` or `max-age=1, max-age=2`
**When** the definition or settings are validated
**Then** validation fails with an error naming `cache_control` and the offending directive, and `max-age="60"` is accepted

### AC: unknown-directive-ignored

**Requirements:** cache-control#req:cache-control-validation

**Given** `max-age=60, future-directive=7`
**When** it is validated
**Then** it is accepted

### AC: reader-drops-malformed-directives

**Requirements:** cache-control#req:cache-control-reader-leniency

**Given** `max-age=soon, no-store`, or `max-age=5, max-age=6`, or `immutable=1`
**When** a reader parses it
**Then** the reader does not fail: it sees only `no-store`, `max-age=5`, and `immutable` respectively

### AC: collection-overrides-database

**Requirements:** cache-control#req:cache-control-precedence

**Given** database `cache_control: "public, max-age=60"` and a collection declaring `no-store`
**When** the effective value is resolved for that collection
**Then** it is `no-store`, and for a collection declaring nothing it is `public, max-age=60`

### AC: database-view-uses-database-value

**Requirements:** cache-control#req:cache-control-precedence

**Given** a database-level `cache_control` and a `$ingitdb` view not owned by one collection
**When** the effective value is resolved for that view
**Then** it is the database value

### AC: scope-excludes-settings-and-definitions

**Requirements:** cache-control#req:cache-control-scope

**Given** a database declaring `cache_control: "max-age=1"`
**When** a reader fetches `.ingitdb/settings.yaml` and a collection definition
**Then** their freshness comes from the origin's header alone, while record files and `.ingr` views get the declared value

### AC: unpinned-read-only-tightens

**Requirements:** cache-control#req:cache-control-origin-combination

**Given** origin `max-age=300` and declared `public, max-age=31536000, immutable`
**When** a file is read from a branch
**Then** the reader treats it as fresh for at most 300 seconds

### AC: pinned-read-may-relax

**Requirements:** cache-control#req:cache-control-origin-combination

**Given** the same origin and declared values
**When** a file is read at a ref the caller marked as a commit
**Then** the reader may treat it as fresh for 31536000 seconds, and `public, immutable` alone gives the same

### AC: no-store-and-no-cache-win

**Requirements:** cache-control#req:cache-control-origin-combination

**Given** `no-store` or `no-cache` declared, or sent by the origin, on a pinned or an unpinned read
**When** the reader decides how long to reuse the file
**Then** it reuses nothing, and for `no-store` writes nothing to any cache

### AC: hex-named-branch-is-not-pinned

**Requirements:** cache-control#req:cache-control-pinned-detection

**Given** a branch named with 40 hex characters and no caller assertion that it is a commit
**When** a file is read at that ref with declared `max-age=31536000`
**Then** the read is treated as unpinned and is fresh for at most the origin's `max-age`

### AC: unsure-means-unpinned

**Requirements:** cache-control#req:cache-control-pinned-detection

**Given** a reader that cannot tell whether the ref is a commit
**When** it decides freshness
**Then** it applies the unpinned rule

### AC: settings-read-failure-fails-closed

**Requirements:** cache-control#req:cache-control-origin-combination

**Given** the database settings cannot be read because of a transient error (not a 404)
**When** a reader decides freshness for a collection's files
**Then** it assumes `no-cache`: it reuses nothing until the settings can be read

### AC: conformance-vectors-pass

**Requirements:** cache-control#req:cache-control-conformance-vectors

**Given** the shared vectors file
**When** an implementation runs every `parse` and `freshness` row
**Then** every row matches

## Not Doing (and Why)

- **A new TTL vocabulary.** The founder ruled for the HTTP standard.
- **Merging directives across levels.** Whole-value override is predictable; partial merges are not.
- **Per-record or per-view values.** Two levels are enough until someone needs a third.
- **Emitting the header from the CLI today.** `ingitdb` has no HTTP serving command; the emission requirement binds the first one that is added.
- **Loosening strict decoding.** An older CLI rejecting the new key is the intended behavior of strict decoding.

## Dependent Modules

- `ingitdb-ts`: `@ingitdb/client` and `@ingitdb/client-github` read the setting and apply the origin-combination rule to the records cache.
- `ingitdb-schema`: the collection JSON Schema lists `cache_control`. (The root-config schema describes the legacy `.ingitdb.yaml`; there is no JSON Schema for `.ingitdb/settings.yaml` yet.)
- `openvaultdb`: the OVDB descriptor carries the same `cacheControl` field with the same semantics; keep the two in step.

## Open Questions

- `ingitdb-ts` does not yet apply the setting to foreign-key views (`$ingitdb/<collection>/$fk`), which are cached on their own TTLs; they are materialized views of the owning collection and should follow its value. Follow-up.
- The shared vectors live in `ingitdb/ingitdb` (PR ingitdb/ingitdb#10); until it lands, the vendored copies are the reference.

---
*This document follows the https://specscore.md/feature-specification*
