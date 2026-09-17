---
format: https://specscore.md/feature-specification
status: Draft
---

# Feature: Record companion body files

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/record-body-file?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/record-body-file?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/record-body-file?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/record-body-file?op=request-change) |
**Status:** Draft
**Date:** 2026-09-17
**Owner:** alex
**Supersedes:** —
**Source Ideas:** —

## Summary

A single-record collection may declare `record_file.body_files`: one or more string fields, each stored as a raw-text file beside the record file. The file name is fixed, not configurable: `<key>.<record-suffix>.<field>.<body-type>`, for example `customer-invoices.query.text.dtql` beside `customer-invoices.query.json`. The body type comes from a per-record type field. Readers merge each body into its field byte-for-byte. Writers split the fields back out through an exported, ordered write plan. They never duplicate a body into the record file or leave a stale body behind. Crash-safe multi-file application is owned by ingitdb/dalgo2ingitdb#15, and every reader runs that driver's recovery first. Definition validation keeps record templates and body file names disjoint, so listing never mistakes a body file for a record. The shape is pinned in `FORMAT.md` and a golden fixture, so the Go and TypeScript readers agree. Closes ingitdb-go#24.

## Problem

Today a record is exactly one file. DataTug keeps a query as a JSON record plus a raw body file beside it. The founder decided on 2026-09-17 to keep that pair under the DALgo-backed project store (`datatug/datatug` Feature `dalgo-project-store`, REQ `driver-prerequisites-are-recorded`).

The record file is already expressible: `name: '{key}/{key}.query.json'`, `format: json`, `type: map[string]any`, `records_dir: '.'`. The body file is not. Its extension also varies per record, because it depends on the query type: `sql` for one query and `dtql` for another (`datatug-core/pkg/storage/filestore/query_content.go`, `queryTypeFileExtension`). Without this Feature, DataTug must either inline the body as an escaped JSON string, which is unreadable in a diff, or keep a sidecar reader beside the driver. The second option is the machinery the cut-over is meant to retire.

Two existing mechanisms come close, and neither fits:

- **`format: markdown` + `content_field`** gives a body, but the body shares one file with YAML frontmatter. DataTug's record is JSON and its body is DTQL or SQL, not Markdown.
- **`{fieldName}` placeholders** in `record_file.name` (Feature `record-file-name-placeholders`) substitute a column value into the record file's own name. They name one file, not a second one. They are also case-preserving and have no character-set bound, and the body type needs both a case rule and a bound (REQ `body-type-resolution`).

## Behavior

### Definition and naming

#### REQ: body-files-definition

`RecordFileDef` MUST accept an optional `body_files` list. Its keys are modelled fields, so the strict (`KnownFields(true)`) definition reader accepts them. Each entry has exactly two keys:

| Key | Required | Meaning |
|-----|----------|---------|
| `field` | yes | The record column that this body file carries. |
| `type_field` | yes | The record column whose value selects this body's type. Several entries MAY share one `type_field`. |

There is no name template. The founder decided on 2026-09-17 that body file names are fixed (REQ `body-file-naming`).

The DataTug queries collection is the reference definition:

```yaml
record_file:
  name: '{key}/{key}.query.json'
  format: json
  type: map[string]any
  records_dir: '.'
  body_files:
    - field: text        # datatug-core QueryDef.Text is `json:"text"`
      type_field: type
columns:
  type:
    type: string
    required: true
  text:
    type: string
```

When `body_files` is omitted or empty, behavior MUST be byte-for-byte what it is today.

#### REQ: body-file-naming

A body file MUST be named `<key>.<record-suffix>.<field>.<body-type>` and MUST sit in the **same directory as its record file**. The parts are:

- `<key>` is the record key, exactly as it is substituted into the record file name;
- `<record-suffix>` is the record template's dotted type suffix: in a record basename template `{key}.<record-suffix>.<ext>`, the single segment between `{key}.` and `.<ext>` (`query` in `{key}.query.json`);
- `<field>` is the entry's `field` name, verbatim;
- `<body-type>` is resolved per record by REQ `body-type-resolution`.

For example:

```
queries/customer-invoices/customer-invoices.query.json
queries/customer-invoices/customer-invoices.query.text.dtql
queries/customer-invoices/customer-invoices.query.notes.md
```

Placing the body in the record's own directory keeps a record and its bodies under one containment check and one directory for the journal (REQ `write-plan-exposed`). Including `<field>` gives every body field of a record its own file name. `<field>` and `<body-type>` never contain a dot, so a body file name parses one way only: the last segment is the body type, the one before it is the field, and the one before that is the record suffix. The key is everything in front. Two different (key, field, type) triples can therefore never produce the same name.

#### REQ: body-files-definition-validated

`RecordFileDef.Validate()` (with column-aware checks in `CollectionDef.Validate()`) MUST reject each of the following at definition-load time, before any record path is read or written:

1. `body_files` on a record type other than `map[string]any` (`SingleRecord`). A list or map file holds many records, so a per-record sibling file has no single owner.
2. `body_files` with `format: markdown`. A Markdown record already has a body (`content_field`), and two body carriers for one record would be ambiguous.
3. `body_files` when the record basename template is not exactly `{key}.<record-suffix>.<ext>`, where `<record-suffix>` and `<ext>` are each one non-empty segment of `[a-z0-9_-]`. Such a template can be preceded by directories, as in `{key}/{key}.query.json`. A bare `{key}.json`, or a name with no `{key}` in its basename, has no record suffix to name bodies by.
4. An entry with an empty `field` or `type_field`, or with any key other than those two.
5. A `field` that does not match `^[a-z0-9_-]{1,64}$`. A body file name must be a safe, dot-free segment, and lowercase-only names cannot collide on a case-insensitive filesystem.
6. A `field` or `type_field` that is not a declared column of type `string`. A computed (`formula`) column as `field` or `type_field`. A `field` equal to any entry's `type_field`. The same `field` in two entries.
7. A `field` equal to the collection's own `<record-suffix>`. With suffix `query`, field `query` would name `x.query.query.json` for body type `json`, and that name matches the record glob `*.query.json`.

#### REQ: suffix-disjointness-covers-body-files

Whole-definition validation (`validator.ReadDefinition`, after every collection and subcollection is loaded) MUST reject a definition in which any collection's record template could match a body file name. This extends the dotted-suffix rule that `dalgo-project-store` REQ `type-suffix-is-self-identifying` states for record templates alone.

Formally, take a body-bearing collection D with record suffix `s` and a body field `f`. Its body file names end in `.s.f.<t>`, where `<t>` is any dot-free segment. Take any collection C whose records base directory is D's record-file directory or an ancestor of it. The rule includes C = D, and includes ancestors because the GitHub reader lists a collection directory recursively. Write the part of C's record basename template after its last `{key}` as `.x1.….xn`. Then C conflicts with D's field `f` when:

- n = 1: a bare `.ext` matches a body whose type equals `ext`; or
- n = 2 and x1 = `f`: for example C's `{key}.text.json` matches `x.query.text.json`; or
- n ≥ 3, x(n-1) = `f` and x(n-2) = `s`.

The body type is an open per-record value, so any `ext` can occur. That is why the check is on the name segments alone and ignores `ext`. The error names both collections, the record template and the body field.

With the rule enforced, a body file cannot match any record matcher, neither the local glob (`{key}` → `*`) nor the GitHub template regex (`dalgo2ingitdb4github/query.go` `buildKeyExtractor`). For example, `x.query.text.json` matches neither `*.query.json` nor `\.query\.json$`.

#### REQ: body-type-resolution

The body type for an entry MUST be the record's `type_field` value with ASCII letters lowercased. For example, `SQL` resolves to `sql`, `DTQL` to `dtql` and `JSON` to `json`. The value MUST be 1 to 32 characters, each an ASCII letter, digit, `_` or `-`. The founder decided on 2026-09-17 that the body type is the lowercased type value.

This is the same bounded derivation as `datatug-core`'s `queryTypeFileExtension`. The resulting file names differ from today's DataTug sidecars (see REQ `datatug-cut-over-referenced`). The type value is record content, and so it is untrusted. Bounding it here stops a value like `../../x` from addressing a file outside the collection.

A `JSON`-typed body is allowed. Its name, `x.query.text.json`, is disjoint from the record glob by REQ `suffix-disjointness-covers-body-files`.

An entry is **unnameable** for a record when that record's `type_field` value is empty, absent or outside the allowed set. A write that carries an unnameable entry's field is refused (REQ `write-splits-bodies-out`). The read rule for an unnameable entry is in REQ `read-merges-bodies-raw`.

### Reading

#### REQ: read-merges-bodies-raw

Every record reader MUST merge each present body file into its entry's `field`, as a string holding the file's exact bytes. That includes this module's `datavalidator` pass, the materializer's `records_reader_fs` and every other `SingleRecord` read path, plus the drivers listed under Dependent Modules. The reader MUST NOT decode, trim, normalise newlines, or strip a BOM. The merge happens before column validation, so each body field is validated like any other `string` column.

The **candidate body files** of entry `f` for a record with key `K` are the files in the record's directory named `K.<record-suffix>.f.<t>`, for any dot-free segment `<t>`.

A read MUST fail with an error that names the record key, the field and the body path in these cases:

- the body file is not valid UTF-8, since records are text-only per `ingitdb/ingitdb` Feature `storage-format`, REQ `text-only`;
- the body path is not a regular file: a directory or symlink on a filesystem, or a Git tree entry whose mode is not `100644` or `100755` (for example `120000`, a symlink, or `160000`, a submodule);
- the record file itself contains the entry's `field` key, so the value would have two sources and neither silently wins;
- the entry is unnameable for that record **and** at least one candidate body file for that entry is present.

An unnameable entry with no candidate body file reads with the same result as a missing body (REQ `missing-body-is-absent-field`).

#### REQ: missing-body-is-absent-field

**Provisional until Open Question 2 is decided.** A missing body file MUST NOT be an error. The record MUST read with that `field` **absent** from its data, not set to the empty string. A present but empty body file MUST read as `field: ""`.

A missing body can only mean "no body" when no multi-file change is pending. REQ `pending-changes-recovered-before-read` guarantees that, so a torn write is never read as a valid record without a body.

### Writing and deleting

#### REQ: write-splits-bodies-out

For each entry in `body_files`, a writer putting a record MUST:

- encode the record file from the record data **without** any entry's `field`, so no body is duplicated into the record file;
- when the field is present with a string value, write its body file with exactly those bytes and no encoding;
- when the field is absent or null, write no body file for it and remove any existing body file for that entry;
- refuse the whole put, before changing any file, when a body field is present but is not a string, or when a present body field's entry is unnameable;
- leave a file untouched when its bytes already equal the bytes being written, per `storage-format` REQ `no-rewrite-without-change`.

#### REQ: delete-and-type-change-leave-no-stale-body

A delete MUST remove the record file and every body file resolved from the stored record's type values. A body file that is already missing is not an error.

A put that changes an entry's resolved body name MUST remove that entry's body file resolved from the **previously stored** record, as part of the same change. A change of `type` from `SQL` to `DTQL` is one such put. After the put, each entry of a record has at most one body file on disk, and it is the one the record's current type names.

#### REQ: write-plan-exposed

This module MUST export a pure function that computes the full file-level change for a record put or delete, without touching the filesystem. Its inputs are the collection definition, the record key, the previously stored state (the record file bytes and each body file's bytes, or none), and the new record data (or none for a delete).

It returns an ordered list of operations. The list is the **exact and complete** set of files the change touches: applying exactly these operations, in this order, yields the new state, and no other file may be written or removed. Each operation carries:

- its kind, `write` or `remove`;
- its path, slash-separated and relative to the database root;
- for `write`, the full new bytes;
- the **expected prior state** of that path: absent, or present with the SHA-256 of its bytes. A journal can use this to detect a concurrent change, and to tell during recovery whether an operation has already been applied.

The order is fixed:

- **Put:** (1) write each body file whose name or bytes change, in `body_files` order; (2) write the record file, if its bytes change; (3) remove each stale body file, in `body_files` order.
- **Delete:** (1) remove the record file; (2) remove each present body file, in `body_files` order.

An unchanged put returns an empty list. Every path in a plan shares one containing directory, and the plan names that directory.

Writers MUST apply this plan instead of re-deriving body names. ingitdb/dalgo2ingitdb#15 owns crash-safe application of multi-file changes: its durable journal, its recovery, and the atomicity of a record and its bodies as one unit. This Feature does not design that journal. It does require that until a working-tree driver provides that atomicity, the driver MUST refuse, with a typed error and before changing any file, any put or delete whose plan has more than one operation. Ordering alone does not make the files consistent. A driver that commits a Git tree in one commit, such as `dalgo2ingitdb4github`, is already atomic and applies the whole plan in that commit.

#### REQ: pending-changes-recovered-before-read

A torn multi-file change must never be read as a valid state. This module MUST export a pending-change recovery hook: an interface that a driver implements, for example `RecoverPendingChanges(ctx, dbRoot) error`. The hook exists because `ingitdb-go` cannot import `dalgo2ingitdb`, the owner of the journal (ingitdb/dalgo2ingitdb#15).

Every reader of a working-tree database MUST do the following before it reads any record of a collection that declares `body_files`:

- If a recovery hook is configured, invoke it, and proceed only when it returns without error. On error, fail with that error.
- If no hook is configured, refuse with a typed error (for example `ErrPendingChangeRecoveryUnavailable`) that names the collection. The reader MUST NOT read records without the hook.

This applies to the `datavalidator` pass, the materializer's records reader, the foreign-key index, and the drivers' own reads. `ingitdb-cli` MUST configure `dalgo2ingitdb`'s hook for its validate and materialize commands.

Reads of a committed Git tree, such as `dalgo2ingitdb4github` or a read at a commit, are exempt. A commit is written atomically, and no journal is attached to it. Collections without `body_files` are unaffected: their changes touch one file each, and #15 makes those atomic by rename.

#### REQ: record-revision-covers-bodies

Any revision, etag or change token that `ingitdb-go` or a driver computes for a record in a collection that declares `body_files` MUST change whenever any of the following changes:

- the record file's existence or bytes;
- the resolved name of any body file, or its absence;
- the existence or bytes of any body file.

This module MUST export the canonical revision input: the byte sequence to hash, with the record file first and then each body in `body_files` order. Each part is length-prefixed, so bytes moving between files also change the input. Drivers hash that sequence, with an HMAC where they need one, and do not frame the parts themselves. Today `dalgo2ingitdb/protected.go` (`revision`, `:482-494`) hashes the record file bytes only, so an edit to a body alone would not change the revision. `datatug-core` already hashes both (`query_revision.go`).

### Isolation

#### REQ: body-files-isolated-from-listing

Listing, globbing and counting records MUST NOT return a body file as a record, or as part of a record key. This applies in `datavalidator` (`singleRecordGlobPattern`, record counts), the materializer, the foreign-key index, `dalgo2ingitdb` (`query.go` `readAllSingleStored`) and `dalgo2ingitdb4github` (`query.go` `scanSingleRecords`).

Record discovery keeps using only record templates. Isolation is guaranteed statically by REQ `body-files-definition-validated` rule 7 and REQ `suffix-disjointness-covers-body-files`, not by per-value refusal. `exclude_regex` continues to apply to record files only. It is not needed, and not consulted, to hide body files.

#### REQ: orphan-body-files-reported

Whole-database validation MUST report a finding for each **orphan body file**. An orphan is a file in a body-bearing collection's record directory named `<k>.<record-suffix>.<f>.<t>`, where `f` is a declared body field, for which either:

- there is no record file for key `<k>`, or
- the record's current type value for that entry resolves to a type other than `<t>`, such as a stale `.query.text.sql` left beside a `DTQL` query by a hand edit.

The finding names the collection and the file path. A stale body beside a nameable entry is a finding, not a read error: the record still reads its current body. The unnameable case is a read error, per REQ `read-merges-bodies-raw`.

### Compatibility

#### REQ: version-skew-fails-safe

A reader or writer that does not understand `body_files` must never silently mishandle a collection that declares it. Such a mistake would write `text` into the record file, leave stale bodies, or drop `body_files` from the definition.

- **Format version.** `FORMAT.md` MUST bump its version from `v1` to `v2`. A collection that declares `body_files` is `v2` content.
- **Minimum versions.** The first `ingitdb-go/ingitdb` release that implements this Feature is the minimum for `body_files`. `dalgo2ingitdb` and `dalgo2ingitdb4github`, both of which currently require `ingitdb-go/ingitdb` v0.5.2, MUST raise their requirement to that release. `FORMAT.md` MUST name that release and the first `ingitdb-ts` release that reads body files.
- **Strict decoding.** Strict older readers, such as `ingitdb-go` releases up to v0.7.0 that use `KnownFields(true)`, already reject a definition that contains `body_files`, and that is the intended fail-safe. Every lenient definition decode in the drivers MUST become strict (`KnownFields(true)`), or MUST preserve `body_files` unchanged. The known lenient decodes are `dalgo2ingitdb`'s `schema_reader.go:105` (`DescribeCollection`) and `schema_modifier.go:445` (`readCollectionDefYAML`).
- **Definition round trip.** A collection alteration that decodes and rewrites the definition (`writeCollectionDefYAML`, `schema_modifier.go:427`) MUST round-trip `body_files` byte-for-byte unless the alteration itself updates it (REQ `schema-alteration-keeps-body-files-consistent`).

#### REQ: schema-alteration-keeps-body-files-consistent

A schema alteration on a collection that declares `body_files` MUST leave a definition that passes REQ `body-files-definition-validated` and REQ `suffix-disjointness-covers-body-files`. Drivers MUST validate the whole altered definition before writing it. The rules for each operation:

- `ApplyRenameField` of a `type_field` column MUST update every entry that references it, in the same definition write. No file is renamed, because body file names do not contain the type field's name.
- `ApplyRenameField` of a body `field` column MUST be refused. The field name is part of every body file's name, so the rename would rename one file per record, a collection-wide multi-file change that this Feature does not specify.
- `ApplyModifyField` that would make a body `field` or `type_field` column non-`string` or computed MUST be refused before any write.
- `ApplyDropField` of a column that any entry references MUST be refused.
- Any alteration that changes `record_file.name` on a collection with `body_files` MUST be refused, because it would change the record suffix in every body file name.

#### REQ: datatug-cut-over-referenced

DataTug's existing sidecars are named `<id>.query.<type>`, for example `.query.dtql` and `.query.sql`. Under this Feature they become `<id>.query.text.<type>`. The rename is DataTug's concern, under the hard cut-over of `datatug/datatug` Feature `dalgo-project-store` (section "Hard cut-over", REQ `filestore-is-cut-over`). This Feature MUST NOT read the old names as bodies. A leftover `<id>.query.sql` is not a body file and is not a record, since it does not match `*.query.json`.

### Cross-language contract

#### REQ: format-contract-and-golden-fixture

The on-disk contract `FORMAT.md` currently lives in `ingitdb/dalgo2ingitdb` (with `testdata/format-fixtures/`). It is mirrored in `ingitdb/ingitdb-ts` at `packages/client-fs/src/__fixtures__/format-fixtures/` and `packages/client-github/src/__fixtures__/format-fixtures/`. `ingitdb-go` has no `FORMAT.md` today.

This Feature MUST change `FORMAT.md`'s title version to `v2` and add a "Body files" section. The section's content MUST state the `body_files` keys, the naming rule, body type resolution, the raw merge, absent versus empty, the disjointness rule, the write plan order, the recovery-before-read rule and the minimum versions. It MUST include this example tree, a `records_dir: '.'` collection with a `{key}/{key}` template:

```
<root>/
  .ingitdb/
    root-collections.yaml                         # queries: queries
  queries/
    .collection/
      definition.yaml                             # record_file:
                                                  #   name: '{key}/{key}.query.json'
                                                  #   format: json
                                                  #   type: map[string]any
                                                  #   records_dir: '.'
                                                  #   body_files:
                                                  #     - {field: text, type_field: type}
                                                  #     - {field: notes, type_field: notes_format}
    customer-invoices/                            # records_dir '.': no $records segment
      customer-invoices.query.json                # {"title": ..., "type": "DTQL", "notes_format": "md"}
      customer-invoices.query.text.dtql           # raw body of field `text`
      customer-invoices.query.notes.md            # raw body of field `notes`
    active-customers/
      active-customers.query.json                 # {"type": "SQL"}
      active-customers.query.text.sql
    draft-query/
      draft-query.query.json                      # {"type": "SQL"}: no body file, so `text` is absent
```

It MUST also add that `queries` collection to the golden fixture tree, with these contents:

- `customer-invoices.query.text.dtql`: a multi-line body with a trailing newline and a non-ASCII character;
- `customer-invoices.query.notes.md`: a non-empty Markdown body;
- `active-customers.query.text.sql`: a body with **no** trailing newline;
- `draft-query`: no body file.

The fixture MUST NOT fall back to a flat `$records/` layout. `ingitdb-go` MUST keep a byte-identical copy of that `queries` subtree under `ingitdb/testdata/format-fixtures/` and read it through the real reader in a test. All copies MUST stay identical, which is the existing `FORMAT.md` rule.

#### REQ: ts-layout-prerequisite-recorded

TypeScript parity depends on a prerequisite that does not exist yet. Today both `ingitdb-ts` clients hard-code a flat `$records/` layout. They ignore `records_dir` and nested name templates (`client-fs/src/client.ts:248-260`, `client-github/src/collection/collection.ts:237`), so they cannot read the `queries` fixture at all. That support is tracked as ingitdb/ingitdb-ts#104. AC `go-and-ts-readers-agree-on-fixture` MUST NOT be recorded as passing before #104 and the `ingitdb-ts` body-file reader both land.

## Acceptance Criteria

### AC: reference-definition-loads-strictly

**Requirements:** record-body-file#req:body-files-definition

**Given** a collection definition carrying the reference `record_file` with `body_files: [{field: text, type_field: type}]` and string columns `type` and `text`
**When** it is read through `validator.ReadDefinition` with the strict decoder
**Then** it loads without error and `RecordFile.BodyFiles` holds exactly one entry with `field: text` and `type_field: type`

### AC: invalid-body-files-definitions-rejected

**Requirements:** record-body-file#req:body-files-definition-validated

**Given** a table of definitions, one per rule in REQ `body-files-definition-validated`:
- `type: '[]map[string]any'`;
- `format: markdown`;
- record name `{key}.json` (no suffix);
- record name `{key}.a.b.json` (a multi-segment suffix);
- an entry carrying a `name` key;
- an entry with no `type_field`;
- `field: Text`;
- `field: te.xt`;
- `field` naming an undeclared column;
- `field` naming an `int` column;
- `field` equal to a `type_field`;
- two entries with `field: text`;
- `field: query` under record name `{key}/{key}.query.json`

**When** each definition is loaded
**Then** every one fails at load time with an error naming `body_files` and the violated rule, and no record file is read

### AC: body-name-and-record-template-overlap-rejected

**Requirements:** record-body-file#req:suffix-disjointness-covers-body-files

**Given** the reference `queries` collection (suffix `query`, body field `text`) in three databases. In the first, a collection `texts` with record name `{key}.text.json` has its records base directory at `queries/`. In the second, the parent `projects` collection has the bare record name `{key}.yaml` and `queries` is its subcollection. In the third, `texts` uses `{key}.snippet.json` in the same place.
**When** each definition is read through `validator.ReadDefinition`
**Then** the first fails, naming `texts`, `{key}.text.json`, `queries` and `text`; the second fails, naming `projects` and `queries`; and the third loads without error

### AC: read-merges-body-bytes-exactly

**Requirements:** record-body-file#req:read-merges-bodies-raw, record-body-file#req:body-type-resolution, record-body-file#req:body-file-naming

**Given** the golden fixture's `queries` collection
**When** records `customer-invoices` and `active-customers` are read through the Go reader
**Then** each record's `text` equals the bytes of `<key>.query.text.<lowercased type>` exactly, with the trailing newline kept for one, absent for the other, and the non-ASCII character intact

### AC: several-body-fields-per-record

**Requirements:** record-body-file#req:body-file-naming, record-body-file#req:read-merges-bodies-raw, record-body-file#req:write-plan-exposed

**Given** the fixture record `customer-invoices`, with `type: DTQL` and `notes_format: md`, and bodies `customer-invoices.query.text.dtql` and `customer-invoices.query.notes.md`
**When** it is read, and a put that changes both `text` and `notes` has its write plan computed
**Then** the read sets `text` and `notes` each to its own file's exact bytes; the plan writes `.query.text.dtql`, then `.query.notes.md`, then the record file, whose bytes contain neither key

### AC: missing-body-reads-as-absent-field

**Requirements:** record-body-file#req:missing-body-is-absent-field

**Given** the fixture record `draft-query` with no body file, and a second record whose `.query.text.sql` exists and is empty
**When** both are read with recovery configured and no pending change
**Then** `draft-query` reads without error and has no `text` key in its data, and the second record's `text` is the empty string

### AC: required-body-missing-is-a-finding-not-a-read-error

**Requirements:** record-body-file#req:missing-body-is-absent-field

**Given** a collection whose `text` column is `required: true` and one record with no body file
**When** the database is validated
**Then** validation reports a missing-required-field finding for `text` on that record, and every other record in the collection is still read and validated

### AC: body-read-failures-are-errors

**Requirements:** record-body-file#req:read-merges-bodies-raw, record-body-file#req:body-type-resolution

**Given** these records:
- one whose `.query.text.sql` holds invalid UTF-8;
- one whose body path is a filesystem symlink;
- one whose body path is a Git tree entry of mode `120000`, read through `dalgo2ingitdb4github`;
- one whose record file also contains a `text` key;
- one whose `type` is `../../x`, with `q1/q1.query.text.sql` present;
- one whose `type` is empty, with `q2/q2.query.text.sql` present

**When** each is read
**Then** each read fails with an error naming the record key, the field and the body path, and no file outside the record's directory is opened

### AC: unnameable-record-without-body-reads

**Requirements:** record-body-file#req:read-merges-bodies-raw

**Given** a record `q3` whose `type` is empty, and a directory that holds no `q3.query.text.<segment>` file
**When** it is read
**Then** it reads without error and has no `text` key

### AC: put-splits-body-out-of-record-file

**Requirements:** record-body-file#req:write-splits-bodies-out, record-body-file#req:write-plan-exposed

**Given** no stored record, and new data `{type: DTQL, title: X, text: "from: invoices\n"}` for key `q1`
**When** the write plan is computed
**Then** the plan is exactly two operations, in order:
1. write `queries/q1/q1.query.text.dtql`, with bytes `from: invoices\n` and expected prior state absent;
2. write `queries/q1/q1.query.json`, with bytes that contain no `text` key and expected prior state absent.

It names `queries/q1` as the containing directory.

### AC: json-typed-body-allowed-and-isolated

**Requirements:** record-body-file#req:body-type-resolution, record-body-file#req:body-files-isolated-from-listing

**Given** new data `{type: JSON, text: "{}"}` for key `q4` in the reference collection
**When** the write plan is computed and applied, and the collection is then listed by `dalgo2ingitdb` (glob `*/*.query.json`) and by `dalgo2ingitdb4github` (template regex ending `\.query\.json$`)
**Then** the plan writes `queries/q4/q4.query.text.json`; both listings return `q4` exactly once, and neither returns a key or path for `q4.query.text.json`

### AC: put-without-body-removes-existing-body

**Requirements:** record-body-file#req:write-splits-bodies-out, record-body-file#req:write-plan-exposed

**Given** a stored record `q1` of type `SQL` with a body file, and new data of type `SQL` with a changed title and no `text` key
**When** the write plan is computed
**Then** the plan is, in order:
1. write the record file, with expected prior state the SHA-256 of the stored bytes;
2. remove `queries/q1/q1.query.text.sql`, with expected prior state the SHA-256 of the stored body.

### AC: non-string-or-unnameable-body-refused-before-any-change

**Requirements:** record-body-file#req:write-splits-bodies-out

**Given** new data with `text: 42`, and new data with `text` present and an empty `type`
**When** each write plan is computed
**Then** each returns an error and no plan, so a driver applying plans changes no file

### AC: type-change-removes-stale-body

**Requirements:** record-body-file#req:delete-and-type-change-leave-no-stale-body, record-body-file#req:write-plan-exposed

**Given** a stored record `q1` of type `SQL` with `q1/q1.query.text.sql`, and new data of type `DTQL` with a `text` value
**When** the write plan is computed
**Then** the plan is, in order:
1. write `q1/q1.query.text.dtql`;
2. write the record file;
3. remove `q1/q1.query.text.sql`.

Exactly one `text` body file remains after the plan is applied.

### AC: delete-removes-record-and-bodies

**Requirements:** record-body-file#req:delete-and-type-change-leave-no-stale-body, record-body-file#req:write-plan-exposed

**Given** a stored `customer-invoices` with both body files, and a stored record `q2` of type `SQL` whose body file is already missing
**When** the delete plans are computed and applied
**Then** `customer-invoices`'s plan removes the record file, then `.query.text.dtql`, then `.query.notes.md`; `q2`'s plan removes only the record file; all those files are gone; and neither delete reports an error

### AC: unchanged-put-yields-empty-plan

**Requirements:** record-body-file#req:write-splits-bodies-out, record-body-file#req:write-plan-exposed

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

**Given** a working-tree database with a `body_files` collection and a collection without one, and a `datavalidator` and materializer configured with no recovery hook
**When** each validates or reads the database
**Then** reading the `body_files` collection fails with the typed recovery-unavailable error naming it, no record of that collection is read, and the other collection is read as before

### AC: reader-runs-recovery-before-read

**Requirements:** record-body-file#req:pending-changes-recovered-before-read

**Given** two recovery hooks:
- one that records its invocation and reports a pending change it rolls forward;
- one that returns an error

**When** a reader configured with each hook reads a `body_files` collection
**Then** the first hook is invoked before the first record file is opened, and its reader sees the post-recovery state; the reader with the second hook fails with that hook's error without reading any record

### AC: revision-covers-bodies

**Requirements:** record-body-file#req:record-revision-covers-bodies

**Given** a stored record `customer-invoices` and its revision `R`
**When** each of these is applied separately:
- only the `notes` body bytes change;
- only `type` changes, so the `text` body name changes, with identical body bytes;
- a byte moves from the end of the record file to the start of the `text` body

**Then** the revision computed from the exported canonical input differs from `R` in every case, including through `dalgo2ingitdb`'s protected profile

### AC: lenient-definition-paths-preserve-body-files

**Requirements:** record-body-file#req:version-skew-fails-safe

**Given** a `dalgo2ingitdb` collection definition declaring `body_files`
**When** it is read through `DescribeCollection`, and separately altered with an unrelated `ApplyAddField`
**Then** neither path drops `body_files`: the described definition carries it, and the rewritten `definition.yaml` still contains it unchanged

### AC: format-version-and-minimums-declared

**Requirements:** record-body-file#req:version-skew-fails-safe, record-body-file#req:format-contract-and-golden-fixture

**Given** the `FORMAT.md` and `go.mod` files after this Feature lands
**When** they are read
**Then** all of the following hold:
- `FORMAT.md` declares `v2`;
- it contains the "Body files" section with the example tree from REQ `format-contract-and-golden-fixture`;
- it names the minimum `ingitdb-go/ingitdb` and `ingitdb-ts` releases;
- `dalgo2ingitdb` and `dalgo2ingitdb4github` require at least that `ingitdb-go/ingitdb` release.

### AC: schema-alteration-keeps-body-files-consistent

**Requirements:** record-body-file#req:schema-alteration-keeps-body-files-consistent

**Given** the reference collection
**When** each of these alterations is attempted separately:
- `type` is renamed to `kind`;
- `text` is renamed to `body`;
- `type` is modified to `int`;
- `text` is dropped;
- `record_file.name` is changed to `{key}/{key}.q.json`

**Then** the rename of `type` rewrites the entry's `type_field` to `kind` and leaves every file in place. Every other alteration is refused, and `definition.yaml` and every file stay unchanged.

### AC: listing-never-returns-body-files

**Requirements:** record-body-file#req:body-files-isolated-from-listing

**Given** the golden fixture's `queries` collection
**When** records are listed and counted by `datavalidator`, the materializer, the foreign-key index, `dalgo2ingitdb` and `dalgo2ingitdb4github`
**Then** each lists exactly `active-customers`, `customer-invoices` and `draft-query`, and no reported key or file path is a body file

### AC: orphan-and-stale-body-files-reported

**Requirements:** record-body-file#req:orphan-body-files-reported

**Given** a `queries` directory holding `q9/q9.query.text.sql` with no `q9/q9.query.json`, and record `q1` of type `DTQL` with both `q1/q1.query.text.dtql` and a leftover `q1/q1.query.text.sql`
**When** the database is validated
**Then** validation reports one orphan-body finding for `q9/q9.query.text.sql` and one for `q1/q1.query.text.sql`, and record `q1` reads with the `.query.text.dtql` body

### AC: legacy-sidecar-names-not-read-as-bodies

**Requirements:** record-body-file#req:datatug-cut-over-referenced

**Given** a record `q5` of type `SQL` with a legacy `q5/q5.query.sql` and no `q5/q5.query.text.sql`
**When** it is read and the collection is listed
**Then** `q5` reads with no `text` key, and the legacy file is neither merged nor listed as a record

### AC: definition-without-body-files-unchanged

**Requirements:** record-body-file#req:body-files-definition

**Given** the existing test suite and golden fixtures with no `body_files` declared anywhere
**When** they run after this Feature
**Then** every existing read, write, listing and validation result is unchanged, and no recovery hook is required

### AC: go-and-ts-readers-agree-on-fixture

**Requirements:** record-body-file#req:format-contract-and-golden-fixture, record-body-file#req:ts-layout-prerequisite-recorded

**Given** the `queries` fixture subtree, in its nested `{key}/{key}.query.json` layout, committed identically in `dalgo2ingitdb`, `ingitdb-ts` (`client-fs` and `client-github`) and `ingitdb-go`; `FORMAT.md` documenting body files; and ingitdb/ingitdb-ts#104 landed
**When** the Go reader test and the `ingitdb-ts` fixture tests read all three query records
**Then** both produce the same field maps: the same `text` and `notes` bytes, and no `text` key for `draft-query`

## Not Doing (and Why)

- **Designing the multi-file journal and its recovery.** ingitdb/dalgo2ingitdb#15 owns crash-safe multi-file changes. This Feature fixes only what #15 consumes: the ordered, complete write plan with expected prior states, the recovery hook every reader calls, and the refusal of multi-operation plans until #15 lands.
- **A configurable body file name.** The founder decided on 2026-09-17 on the fixed `<key>.<record-suffix>.<field>.<body-type>` name. It is what makes the disjointness rule a static check.
- **Migrating DataTug's `.query.<type>` sidecars.** That is DataTug's hard cut-over (REQ `datatug-cut-over-referenced`).
- **Renaming a body field.** That renames one file per record. It needs a collection-wide multi-file change, so the rename is refused for now.
- **Multi-segment record suffixes, such as `{key}.a.b.json`, in body-bearing collections.** DataTug uses single-segment suffixes, and one segment keeps name parsing unambiguous.
- **Implementing the TypeScript reader or its layout support.** Parity is made testable here through `FORMAT.md` and the shared fixture. The `ingitdb-ts` work is ingitdb/ingitdb-ts#104 plus its own body-file change.
- **Body files for list and map record types.** A many-record file has no per-record sibling owner.
- **Binary bodies or any encoding.** Records are text-only (`storage-format` REQ `text-only`).
- **A body-size cap.** `datatug-core` enforces its own cap before calling the store. A storage-level cap can be added later without changing this shape.
- **Repairing a torn state that was already committed to Git.** Recovery runs against the working tree and journal. A user commit of a working tree mid-crash is outside what a reader can detect.

## Rehearse Integration

No Rehearse stubs are scaffolded: `specscore.yaml` declares no rehearse configuration. Every AC is directly executable as a Go test: definition-load table tests through `validator.ReadDefinition`, pure write-plan and revision-input unit tests, temp-dir and golden-fixture tests through the real readers and `datavalidator`, and driver tests in `dalgo2ingitdb` and `dalgo2ingitdb4github`. The one exception is the TypeScript half of `go-and-ts-readers-agree-on-fixture`, which is a vitest in `ingitdb-ts`. This is recorded as an explicit skip rather than an omission.

## Dependent Modules

- **`ingitdb/dalgo2ingitdb`** owns `FORMAT.md` and the fixture. It:
  - applies the write plan, refusing multi-operation plans until #15;
  - merges bodies on `Get` and query reads (`record_io.go`, `query.go` `readAllSingleStored`);
  - implements the recovery hook (#15);
  - computes protected revisions from the canonical input (`protected.go`);
  - makes the `schema_reader.go` and `schema_modifier.go` definition decodes strict or `body_files`-preserving;
  - applies the field alteration rules;
  - raises its `ingitdb-go/ingitdb` requirement.
- **`ingitdb/dalgo2ingitdb4github`**:
  - merges bodies and excludes body files in `query.go` `scanSingleRecords` (`:172-203`) and in single-record reads;
  - refuses non-regular tree entries (mode `120000`, `160000`);
  - applies each write plan in one commit;
  - raises its `ingitdb-go/ingitdb` requirement.
- **`ingitdb/ingitdb-ts`** needs ingitdb/ingitdb-ts#104 (`records_dir` and nested templates). It then reads body files in `client-fs` and `client-github` and mirrors the fixture.
- **`ingitdb/ingitdb-cli`** writes records through `dalgo2ingitdb`, so it inherits body-file handling. It configures `dalgo2ingitdb`'s recovery hook for validate and materialize. Any direct file write it keeps must use the write plan.
- **`datatug/datatug-core`**:
  - declares `body_files: [{field: text, type_field: type}]` on its queries collection (`dalgo-project-store`);
  - maps `QueryDef.Text == ""` to an absent `text` field when it has no body;
  - renames its sidecars to `.query.text.<type>` under its hard cut-over.

## Open Questions

1. **Where `FORMAT.md` lives.** It sits in `dalgo2ingitdb` today, but `ingitdb-go` owns the definition schema this Feature extends. Should `FORMAT.md` and the canonical fixture move to `ingitdb-go`, with the other repos mirroring it? Until someone decides, this Feature adds to the existing `dalgo2ingitdb` copy and mirrors the subtree into `ingitdb-go`.
2. **A missing body file: absent field, or read error?** Choose one:
   - (A) The record reads with the body field absent. Authors who need a body mark the column `required: true`, and validation reports it as a finding.
   - (B) A missing body file is a read error for that record.

   **Recommendation: A.** DataTug already has queries without a body: `datatug-core` loads them as empty text and writes no sidecar (`store_queries.go`, `readQueryTextSidecar`). Under B, one such query fails the whole collection listing. A keeps absent (no file) distinct from empty (an empty file), so the read-write round trip is exact. The torn-write risk that B guards against is closed instead by recovery before read (REQ `pending-changes-recovered-before-read`). REQ `missing-body-is-absent-field` is written as A, provisionally.

Resolved: the founder decided on 2026-09-17 that the body file name is fixed (`<key>.<record-suffix>.<field>.<body-type>`) and that the body type is the lowercased type value, so the extension is not taken from a closed list. The earlier question about `JSON`-typed queries no longer applies, because `x.query.text.json` is disjoint from `*.query.json`.

---
*This document follows the https://specscore.md/feature-specification*
