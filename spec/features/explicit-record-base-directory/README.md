---
format: https://specscore.md/feature-specification
status: Draft
---

# Feature: Explicit record base directory

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/explicit-record-base-directory?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/explicit-record-base-directory?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/explicit-record-base-directory?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/explicit-record-base-directory?op=request-change) |
**Status:** Draft
**Source Ideas:** —

## Summary

Allow a collection to set `record_file.records_dir` explicitly, including `.`
to place keyed records directly under `data_dir`, while preserving the existing
implicit `$records` directory when the field is omitted.

## Problem

InGitDB currently forces every single-record file whose name contains `{key}`
under `$records/`. That default keeps README files visible, but it also makes a
declared path such as `{key}/space.yaml` resolve to
`$records/<key>/space.yaml`. Applications with an established record-root
contract cannot represent it without changing their public repository layout.

## Behavior

### Record placement

#### REQ: explicit-records-directory

`record_file.records_dir` MAY declare a clean repository-relative directory.
The value `.` MUST mean the collection's resolved `data_dir` itself. A nested
relative value MUST be joined below `data_dir`.

#### REQ: omitted-field-preserves-default

When `records_dir` is omitted, a record filename containing `{key}` MUST retain
the existing `$records` base directory and a static filename MUST retain the
existing empty base directory.

#### REQ: records-directory-is-path-safe

An explicit `records_dir` MUST reject an empty value, absolute path, backslash,
dot segment other than the single value `.`, parent traversal, or a value that
is not already in clean slash-separated form.

## Acceptance Criteria

### AC: explicit-dot-places-keyed-record-at-data-root

**Requirements:** explicit-record-base-directory#req:explicit-records-directory

**Given** a single-record collection with `data_dir: spaces`,
`records_dir: .`, and `name: "{key}/space.yaml"`
**When** InGitDB resolves the record path for key `space-1`
**Then** the path is `spaces/space-1/space.yaml` without a `$records` segment

### AC: omitted-setting-is-backwards-compatible

**Requirements:** explicit-record-base-directory#req:omitted-field-preserves-default

**Given** an existing single-record collection with `name: "{key}.yaml"` and
no `records_dir`
**When** InGitDB resolves its records base directory
**Then** the base remains `$records`

### AC: unsafe-records-directory-is-rejected

**Requirements:** explicit-record-base-directory#req:records-directory-is-path-safe

**Given** a collection definition whose `records_dir` is absolute, contains a
backslash, contains parent traversal, or is not clean
**When** InGitDB validates the definition
**Then** validation fails before any record path is read or written

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/feature-specification*
