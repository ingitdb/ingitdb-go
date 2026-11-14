---
format: https://specscore.md/feature-specification
status: Approved
---

# Feature: Subcollection record validation

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/subcollection-record-validation?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/subcollection-record-validation?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/subcollection-record-validation?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/subcollection-record-validation?op=request-change) |
**Status:** Approved
**Date:** 2026-07-17
**Owner:** alex
**Supersedes:** —
**Source Ideas:** —

## Summary

Make whole-database validation recurse into subcollection *records*. Today `datavalidator.simpleValidator.Validate` iterates only `def.Collections` — the root collections — for its per-record schema pass, so a record stored in a subcollection is never checked against its declared schema. Every constraint an author writes on a subcollection column (type, `enum`, `required`, `required_when`, length, value-range, `foreign_key`, `min_records_count`/`max_records_count`) is silently inert. This Feature walks each collection's `SubCollections` recursively, validates their records with the same per-record checks the root pass uses, pins down and documents the on-disk storage convention for nested subcollection data, and reconciles the change with the foreign-key pass (which already descends into subcollections but, given the same undefined data path, never actually reached their records).

## Problem

A subcollection is a collection nested under a parent record — `orders/order_details` in `demo-ingitdb` is the working precedent, where each order owns a list of line items. inGitDB models this: `CollectionDef.SubCollections` is loaded by `validator/def_validator.go` (`loadSubCollections`, `loadSubCollectionsShared`), and `CollectionDef.Validate` recurses into subcollection *definitions*. But the record-validation layer does not.

`datavalidator.simpleValidator.Validate` loops `def.Collections` and calls `validateCollectionRecords` per root collection. It never descends into `colDef.SubCollections`. So the schema constraints a subcollection declares — `order_details.product_id` is `required` and a `foreign_key: products`, `quantity` is an `int`, `$ID` has `max_length: 64` — are read at load time and then never enforced against a single stored record. This is the same silent-no-op failure the column-validation and record-count Features exist to end: config that looks live and does nothing.

Two concrete consequences motivate this now:

- The record-count Feature (`spec/features/record-count-constraints`) deliberately scoped its enforcement to root collections and left an explicit open finding: "*When the validator learns to walk subcollection records, record-count enforcement should extend to subcollections with no schema change here.*" The original `geo-ingitdb` bug (ingitdb-go#8) declared `min_records_count` on three *subcollections*; root-only enforcement never covers the exact original sites. This Feature is the one that owns walking subcollection records, so it closes that loop.
- A recent `geo-ingitdb` rebuild had to be modelled as flat root collections plus foreign keys instead of a natural nested hierarchy, specifically because nested subcollection data was not validated and its on-disk path was undefined. Establishing and enforcing the convention removes that reason.

### Why the storage path is undefined today

A subcollection has no single data directory — its records live once per parent record. The loaders reflect that they had no data path to assign: in the old layout `readCollectionDef` sets a subcollection's `DirPath` to its **schema** directory (`.collection/subcollections/<sub>/`), and in the shared layout `readCollectionDefShared` sets it to the shared data root. Neither points at record data. So when the foreign-key pass (`datavalidator/foreign_key_check.go`) walks `col.SubCollections` and calls `loadCollectionRecords(sub)`, it computes a records path under that schema/shared directory, finds no file, and silently reads zero records — the descent is real but reaches nothing. Any convention this Feature defines must therefore be resolved *per parent record*, not read from the subcollection's loaded `DirPath`.

## Behavior

### REQ: subcollection-storage-convention

The on-disk storage convention for subcollection records MUST be defined and documented as follows, and the validator MUST resolve subcollection data by it rather than by the subcollection's loaded `DirPath`. Every other reader and writer (the storage drivers, `ingitdb select`, collection-group reads) MUST use the same location.

A parent record MAY own a directory named by its record key, located as a sibling of where that parent collection stores its records — that is, under the parent collection's records base path (`<parent DirPath>/<parent record_file records-base-path>/<parentKey>/`, where the records-base-path is the parent's `record_file.records_dir` when it is set, `.` meaning empty (see [explicit-record-base-directory](../explicit-record-base-directory/README.md)), and otherwise `$records` when the parent's `record_file.name` contains `{key}` and empty otherwise; this is `RecordFileDef.RecordsBasePath`). That per-record directory holds the parent record's subcollections, one subdirectory per subcollection, named by the subcollection id. A subcollection's records then live inside its subdirectory exactly as a root collection's do, addressed by the subcollection's own `record_file` (its own records-base-path and name).

The effective data directory for one parent record's instance of a subcollection is therefore:

```
<parent DirPath>/<parent records-base-path>/<parentKey>/<subId>/
```

and its record file is that directory joined with the subcollection's own records-base-path and `record_file.name`. `demo-ingitdb` is the precedent: parent `orders` stores records at `orders/$records/{key}.yaml`, and order `ord001`'s `order_details` subcollection (a `[]map[string]any` in `details.json`) lives at `orders/$records/ord001/order_details/details.json`. The rule is recursive: a sub-subcollection of a subcollection record resolves the same way, treating the subcollection instance as the parent. Because the parent record file (`ord001.yaml`) and the parent record's subcollection directory (`ord001/`) are siblings differing only by extension, `countRecords` already de-duplicates them so a record owning subcollections is not double-counted.

### REQ: parent-record-optional

A parent record MAY be absent while its subcollections hold records. This is how Firestore works: `ext/commitius/cmSessions/{id}` can exist although no document `ext/commitius` was ever written. A parent key MUST therefore be taken both from the parent's records and from the per-record directories that exist under the parent's records base path: a directory `<parent DirPath>/<parent records-base-path>/<name>/` that contains a directory `<subId>/` is the parent instance `<name>` of subcollection `<subId>`, whether or not a record file for `<name>` exists. The rule applies at every depth, so a record two missing parents below a root collection is validated. Which directories are parent instances is decided by [REQ: parent-dirs-one-rule](#req-parent-dirs-one-rule), and a directory that does not contain `<subId>/` MUST NOT create an instance (so no record-count finding is raised for it). A parent key found both ways is one instance, not two. A missing parent is not a record: it MUST NOT be counted in the parent collection's record count. If the parent's records cannot be read (a map- or list-of-records file that fails to parse), the directory-found instances MUST still be validated. Before this rule such records were never opened, so validation passed whatever they held (a database with invalid data below a missing parent therefore fails now, by design); an author had to write an empty `{}` parent record to get them validated, and that workaround keeps working.

**Layout.** Discovery reads the location the definition gives an instance, `<parent DirPath>/<parent records-base-path>/<name>/<subId>/` ([REQ: subcollection-storage-convention](#req-subcollection-storage-convention)), for every `records_dir`: with `records_dir: .` (the Firestore-mirror layout) that is `<parent DirPath>/<name>/<subId>/`; under the default `$records` it is `<parent DirPath>/$records/<name>/<subId>/`; under a custom `records_dir: data` it is `<parent DirPath>/data/<name>/<subId>/`. Data in any other place is mislaid ([REQ: mislaid-subcollection-records-rejected](#req-mislaid-subcollection-records-rejected)).

### REQ: parent-dirs-one-rule

One exported rule, `ingitdb.ScanParentDirs`, MUST decide which directories are parent instances, for validation and for any reader that enumerates instances (the local storage driver's collection-group read), so a reader can never return a record that validation did not consider. The rule is applied to the directory the definition gives (`<parent DirPath>/<parent records-base-path>/`). The rule: hidden entries are not parents; the names inGitDB itself writes in a collection directory (`$records`, `$views`, `$ingitdb`, and the first path segment of the parent's own `records_dir`, so `data` for `records_dir: data/items`) are not parents, even under `records_dir: .` where a mirrored Firestore document could carry such an id (such a document's subcollections are not read: rename the id); every other name is a parent key, including other `$`-prefixed names, because Firestore allows document ids such as `$x`; names matched by the parent's `exclude_regex` are not parents; and an instance directory reached through a symbolic link is used only when it resolves inside the database root. One that resolves outside MUST NOT be read, also when the parent has a record file, and validation MUST report it as an error naming the link (as the collection-group read refuses it), so a pass that skipped records does not succeed. The parent directory MUST be read once per level, not once per subcollection.

### REQ: mislaid-subcollection-records-rejected

Every reader MUST follow the layout the definition gives ([REQ: subcollection-storage-convention](#req-subcollection-storage-convention)). When a parent's records-base-path is not empty (its `records_dir` is not `.`), an instance directory found at `<parent DirPath>/<key>/<subId>/` holding at least one non-hidden file is mislaid: that is the layout earlier storage-driver versions (dalgo2ingitdb, dalgo2ingitdb4local) wrote. Validation MUST NOT validate its records and MUST report one error finding per mislaid directory, with `CollectionID` the subcollection's full path, `RecordKey` the parent key, `FilePath` the found directory, and a message naming the found path, the number of files, the path the definition expects, and the two fixes: move the files there, or set `record_file.records_dir: "."` on the parent and move the parent's own records out of its records directory. This makes a database that kept such data and used to pass fail; that is intended (owner decision, 2026-11-14: the layout the definition defines is the right one, and a detectable other layout is an error with details).

### REQ: subcollection-records-schema-validated

Whole-database validation MUST recurse into every collection's `SubCollections`, to arbitrary depth, and validate each subcollection record with the same per-record checks the root pass applies (`validateRecordData` plus the foreign-key pass): declared column types, `enum` membership, `required`, `required_when`, `length`/`min_length`/`max_length`, `min_value`/`max_value`, the undeclared-field check, the computed-column-must-not-be-stored rule, and foreign-key existence. A violation in a subcollection record MUST be reported as a finding whose `CollectionID` is the subcollection's full slash-path (e.g. `orders/order_details`) and whose `FilePath` identifies the concrete on-disk record file of the owning parent instance, so two parent records' instances of the same subcollection are distinguishable.

Record parsing/dispatch MUST reuse the existing per-record machinery (`validateCollectionRecords` and its `SingleRecord`/`MapOfRecords`/`ListOfRecords` handlers), so a subcollection is validated identically to a root collection of the same record type, including its parse-error and unsupported-record-type reporting.

### REQ: subcollection-record-count-enforced

A subcollection MAY declare `min_records_count`/`max_records_count`; these MUST be enforced per parent-record instance. Each parent record's instance of the subcollection is an independent collection with its own record count, so `min_records_count: 1` on a subcollection means "every parent record's instance of this subcollection must be non-empty". A violation MUST be a collection-level finding (no `FieldName`) naming the subcollection's full path, the bound, and the actual count, with `FilePath` set to the offending instance's data directory so the owning parent is identifiable. This fulfils the open finding the record-count Feature left for the pass that walks subcollection records.

### REQ: foreign-key-pass-reconciled

The foreign-key pass MUST locate subcollection records by the same per-parent-record convention (REQ subcollection-storage-convention), so that a `foreign_key` declared on a subcollection column is actually checked against its target. Both passes MUST share one subcollection-instance enumeration so their notion of "where a subcollection's records live" cannot drift. The foreign-key index remains built from root collections only — a `foreign_key` resolves module-relative to a root collection, never to a subcollection — and target resolution for a subcollection column continues to use the subcollection's full slash-path as the declaring id (`ingitdb.ResolveForeignKey`), preserving module-relative resolution.

### REQ: root-validation-unchanged

Adding subcollection validation MUST NOT change the findings produced for root collections. The existing root pass (per-collection schema validation, record-count enforcement, and record-count bookkeeping) is retained as-is; subcollection validation is additive. Record-count bookkeeping (`SetRecordCounts`) for a subcollection MUST aggregate across all parent-record instances under the subcollection's full path, so later instances do not overwrite earlier ones.

## Acceptance Criteria

### AC: subcollection-record-type-error-surfaced

**Requirements:** subcollection-record-validation#req:subcollection-records-schema-validated

**Given** a root collection whose subcollection declares a column `qty` of type `int`, and one parent record whose subcollection record stores `qty: "lots"` (a string)
**When** the database is validated through the real reader and validator
**Then** validation reports a wrong-type finding whose `CollectionID` is the subcollection full path and whose `FilePath` names the parent instance's record file

### AC: subcollection-required-field-enforced

**Requirements:** subcollection-record-validation#req:subcollection-records-schema-validated

**Given** a subcollection declaring a `required` column that one parent record's subcollection record omits
**When** the database is validated
**Then** validation reports a missing-required-field finding for that subcollection record

### AC: subcollection-undeclared-field-surfaced

**Requirements:** subcollection-record-validation#req:subcollection-records-schema-validated

**Given** a subcollection record carrying a field the subcollection definition does not declare
**When** the database is validated
**Then** validation reports an undeclared-field finding for that field on the subcollection record

### AC: subcollection-foreign-key-checked

**Requirements:** subcollection-record-validation#req:foreign-key-pass-reconciled

**Given** a subcollection column declaring `foreign_key` to a root collection, and one subcollection record whose value has no matching key in that target
**When** the database is validated
**Then** validation reports a dangling foreign-key finding naming the field, the value, and the target collection

### AC: valid-subcollection-records-pass

**Requirements:** subcollection-record-validation#req:subcollection-records-schema-validated

**Given** a subcollection every one of whose records satisfies its declared schema and foreign keys
**When** the database is validated
**Then** the subcollection contributes no finding

### AC: nested-subcollection-records-validated

**Requirements:** subcollection-record-validation#req:subcollection-records-schema-validated, subcollection-record-validation#req:subcollection-storage-convention

**Given** a subcollection that itself declares a subcollection, and a schema violation in a record two levels below a root collection
**When** the database is validated
**Then** the violation is reported, demonstrating that recursion reaches arbitrary depth via the per-parent-record path convention

### AC: subcollection-min-records-count-enforced-per-instance

**Requirements:** subcollection-record-validation#req:subcollection-record-count-enforced

**Given** a subcollection declaring `min_records_count: 1` and a parent record whose instance of that subcollection holds zero records
**When** the database is validated
**Then** validation reports a collection-level record-count finding naming the subcollection full path and the offending parent instance

### AC: root-findings-unchanged-by-subcollection-walk

**Requirements:** subcollection-record-validation#req:root-validation-unchanged

**Given** a database whose only defects are in root-collection records
**When** it is validated before and after this Feature
**Then** the set of root-collection findings is identical — subcollection validation adds findings only for subcollection records

### AC: demo-ingitdb-subcollections-validate-clean

**Requirements:** subcollection-record-validation#req:subcollection-records-schema-validated, subcollection-record-validation#req:foreign-key-pass-reconciled

**Given** `demo-ingitdb`, whose `orders/order_details` subcollection records satisfy their schema and foreign keys
**When** it is loaded and validated through the real reader and validator
**Then** no finding is reported whose `CollectionID` is `orders/order_details` — the newly-walked subcollection is clean, and the database's pre-existing root-level findings (unrelated `commerce.addresses` type errors) are unchanged

### AC: parentless-subcollection-records-validated

**Requirements:** subcollection-record-validation#req:parent-record-optional

**Given** a parent collection `ext` with no record file `ext/commitius.json`, and subcollection records `ext/commitius/sessions/s1.json` (`minutes: "thirty"` where `minutes` is an `int`) and `s2.json` (valid)
**When** the database is validated
**Then** one wrong-type finding is reported for `s1.json` under `ext/sessions`, the record counts for `ext/sessions` are 1 passed of 2, and `ext` counts 0 records

### AC: parentless-nested-records-validated

**Requirements:** subcollection-record-validation#req:parent-record-optional

**Given** `spaces/s1.json` exists but `spaces/s1/ext/commitius.json` does not, and `spaces/s1/ext/commitius/checkIns/c1.json` breaks its schema; and `spaces/s2/...` has no parent record at either level
**When** the database is validated
**Then** the finding for `c1.json` is reported under `spaces/ext/checkIns`, and both `checkIns` records are counted

### AC: parentless-foreign-key-checked

**Requirements:** subcollection-record-validation#req:parent-record-optional, subcollection-record-validation#req:foreign-key-pass-reconciled

**Given** a record below a missing parent whose `foreign_key` column holds a value with no target record
**When** the database is validated
**Then** a dangling foreign-key finding is reported for it

### AC: parentless-discovery-ignores-other-entries

**Requirements:** subcollection-record-validation#req:parent-record-optional

**Given** a subcollection with `min_records_count: 1` and, under the parent's records base path, a hidden directory, `$records` and `$views` directories, a name matched by `exclude_regex`, a directory without the subcollection's directory, and a file named like the subcollection
**When** the database is validated
**Then** none of them is a parent instance: no finding is raised for them

### AC: parent-dirs-one-rule

**Requirements:** subcollection-record-validation#req:parent-dirs-one-rule

**Given** below a parent with `records_dir: .`: a `$x` directory, a symbolic link to a directory inside the database, a symbolic link to a directory outside it, `$views`, `$records`, `$ingitdb`, the parent's own `records_dir`, a hidden directory and a name matched by `exclude_regex`, each holding the subcollection
**When** the database is validated, and `ingitdb.ScanParentDirs` is asked for the parent keys
**Then** `$x` and the inside link are parent instances and their records are validated; the outside link is not read and is reported as one error naming the link; the other names are not parents

### AC: mislaid-subcollection-records-rejected

**Requirements:** subcollection-record-validation#req:mislaid-subcollection-records-rejected

**Given** `spaces` with the default `records_dir` and records of its `notes` subcollection both at `spaces/$records/s1/notes/` and, as earlier storage drivers wrote them, at `spaces/family/notes/` (two files)
**When** the database is validated
**Then** the `$records` record is validated, and one error for `spaces/notes`, parent key `family`, says that 2 files at `spaces/family/notes/` belong at `spaces/$records/family/notes/` and how to fix it

## Not Doing (and Why)

- **Materialize-time subcollection checks** — as with the root record checks and record-count enforcement, validation is the single check point. A materialize-time check can be added later without changing this contract.
- **Fixing surfaced data defects in workspace databases** — walking subcollection records may reveal pre-existing defects that were never checked before (e.g. `demo-commerce-ingitdb` has four `products` root records with malformed YAML that fail to parse; because those keys never enter the foreign-key index, the `order_details` records that reference them now surface as dangling foreign keys). Those are data defects in a separate repository, reported for the maintainer, not fixed by this library change.
- **A new record_file field for subcollection layout** — the convention is derived from the existing `record_file` (`RecordsBasePath` + `name`) and the parent record key. No schema addition is needed, so none is made.
- **Changing how subcollection `DirPath` is assigned at load** — the loaders' `DirPath` for a subcollection is left as-is (it is used elsewhere for schema-relative concerns); the validator resolves data per parent record rather than trusting that field. Re-defining the loaded `DirPath` is a larger change with broader blast radius and is out of scope.

## Rehearse Integration

No Rehearse stubs are scaffolded: `specscore.yaml` declares no rehearse configuration, and every AC here is directly executable as a Go test — table tests against `datavalidator` over temp-dir fixtures built through the real reader (`validator.ReadDefinition` + `datavalidator.NewValidator().Validate`), plus an assertion over the real `demo-ingitdb` database. Recorded as an explicit skip rather than an omission.

## Dependent Modules

This Feature builds on the column-validation stack (per-record checks, strict `KnownFields(true)` decoding) and the record-count-constraints Feature (the `min_records_count`/`max_records_count` bounds it now extends to subcollections). It adds no exported Go API and no definition-schema field, so no downstream compile site needs updating; the only behavior change is additional findings for subcollection records that were previously unchecked.

## Open Questions

- For a parent collection that stores all its records in one file (a `map`/`list` collection whose `record_file.name` has no `{key}`), the per-record subcollection directory is `<parent DirPath>/<parentKey>/<subId>/`. This is the documented convention but is unexercised by any current workspace database (every subcollection parent in the workspace is a `{key}`-per-file collection); it can be revisited if such a layout is ever authored.

---
*This document follows the https://specscore.md/feature-specification*
