---
format: https://specscore.md/feature-specification
status: Approved
---

# Feature: Record-file name placeholder substitution and path-separator safety

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/record-file-name-placeholders?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/record-file-name-placeholders?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/record-file-name-placeholders?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/record-file-name-placeholders?op=request-change) |
**Status:** Approved
**Date:** 2026-07-17
**Owner:** alex
**Supersedes:** —
**Source Ideas:** —

## Summary

`RecordFileDef.GetRecordFileName` computes a record's on-disk file name from the `record_file.name` pattern by substituting `{key}` and `{fieldName}` placeholders. Two defects make it produce data-losing or wrong file names: the `{fieldName}` substitution loop is guarded by an inverted condition and never runs, and a substituted value containing a path separator is written literally into the name, nesting the record file where the reader can never glob it back. This Feature makes `{fieldName}` substitution work and makes the function reject any substituted value that contains a path separator, turning silent data loss into a loud, actionable error. Closes ingitdb-go#1 and ingitdb-go#2.

## Problem

`record_file.name` is documented (`ingitdb-specs/docs/schema/collection.md`) to support both `{key}` and `{fieldName}` placeholders. `GetRecordFileName` (`record_file_def.go`) is the library's single point of truth for turning a record into a file name. Two independent bugs live there.

**`{fieldName}` never substitutes (ingitdb-go#2).** The substitution loop is written `for colName := range data { if colName != "" { continue } ... }` — it skips every column whose name is not the empty string, i.e. every real column, so the loop body is dead. A pattern like `{status}-{key}.json` is written literally as `{status}-...json`. The guard is inverted; the intent was to skip the (never-present) empty-named column and process the rest. The dead body also means the substitution scanned columns looking for placeholders rather than the reverse, so the existing unit test only ever exercised an empty-named column into a `{}` placeholder — it encoded the buggy behaviour rather than the documented one.

**A path separator in a substituted value silently loses the record (ingitdb-go#1).** With `name: "{key}.json"`, a key of `telegram/inline-keyboard` substitutes literally to `telegram/inline-keyboard.json`, which `filepath.Join` writes as a nested directory under `$records/`. The records reader globs with `*` (`materializer/records_reader_fs.go`), and Go's `filepath.Glob` `*` never matches a path separator, so `$records/*.json` cannot see `$records/telegram/inline-keyboard.json`. The write succeeds, the file exists, and `select` silently returns nothing — the worst failure mode available: no signal at write time, no signal at read time, a plausible-looking result set with records missing. A `/` in a key is not exotic; namespacing by prefix (`telegram/…`, `whatsapp/…`) is a natural modelling instinct, and inGitDB's own `--id` grammar (`<collection>/<key>`) primes users to treat `/` as a legal separator. Once `{fieldName}` substitution works, the same hazard applies to any substituted field value containing a separator, so both placeholder kinds must be guarded identically.

The write and read paths disagree about what a legal name segment is, and no test covers the disagreement. Making the file-name computer refuse a separator is the cheapest fix that fences the whole class: whatever the caller does with the returned name, a data-losing key can no longer be turned into a path.

## Behavior

### REQ: fieldname-placeholder-substitution

`GetRecordFileName` MUST substitute each `{fieldName}` placeholder in `record_file.name` with the corresponding column value from the record's data, formatted with default Go value formatting (`fmt.Sprintf("%v", value)`). A column whose name is the empty string MUST be skipped (there is no `{}` placeholder concept). Substitution of a given placeholder replaces its first occurrence, consistent with the existing `{key}` handling.

### REQ: key-placeholder-substitution-preserved

`GetRecordFileName` MUST continue to substitute the first `{key}` placeholder with the record's key string, unchanged from prior behaviour except for the path-separator guard below.

### REQ: reject-path-separator-in-segment

`GetRecordFileName` MUST return a non-nil error, and MUST NOT return a usable file name, when any value substituted into the pattern — the `{key}` value or any `{fieldName}` value — contains a path separator (`/` or `\`). The error MUST name the offending placeholder and value so the caller can surface an actionable message. This turns the silent-nesting data loss into a hard failure at the point the name is computed, regardless of what the caller does next.

A value with no path separator MUST substitute normally and MUST NOT error.

## Acceptance Criteria

### AC: fieldname-placeholder-is-substituted

**Requirements:** record-file-name-placeholders#req:fieldname-placeholder-substitution

**Given** a `RecordFileDef` with `name: "{status}-{key}.json"` and a record whose data has `status: "native"` and key `inline-keyboard`
**When** `GetRecordFileName` is called
**Then** it returns `native-inline-keyboard.json` and no error

### AC: empty-named-column-is-skipped

**Requirements:** record-file-name-placeholders#req:fieldname-placeholder-substitution

**Given** a `RecordFileDef` with `name: "{key}.json"` and a record whose data contains a column named `""`
**When** `GetRecordFileName` is called
**Then** the empty-named column is not substituted and the result is the key followed by `.json`

### AC: key-placeholder-still-substitutes

**Requirements:** record-file-name-placeholders#req:key-placeholder-substitution-preserved

**Given** a `RecordFileDef` with `name: "{key}.yaml"` and a record with key `task-1`
**When** `GetRecordFileName` is called
**Then** it returns `task-1.yaml` and no error

### AC: slash-in-key-is-rejected

**Requirements:** record-file-name-placeholders#req:reject-path-separator-in-segment

**Given** a `RecordFileDef` with `name: "{key}.json"` and a record whose key is `telegram/inline-keyboard`
**When** `GetRecordFileName` is called
**Then** it returns an error naming the `key` placeholder and the offending value, and returns no usable file name

### AC: slash-in-field-value-is-rejected

**Requirements:** record-file-name-placeholders#req:reject-path-separator-in-segment

**Given** a `RecordFileDef` with `name: "{status}-{key}.json"` and a record whose `status` value is `in/progress`
**When** `GetRecordFileName` is called
**Then** it returns an error naming the `status` placeholder and the offending value, and returns no usable file name

### AC: static-name-round-trips

**Requirements:** record-file-name-placeholders#req:key-placeholder-substitution-preserved

**Given** a `RecordFileDef` with a static `name: "records.json"` (no placeholders)
**When** `GetRecordFileName` is called
**Then** it returns `records.json` and no error

## Not Doing (and Why)

- **Erroring on an unresolved placeholder** — a pattern referencing a `{foo}` column the record lacks passes through literally. ingitdb-go#2 raised this as a "consider". It is deferred: distinguishing an intended placeholder from incidental brace text is ambiguous, and the concrete data-loss risk this Feature closes is the separator, not a stray brace. A misconfigured pattern that leaves `{foo}` in a name is visible on disk; a nested, unreadable record is not.
- **Percent-encoding separators instead of rejecting them** — ingitdb-go#1 option 2 would preserve slashed keys as a first-class concept, but that is a data-model change (the reader would have to decode on read) far larger than the debt being cleared. Rejecting is the cheapest fix that removes the silent loss; encoding can be layered on later without contradicting this contract.
- **Wiring the CLI write path through `GetRecordFileName`** — the `ingitdb-cli` insert path currently computes file names inline and does not call this function, so this library fix hardens the chokepoint without yet being reached by every writer. Adopting `GetRecordFileName` (and surfacing its error) in the CLI is a separate change in that repo; this Feature makes the primitive correct and safe so that adoption is a drop-in.

## Rehearse Integration

No Rehearse stubs are scaffolded: `specscore.yaml` in this repo declares no rehearse configuration, and every AC here is directly executable as a Go table test against `RecordFileDef.GetRecordFileName`. Recorded as an explicit skip rather than an omission.

## Dependent Modules

`GetRecordFileName` has no in-repo production caller (only its unit test), and no caller in `ingitdb-cli`. Changing its return signature from `string` to `(string, error)` is therefore a contained change: only the unit test is updated. The signature change is the mechanism by which the function can "fail loudly".

## Open Questions

- Should the separator set be OS-specific (`os.PathSeparator` only) rather than always rejecting both `/` and `\`? Both are rejected here so a database authored on one OS cannot silently nest on another. Revisit only if a legitimate key needs a literal backslash.

---
*This document follows the https://specscore.md/feature-specification*
