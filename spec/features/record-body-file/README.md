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

A single-record collection may declare `record_file.body_files`: one or more string fields, each stored as a raw-text file beside the record file. The record file holds an explicit reference in place of the value, `{"$file": "<name>"}`. It holds `null` for a null value, and omits the key for an absent field.

A body file's name is fixed: `<key>.<record-suffix>.<field>.<ext>`, for example `customer-invoices.query.text.dtql` beside `customer-invoices.query.json`. The extension is either hardcoded per entry (`format`) or taken per record from a field (`format_field`). One collection can therefore mix `sql`, `dtql`, `http` and `graphql` bodies.

- **Reading:** readers fetch only referenced files and merge each one byte-for-byte.
- **Writing:** writers split the fields out through an exported, ordered write plan. They never duplicate a body into the record file or leave a stale body behind.
- **Crash safety:** multi-file changes are owned by ingitdb/dalgo2ingitdb#15, and every reader runs that driver's recovery first.
- **Listing:** definition validation keeps record templates and body file names disjoint, so listing never mistakes a body file for a record.
- **Contract:** YAML conformance vectors in `ingitdb/ingitdb` (ingitdb/ingitdb#9), plus the definition's JSON Schema (ingitdb/ingitdb-schema#9). Every implementation vendors and runs the vectors.

Closes ingitdb-go#24.

## Problem

Today a record is exactly one file. DataTug keeps a query as a JSON record plus a raw body file beside it. The founder decided on 2026-09-17 to keep that pair under the DALgo-backed project store (`datatug/datatug` Feature `dalgo-project-store`, REQ `driver-prerequisites-are-recorded`).

The record file is already expressible: `name: '{key}/{key}.query.json'`, `format: json`, `type: map[string]any`, `records_dir: '.'`. The body file is not.

The body's text type also varies per query, and the user chooses it for each one. Founder, 2026-09-17: *"The collection can have queries with different text types - some are sql, some DTQL, some http, some graphql. It's defined by user per query."* Other body fields, such as notes, always have one type.

Without this Feature, DataTug must either inline the body as an escaped JSON string, which is unreadable in a diff, or keep a sidecar reader beside the driver. The second option is the machinery the cut-over is meant to retire.

Two existing mechanisms come close, and neither fits:

- **`format: markdown` + `content_field`** gives a body, but the body shares one file with YAML frontmatter. DataTug's record is JSON and its body is SQL, DTQL, HTTP or GraphQL, not Markdown.
- **`{fieldName}` placeholders** in `record_file.name` (Feature `record-file-name-placeholders`) substitute a column value into the record file's own name. They name one file, not a second one. They are also case-preserving and have no character-set bound, and an extension needs both a case rule and a bound (REQ `body-extension-resolution`).

## Behavior

### Definition and naming

#### REQ: body-files-definition

`RecordFileDef` MUST accept an optional `body_files` list. Its keys are modelled fields, so the strict (`KnownFields(true)`) definition reader accepts them. Each entry has `field` and **exactly one** of `format` or `format_field`:

| Key | Required | Meaning |
|-----|----------|---------|
| `field` | yes | The record column that this body file carries. |
| `format` | one of the two | A hardcoded extension, the same for every record, for example `md` for notes. |
| `format_field` | one of the two | The name of a record field whose value, lowercased and validated, is this body's extension, chosen per record. Several entries MAY share one `format_field`. |

The founder decided on 2026-09-17 that the extension is *"either hardcoded or driven by value of some field on record"*. There is no name template (REQ `body-file-naming`).

The DataTug queries collection is the reference definition:

```yaml
record_file:
  name: '{key}/{key}.query.json'
  format: json
  type: map[string]any
  records_dir: '.'
  body_files:
    - field: text              # the query text; its type is chosen per query
      format_field: textFormat # SQL | DTQL | HTTP | GraphQL -> sql | dtql | http | graphql
    - field: notes             # free-form notes, always Markdown
      format: md
columns:
  textFormat:
    type: string
  text:
    type: string
  notes:
    type: string
```

When `body_files` is omitted or empty, behavior MUST be byte-for-byte what it is today.

#### REQ: body-file-naming

A body file MUST be named `<key>.<record-suffix>.<field>.<ext>` and MUST sit in the **same directory as its record file**. The parts are:

- `<key>` is the record key, exactly as it is substituted into the record file name;
- `<record-suffix>` is the record template's dotted type suffix: in a record basename template `{key}.<record-suffix>.<file-ext>`, the single segment between `{key}.` and `.<file-ext>` (`query` in `{key}.query.json`);
- `<field>` is the entry's `field` name, verbatim;
- `<ext>` is the entry's `format`, or the extension resolved from its `format_field` (REQ `body-extension-resolution`).

One collection mixing the four query text types:

```
queries/active-customers/active-customers.query.json     # {"text": {"$file": "active-customers.query.text.sql"}, "textFormat": "SQL", "notes": null}
queries/active-customers/active-customers.query.text.sql
queries/customer-invoices/customer-invoices.query.json   # {"text": {"$file": "customer-invoices.query.text.dtql"}, "textFormat": "DTQL", "notes": {"$file": "customer-invoices.query.notes.md"}}
queries/customer-invoices/customer-invoices.query.text.dtql
queries/customer-invoices/customer-invoices.query.notes.md
queries/country-facts/country-facts.query.json           # {"text": {"$file": "country-facts.query.text.http"}, "textFormat": "HTTP"}
queries/country-facts/country-facts.query.text.http
queries/customer-graph/customer-graph.query.json         # {"text": {"$file": "customer-graph.query.text.graphql"}, "textFormat": "GraphQL"}
queries/customer-graph/customer-graph.query.text.graphql
```

Because the nested layout is required (REQ `body-files-definition-validated` rule 3), the record's directory belongs to that one record. Placing the body there keeps a record and its bodies under one containment check and one directory for the journal (REQ `write-plan-exposed`). Including `<field>` gives every body field of a record its own file name. `<field>` and `<ext>` never contain a dot, so a body file name parses one way only: the last segment is the extension, the one before it is the field, and the one before that is the record suffix. The key is everything in front. Two different (key, field, extension) triples can therefore never produce the same name.

#### REQ: body-files-definition-validated

`RecordFileDef.Validate()` (with column-aware checks in `CollectionDef.Validate()`) MUST reject each of the following at definition-load time, before any record path is read or written:

1. `body_files` on a record type other than `map[string]any` (`SingleRecord`). A list or map file holds many records, so a per-record sibling file has no single owner.
2. `body_files` with `format: markdown` on the record file. A Markdown record already has a body (`content_field`), and two body carriers for one record would be ambiguous.
3. `body_files` when `record_file.name` is not exactly `{key}/{key}.<record-suffix>.<file-ext>`, where `<record-suffix>` and `<file-ext>` are each one non-empty segment of `[a-z0-9_-]` (error kind `body-files-record-name`). This **nested layout** gives every record its own directory, holding exactly one record file and that record's bodies, and it is what DataTug uses. A flat `{key}.query.json` is rejected: keys may contain dots, so one record's safe body pattern (REQ `stored-reference-access-guard`) could match another record's files in a shared directory, for example record `a` against `a.b.query.json`. A bare `{key}.json`, or a basename without `{key}`, has no record suffix to name bodies by.
4. An entry with an empty `field`, with both `format` and `format_field`, with neither, or with any other key.
5. A `field` that does not match `^[a-z0-9_-]{1,64}$`. A body file name must be a safe, dot-free segment, and lowercase-only names cannot collide on a case-insensitive filesystem.
6. A `format` that does not match `^[a-z0-9_-]{1,32}$`.
7. A `field` that is not a declared column of type `string`. A `format_field` that is not a declared column of type `string`. A computed (`formula`) column as `field` or `format_field`. A `field` equal to any entry's `format_field`. The same `field` in two entries.
8. A `field` equal to the collection's own `<record-suffix>`. With suffix `query`, field `query` would name `x.query.query.json` for extension `json`, and that name matches the record glob `*.query.json`.

#### REQ: suffix-disjointness-covers-body-files

Whole-definition validation (`validator.ReadDefinition`, after every collection and subcollection is loaded) MUST reject a definition in which any collection's record template could match a body file name. This extends the dotted-suffix rule that `dalgo-project-store` REQ `type-suffix-is-self-identifying` states for record templates alone.

Formally, take a body-bearing collection D with record suffix `s` and a body entry with field `f`. Its body file names end in `.s.f.<t>`. Here `<t>` is the entry's `format` when it has one, and any dot-free segment when it has a `format_field`.

Take any collection C whose records base directory is D's record-file directory or an ancestor of it. The rule includes C = D, and includes ancestors because the GitHub reader lists a collection directory recursively.

Write the part of C's record basename template after its last `{key}` as `.x1.….xn`, where `xn` is C's file extension. Then C conflicts with D's entry when `xn` can equal `<t>`, and one of the following holds:

- n = 1: a bare `.ext`;
- n = 2 and x1 = `f`: for example C's `{key}.text.json` against `x.query.text.json`;
- n ≥ 3, x(n-1) = `f` and x(n-2) = `s`.

When C's record basename has **no** `{key}`, as in `{key}/record.json` or `all.json`, the part matched against a basename is a fixed literal, y1.….ym. C then conflicts only when m ≥ 4, y(m-1) = `f`, y(m-2) = `s` and `ym` can equal `<t>`. An example is the literal `a.query.text.json` against a `format_field` entry.

An entry with a `format_field` can produce any extension, so for it "can equal" is always true. The error names both collections, the record template and the body field.

With the rule enforced, a body file cannot match any record matcher, neither the local glob (`{key}` → `*`) nor the GitHub template regex (`dalgo2ingitdb4github/query.go` `buildKeyExtractor`). For example, `x.query.text.json` matches neither `*.query.json` nor `\.query\.json$`.

#### REQ: body-extension-resolution

For an entry with `format`, the extension is that value.

For an entry with `format_field`, the extension MUST be the record's `format_field` value with ASCII letters lowercased. For example, `SQL` becomes `sql`, `DTQL` becomes `dtql`, `HTTP` becomes `http`, `GraphQL` becomes `graphql` and `JSON` becomes `json`. The value MUST be 1 to 32 characters, each an ASCII letter, digit, `_` or `-`. The value is record content, and so it is untrusted. Bounding it stops a value like `../../x` from addressing a file outside the collection.

The extension is **unresolved** when the `format_field` value is null, empty, absent or outside that set. An unresolved extension is an error only for an entry whose body is present: a referenced file on read, or a string value on write (REQ `body-field-reference-encoding`). It is not an error for an entry whose value is null or absent.

A `JSON` text is allowed. Its body name, `x.query.text.json`, is disjoint from the record glob by REQ `suffix-disjointness-covers-body-files`.

### Record encoding

#### REQ: body-field-reference-encoding

The founder decided on 2026-09-17 on option A, via explicit references: *"storing in record json reference to the file and exploit null for nullable fields - empty value and null are different values."*

In the **record file**, a declared body field's value MUST be exactly one of the following:

| Stored in record file | Field value | Body file |
|-----------------------|-------------|-----------|
| `{"$file": "<name>"}` | the string held in `<name>`; a 0-byte file is `""` | MUST exist |
| `null` | `null` | MUST NOT exist |
| key absent | absent | MUST NOT exist |

Rules:

- `<name>` MUST equal the name that REQ `body-file-naming` derives for this record, field and extension. It is a plain basename that is resolved in the record's directory.
- The reference object MUST have exactly one key, `$file`, and its value MUST be a string. Keys that start with `$` are **reserved** inside body field values. A body field is a `string` column, so its value is never an object, and a genuine object containing `$file` cannot occur by construction.
- An inline string for a declared body field is invalid, as is any other value: a number, a boolean, a list, or an object that is not a well-formed reference.
- Presence is decided by the reference alone. Nothing is inferred from the format value, and readers never look for unreferenced files.

A missing referenced file can be judged corrupt only when no multi-file change is pending. REQ `pending-changes-recovered-before-read` guarantees that.

#### REQ: stored-reference-access-guard

A stored `$file` value is record content, and so it is untrusted. Before **any** file operation that follows a stored reference, an implementation MUST classify the reference. The operations covered are: a read, computing the write plan (reading prior bytes, removing a stale body), a delete, a repair (REQ `drifted-record-repair`), validation, and #15's recovery (every body path a journal names).

A reference is **safe** when it is a plain basename, resolved in the record's own directory, that matches:

```
^<key>\.[a-z0-9_-]+\.[a-z0-9_-]{1,64}\.[a-z0-9_-]{1,32}$
```

Here `<key>` is this record's key, matched literally. The pattern then allows any single-segment suffix, any single-segment field and any extension. This is deliberately wider than the current definition, so references left behind by a permitted definition change stay reachable. Examples are a body field renamed from `notes` to `memo`, a record suffix change, or a `format` change. A safe name has no separator and no `..` segment, so it always resolves inside the record's directory.

A reference is **current** when it is safe and equals the name that REQ `body-file-naming` derives from the definition and the record now.

Rules:

- **Read:** requires current. A safe reference that is not current fails with `body-reference-mismatch`, and no file is opened.
- **Plans, deletes, repair, validation and recovery:** they MAY follow any safe reference, current or not.
- **File-type check:** before following a safe reference, the implementation MUST check the file's type with `lstat` (or the Git tree entry's mode), never with a call that follows symlinks. Only a regular file (`100644` or `100755` in a Git tree) is followed. Anything else is `non-regular-body` and is never opened.
- **Unsafe references:** such as `../../x`, `a/b`, another record's key, or an extension with a dot or uppercase letter. The implementation MUST NOT open, stat, read, write or remove the named path:
  - **Read:** the read fails with `unsafe-body-reference`.
  - **Write plan, delete or repair:** the plan proceeds without any operation on that path, and lists the reference under `skipped`, as `{field, reference, kind: unsafe-body-reference}`. The driver surfaces `skipped` entries to its caller.
  - **Validation:** `unsafe-body-reference` is reported as a finding on the record.

A safe reference that is not current is **drift** (REQ `name-drift-is-a-validation-finding`).

**Protected paths.** For a record, the protected paths are:
- the record file;
- the current name of every `body_files` entry of the record (for a put, under the new data; for a repair, under the stored data);
- every path that more than one entry of the record references.

A stored reference held by entry E is **shared** when it names a protected path other than E's own current name. For example, `text` holds `{"$file": "q1.query.notes.md"}` while `notes` is current at that name. A plan or repair MUST NOT remove or overwrite a protected path because of a stale or shared reference, and MUST NOT read a shared reference's file as E's value. It leaves the file in place and lists the reference under `skipped` as `{field, reference, kind: shared-body-reference}`. Validation reports `shared-body-reference` as a finding. A delete is the exception: it removes the record's files as a whole, and removes each distinct safe path once.

### Reading

#### REQ: read-merges-referenced-bodies

Every record reader MUST resolve each body field from the record file per REQ `body-field-reference-encoding`, and MUST fetch **only** referenced files. That covers this module's `datavalidator` pass, the materializer's `records_reader_fs` and every other `SingleRecord` read path, plus `dalgo2ingitdb`, `dalgo2ingitdb4github` and both `ingitdb-ts` clients. A referenced file's exact bytes become the field's string value. The reader MUST NOT decode, trim, normalise newlines, or strip a BOM. The merge happens before column validation, so each body field is validated like any other `string` column.

A read MUST fail, with an error of the given kind naming the record key, the field and, where there is one, the body path, in this order of checks:

- `inline-body-value`: the record file holds a string for a body field;
- `invalid-body-reference`: the record file holds any other non-null, non-reference value for a body field, or a reference with extra or `$`-prefixed keys, or a non-string `$file`;
- `unsafe-body-reference`: the reference fails REQ `stored-reference-access-guard`. No file is opened;
- `format-unresolved`: a reference is present and the entry's extension is unresolved. No file is opened;
- `body-reference-mismatch`: the reference is safe but not current. No file is opened;
- `missing-body`: the referenced file does not exist;
- `non-regular-body`: the referenced path is not a regular file, as determined by `lstat` or the tree entry mode, without following symlinks. On a filesystem that means a directory or a symlink. In a Git tree it means an entry whose mode is not `100644` or `100755`, for example `120000` (a symlink) or `160000` (a submodule);
- `invalid-utf8`: the referenced file is not valid UTF-8, since records are text-only per `ingitdb/ingitdb` Feature `storage-format`, REQ `text-only`.

### Writing and deleting

#### REQ: write-splits-bodies-out

For each entry in `body_files`, a writer putting a record MUST do the following:

- **String value:** write the body file, at the derived name, with exactly those bytes and no encoding. Store `{"$file": "<name>"}` in the record file.
- **Null value:** store `null` in the record file, and remove any existing body file for that entry.
- **Absent value:** omit the key from the record file, and remove any existing body file for that entry.
- **Refusal:** refuse the whole put, before changing any file, when a body field holds a value that is not a string, null or absent (`body-not-string`), or when a string value's entry has an unresolved extension (`format-unresolved`).
- **No-op:** leave a file untouched when its bytes already equal the bytes being written, per `storage-format` REQ `no-rewrite-without-change`.

The stored record file never contains a body's content.

#### REQ: delete-and-format-change-leave-no-stale-body

A delete MUST remove the record file and every body file that the stored record references through a **safe** reference (REQ `stored-reference-access-guard`). A referenced file that is already missing is not an error for a delete. An unsafe reference is skipped and reported, never followed.

A put that changes an entry's body name MUST remove the body file that the **previously stored** record referenced, as part of the same change, when that reference is safe. An unsafe one is skipped and reported. Changing `textFormat` from `SQL` to `DTQL` is one such put. After the put, each entry has at most one body file on disk, and it is exactly the one the new record references.

#### REQ: write-plan-exposed

This module MUST export a pure function that computes the full file-level change for a record put or delete, without touching the filesystem. Its inputs are the collection definition, the record key, the previously stored state (the record file bytes and the bytes of each body file behind a safe reference, or none), and the new record data (or none for a delete). The caller gathers that state under REQ `stored-reference-access-guard`, and the function never asks for bytes behind an unsafe reference.

It returns an ordered list of operations, plus a `skipped` list of unsafe stored references (REQ `stored-reference-access-guard`). The list is the **exact and complete** set of files the change touches: applying exactly these operations, in this order, yields the new state, and no other file may be written or removed. Each operation carries:

- its kind, `write` or `remove`;
- its path, slash-separated and relative to the database root;
- for `write`, the full new bytes;
- the **expected prior state** of that path: absent, or present with the SHA-256 of its bytes. A journal can use this to detect a concurrent change, and to tell during recovery whether an operation has already been applied.

The order is fixed:

- **Put:** (1) write each body file whose name or bytes change, in `body_files` order; (2) write the record file, if its bytes change; (3) remove each stale body file, in `body_files` order, except a protected path (REQ `stored-reference-access-guard`).
- **Delete:** (1) remove the record file; (2) remove each present body file behind a safe reference, in `body_files` order. Each distinct path is removed **once**: when several entries reference the same path, only the first entry in `body_files` order emits the `remove`, and unsafe references are skipped (REQ `stored-reference-access-guard`).

Writing bodies before the record file means a record file never references a file that has not yet been written. An unchanged put returns an empty list. Every path in a plan shares one containing directory, and the plan names that directory. The guard is what keeps this invariant true for removals driven by stored references.

Writers MUST apply this plan instead of re-deriving body names. ingitdb/dalgo2ingitdb#15 owns crash-safe application of multi-file changes: its durable journal, its recovery, and the atomicity of a record and its bodies as one unit. This Feature does not design that journal. It does require that until a working-tree driver provides that atomicity, the driver MUST refuse, with a typed error and before changing any file, any put or delete whose plan has more than one operation. Ordering alone does not make the files consistent. A driver that commits a Git tree in one commit, such as `dalgo2ingitdb4github`, is already atomic and applies the whole plan in that commit.

#### REQ: pending-changes-recovered-before-read

A torn multi-file change must never be read as a valid state. This module MUST export a pending-change recovery hook: an interface that a driver implements, for example `RecoverPendingChanges(ctx, dbRoot) error`. The hook exists because `ingitdb-go` cannot import `dalgo2ingitdb`, the owner of the journal (ingitdb/dalgo2ingitdb#15).

**Locking.** The hook MUST acquire the driver's transaction lock for the whole of recovery. The driver chooses shared or exclusive, and #15 defines which. A writer holds that lock while its change and journal are live, so a hook that waits for the lock can never roll back or roll forward a write that is still in flight. It acts only on a journal whose writer has released the lock or died. The reader then reads under the driver's shared lock, so no new change can start between recovery and read.

**Single-operation plans.** A plan with one operation, such as a body-only edit or a record-only edit, has no multi-file journal. It relies on #15's per-file temp file, fsync and rename (and directory fsync) to be crash-atomic.

Every reader of a working-tree database MUST do the following before it reads any record of a collection that declares `body_files`:

- If a recovery hook is configured, invoke it, and proceed only when it returns without error. On error, fail with that error.
- If no hook is configured, refuse with a typed error (for example `ErrPendingChangeRecoveryUnavailable`) that names the collection. The reader MUST NOT read records without the hook.

Reads of a committed Git tree, such as `dalgo2ingitdb4github` or a read at a commit, are exempt. A commit is written atomically, and no journal is attached to it. Collections without `body_files` are unaffected: their changes touch one file each, and #15 makes those atomic by rename.

#### REQ: every-body-files-reader-configures-recovery

Every caller that reads records of a working-tree database MUST configure the recovery hook, because it may meet a `body_files` collection. The callers below were found by searching the Go checkouts under `/home/ai/projects` (worktrees excluded) on 2026-09-17 for imports of `ingitdb-go/ingitdb/datavalidator`, `materializer` and `docsbuilder`.

**Entry points in `ingitdb-go` that read records**, which MUST accept a hook and apply REQ `pending-changes-recovered-before-read`:

- `datavalidator.NewValidator().Validate`, including its foreign-key index pass;
- `datavalidator.NewIncrementalValidator(...).ValidateChanges`;
- `materializer.NewFileRecordsReader().ReadRecords`, and through it `materializer.NewViewBuilder`;
- `docsbuilder.UpdateDocs`.

**Callers**, which MUST pass `dalgo2ingitdb`'s hook:

- **`ingitdb/ingitdb-cli`**, in `cmd/ingitdb/main.go` (wiring), and in `commands/`: `validate.go`, `diff.go`, `pull.go`, `ci.go`, `materialize.go`, `docs_update.go`, `conflict_resolver.go`, `record_merge_resolver.go`, `select_output.go`, `view_builder_helper.go` and `seams.go`.
- **`ingitdb/ingitdb-action`**, which runs `ingitdb validate` from a pinned `ingitdb-cli` release (`scripts/validate.sh:234`). It is covered through the CLI, and its pinned `cli-version` MUST be a release that configures the hook.
- **`synchestra-io/synchestra`**, in `pkg/cli/main.go`, which embeds `ingitdb-cli` commands and imports `materializer` directly.

Importers of only `ingitdb-go/ingitdb/validator`, which reads definitions but not records, read records through `dalgo2ingitdb`. That driver runs its own recovery before its reads, so they need no separate wiring. They are `openvaultdb-go` (`pkg/mount`), `sneat-go` (`pkg/sneatstorage`), `specscore-cli` (`pkg/journal`), `wb` (`internal/hubstore`), `synchestra` (`pkg/state/gitstore`, `pkg/cli/state`) and `datatug-cli` (`pkg/incidentstore`, `pkg/dbcopy`).

#### REQ: record-revision-covers-bodies

Any revision, etag or change token that `ingitdb-go` or a driver computes for a record in a collection that declares `body_files` MUST be a hash of the **canonical revision input** below. It therefore changes whenever the record file's existence or bytes change, whenever a body field moves between reference, null and absent, and whenever a referenced file's name, existence or bytes change.

This module MUST export a function that builds this input. Drivers hash it, with an HMAC where they need one, and never frame the parts themselves. The layout uses these building blocks:

- `u64` means an unsigned 64-bit integer, **big-endian**.
- `lp(x)` means `u64(len(x))` followed by the bytes of `x`.

The layout, in order:

1. The magic `ingitdb-record-revision-v1`, as ASCII bytes, followed by one `0x00` byte.
2. `lp(record path)`: the record file path, slash-separated and relative to the database root, as UTF-8.
3. The record file: `0x00` if absent, or `0x01` followed by `lp(record file bytes)` if present.
4. For each entry, in `body_files` order:
   1. `lp(field name)`;
   2. a state byte: `0x00` for absent, `0x01` for null, `0x02` for a reference;
   3. for a reference only: `lp(reference name)`, then `0x00` if the referenced file is missing, or `0x01` followed by `lp(body file bytes)` if it exists.

The vector `revision-input-layout` pins the layout. Its definition has three entries: `text` (`format_field: textFormat`), `notes` (`format: md`) and `summary` (`format: txt`). The record is `queries/q1/q1.query.json` with these bytes, including a final newline:

```
{"notes":null,"text":{"$file":"q1.query.text.sql"},"textFormat":"SQL"}
```

`q1.query.text.sql` holds `select 1` with no trailing newline, so `text` is a reference, `notes` is null and `summary` is absent. For that input:

- the input is 224 bytes; in base64 it is `aW5naXRkYi1yZWNvcmQtcmV2aXNpb24tdjEAAAAAAAAAABhxdWVyaWVzL3ExL3ExLnF1ZXJ5Lmpzb24BAAAAAAAAAEd7Im5vdGVzIjpudWxsLCJ0ZXh0Ijp7IiRmaWxlIjoicTEucXVlcnkudGV4dC5zcWwifSwidGV4dEZvcm1hdCI6IlNRTCJ9CgAAAAAAAAAEdGV4dAIAAAAAAAAAEXExLnF1ZXJ5LnRleHQuc3FsAQAAAAAAAAAIc2VsZWN0IDEAAAAAAAAABW5vdGVzAQAAAAAAAAAHc3VtbWFyeQA=`;
- its SHA-256 is `322fdd8c2f9e7a37fb98844e9fc76d76a482f3a126a8f75260816aba27038dc8`.

Today `dalgo2ingitdb/protected.go` (`revision`, `:482-494`) hashes the record file bytes only, so an edit to a body alone would not change the revision. It MUST switch to this input.

### Isolation

#### REQ: body-files-isolated-from-listing

Listing, globbing and counting records MUST NOT return a body file as a record, or as part of a record key. This applies in `datavalidator` (`singleRecordGlobPattern`, record counts), the materializer, the foreign-key index, `dalgo2ingitdb` (`query.go` `readAllSingleStored`), `dalgo2ingitdb4github` (`query.go` `scanSingleRecords`) and both `ingitdb-ts` clients.

Consider a template whose basename contains no `{key}`, such as `{key}/record.json`. Both matchers pin that basename to its literal text: the glob `*/record.json`, and the GitHub regex `^(.*?)/record\.json$`. A body basename is `<key>.<s>.<f>.<ext>` with a non-empty key, so it has at least four dot-separated segments. It can therefore equal the literal only when the literal also has at least four segments and ends in `.<s>.<f>.<ext>`. `record.json` has two segments and can never match. The one literal shape that could match is rejected at load time (REQ `suffix-disjointness-covers-body-files`).

Record discovery keeps using only record templates. Isolation is guaranteed statically by REQ `body-files-definition-validated` rule 8 and REQ `suffix-disjointness-covers-body-files`, not by per-value refusal. `exclude_regex` continues to apply to record files only. It is not needed, and not consulted, to hide body files.

#### REQ: orphan-body-files-reported

Whole-database validation MUST report a finding for each **orphan body file**. An orphan is a file in a body-bearing collection's record directory that matches the safe pattern of REQ `stored-reference-access-guard` for some key `<k>`, is not itself a listed record file, and is not referenced by any listed record. The file's suffix and field need not be current. Examples:

- there is no record file for key `<k>`;
- the record's `f` is null or absent;
- the record references a different name, such as a stale `.query.text.sql` beside a record that references `.query.text.dtql`;
- a body file whose record file is no longer listed after a `record_file.name` change (REQ `name-drift-is-a-validation-finding`).

The finding names the collection and the file path. It is a finding, not a read error: readers never look at unreferenced files, so the record still reads.

Validation MUST also report each read error from REQ `read-merges-referenced-bodies` as a finding on the record, including `unsafe-body-reference`, `body-reference-mismatch` and `format-unresolved`. Validation never opens a file behind an unsafe reference.

### Implementations

#### REQ: implementations-current-and-strict

The project is in private beta, and existing data carries no compatibility constraints (founder, 2026-09-17). There is therefore one rule: every implementation (`ingitdb-go`, `dalgo2ingitdb`, `dalgo2ingitdb4github`, `ingitdb-ts`) MUST be on the latest release of its `ingitdb` dependencies and MUST pass the conformance vectors (REQ `conformance-vectors-are-the-contract`). There are no minimum-version tables, no format version numbers and no skew shims.

Separately, for correctness: every definition decode MUST be strict (`KnownFields(true)` or its equivalent), so an unmodelled key is an error and is never silently dropped. The known lenient decodes are `dalgo2ingitdb`'s `schema_reader.go:105` (`DescribeCollection`) and `schema_modifier.go:445` (`readCollectionDefYAML`). A collection alteration that decodes and rewrites the definition (`writeCollectionDefYAML`, `schema_modifier.go:427`) MUST keep `body_files` unless the alteration itself updates it (REQ `schema-alteration-keeps-body-files-consistent`).

#### REQ: schema-alteration-keeps-body-files-consistent

A schema alteration on a collection that declares `body_files` MUST leave a definition that passes REQ `body-files-definition-validated` and REQ `suffix-disjointness-covers-body-files`. Drivers MUST validate the whole altered definition before writing it. The rules for each operation:

- `ApplyRenameField` of a `format_field` column MUST update every entry that references it, in the same definition write. No file is renamed, because body file names use the field's value, not its name.
- `ApplyRenameField` of a body `field` column MUST update that entry's `field` in the same definition write. As today (`schema_modifier.go` `rewriteRecordFiles`), it moves each record's value to the new key unchanged, so the stored `$file` reference keeps its old name and becomes drift, which repair fixes. No body file is renamed by the alteration itself.
- `ApplyModifyField` that would make a body `field` or `format_field` column non-`string` or computed MUST be refused before any write.
- `ApplyDropField` of a column that any entry references MUST be refused.
- Alterations that change body file names are neither special-cased nor refused. They include renaming a body `field` column, changing `record_file.name`, changing an entry's `format`, and switching between `format` and `format_field`. REQ `name-drift-is-a-validation-finding` covers them.

#### REQ: name-drift-is-a-validation-finding

A body file's name is always a pure function of the definition and the record's own values: `<key>.<record-suffix>.<field>.<ext>`. There is no rename or migration feature (founder, 2026-09-17).

- **Per-record format change.** A change to one record's format value, for example `textFormat` from `SQL` to `DTQL`, is an ordinary put (REQ `delete-and-format-change-leave-no-stale-body`).
- **Definition change.** A definition change can leave existing records whose `$file` references or body files no longer match the naming rule. Ordinary whole-database validation catches this, with no special refusal or rename logic: each such reference is reported as a `body-reference-mismatch` finding (REQ `read-merges-referenced-bodies`), and each file left unreferenced as an orphan finding (REQ `orphan-body-files-reported`). A drifted record cannot be read (`body-reference-mismatch`), so a caller that does not already hold its body values fixes it with REQ `drifted-record-repair`.
- **Record file moves are out of scope.** A `record_file.name` change also changes the record file's own name, for example `q1/q1.query.json` to `q1/q1.q.json`. This Feature takes the smaller option: repair never moves a record file. Existing record files that the new template no longer matches are not listed, so they are invisible to reads, repair and record-scoped validation. Whole-database validation MUST report each as an `unlisted-record-file` finding: a file under the records base directory that sits at the template's directory depth, ends in the template's file extension, does not match the template, is not safe-referenced by a listed record, and is not excluded by `exclude_regex`. Their body files are reported as orphans. Once the author moves a record file to its current name, for example with `git mv`, its body references with the old suffix are safe drift, and repair fixes them.

#### REQ: drifted-record-repair

This module MUST export a **repair plan** function for one record, and drivers MUST expose it as a repair operation. It is not a migration feature. It is the ordinary write plan with a different source for body values:

- For each entry whose stored reference is safe, not current and not shared, whatever differs (extension, field or suffix), the new value is the bytes of the file behind the stored reference. A shared reference is skipped and reported (REQ `stored-reference-access-guard`), and that entry is written as `null`, so the record reads again without taking another entry's body. The same checks apply: the file exists, `lstat` shows a regular file, and it is valid UTF-8. Otherwise the repair of that record fails with that error kind.
- Every other field keeps its stored value.
- The plan is then computed exactly as for a put: write the body at the derived name, rewrite the reference, and remove the drifted file. Every file stays inside the record's directory, and the plan is applied under #15 like any put.
- Unsafe references are skipped and reported, never followed.
- A record with no drift yields an empty plan.
- Repair works only on listed records, and never moves the record file itself (REQ `name-drift-is-a-validation-finding`).

Validation MAY offer a fix mode that runs the repair for each record with a drift finding. Each record is its own change. There is no collection-wide transaction.

### Cross-language contract

#### REQ: conformance-vectors-are-the-contract

The normative on-disk contract MUST be YAML, not Markdown (founder, 2026-09-17: *"Yaml can have comments for humans. Let's go with yaml."*). It lives in `conformance/record-layout/` in the standard repository `ingitdb/ingitdb` (tracked as ingitdb/ingitdb#9) and follows the `conformance/computed-columns/` pattern. The directory holds:

- `README.md`: **normative**, as in computed-columns. It defines the vector format, the error kinds and the profiles below.
- `vectors.yaml`: behaviour vectors.
- `validation_vectors.yaml`: definition and load rejections.

Every vector carries a YAML comment that explains it for humans. The definition shape, `record_file.records_dir` and `record_file.body_files` (`field` plus exactly one of `format` or `format_field`), is also added to `ingitdb/ingitdb-schema`'s `ingitdb-collection.schema.json` (tracked as ingitdb/ingitdb-schema#9). Every vector's definition MUST validate against that schema, except definitions a validation vector expects to be rejected.

**File format.** Both files have the top-level keys `version` (an integer, `1`) and `vectors` (a list). Each vector in `vectors.yaml` has the keys below. Exactly one `expect_*` key is present.

| Key | Meaning |
|-----|---------|
| `name` | A unique kebab-case id. |
| `definition` | The collection definition, exactly as in `.collection/definition.yaml`, for the collection `queries` at directory `queries`. |
| `setup_files` | The files present before the operation (see "File entries"). |
| `op` | `{kind, key?, data?, action?}`, where `kind` is one of `put`, `delete`, `read`, `list`, `validate`, `plan`, `repair` or `revision`, and `action` (`put`, `delete` or `repair`) applies to `plan`. In `data`, a body field is a string, `null`, or left out. |
| `expect_files` | For `put`, `delete` and `repair`: the complete set of files after the operation, including any file outside the collection, which MUST be unchanged. |
| `expect_record` | For `read`: the exact field map returned, in which `null` and a missing key are distinct. |
| `expect_list` | For `list`: the sorted record keys. |
| `expect_findings` | For `validate`: a list of `{kind, path}`. |
| `expect_plan` | For `plan`: `{operations, skipped}`. `operations` is ordered, each `{kind: write\|remove, path, content?, prior}`, where `prior` is `absent` or `sha256:<hex>`. `skipped` lists `{field, reference, kind}`. |
| `expect_revision` | For `revision`: `{input: base64:<...>, sha256: <hex>}`. |
| `expect_error` | An error kind. Kinds are compared, never message text. |

`validation_vectors.yaml` vectors have `name`, `collections` (a map from collection id to `{dir, definition}`, so disjointness cases can declare several collections) and `expect_error`, plus `target` (the offending collection id), mirroring computed-columns' `target`.

**File entries.** Each entry is `{path, content?, mode?}`:

- `path` is slash-separated and relative to the database root.
- `content` is a string. By default it is UTF-8 text, and its bytes are exactly the YAML string value. No implementation adds or strips a trailing newline, so authors use the `|` block scalar for exactly one final newline and `|-` for none. A value that starts with `base64:` is decoded from standard base64 into arbitrary bytes; the invalid-UTF-8 case uses this. Text that itself begins with `base64:` MUST be written in base64.
- `mode` is a Git tree mode string: `100644` (the default), `100755`, `120000` (a symlink, where `content` is the target) or `160000` (a submodule, with no `content`). Git-tree runners use the mode as is. Filesystem runners create `120000` as a symlink and `160000` as an empty directory.

**Error kinds** (the README defines the complete table):
- behaviour and findings: `unlisted-record-file`, `shared-body-reference`, `inline-body-value`, `invalid-body-reference`, `unsafe-body-reference`, `format-unresolved`, `body-reference-mismatch`, `missing-body`, `non-regular-body`, `invalid-utf8`, `body-not-string`;
- validation: `body-files-record-type`, `body-files-markdown`, `body-files-record-name`, `body-files-entry`, `body-field-name`, `body-format-value`, `body-field-column`, `body-format-field-column`, `body-field-equals-suffix`, `suffix-overlap`.

**Profiles.**
- The **writer** profile is the `put`, `delete`, `repair`, `plan` and `revision` vectors.
- The **reader** profile is the `read`, `list` and `validate` vectors, plus every validation vector.

Implementations run profiles as follows:
- `ingitdb-go` runs both profiles: `plan` and `revision` against its exported functions, and reads through its validator and readers.
- `dalgo2ingitdb` runs both profiles.
- `dalgo2ingitdb4github` runs both profiles against an in-memory Git tree.
- The `ingitdb-ts` clients run the **reader profile only**.

**Required coverage.** The vectors MUST cover at least:

- `records_dir: '.'` with a `{key}/{key}.query.json` name, placing records at `queries/<key>/<key>.query.json` with no `$records` segment;
- one collection holding `sql`, `dtql`, `http` and `graphql` text bodies resolved from `textFormat`, and a hardcoded `format: md` notes body;
- body naming `<key>.<suffix>.<field>.<ext>`, and exact raw bytes: a trailing newline kept, one absent, a non-ASCII character intact, and a 0-byte file read as `""`;
- the three stored states per field, reference, `null` and absent, on read and on put, including `null` versus `""`;
- several bodies per record (`.query.text.dtql` and `.query.notes.md`);
- a `JSON` text body, `x.query.text.json`, absent from `expect_list`;
- a `put` that splits bodies out, a `textFormat` change that removes the stale body and rewrites the reference, and the matching `expect_plan` order and `prior` hashes;
- a `delete` that removes the record and every referenced body;
- every behaviour error kind, including an inline string, a reference with an extra `$` key, `format-unresolved`, a mismatched name (`../../x`), a missing referenced file, an invalid-UTF-8 body (`base64:`), and referenced paths of mode `120000` and `160000`;
- an unreferenced body file that no reader fetches, and the orphan findings (`validate`);
- `revision-input-layout`, pinning REQ `record-revision-covers-bodies`;
- protected paths: a record whose `text` holds a stale reference to its live `q1.query.notes.md`. A `put` changing `text` writes `q1.query.text.<ext>`, leaves `q1.query.notes.md` in place (`expect_files`) and lists `shared-body-reference` under `skipped`, and `validate` reports the finding. A `repair` does not copy the notes body into `text`;
- a flat `{key}.query.json` template with `body_files`, rejected at load with `body-files-record-name` (validation vector);
- the access guard: a `delete`, and a `put` that changes `textFormat`, each on a record whose stored `text` reference is `../../x`, with a sentinel file at that path. In both, the sentinel is untouched (`expect_files`), the plan lists the `skipped` reference, and `validate` beforehand reports `unsafe-body-reference`;
- `repair` of three kinds of drift, each renaming the body inside the directory and leaving validation clean: an extension change (`notes` from `md` to `txt`), a body field rename (`notes` to `memo`, the stored reference still `q1.query.notes.md`), and a suffix change (template `{key}/{key}.q.json`, with the record file already at `q1/q1.q.json` and a reference still to `q1.query.text.sql`);
- an `unlisted-record-file` finding for `q2/q2.query.json` after a suffix change, which repair does not touch;
- a safe reference to a symlink, which `lstat` classifies as `non-regular-body` without following it;
- every rejection in REQ `body-files-definition-validated`, and the `suffix-overlap` rejections in REQ `suffix-disjointness-covers-body-files`, including the bare-template ancestor and the literal-basename case.

**Vendoring.** Each implementation MUST vendor both YAML files under a header that names the source and says "re-sync from the standard, do not edit", as `dalgo2ingitdb/testdata/conformance_vectors.yaml` does. Each MUST run its profiles in CI, with a check that fails when a vendored copy differs from the standard's. `FORMAT.md` is retired as a contract. The `format-fixtures` trees in `dalgo2ingitdb` and `ingitdb-ts` MAY be generated from the vectors, or dropped.

#### REQ: ts-layout-prerequisite-recorded

Today both `ingitdb-ts` clients hard-code a flat `$records/` layout. They ignore `records_dir` and nested name templates (`client-fs/src/client.ts:248-260`, `client-github/src/collection/collection.ts:237`), so they cannot pass the reader-profile `records_dir: '.'` vectors. That support is tracked as ingitdb/ingitdb-ts#104. The `ingitdb-ts` reader-profile run in AC `every-implementation-runs-vendored-vectors` MUST NOT be recorded as passing before #104 and the `ingitdb-ts` body-file reader both land.

## Acceptance Criteria

### AC: reference-definition-loads-strictly

**Requirements:** record-body-file#req:body-files-definition

**Given** a collection definition carrying the reference `record_file`, with `body_files: [{field: text, format_field: textFormat}, {field: notes, format: md}]` and string columns `textFormat`, `text` and `notes`
**When** it is read through `validator.ReadDefinition` with the strict decoder
**Then** it loads without error, and `RecordFile.BodyFiles` holds exactly those two entries

### AC: invalid-body-files-definitions-rejected

**Requirements:** record-body-file#req:body-files-definition-validated

**Given** a table of definitions, one per rule in REQ `body-files-definition-validated`:
- `type: '[]map[string]any'`;
- `format: markdown`;
- record name `{key}.query.json` (a flat layout);
- record name `x/{key}.query.json` (a directory that is not the key);
- record name `{key}/{key}.json` (no suffix);
- record name `{key}/{key}.a.b.json` (a multi-segment suffix);
- an entry carrying a `name` key;
- an entry with both `format` and `format_field`;
- an entry with neither;
- `field: Text`;
- `field: te.xt`;
- `format: Md`;
- `format: m.d`;
- `field` naming an undeclared column;
- `field` naming an `int` column;
- `format_field` naming an `int` column;
- `field` equal to a `format_field`;
- two entries with `field: text`;
- `field: query` under record name `{key}/{key}.query.json`

**When** each definition is loaded
**Then** every one fails at load time with the error kind for its rule, naming `body_files`, and no record file is read

### AC: body-name-and-record-template-overlap-rejected

**Requirements:** record-body-file#req:suffix-disjointness-covers-body-files

**Given** the reference `queries` collection (suffix `query`) in four databases:
- a collection `texts` with record name `{key}.text.json`, with its records base directory at `queries/`;
- a parent collection `projects` with the bare record name `{key}.yaml`, with `queries` as its subcollection;
- `texts` using `{key}.snippet.json` in the same place;
- a collection `notes` with record name `{key}.notes.json` in the same place

**When** each definition is read through `validator.ReadDefinition`
**Then** the first fails, naming `texts`, `{key}.text.json`, `queries` and `text`, because `format_field` allows `json`. The second fails, naming `projects` and `queries`. The third and fourth load without error: the `notes` entry's fixed `format: md` can never end in `.json`.

### AC: mixed-text-formats-in-one-collection

**Requirements:** record-body-file#req:body-extension-resolution, record-body-file#req:body-file-naming, record-body-file#req:read-merges-referenced-bodies

**Given** the reference `queries` collection holding the four records of REQ `body-file-naming`, whose `textFormat` values are `SQL`, `DTQL`, `HTTP` and `GraphQL`, each with a matching `$file` reference and body file. The `customer-invoices` DTQL body is multi-line, with a trailing newline and a non-ASCII character. The `active-customers` SQL body has no trailing newline.
**When** each record is read through the Go reader
**Then** each record's `text` equals the bytes of `<key>.query.text.<sql|dtql|http|graphql>` exactly, with the newlines and the non-ASCII character intact

### AC: stored-states-read-distinctly

**Requirements:** record-body-file#req:body-field-reference-encoding, record-body-file#req:read-merges-referenced-bodies

**Given** recovery configured and no pending change, and four records:
- `r1`, with `"notes": {"$file": "r1.query.notes.md"}` and a 0-byte `r1.query.notes.md`;
- `r2`, with `"notes": null`;
- `r3`, with no `notes` key;
- `r4`, with `"notes": null` and a leftover `r4.query.notes.md`

**When** each is read
**Then**:
- `r1`'s `notes` is `""`;
- `r2`'s `notes` is `null`;
- `r3` has no `notes` key;
- `r4`'s `notes` is `null`, and the leftover file is never opened.

### AC: several-body-fields-per-record

**Requirements:** record-body-file#req:body-file-naming, record-body-file#req:read-merges-referenced-bodies, record-body-file#req:write-plan-exposed

**Given** a record `customer-invoices` with `textFormat: DTQL` and references to `customer-invoices.query.text.dtql` and `customer-invoices.query.notes.md`
**When** it is read, and a put that changes both `text` and `notes` has its write plan computed
**Then** the read sets `text` and `notes` each to its own file's exact bytes; the plan writes `.query.text.dtql`, then `.query.notes.md`, then the record file, whose bytes hold only the two references

### AC: body-read-failures-are-errors

**Requirements:** record-body-file#req:read-merges-referenced-bodies, record-body-file#req:body-field-reference-encoding, record-body-file#req:body-extension-resolution, record-body-file#req:stored-reference-access-guard

**Given** these records:
- one whose record file holds `"text": "select 1"`;
- one whose record file holds `"text": {"$file": "q.query.text.sql", "$x": 1}`;
- one with a `text` reference to `../../x`;
- one with a `text` reference and an empty `textFormat`;
- one with a safe but drifted `text` reference;
- one with a `text` reference to a file that does not exist;
- one whose referenced file is a filesystem symlink;
- one whose referenced file is a Git tree entry of mode `120000`, read through `dalgo2ingitdb4github`;
- one whose referenced file holds invalid UTF-8

**When** each is read
**Then** each read fails with, in order: `inline-body-value`, `invalid-body-reference`, `unsafe-body-reference`, `format-unresolved`, `body-reference-mismatch`, `missing-body`, `non-regular-body`, `non-regular-body` and `invalid-utf8`. Each error names the record key and the field. The first five open no body file, and no file outside the record's directory is opened.

### AC: put-splits-body-out-of-record-file

**Requirements:** record-body-file#req:write-splits-bodies-out, record-body-file#req:write-plan-exposed

**Given** no stored record, and new data `{textFormat: DTQL, title: X, text: "from: invoices\n", notes: null}` for key `q1`
**When** the write plan is computed
**Then** the plan is exactly two operations, in order:
1. write `queries/q1/q1.query.text.dtql`, with bytes `from: invoices\n` and expected prior state absent;
2. write `queries/q1/q1.query.json`, whose bytes hold `"text": {"$file": "q1.query.text.dtql"}` and `"notes": null` and no body content, with expected prior state absent.

It names `queries/q1` as the containing directory.

### AC: json-text-allowed-and-isolated

**Requirements:** record-body-file#req:body-extension-resolution, record-body-file#req:body-files-isolated-from-listing

**Given** new data `{textFormat: JSON, text: "{}"}` for key `q4` in the reference collection
**When** the write plan is computed and applied, and the collection is then listed by `dalgo2ingitdb` (glob `*/*.query.json`) and by `dalgo2ingitdb4github` (template regex ending `\.query\.json$`)
**Then** the plan writes `queries/q4/q4.query.text.json`; both listings return `q4` exactly once, and neither returns a key or path for `q4.query.text.json`

### AC: null-or-absent-put-removes-existing-body

**Requirements:** record-body-file#req:write-splits-bodies-out, record-body-file#req:write-plan-exposed

**Given** a stored record `q1` that references `q1.query.text.sql`, and two kinds of new data: one with `text: null`, one with no `text` key
**When** each write plan is computed
**Then** each plan is, in order:
1. write the record file, holding `"text": null` or no `text` key respectively, with expected prior state the SHA-256 of the stored bytes;
2. remove `queries/q1/q1.query.text.sql`, with expected prior state the SHA-256 of the stored body.

### AC: invalid-body-values-refused-before-any-change

**Requirements:** record-body-file#req:write-splits-bodies-out

**Given** three kinds of new data: `text: 42`; `text: {"$file": "x"}`; and a `text` string with `textFormat: "S Q L"`
**When** each write plan is computed
**Then** the first two return `body-not-string`, the third returns `format-unresolved`, and none returns a plan, so a driver applying plans changes no file

### AC: format-change-removes-stale-body

**Requirements:** record-body-file#req:delete-and-format-change-leave-no-stale-body, record-body-file#req:write-plan-exposed

**Given** a stored record `q1` with `textFormat: SQL` that references `q1/q1.query.text.sql`, and new data with `textFormat: DTQL` and a `text` value
**When** the write plan is computed
**Then** the plan is, in order:
1. write `q1/q1.query.text.dtql`;
2. write the record file, now referencing `q1.query.text.dtql`;
3. remove `q1/q1.query.text.sql`.

Exactly one `text` body file remains after the plan is applied.

### AC: flat-layout-with-body-files-rejected

**Requirements:** record-body-file#req:body-files-definition-validated

**Given** the reference `queries` definition with `record_file.name` changed to the flat `{key}.query.json` (with `records_dir: '.'`), and a second copy using the nested `{key}/{key}.query.json`
**When** each is loaded through `validator.ReadDefinition`
**Then** the flat one fails at load time with `body-files-record-name`, naming `body_files` and the required `{key}/{key}.<suffix>.<ext>` shape, and no record file is read; the nested one loads

### AC: live-body-survives-shared-stale-reference

**Requirements:** record-body-file#req:stored-reference-access-guard, record-body-file#req:write-plan-exposed, record-body-file#req:drifted-record-repair

**Given** record `q1` (`textFormat: SQL`) whose `notes` references its live `q1.query.notes.md`, and whose `text` holds `{"$file": "q1.query.notes.md"}`
**When** the database is validated, a put with a new `text` value is planned and applied, and, separately, the original record is repaired
**Then** the outcomes are:
- validation reports `shared-body-reference` for `q1`'s `text`;
- the put plan writes `q1.query.text.sql` and the record file, removes nothing, and lists `shared-body-reference` under `skipped`;
- after the put, `q1.query.notes.md` still exists with its original bytes, and `q1` reads with both bodies;
- the repair does not read `q1.query.notes.md` as `text`: it writes `text` as `null`, leaves `q1.query.notes.md` untouched, and reports the skipped reference.

### AC: unsafe-stored-reference-never-followed

**Requirements:** record-body-file#req:stored-reference-access-guard, record-body-file#req:delete-and-format-change-leave-no-stale-body, record-body-file#req:write-plan-exposed

**Given** a sentinel file `outside.txt` at the database root. Record `q1` (`textFormat: SQL`) stores `"text": {"$file": "../../outside.txt"}`, and record `q2` stores the same reference.
**When** the database is validated; then `q1` is deleted; then `q2` is put with `textFormat: DTQL` and a `text` value, through its write plan and its driver
**Then** the outcomes are:
- validation reports `unsafe-body-reference` for `q1` and `q2`;
- `q1`'s delete plan removes only `queries/q1/q1.query.json`, and lists the reference under `skipped`;
- `q2`'s put plan writes `q2.query.text.dtql` and the record file, removes nothing, and lists the reference under `skipped`;
- `outside.txt` is never opened, stat'ed or changed, and every plan path stays inside the record's directory.

### AC: delete-removes-record-and-bodies

**Requirements:** record-body-file#req:delete-and-format-change-leave-no-stale-body, record-body-file#req:write-plan-exposed

**Given** a stored `customer-invoices` that references both body files, and a stored record `q2` whose `text` reference points to a file that is already missing
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

### AC: recovery-hook-holds-transaction-lock

**Requirements:** record-body-file#req:pending-changes-recovered-before-read

**Given** a writer that holds the driver's transaction lock with a live two-operation journal, paused after its first operation
**When** a reader with the driver's recovery hook starts reading the collection
**Then** the hook waits for the lock and does not touch the journal or either file while the writer holds it; once the writer completes and releases the lock, the reader sees the complete new state

### AC: every-caller-configures-recovery

**Requirements:** record-body-file#req:every-body-files-reader-configures-recovery

**Given** a working-tree database whose `queries` collection declares `body_files`
**When** each of these is run against it: `ingitdb validate`, `ingitdb diff`, `ingitdb pull`, `ingitdb ci`, `ingitdb materialize`, `ingitdb docs update`, a conflict resolution, `ingitdb-action` on a pinned `cli-version`, and `synchestra`'s embedded commands
**Then** each invokes `dalgo2ingitdb`'s recovery hook before reading `queries`, and none fails with the recovery-unavailable error

### AC: revision-covers-bodies

**Requirements:** record-body-file#req:record-revision-covers-bodies

**Given** a stored record `customer-invoices` and its revision `R`
**When** each of these is applied separately:
- only the `notes` body bytes change;
- only `textFormat` changes, so the `text` reference and file name change, with identical body bytes;
- `notes` changes from a reference to a 0-byte file into `null`;
- `notes` changes from `null` to absent

**Then** the revision computed from the exported canonical input differs from `R` in every case, including through `dalgo2ingitdb`'s protected profile

### AC: revision-input-layout-pinned

**Requirements:** record-body-file#req:record-revision-covers-bodies

**Given** the `revision-input-layout` vector from REQ `record-revision-covers-bodies`: `text` is a reference, `notes` is null and `summary` is absent
**When** the exported revision-input function builds the input
**Then** the input is 224 bytes, equal to the vector's base64, and its SHA-256 is `322fdd8c2f9e7a37fb98844e9fc76d76a482f3a126a8f75260816aba27038dc8`

### AC: lenient-definition-paths-become-strict

**Requirements:** record-body-file#req:implementations-current-and-strict

**Given** a `dalgo2ingitdb` collection definition declaring `body_files`, and a second definition carrying an unknown key under `record_file`
**When** each is read through `DescribeCollection`, and each is altered with an unrelated `ApplyAddField`
**Then** the first keeps `body_files` in both the described and the rewritten definition, and the second fails to decode on both paths

### AC: every-implementation-runs-vendored-vectors

**Requirements:** record-body-file#req:conformance-vectors-are-the-contract, record-body-file#req:implementations-current-and-strict, record-body-file#req:ts-layout-prerequisite-recorded

**Given** `conformance/record-layout/` in `ingitdb/ingitdb` with a normative `README.md`, `vectors.yaml` and `validation_vectors.yaml` in the format of REQ `conformance-vectors-are-the-contract`, covering every required case, and the `record_file` fields in `ingitdb-collection.schema.json`
**When** CI runs in `ingitdb-go`, `dalgo2ingitdb`, `dalgo2ingitdb4github` and `ingitdb-ts`
**Then** in each repository:
- both vendored files carry the re-sync header;
- the re-sync check passes;
- the repository's profiles pass: both profiles for the three Go implementations, and the reader profile only for `ingitdb-ts`;
- every accepted definition validates against the JSON Schema.

The `ingitdb-ts` run is gated on ingitdb/ingitdb-ts#104.

### AC: schema-alteration-keeps-body-files-consistent

**Requirements:** record-body-file#req:schema-alteration-keeps-body-files-consistent

**Given** the reference collection holding at least one record
**When** each of these alterations is attempted separately:
- `textFormat` is renamed to `lang`;
- `textFormat` is modified to `int`;
- `text` is dropped

**Then** the rename rewrites the entry's `format_field` to `lang` and leaves every file in place. The modify and the drop are refused, and leave `definition.yaml` and every file unchanged.

### AC: definition-change-drift-reported-by-validation

**Requirements:** record-body-file#req:name-drift-is-a-validation-finding, record-body-file#req:drifted-record-repair, record-body-file#req:stored-reference-access-guard

**Given** a populated reference collection, in three separate scenarios:
- **(a) format change:** record `q1` references `q1.query.notes.md`, and `notes` is then changed to `format: txt`;
- **(b) field rename:** `notes` is renamed to `memo` with `ApplyRenameField`, so `q1`'s `memo` holds `{"$file": "q1.query.notes.md"}`;
- **(c) suffix change:** `record_file.name` is changed to `{key}/{key}.q.json`. The author has moved `q1/q1.query.json` to `q1/q1.q.json`, which still references `q1.query.text.sql`, but has not moved `q2/q2.query.json`.

**When** the database is validated, `q1` is read, `q1` is repaired, and the database is validated again
**Then** the outcomes are:
- In every scenario, the first validation reports `body-reference-mismatch` for `q1`, and no special refusal or rename step runs.
- In every scenario, the read of `q1` fails with `body-reference-mismatch` and opens no file.
- In every scenario, the reference is classed safe, not unsafe.
- The repairs write, respectively, `q1.query.notes.txt`, `q1.query.memo.md` and `q1.q.text.sql`, each with the old file's bytes. Each rewrites the reference, removes the old file, and stays inside `queries/q1/`.
- After each repair, `q1` reads with its original text, and validation reports no finding for `q1`.
- In (c), validation still reports `unlisted-record-file` for `q2/q2.query.json` and orphans for its body files, and repair never touches `q2`.

### AC: symlinked-body-classified-by-lstat

**Requirements:** record-body-file#req:stored-reference-access-guard

**Given** a record whose safe `text` reference names a symlink to a regular file elsewhere in the repository
**When** it is read, validated, deleted and repaired
**Then** the read fails with `non-regular-body`, and validation reports it. The delete and repair plans do not read through the link, and the link target is never opened.

### AC: literal-basename-template-cannot-match-body

**Requirements:** record-body-file#req:body-files-isolated-from-listing, record-body-file#req:suffix-disjointness-covers-body-files

**Given** a collection with record name `{key}/record.json` whose records base directory is the reference `queries` directory, and a second database where that literal is `a.query.text.json` instead
**When** each definition is loaded and, for the first, `queries` holds `x/x.query.text.json` and `x/x.query.json`
**Then** the first loads, and its listing never returns `x.query.text.json`; the second fails with `suffix-overlap`

### AC: listing-never-returns-body-files

**Requirements:** record-body-file#req:body-files-isolated-from-listing

**Given** the reference `queries` collection holding the four records of REQ `body-file-naming` with all their body files
**When** records are listed and counted by `datavalidator`, the materializer, the foreign-key index, `dalgo2ingitdb` and `dalgo2ingitdb4github`
**Then** each lists exactly `active-customers`, `country-facts`, `customer-graph` and `customer-invoices`, and no reported key or file path is a body file

### AC: orphan-body-files-reported

**Requirements:** record-body-file#req:orphan-body-files-reported

**Given** a `queries` directory holding three files:
- `q9/q9.query.text.sql`, with no `q9/q9.query.json`;
- a leftover `q1/q1.query.text.sql` beside a record `q1` that references `q1.query.text.dtql`;
- `q2/q2.query.notes.md` beside a record `q2` whose `notes` is `null`

**When** the database is validated
**Then** validation reports exactly three orphan-body findings, one for each of those files. Records `q1` and `q2` still read: `q1` with its `.query.text.dtql` body and `q2` with `notes: null`.

### AC: definition-without-body-files-unchanged

**Requirements:** record-body-file#req:body-files-definition

**Given** the existing test suite, with no `body_files` declared anywhere
**When** they run after this Feature
**Then** every existing read, write, listing and validation result is unchanged, and no recovery hook is required

## Not Doing (and Why)

- **Designing the multi-file journal and its recovery.** ingitdb/dalgo2ingitdb#15 owns crash-safe multi-file changes. This Feature fixes only what #15 consumes: the ordered, complete write plan with expected prior states, the recovery hook every reader calls, and the refusal of multi-operation plans until #15 lands.
- **A configurable body file name.** The founder decided on 2026-09-17 on the fixed `<key>.<record-suffix>.<field>.<ext>` name. The `$file` reference must equal it, which keeps the disjointness rule a static check and keeps references from pointing anywhere else.
- **A rename or migration feature for body file names.** A name is a pure function of the definition and the record's values. Drift after a definition change is a validation finding (REQ `name-drift-is-a-validation-finding`), fixed per record by a put or by the repair plan (REQ `drifted-record-repair`), which is the ordinary write plan. There is no collection-wide migration transaction.
- **Renaming DataTug's existing `.query.<type>` sidecars.** That belongs to DataTug's hard cut-over (`datatug/datatug` Feature `dalgo-project-store`).
- **Flat layouts or multi-segment record suffixes in body-bearing collections.** Only `{key}/{key}.<suffix>.<ext>` is allowed. DataTug uses that layout, and it keeps every record's files alone in their own directory and name parsing unambiguous.
- **Writing the vectors or the schema change here.** They are owned by `ingitdb/ingitdb` (#9) and `ingitdb/ingitdb-schema` (#9). This Feature states what they must cover.
- **Implementing the TypeScript reader or its layout support.** Parity is proven by the shared vectors. The `ingitdb-ts` work is ingitdb/ingitdb-ts#104 plus its own body-file change.
- **Compatibility with pre-beta data or older releases.** The project is in private beta. Every implementation moves to latest (REQ `implementations-current-and-strict`).
- **Markdown documentation of the format.** The YAML comments are the documentation. Markdown MAY be added later, never as a contract.
- **Body files for list and map record types.** A many-record file has no per-record sibling owner.
- **Binary bodies or any encoding.** Records are text-only (`storage-format` REQ `text-only`).
- **A body-size cap.** `datatug-core` enforces its own cap before calling the store. A storage-level cap can be added later without changing this shape.
- **Repairing a torn state that was already committed to Git.** Recovery runs against the working tree and journal. A user commit of a working tree mid-crash is outside what a reader can detect.

## Rehearse Integration

No Rehearse stubs are scaffolded: `specscore.yaml` declares no rehearse configuration. Every AC is directly executable as a Go test: definition-load table tests through `validator.ReadDefinition`, pure write-plan and revision-input unit tests, temp-dir tests through the real readers and `datavalidator`, a vendored-vector runner in each implementation, and driver tests in `dalgo2ingitdb` and `dalgo2ingitdb4github`. The one exception is the `ingitdb-ts` vector runner, which is a vitest. This is recorded as an explicit skip rather than an omission.

## Dependent Modules

- **`ingitdb/ingitdb`** owns `conformance/record-layout/` (`README.md`, `vectors.yaml`, `validation_vectors.yaml`), tracked as #9.
- **`ingitdb/ingitdb-schema`** adds `records_dir` and `body_files` to `ingitdb-collection.schema.json` (#9).
- **`ingitdb-go`** itself vendors and runs the vectors against its validator, readers, write plan and revision input.
- **`ingitdb/dalgo2ingitdb`** vendors and runs the vectors, and retires `FORMAT.md` as a contract. It:
  - applies the write plan, refusing multi-operation plans until #15;
  - resolves `$file` references on `Get` and query reads (`record_io.go`, `query.go` `readAllSingleStored`);
  - implements the recovery hook (#15);
  - computes protected revisions from the canonical input (`protected.go`);
  - makes the `schema_reader.go` and `schema_modifier.go` definition decodes strict;
  - applies the field alteration rules;
  - moves to the latest `ingitdb-go/ingitdb`.
- **`ingitdb/dalgo2ingitdb4github`**:
  - resolves references and fetches only referenced blobs in `query.go` `scanSingleRecords` (`:172-203`) and in single-record reads;
  - refuses non-regular tree entries (mode `120000`, `160000`);
  - applies each write plan in one commit;
  - vendors and runs the vectors;
  - moves to the latest `ingitdb-go/ingitdb`.
- **`ingitdb/ingitdb-ts`** needs ingitdb/ingitdb-ts#104 (`records_dir` and nested templates). It then resolves `$file` references in `client-fs` and `client-github`, fetching only referenced files, and vendors and runs the reader profile.
- **`ingitdb/ingitdb-cli`** writes records through `dalgo2ingitdb`, so it inherits body-file handling. It configures `dalgo2ingitdb`'s recovery hook at every record-reading entry point listed in REQ `every-body-files-reader-configures-recovery`. Any direct file write it keeps must use the write plan.
- **`ingitdb/ingitdb-action`** pins an `ingitdb-cli` release that configures the hook.
- **`synchestra-io/synchestra`** configures the hook where it embeds `ingitdb-cli` commands and `materializer`.
- **`datatug/datatug-core`** declares the reference `queries` definition (`text` with `format_field: textFormat`, `notes` with `format: md`) under `dalgo-project-store`, and maps its model's null and empty values to the three stored states.

## Open Questions

None at this time.

Resolved on 2026-09-17 by founder decision:
- body file names are fixed, `<key>.<record-suffix>.<field>.<ext>`;
- the extension is either hardcoded (`format`) or taken per record from a field (`format_field`);
- presence is an explicit `{"$file": ...}` reference in the record file: `null` and absent are distinct, and a missing referenced file is an error (the former missing-body question, option A);
- `body_files` is a plural list;
- the contract is YAML conformance vectors plus JSON Schema, owned by `ingitdb/ingitdb` and `ingitdb/ingitdb-schema`;
- the project is in private beta, so there are no legacy-compatibility constraints;
- body file names are never renamed by a dedicated feature: a per-record format change is an ordinary put, and definition drift is a validation finding;
- per-record repair is kept (REQ `drifted-record-repair`), founder 2026-09-17: *"Keep it"*.

---
*This document follows the https://specscore.md/feature-specification*
