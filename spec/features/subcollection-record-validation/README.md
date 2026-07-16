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

The on-disk storage convention for subcollection records MUST be defined and documented as follows, and the validator MUST resolve subcollection data by it rather than by the subcollection's loaded `DirPath`.

A parent record MAY own a directory named by its record key, located as a sibling of where that parent collection stores its records — that is, under the parent collection's records base path (`<parent DirPath>/<parent record_file records-base-path>/<parentKey>/`, where the records-base-path is `$records` when the parent's `record_file.name` contains `{key}` and empty otherwise). That per-record directory holds the parent record's subcollections, one subdirectory per subcollection, named by the subcollection id. A subcollection's records then live inside its subdirectory exactly as a root collection's do, addressed by the subcollection's own `record_file` (its own records-base-path and name).

The effective data directory for one parent record's instance of a subcollection is therefore:

```
<parent DirPath>/<parent records-base-path>/<parentKey>/<subId>/
```

and its record file is that directory joined with the subcollection's own records-base-path and `record_file.name`. `demo-ingitdb` is the precedent: parent `orders` stores records at `orders/$records/{key}.yaml`, and order `ord001`'s `order_details` subcollection (a `[]map[string]any` in `details.json`) lives at `orders/$records/ord001/order_details/details.json`. The rule is recursive: a sub-subcollection of a subcollection record resolves the same way, treating the subcollection instance as the parent. Because the parent record file (`ord001.yaml`) and the parent record's subcollection directory (`ord001/`) are siblings differing only by extension, `countRecords` already de-duplicates them so a record owning subcollections is not double-counted.

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
