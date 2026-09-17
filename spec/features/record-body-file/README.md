---
format: https://specscore.md/feature-specification
status: Draft
---

# Feature: Record companion body file

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/record-body-file?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/record-body-file?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/record-body-file?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/record-body-file?op=request-change) |
**Status:** Draft
**Date:** 2026-09-17
**Owner:** alex
**Supersedes:** —
**Source Ideas:** —

## Summary

A single-record collection may declare `record_file.body_file`: a companion raw-text file beside each record file that carries one string field. For example, `customer-invoices.query.dtql` sits beside `customer-invoices.query.json`. A per-record type field selects the body file's extension. Readers merge the body into the field byte-for-byte. Writers split the field back out through an exported, ordered write plan. They never duplicate the body into the record file or leave a stale body behind. Crash-safe multi-file application is owned by ingitdb/dalgo2ingitdb#15, and every reader runs that driver's recovery first. Listing never mistakes a body file for a record. The shape is pinned in `FORMAT.md` and a golden fixture, so the Go and TypeScript readers agree. Closes ingitdb-go#24.

## Problem

Today a record is exactly one file. DataTug keeps a query as a JSON record plus a raw body file beside it. The founder decided on 2026-09-17 to keep that pair under the DALgo-backed project store (`datatug/datatug` Feature `dalgo-project-store`, REQ `driver-prerequisites-are-recorded`):

```
projects/p1/queries/customer-invoices/customer-invoices.query.json   # title, type, params, target
projects/p1/queries/customer-invoices/customer-invoices.query.dtql   # raw DTQL text
```

The record file is already expressible: `name: '{key}/{key}.query.json'`, `format: json`, `type: map[string]any`, `records_dir: '.'`. The body file is not. The body's extension also varies per record: `datatug-core` names it `<id>.query.<lowercase QueryDef.Type>`, so it is `.query.sql` for one query and `.query.dtql` for another (`datatug-core/pkg/storage/filestore/query_content.go`, `queryBodyFileName`). Without this Feature, DataTug must either inline the body as an escaped JSON string, which is unreadable in a diff, or keep a sidecar reader beside the driver. The second option is the machinery the cut-over is meant to retire.

Two existing mechanisms come close, and neither fits:

- **`format: markdown` + `content_field`** gives a body, but the body shares one file with YAML frontmatter. DataTug's record is JSON and its body is DTQL or SQL, not Markdown.
- **`{fieldName}` placeholders** in `record_file.name` (Feature `record-file-name-placeholders`) substitute a column value into the record file's own name. They name one file, not a second one. They are also case-preserving and have no character-set bound, and the body type needs both a case rule and a bound (see REQ `body-type-resolution`).

## Behavior

### Definition

#### REQ: body-file-definition

`RecordFileDef` MUST accept an optional `body_file` mapping with these keys. Because they are modelled fields, the strict (`KnownFields(true)`) definition reader accepts them:

| Key | Required | Meaning |
|-----|----------|---------|
| `name` | yes | A name template for the body file, resolved in the **directory that holds the record file**. It MUST contain `{key}` exactly once. It MAY end in `.{body_type}`. |
| `field` | yes | The record column that the body file carries. |
| `type_field` | only with `{body_type}` | The record column whose value selects the body type. |

The DataTug queries collection is the reference definition:

```yaml
record_file:
  name: '{key}/{key}.query.json'
  format: json
  type: map[string]any
  records_dir: '.'
  body_file:
    name: '{key}.query.{body_type}'
    field: text
    type_field: type
columns:
  type:
    type: string
    required: true
  text:
    type: string
```

A body file whose name is a plain basename always sits in the same directory as its record file. That keeps a record and its body under one containment check and one directory to fsync (REQ `write-plan-exposed`). It also makes the isolation rules in REQ `body-files-isolated-from-listing` a basename comparison, not a tree search. When `body_file` is omitted, behavior MUST be byte-for-byte what it is today.

#### REQ: body-file-definition-validated

`RecordFileDef.Validate()` (with column-aware checks in `CollectionDef.Validate()`) MUST reject each of the following at definition-load time, before any record path is read or written:

1. `body_file` on a record type other than `map[string]any` (`SingleRecord`). A list or map file holds many records, so a per-record sibling file has no single owner.
2. `body_file` with `format: markdown`. A Markdown record already has a body (`content_field`), and two body carriers for one record would be ambiguous.
3. `body_file` when `record_file.name` has no `{key}`, since the body file is per record.
4. An empty `name` or `field`. A `name` containing `/`, `\`, `..`, or a control character. A `name` whose `{key}` count is not exactly one. A `name` with any placeholder other than `{key}` and `{body_type}`. A `{body_type}` that is not the final segment directly after a `.`.
5. `{body_type}` in `name` without `type_field`, or `type_field` without `{body_type}`.
6. A `field` or `type_field` that is not a declared column of type `string`. A `field` equal to `type_field`. A computed (`formula`) column as `field` or `type_field`.
7. A static `name` (no `{body_type}`) whose basename template equals the record file's basename template, or whose resolved basenames would match the record file's basename glob. See REQ `body-files-isolated-from-listing`.

#### REQ: body-type-resolution

When `name` ends in `.{body_type}`, the body type for a record MUST be the record's `type_field` value with ASCII letters lowercased. For example, `SQL` resolves to `sql` and `DTQL` resolves to `dtql`. The value MUST be 1 to 32 characters, each an ASCII letter, digit, `_` or `-`. A value outside that set can never name a body file. This includes a value with a dot, a separator, or a non-ASCII character.

This keeps the body extensions that `datatug-core`'s `queryTypeFileExtension` produces today (`.query.sql`, `.query.dtql`, `.query.http`). It does not keep full DataTug paths: today's demo projects group queries in folders (`queries/customers/customer-invoices.query.json`), and the canonical layout under `dalgo-project-store` is `queries/<id>/<id>.query.json`. The type value is record content, and so it is untrusted. Bounding it here is what stops a value like `../../x` from addressing a file outside the collection.

Deriving the extension from the open type value, rather than from a closed list of types in the definition, is **provisional** until Open Question 3 is decided.

A record whose `type_field` value is empty, absent, outside the allowed set, or colliding (REQ `body-files-isolated-from-listing`) is **unnameable**: no body file name can be derived for it. A write that carries the body field for an unnameable record is refused (REQ `write-splits-body-out`). The read rules for an unnameable record are in REQ `read-merges-body-raw`.

### Reading

#### REQ: read-merges-body-raw

Every record reader MUST merge a present body file into the record's `field` as a string holding the file's exact bytes. That includes this module's `datavalidator` pass, the materializer's `records_reader_fs` and every other `SingleRecord` read path, plus the drivers listed under Dependent Modules. The reader MUST NOT decode, trim, normalise newlines, or strip a BOM. The merge happens before column validation, so the body field is validated like any other `string` column.

The **candidate body files** of a record with key `K` are the files in the record file's directory that match the body name template with `{key}` = `K` and `{body_type}` = any single segment with no dot, excluding the record file itself.

A read MUST fail with an error that names the record key and the body path in these cases:

- the body file is not valid UTF-8, since records are text-only per `ingitdb/ingitdb` Feature `storage-format`, REQ `text-only`;
- the body path is not a regular file: a directory or symlink on a filesystem, or a Git tree entry whose mode is not `100644` or `100755` (for example `120000`, a symlink, or `160000`, a submodule);
- the record file itself contains the `field` key, so the value would have two sources and neither silently wins;
- the record is unnameable (REQ `body-type-resolution`) **and** at least one candidate body file is present.

An unnameable record with no candidate body file reads successfully, with the same result as a missing body (REQ `missing-body-is-absent-field`).

#### REQ: missing-body-is-absent-field

**Provisional until Open Question 2 is decided.** A missing body file MUST NOT be an error. The record MUST read with `field` **absent** from its data, not set to the empty string. A present but empty body file MUST read as `field: ""`.

A missing body can only mean "no body" when no multi-file change is pending. REQ `pending-changes-recovered-before-read` guarantees that, so a torn record-and-body write is never read as a valid record without a body.

### Writing and deleting

#### REQ: write-splits-body-out

A writer putting a record into a collection with `body_file` MUST:

- encode the record file from the record data **without** `field`, so the body is never duplicated into the record file;
- when `field` is present with a string value, write the body file at its resolved name with exactly those bytes and no encoding;
- when `field` is absent or null, write no body file and remove any existing body file for that record;
- refuse the whole put before changing any file when `field` is present but not a string, or when the record is unnameable (REQ `body-type-resolution`);
- leave a file untouched when its bytes already equal the bytes being written, per `storage-format` REQ `no-rewrite-without-change`.

#### REQ: delete-and-type-change-leave-no-stale-body

A delete MUST remove the record file and the body file resolved from the stored record's `type_field` value. A body file that is already missing is not an error.

A put that changes the resolved body name MUST remove the body file resolved from the **previously stored** record, as part of the same change. A type change, for example `SQL` → `DTQL`, is one such put. After the put, a record has at most one body file on disk, and it is the one its current type names.

#### REQ: write-plan-exposed

This module MUST export a pure function that computes the full file-level change for a record put or delete, without touching the filesystem. Its inputs are the collection definition, the record key, the previously stored state (the record file bytes and body file bytes, or none), and the new record data (or none for a delete).

It returns an ordered list of operations. The list is the **exact and complete** set of files the change touches: applying exactly these operations, in this order, yields the new state, and no other file may be written or removed. Each operation carries:

- its kind, `write` or `remove`;
- its path, slash-separated and relative to the database root;
- for `write`, the full new bytes;
- the **expected prior state** of that path: absent, or present with the SHA-256 of its bytes. A journal can use this to detect a concurrent change, and to tell during recovery whether an operation has already been applied.

The order is fixed:

- **Put:** (1) write the body file, if its name or bytes change; (2) write the record file, if its bytes change; (3) remove the stale body file, if there is one (a type change, or `field` now absent).
- **Delete:** (1) remove the record file; (2) remove the body file, if present.

An unchanged put returns an empty list. Every path in a plan shares one containing directory, and the plan names that directory.

Writers MUST apply this plan instead of re-deriving body names. ingitdb/dalgo2ingitdb#15 owns crash-safe application of multi-file changes: its durable journal, its recovery, and the atomicity of a record and its body as one unit. This Feature does not design that journal. It does require that until a driver provides that atomicity, the driver MUST refuse, with a typed error and before changing any file, any put or delete whose plan has more than one operation. Ordering alone does not make a pair consistent. A write that commits a Git tree in one commit (such as `dalgo2ingitdb4github`) is already atomic and applies the whole plan in that commit.

#### REQ: pending-changes-recovered-before-read

A torn multi-file change must never be read as a valid state. This module MUST export a pending-change recovery hook: an interface that a driver implements, for example `RecoverPendingChanges(ctx, dbRoot) error`. The hook exists because `ingitdb-go` cannot import `dalgo2ingitdb`, the owner of the journal (ingitdb/dalgo2ingitdb#15).

Every reader of a working-tree database MUST do the following before it reads any record of a collection that declares `body_file`:

- If a recovery hook is configured, invoke it, and proceed only when it returns without error. On error, fail with that error.
- If no hook is configured, refuse with a typed error (for example `ErrPendingChangeRecoveryUnavailable`) that names the collection. The reader MUST NOT read records without the hook.

This applies to the `datavalidator` pass, the materializer's records reader, the foreign-key index, and the drivers' own reads. `ingitdb-cli` MUST configure `dalgo2ingitdb`'s hook for its validate and materialize commands.

Reads of a committed Git tree, such as `dalgo2ingitdb4github` or a read at a commit, are exempt. A commit is written atomically, and no journal is attached to it. Collections without `body_file` are unaffected: their changes touch one file each, and #15 makes those atomic by rename.

#### REQ: record-revision-covers-body

Any revision, etag or change token that `ingitdb-go` or a driver computes for a record in a collection that declares `body_file` MUST change whenever any of the following changes:

- the record file's existence or bytes;
- the resolved body file name, or its absence;
- the body file's existence or bytes.

This module MUST export the canonical revision input: the byte sequence to hash, with each part length-prefixed so that bytes moving between the record file and the body file also change it. Drivers hash that sequence, with an HMAC where they need one, and do not frame the parts themselves. Today `dalgo2ingitdb/protected.go` (`revision`, `:482-494`) hashes the record file bytes only, so an edit to the body alone would not change the revision. `datatug-core` already hashes both (`query_revision.go`).

### Isolation

#### REQ: body-files-isolated-from-listing

Listing, globbing and counting records MUST NOT return a body file as a record, or as part of a record key. This applies in `datavalidator` (`singleRecordGlobPattern`, record counts), the materializer, the foreign-key index, `dalgo2ingitdb` (`query.go` `readAllSingleStored`) and `dalgo2ingitdb4github` (`query.go` `scanSingleRecords`).

Record discovery keeps using only the record file's name template. Two rules keep the dotted-suffix discriminators unambiguous:

- **Per value:** a resolved body basename that equals the record file's resolved basename, or that matches the record file's basename glob (`{key}` → `*`), MUST be refused on read and on write, making the record unnameable. Under `{key}/{key}.query.json`, a type `JSON` would resolve to the record file itself, so it is refused. Under a flat `{key}.json` with body `{key}.body.{body_type}`, the type `json` is refused because `q1.body.json` would list as record `q1.body`.
- **Structure:** `{body_type}` holds no dots and is always the final dot segment (REQ `body-file-definition-validated`, REQ `body-type-resolution`). A body basename therefore parses uniquely: the type is the last dot segment, and the key is what lies between the template's fixed prefix and fixed suffix. Two different (key, type) pairs can never produce the same body file name.

`exclude_regex` continues to apply to record files only. It is not needed, and not consulted, to hide body files.

#### REQ: orphan-body-files-reported

Whole-database validation MUST report a finding for each **orphan body file**. An orphan is a file in a record directory that matches the body name template (`{key}` and `{body_type}` → dot-free wildcards) and is not a record file, and for which either:

- there is no record file for its key, or
- the record's current `type_field` value resolves to a different body name, such as a stale `.query.sql` left beside a `DTQL` query by a hand edit.

The finding names the collection and the file path. A stale body beside a nameable record is a finding, not a read error: that record still reads its current body. The unnameable case is a read error, per REQ `read-merges-body-raw`.

### Compatibility

#### REQ: version-skew-fails-safe

A reader or writer that does not understand `body_file` must never silently mishandle a collection that declares it. Such a mistake would write `text` into the record file, leave stale bodies, or drop `body_file` from the definition.

- **Format version.** `FORMAT.md` MUST bump its version from `v1` to `v2`. A collection that declares `body_file` is `v2` content.
- **Minimum versions.** The first `ingitdb-go/ingitdb` release that implements this Feature is the minimum for `body_file`. `dalgo2ingitdb` and `dalgo2ingitdb4github`, both of which currently require `ingitdb-go/ingitdb` v0.5.2, MUST raise their requirement to that release. `FORMAT.md` MUST name that release and the first `ingitdb-ts` release that reads body files.
- **Strict decoding.** Strict older readers, such as `ingitdb-go` releases up to v0.7.0 that use `KnownFields(true)`, already reject a definition that contains `body_file`, and that is the intended fail-safe. Every lenient definition decode in the drivers MUST become strict (`KnownFields(true)`), or MUST preserve `body_file` unchanged. The known lenient decodes are `dalgo2ingitdb`'s `schema_reader.go:105` (`DescribeCollection`) and `schema_modifier.go:445` (`readCollectionDefYAML`).
- **Definition round trip.** A collection alteration that decodes and rewrites the definition (`writeCollectionDefYAML`, `schema_modifier.go:427`) MUST round-trip `body_file` byte-for-byte unless the alteration itself updates it (REQ `schema-alteration-keeps-body-file-consistent`).

#### REQ: schema-alteration-keeps-body-file-consistent

A schema alteration on a collection that declares `body_file` MUST leave a definition that passes REQ `body-file-definition-validated`. The rules for each operation:

- `ApplyRenameField` of the `body_file.field` column or the `body_file.type_field` column MUST update the matching `body_file` key in the same definition write. No body file is renamed, because body names do not contain the field name.
- `ApplyModifyField` that would make either column non-`string` or computed MUST be refused before any write.
- `ApplyDropField` of either column MUST be refused while `body_file` references it.

Drivers MUST run `Validate()` on the altered definition before writing it.

### Cross-language contract

#### REQ: format-contract-and-golden-fixture

The on-disk contract `FORMAT.md` currently lives in `ingitdb/dalgo2ingitdb` (with `testdata/format-fixtures/`). It is mirrored in `ingitdb/ingitdb-ts` at `packages/client-fs/src/__fixtures__/format-fixtures/` and `packages/client-github/src/__fixtures__/format-fixtures/`. `ingitdb-go` has no `FORMAT.md` today.

This Feature MUST add a "Body files" section to that `FORMAT.md`. The section states the `body_file` keys, body type resolution, the raw merge, absent versus empty, the isolation rule, the write plan order, the recovery-before-read rule and the `v2` version.

It MUST also add a `queries` root collection to the golden fixture tree, using the reference definition from REQ `body-file-definition` (nested `{key}/{key}.query.json`, `records_dir: '.'`; the fixture MUST NOT fall back to a flat `$records/` layout):

- `customer-invoices`: `type: DTQL` plus `customer-invoices.query.dtql`, a multi-line body with a trailing newline and a non-ASCII character;
- `active-customers`: `type: SQL` plus `active-customers.query.sql`, a body with **no** trailing newline;
- `draft-query`: `type: SQL` and no body file.

`ingitdb-go` MUST keep a byte-identical copy of that `queries` subtree under `ingitdb/testdata/format-fixtures/` and read it through the real reader in a test. All copies MUST stay identical, which is the existing `FORMAT.md` rule.

#### REQ: ts-layout-prerequisite-recorded

TypeScript parity depends on a prerequisite that does not exist yet. Today both `ingitdb-ts` clients hard-code a flat `$records/` layout. They ignore `records_dir` and nested name templates (`client-fs/src/client.ts:248-260`, `client-github/src/collection/collection.ts:237`), so they cannot read the `queries` fixture at all. That support is tracked as ingitdb/ingitdb-ts#104. AC `go-and-ts-readers-agree-on-fixture` MUST NOT be recorded as passing before #104 and the `ingitdb-ts` body-file reader both land.

## Acceptance Criteria

### AC: reference-definition-loads-strictly

**Requirements:** record-body-file#req:body-file-definition

**Given** a collection definition carrying the reference `record_file` with `body_file: {name: '{key}.query.{body_type}', field: text, type_field: type}` and string columns `type` and `text`
**When** it is read through `validator.ReadDefinition` with the strict decoder
**Then** it loads without error and `RecordFile.BodyFile` holds exactly the declared `name`, `field` and `type_field`

### AC: invalid-body-file-definitions-rejected

**Requirements:** record-body-file#req:body-file-definition-validated

**Given** a table of definitions, one per rule in REQ `body-file-definition-validated`: `type: '[]map[string]any'`; `format: markdown`; record name `all.json`; body name `q.sql` (no `{key}`); body name `{key}/{key}.sql`; body name `{key}.{body_type}.sql`; body name `{key}.{title}`; `{body_type}` without `type_field`; `type_field` without `{body_type}`; `field` naming an undeclared column; `field` naming an `int` column; `field` equal to `type_field`; and a static body name `{key}.query.json` beside record name `{key}/{key}.query.json`
**When** each definition is loaded
**Then** every one fails at load time with an error naming `body_file` and the violated rule, and no record file is read

### AC: read-merges-body-bytes-exactly

**Requirements:** record-body-file#req:read-merges-body-raw, record-body-file#req:body-type-resolution

**Given** the golden fixture's `queries` collection
**When** records `customer-invoices` and `active-customers` are read through the Go reader
**Then** each record's `text` equals its body file's bytes exactly (a trailing newline kept for one, absent for the other, the non-ASCII character intact), and each resolved body name uses the lowercased type (`.query.dtql`, `.query.sql`)

### AC: missing-body-reads-as-absent-field

**Requirements:** record-body-file#req:missing-body-is-absent-field

**Given** the fixture record `draft-query` with no body file, and a second record whose body file exists and is empty
**When** both are read with recovery configured and no pending change
**Then** `draft-query` reads without error and has no `text` key in its data, and the second record's `text` is the empty string

### AC: required-body-missing-is-a-finding-not-a-read-error

**Requirements:** record-body-file#req:missing-body-is-absent-field

**Given** a collection whose `text` column is `required: true` and one record with no body file
**When** the database is validated
**Then** validation reports a missing-required-field finding for `text` on that record, and every other record in the collection is still read and validated

### AC: body-read-failures-are-errors

**Requirements:** record-body-file#req:read-merges-body-raw, record-body-file#req:body-type-resolution

**Given** records whose body file holds invalid UTF-8; whose body path is a filesystem symlink; whose body path is a Git tree entry of mode `120000` read through `dalgo2ingitdb4github`; whose record file also contains a `text` key; whose `type` is `../../x` with `q1/q1.query.sql` present; and whose `type` is empty with `q2/q2.query.sql` present
**When** each is read
**Then** each read fails with an error naming the record key and the body path, and no file outside the record's directory is opened

### AC: unnameable-record-without-body-reads

**Requirements:** record-body-file#req:read-merges-body-raw

**Given** a record `q3` whose `type` is empty and whose directory holds no file matching `q3.query.<segment>` other than its record file
**When** it is read
**Then** it reads without error and has no `text` key

### AC: put-splits-body-out-of-record-file

**Requirements:** record-body-file#req:write-splits-body-out, record-body-file#req:write-plan-exposed

**Given** no stored record, and new data `{type: DTQL, title: X, text: "from: invoices\n"}` for key `q1`
**When** the write plan is computed
**Then** the plan is exactly two operations, in order: write `queries/q1/q1.query.dtql` with bytes `from: invoices\n` and expected prior state absent, then write `queries/q1/q1.query.json` with bytes that contain no `text` key and expected prior state absent; it names `queries/q1` as the containing directory

### AC: put-without-body-removes-existing-body

**Requirements:** record-body-file#req:write-splits-body-out, record-body-file#req:write-plan-exposed

**Given** a stored record `q1` of type `SQL` with a body file, and new data of type `SQL` with a changed title and no `text` key
**When** the write plan is computed
**Then** the plan is, in order: write the record file (expected prior state is the stored bytes' SHA-256), then remove `queries/q1/q1.query.sql` (expected prior state is the stored body's SHA-256)

### AC: non-string-or-unnameable-body-refused-before-any-change

**Requirements:** record-body-file#req:write-splits-body-out, record-body-file#req:body-files-isolated-from-listing

**Given** new data with `text: 42`, new data with `text` present and an empty `type`, and new data with `type: JSON` under record name `{key}/{key}.query.json`
**When** each write plan is computed
**Then** each returns an error and no plan, so a driver applying plans changes no file

### AC: type-change-removes-stale-body

**Requirements:** record-body-file#req:delete-and-type-change-leave-no-stale-body, record-body-file#req:write-plan-exposed

**Given** a stored record `q1` of type `SQL` with `q1/q1.query.sql`, and new data of type `DTQL` with a `text` value
**When** the write plan is computed
**Then** the plan is, in order: write `q1/q1.query.dtql`, write the record file, remove `q1/q1.query.sql`; exactly one body file remains after it is applied

### AC: delete-removes-record-and-body

**Requirements:** record-body-file#req:delete-and-type-change-leave-no-stale-body, record-body-file#req:write-plan-exposed

**Given** a stored record `q1` of type `DTQL` with a body file, and a stored record `q2` of type `SQL` whose body file is already missing
**When** the delete plans are computed and applied
**Then** `q1`'s plan removes the record file and then the body file, `q2`'s plan removes only the record file, both files of `q1` and the record file of `q2` are gone, and neither delete reports an error

### AC: unchanged-put-yields-empty-plan

**Requirements:** record-body-file#req:write-splits-body-out, record-body-file#req:write-plan-exposed

**Given** a stored record whose record file bytes and body file bytes equal what the new data encodes to
**When** the write plan is computed and applied by a driver
**Then** the plan is empty, no file is modified, and `git status` shows a clean tree

### AC: multi-operation-plan-refused-without-atomic-driver

**Requirements:** record-body-file#req:write-plan-exposed

**Given** a working-tree driver that does not yet provide the multi-file atomicity of ingitdb/dalgo2ingitdb#15
**When** it receives a put whose plan has two operations
**Then** it refuses with a typed error and changes no file

### AC: reader-refuses-without-recovery-hook

**Requirements:** record-body-file#req:pending-changes-recovered-before-read

**Given** a working-tree database with a `body_file` collection and a collection without one, and a `datavalidator` and materializer configured with no recovery hook
**When** each validates or reads the database
**Then** reading the `body_file` collection fails with the typed recovery-unavailable error naming it, no record of that collection is read, and the other collection is read as before

### AC: reader-runs-recovery-before-read

**Requirements:** record-body-file#req:pending-changes-recovered-before-read

**Given** a recovery hook that records its invocation and reports a pending change it rolls forward, and a second hook that returns an error
**When** a reader with the first hook reads a `body_file` collection, and a reader with the second hook does the same
**Then** the first hook is invoked before the first record file is opened and the reader sees the post-recovery state, and the second reader fails with the hook's error without reading any record

### AC: revision-covers-body

**Requirements:** record-body-file#req:record-revision-covers-body

**Given** a stored record `q1` and its revision `R`
**When** only the body bytes change; separately, only the type changes (so the body name changes) with identical body bytes; separately, a byte moves from the end of the record file to the start of the body file
**Then** the revision computed from the exported canonical input differs from `R` in every case, including through `dalgo2ingitdb`'s protected profile

### AC: lenient-definition-paths-preserve-body-file

**Requirements:** record-body-file#req:version-skew-fails-safe

**Given** a `dalgo2ingitdb` collection definition declaring `body_file`
**When** it is read through `DescribeCollection`, and separately altered with an unrelated `ApplyAddField`
**Then** neither path drops `body_file`: the described definition carries it, and the rewritten `definition.yaml` still contains it unchanged

### AC: format-version-and-minimums-declared

**Requirements:** record-body-file#req:version-skew-fails-safe

**Given** the `FORMAT.md` and `go.mod` files after this Feature lands
**When** they are read
**Then** `FORMAT.md` declares `v2` and names the minimum `ingitdb-go/ingitdb` and `ingitdb-ts` releases, and `dalgo2ingitdb` and `dalgo2ingitdb4github` require at least that `ingitdb-go/ingitdb` release

### AC: schema-alteration-keeps-body-file-consistent

**Requirements:** record-body-file#req:schema-alteration-keeps-body-file-consistent

**Given** the reference collection
**When** `text` is renamed to `body`; `type` is modified to `int`; and `text` is dropped
**Then** the rename rewrites `body_file.field` to `body` with every body file left in place; the modify and the drop are refused and `definition.yaml` is unchanged

### AC: listing-never-returns-body-files

**Requirements:** record-body-file#req:body-files-isolated-from-listing

**Given** the golden fixture's `queries` collection, and a flat collection with record name `{key}.json` and body name `{key}.body.{body_type}` that holds `q1.json` of type `txt` plus `q1.body.txt`
**When** records are listed and counted by `datavalidator`, the materializer, the foreign-key index, `dalgo2ingitdb` and `dalgo2ingitdb4github`
**Then** `queries` lists exactly `active-customers`, `customer-invoices` and `draft-query`, the flat collection lists exactly `q1`, and no reported key or file path is a body file

### AC: orphan-and-stale-body-files-reported

**Requirements:** record-body-file#req:orphan-body-files-reported

**Given** a `queries` directory holding `q9/q9.query.sql` with no `q9/q9.query.json`, and record `q1` of type `DTQL` with both `q1/q1.query.dtql` and a leftover `q1/q1.query.sql`
**When** the database is validated
**Then** validation reports one orphan-body finding for `q9/q9.query.sql` and one for `q1/q1.query.sql`, and record `q1` reads with the `.query.dtql` body

### AC: definition-without-body-file-unchanged

**Requirements:** record-body-file#req:body-file-definition

**Given** the existing test suite and golden fixtures with no `body_file` declared anywhere
**When** they run after this Feature
**Then** every existing read, write, listing and validation result is unchanged, and no recovery hook is required

### AC: go-and-ts-readers-agree-on-fixture

**Requirements:** record-body-file#req:format-contract-and-golden-fixture, record-body-file#req:ts-layout-prerequisite-recorded

**Given** the `queries` fixture subtree in its nested `{key}/{key}.query.json` layout committed identically in `dalgo2ingitdb`, `ingitdb-ts` (`client-fs` and `client-github`) and `ingitdb-go`, `FORMAT.md` documenting body files, and ingitdb/ingitdb-ts#104 landed
**When** the Go reader test and the `ingitdb-ts` fixture tests read all three query records
**Then** both produce the same field maps: the same `text` bytes for the two bodied records and no `text` key for `draft-query`

## Not Doing (and Why)

- **Designing the multi-file journal and its recovery.** ingitdb/dalgo2ingitdb#15 owns crash-safe multi-file changes. This Feature fixes only what #15 consumes: the ordered, complete write plan with expected prior states, the recovery hook every reader calls, and the refusal of multi-operation plans until #15 lands.
- **Implementing the TypeScript reader or its layout support.** Parity is made testable here through `FORMAT.md` and the shared fixture. The `ingitdb-ts` work is ingitdb/ingitdb-ts#104 plus its own body-file change.
- **More than one body file per record, or body files for list and map record types.** DataTug needs one body per `SingleRecord`. A many-record file has no per-record sibling owner.
- **Binary bodies or any encoding.** Records are text-only (`storage-format` REQ `text-only`). A body is raw UTF-8 text and nothing else.
- **Placing a body file in a different directory from its record file.** A same-directory basename keeps containment, isolation and journal scope simple. No known consumer needs another location.
- **A body-size cap.** `datatug-core` enforces its own cap before calling the store. A storage-level cap can be added later without changing this shape.
- **Repairing a torn state that was already committed to Git.** Recovery runs against the working tree and journal. A user commit of a working tree mid-crash is outside what a reader can detect.

## Rehearse Integration

No Rehearse stubs are scaffolded: `specscore.yaml` declares no rehearse configuration. Every AC is directly executable as a Go test: definition-load table tests through `validator.ReadDefinition`, pure write-plan and revision-input unit tests, temp-dir and golden-fixture tests through the real readers and `datavalidator`, and driver tests in `dalgo2ingitdb` and `dalgo2ingitdb4github`. The one exception is the TypeScript half of `go-and-ts-readers-agree-on-fixture`, which is a vitest in `ingitdb-ts`. This is recorded as an explicit skip rather than an omission.

## Dependent Modules

- **`ingitdb/dalgo2ingitdb`** owns `FORMAT.md` and the fixture. It applies the write plan (refusing multi-operation plans until #15), merges bodies on `Get` and query reads (`record_io.go`, `query.go` `readAllSingleStored`), implements the recovery hook (#15), computes protected revisions from the canonical input (`protected.go`), makes `schema_reader.go` and `schema_modifier.go` definition decodes strict or `body_file`-preserving, and handles field rename, modify and drop. It raises its `ingitdb-go/ingitdb` requirement.
- **`ingitdb/dalgo2ingitdb4github`** merges bodies and excludes body files in `query.go` `scanSingleRecords` (`:172-203`) and in single-record reads, refuses non-regular tree entries (mode `120000`, `160000`), applies each write plan in one commit, and raises its `ingitdb-go/ingitdb` requirement.
- **`ingitdb/ingitdb-ts`** needs ingitdb/ingitdb-ts#104 (`records_dir` and nested templates), then reads body files in `client-fs` and `client-github` and mirrors the fixture.
- **`ingitdb/ingitdb-cli`** writes records through `dalgo2ingitdb`, so it inherits body-file handling. It configures `dalgo2ingitdb`'s recovery hook for validate and materialize. Any direct file write it keeps must use the write plan.
- **`datatug/datatug-core`** declares `body_file` on its queries collection (`dalgo-project-store`) and maps `QueryDef.Text == ""` to an absent `text` field when it has no body.

## Open Questions

1. **Where `FORMAT.md` lives.** It sits in `dalgo2ingitdb` today, but `ingitdb-go` owns the definition schema this Feature extends. Should `FORMAT.md` and the canonical fixture move to `ingitdb-go`, with the other repos mirroring it? Until someone decides, this Feature adds to the existing `dalgo2ingitdb` copy and mirrors the subtree into `ingitdb-go`.
2. **A missing body file: absent field, or read error?** Choose one:
   - (A) The record reads with the body field absent. Authors who need a body mark the column `required: true`, and validation reports it as a finding.
   - (B) A missing body file is a read error for that record.

   **Recommendation: A.** DataTug already has queries without a body: `datatug-core` loads them as empty text and writes no sidecar (`store_queries.go`, `readQueryTextSidecar`). Under B, one such query fails the whole collection listing. A keeps absent (no file) distinct from empty (an empty file), so the read-write round trip is exact. The torn-write risk that B guards against is closed instead by recovery before read (REQ `pending-changes-recovered-before-read`). REQ `missing-body-is-absent-field` is written as A, provisionally.
3. **Body extension: open type value, or closed list?** Choose one:
   - (A) Derive the extension from the record's type value, lowercased and limited to 1–32 characters of `[A-Za-z0-9_-]`.
   - (B) List the allowed types and their extensions in the definition, for example `body_types: {SQL: sql, DTQL: dtql, HTTP: http}`.

   **Recommendation: A.** The set of query types belongs to DataTug's model. Under B, every existing project's definition needs a migration whenever DataTug adds a type. B's one advantage, catching extension collisions at load time, is covered by A's per-value refusal (REQ `body-files-isolated-from-listing`). B can be added later as an optional override without breaking A. REQ `body-type-resolution` is written as A, provisionally.

---
*This document follows the https://specscore.md/feature-specification*
