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

A single-record collection may declare `record_file.body_file`: a companion raw-text file beside each record file that carries one string field. For example, `customer-invoices.query.dtql` sits beside `customer-invoices.query.json`. A per-record type field selects the body file's extension. Readers merge the body into the field byte-for-byte. Writers split the field back out, never duplicate it into the record file, and never leave a stale body behind. Listing never mistakes a body file for a record. The shape is pinned in `FORMAT.md` and a golden fixture, so the Go and TypeScript readers agree. Closes ingitdb-go#24.

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

This mirrors `datatug-core`'s `queryTypeFileExtension`, so existing DataTug projects keep their file names. The type value is record content, and so it is untrusted. Bounding it here is what stops a value like `../../x` from addressing a file outside the collection.

This Feature derives the type openly instead of using a closed `type → extension` map in the definition. The set of query types belongs to DataTug's model, not to the storage definition. A closed map would force a definition migration in every existing project whenever DataTug adds a type. The collision risk that a closed map would catch statically is checked per value instead (REQ `body-files-isolated-from-listing`).

A record with an empty or absent `type_field` value has no body file name. It MUST NOT carry a body. A read looks for no body file, and a write that carries the body field is refused (REQ `write-splits-body-out`).

### Reading

#### REQ: read-merges-body-raw

Every record reader in this module MUST merge a present body file into the record's `field` as a string holding the file's exact bytes. That includes the `datavalidator` pass, the materializer's `records_reader_fs`, and every other `SingleRecord` read path. The reader MUST NOT decode, trim, normalise newlines, or strip a BOM. The merge happens before column validation, so the body field is validated like any other `string` column.

A read MUST fail with an error that names the record and the body path in these cases:

- the body file is not valid UTF-8, since records are text-only per `ingitdb/ingitdb` Feature `storage-format`, REQ `text-only`;
- the body file is not a regular file, such as a directory or a symlink;
- the record file itself contains the `field` key, so the value would have two sources and neither silently wins;
- the record's `type_field` value cannot name a body file (REQ `body-type-resolution`) or resolves to a colliding name (REQ `body-files-isolated-from-listing`), and a body file for that record could exist.

#### REQ: missing-body-is-absent-field

A missing body file MUST NOT be an error. The record MUST read with `field` **absent** from its data. It is not set to the empty string. A present but empty body file MUST read as `field: ""`.

The reasons:

- Not every record has a body. `datatug-core` loads a query without a sidecar as an empty text and writes no sidecar for an empty text (`store_queries.go`, `readQueryTextSidecar`). Existing projects contain such queries, and an error would make one missing file fail a whole collection listing.
- An author who needs a body declares `required: true` on the `field` column. The existing required-field check then reports a missing body as a validation finding, not as a read failure. Strictness stays in the schema, where it belongs.
- Keeping "absent" separate from "empty" makes the read and write round trip exact. An empty body file reads as `""` and writes back as an empty file. A missing file reads as absent and writes back as no file. That keeps `storage-format` REQ `no-rewrite-without-change` true.

### Writing and deleting

#### REQ: write-splits-body-out

A writer putting a record into a collection with `body_file` MUST:

- encode the record file from the record data **without** `field`, so the body is never duplicated into the record file;
- when `field` is present with a string value, write the body file at its resolved name with exactly those bytes and no encoding;
- when `field` is absent or null, write no body file and remove any existing body file for that record;
- refuse the whole put before changing any file when `field` is present but not a string, or when the body name cannot be resolved (REQ `body-type-resolution`, REQ `body-files-isolated-from-listing`);
- leave a body file untouched when its bytes already equal the value being written, per `storage-format` REQ `no-rewrite-without-change`.

#### REQ: delete-and-type-change-leave-no-stale-body

A delete MUST remove the record file and the body file resolved from the stored record's `type_field` value. A body file that is already missing is not an error.

A put that changes the resolved body name MUST remove the body file resolved from the **previously stored** record, as part of the same change. A type change, for example `SQL` → `DTQL`, is one such put. After the put, a record has at most one body file on disk, and it is the one its current type names.

#### REQ: write-plan-exposed

This module MUST export a pure function that computes the full file-level change for a record put or delete, without touching the filesystem. Its inputs are the collection definition, the record key, the previously stored record data (or none), and the new record data (or none for a delete). It returns:

- the record file path and its encoded bytes, or a removal;
- the body file path and its bytes, or a removal, or nothing;
- every stale body path to remove;
- the containing directory, which every path shares.

Writers such as `dalgo2ingitdb` MUST apply this plan instead of re-deriving body names. That gives ingitdb/dalgo2ingitdb#15 (crash-safe writes) one input to journal and apply atomically: at most one record file, one body file and one stale body, all in one directory. This Feature does not specify the journal, the temp-file scheme or the recovery. The plan function is also where REQ `write-splits-body-out` and REQ `delete-and-type-change-leave-no-stale-body` are unit-testable without a driver.

### Isolation

#### REQ: body-files-isolated-from-listing

Listing, globbing and counting records MUST NOT return a body file as a record, or as part of a record key. This applies in `datavalidator` (`singleRecordGlobPattern`, record counts), the materializer and the foreign-key index.

Record discovery keeps using only the record file's name template. Two rules keep the dotted-suffix discriminators unambiguous:

- **Per value:** a resolved body basename that equals the record file's resolved basename, or that matches the record file's basename glob (`{key}` → `*`), MUST be refused on read and on write. Under `{key}/{key}.query.json`, the type `JSON` would resolve to `customer-invoices.query.json`, which is the record file itself, so it is refused. Under a flat `{key}.json` with body `{key}.body.{body_type}`, the type `json` is refused because `q1.body.json` would list as record `q1.body`.
- **Structure:** `{body_type}` holds no dots and is always the final dot segment (REQ `body-file-definition-validated`, REQ `body-type-resolution`). A body basename therefore parses uniquely: the type is the last dot segment, and the key is what lies between the template's fixed prefix and fixed suffix. Two different (key, type) pairs can never produce the same body file name.

`exclude_regex` continues to apply to record files only. It is not needed, and not consulted, to hide body files.

#### REQ: orphan-body-files-reported

Whole-database validation MUST report a finding for each **orphan body file**. An orphan is a file in a record directory that matches the body name template (`{key}` and `{body_type}` → wildcards) and is not a record file, and for which either:

- there is no record file for its key, or
- the record's current `type_field` value resolves to a different body name, such as a stale `.query.sql` left beside a `DTQL` query by a hand edit or a crash.

The finding names the collection and the file path. It is a finding, not a read error: the record itself still reads correctly.

### Cross-language contract

#### REQ: format-contract-and-golden-fixture

The on-disk contract `FORMAT.md` currently lives in `ingitdb/dalgo2ingitdb` (with `testdata/format-fixtures/`). It is mirrored in `ingitdb/ingitdb-ts` at `packages/client-fs/src/__fixtures__/format-fixtures/` and `packages/client-github/src/__fixtures__/format-fixtures/`. `ingitdb-go` has no `FORMAT.md` today. This Feature MUST add a "Body files" section to that `FORMAT.md`, stating the `body_file` keys, body type resolution, the raw merge, absent versus empty, and the isolation rule.

It MUST also add a `queries` root collection to the golden fixture tree, using the reference definition from REQ `body-file-definition`, with:

- `customer-invoices`: `type: DTQL` plus `customer-invoices.query.dtql`, a multi-line body with a trailing newline and a non-ASCII character;
- `active-customers`: `type: SQL` plus `active-customers.query.sql`, a body with **no** trailing newline;
- `draft-query`: `type: SQL` and no body file.

`ingitdb-go` MUST keep a byte-identical copy of that `queries` subtree under `ingitdb/testdata/format-fixtures/` and read it through the real reader in a test. All copies MUST stay identical, which is the existing `FORMAT.md` rule.

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
**When** both are read
**Then** `draft-query` reads without error and has no `text` key in its data, and the second record's `text` is the empty string

### AC: required-body-missing-is-a-finding-not-a-read-error

**Requirements:** record-body-file#req:missing-body-is-absent-field

**Given** a collection whose `text` column is `required: true` and one record with no body file
**When** the database is validated
**Then** validation reports a missing-required-field finding for `text` on that record, and every other record in the collection is still read and validated

### AC: body-read-failures-are-errors

**Requirements:** record-body-file#req:read-merges-body-raw, record-body-file#req:body-type-resolution

**Given** four records: one whose body file holds invalid UTF-8; one whose body path is a symlink; one whose record file also contains a `text` key; and one whose `type` is `../../x`
**When** each is read
**Then** each read fails with an error naming the record key and the body file path, and no file outside the record's directory is opened

### AC: put-splits-body-out-of-record-file

**Requirements:** record-body-file#req:write-splits-body-out, record-body-file#req:write-plan-exposed

**Given** no stored record, and new data `{type: DTQL, title: X, text: "from: invoices\n"}` for key `q1`
**When** the write plan is computed
**Then** the plan writes `q1/q1.query.json` with bytes that contain no `text` key, writes `q1/q1.query.dtql` with exactly `from: invoices\n`, removes nothing, and names `q1/` as the single containing directory

### AC: put-without-body-removes-existing-body

**Requirements:** record-body-file#req:write-splits-body-out

**Given** a stored record `q1` of type `SQL` with a body file, and new data of type `SQL` with no `text` key
**When** the write plan is computed
**Then** the plan writes the record file and removes `q1/q1.query.sql`

### AC: non-string-or-unnameable-body-refused-before-any-change

**Requirements:** record-body-file#req:write-splits-body-out, record-body-file#req:body-files-isolated-from-listing

**Given** new data with `text: 42`, new data with `text` present and an empty `type`, and new data with `type: JSON` under record name `{key}/{key}.query.json`
**When** each write plan is computed
**Then** each returns an error and no plan, so a driver applying plans changes no file

### AC: type-change-removes-stale-body

**Requirements:** record-body-file#req:delete-and-type-change-leave-no-stale-body

**Given** a stored record `q1` of type `SQL` with `q1/q1.query.sql`, and new data of type `DTQL` with a `text` value
**When** the write plan is computed
**Then** the plan writes `q1/q1.query.dtql` and removes `q1/q1.query.sql`, so exactly one body file remains after it is applied

### AC: delete-removes-record-and-body

**Requirements:** record-body-file#req:delete-and-type-change-leave-no-stale-body

**Given** a stored record `q1` of type `DTQL` with a body file, and a stored record `q2` of type `SQL` whose body file is already missing
**When** the delete plans are computed and applied
**Then** both the record file and the body file of `q1` are gone, `q2`'s record file is gone, and neither delete reports an error

### AC: unchanged-body-not-rewritten

**Requirements:** record-body-file#req:write-splits-body-out

**Given** a stored record whose body file bytes equal the new `text` value
**When** the record is written back unchanged through a driver that applies the plan
**Then** neither the body file nor the record file is modified, and `git status` shows a clean tree

### AC: listing-never-returns-body-files

**Requirements:** record-body-file#req:body-files-isolated-from-listing

**Given** the golden fixture's `queries` collection, and a flat collection with record name `{key}.json` and body name `{key}.body.{body_type}` that holds `q1.json` of type `txt` plus `q1.body.txt`
**When** records are listed and counted by `datavalidator`, the materializer and the foreign-key index
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
**Then** every existing read, write, listing and validation result is unchanged

### AC: go-and-ts-readers-agree-on-fixture

**Requirements:** record-body-file#req:format-contract-and-golden-fixture

**Given** the `queries` fixture subtree committed identically in `dalgo2ingitdb`, `ingitdb-ts` (`client-fs` and `client-github`) and `ingitdb-go`, and `FORMAT.md` documenting body files
**When** the Go reader test and the `ingitdb-ts` fixture tests read all three query records
**Then** both produce the same field maps: the same `text` bytes for the two bodied records and no `text` key for `draft-query`

## Not Doing (and Why)

- **Crash-safe writing of the record and body as one change.** That is ingitdb/dalgo2ingitdb#15. This Feature only exposes the single-directory write plan the journal consumes (REQ `write-plan-exposed`).
- **Implementing the TypeScript reader.** Parity is made testable here through `FORMAT.md` and the shared fixture. The `ingitdb-ts` change is its own PR in that repo.
- **More than one body file per record, or body files for list and map record types.** DataTug needs one body per `SingleRecord`. A many-record file has no per-record sibling owner.
- **A closed `type → extension` map in the definition.** The open, bounded derivation keeps DataTug's type set out of storage definitions (REQ `body-type-resolution`). A closed map can be added later as an optional key without breaking this shape.
- **Binary bodies or any encoding.** Records are text-only (`storage-format` REQ `text-only`). A body is raw UTF-8 text and nothing else.
- **Placing a body file in a different directory from its record file.** A same-directory basename keeps containment, isolation and atomic-rename scope simple. No known consumer needs another location.
- **A body-size cap.** `datatug-core` enforces its own cap before calling the store. A storage-level cap can be added later without changing this shape.

## Rehearse Integration

No Rehearse stubs are scaffolded: `specscore.yaml` declares no rehearse configuration. Every AC is directly executable as a Go test: definition-load table tests through `validator.ReadDefinition`, pure write-plan unit tests, and temp-dir and golden-fixture tests through the real readers and `datavalidator`. The one exception is the TypeScript half of `go-and-ts-readers-agree-on-fixture`, which is a vitest in `ingitdb-ts`. This is recorded as an explicit skip rather than an omission.

## Dependent Modules

- **`ingitdb/dalgo2ingitdb`** applies the write plan for put and delete, merges bodies on `Get` and query reads (`record_io.go`, `query.go` `readAllSingleStored`), and owns `FORMAT.md` and the fixture. Crash atomicity is #15.
- **`ingitdb/ingitdb-ts`** reads body files in `client-fs` and `client-github` and mirrors the fixture.
- **`ingitdb/ingitdb-cli`** writes records through `dalgo2ingitdb`, so it inherits body-file handling. Any direct file write it keeps must use the write plan.
- **`datatug/datatug-core`** declares `body_file` on its queries collection (`dalgo-project-store`) and maps `QueryDef.Text == ""` to an absent `text` field when it has no body.

## Open Questions

- **Where `FORMAT.md` lives.** It sits in `dalgo2ingitdb` today, but `ingitdb-go` owns the definition schema this Feature extends. Should `FORMAT.md` and the canonical fixture move to `ingitdb-go`, with the other repos mirroring it? Until someone decides, this Feature adds to the existing `dalgo2ingitdb` copy and mirrors the subtree into `ingitdb-go`.
- **Legacy DataTug `JSON`-typed queries.** Under `{key}/{key}.query.json`, a query of type `JSON` would name its body `<id>.query.json`, which is the record file itself, so REQ `body-files-isolated-from-listing` refuses it. DataTug's demo projects contain `JSON`-typed queries. Should DataTug migrate those queries to another type, or should the definition gain an explicit per-type override? This is a DataTug data decision, not a storage one.

---
*This document follows the https://specscore.md/feature-specification*
