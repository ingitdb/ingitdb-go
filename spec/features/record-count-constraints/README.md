---
format: https://specscore.md/feature-specification
status: Draft
---

# Feature: Record-count constraints

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/record-count-constraints?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/record-count-constraints?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/record-count-constraints?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/record-count-constraints?op=request-change) |
**Status:** Draft
**Date:** 2026-07-16
**Owner:** alex
**Supersedes:** —
**Source Ideas:** —

## Summary

Enforce the `min_records_count` and `max_records_count` bounds a collection definition may declare: reject an invalid bound at definition-load time, and fire a collection-level validation error when a collection holds fewer than `min_records_count` or more than `max_records_count` records. Closes ingitdb-go#8.

## Problem

`min_records_count` and `max_records_count` are a *collection-level* integrity invariant — "this collection must not be empty", "this collection must hold at most N records". `geo-ingitdb` declared them on three subcollection definitions (`.ingitdb-subcol.states.yaml`, `.ingitdb-subcol.$admin_divisions_base.yaml`, `.ingitdb-subcol.$admin_divisions.settlements.yaml`), where `min_records_count: 1` reads as "this subcollection must not be empty" — a real constraint the author expected to be enforced.

inGitDB has **never** implemented them. Before the column-validation stack they were read and silently discarded; a `yaml.Unmarshal` with no `KnownFields` accepted the keys, mapped them to nothing, and reported the definition valid. `min_records_count`/`max_records_count` have zero readers in the Go code (verified: the only occurrences are this repo's spec text and the strict-decode test that lists them among keys it expects rejected). This is the same failure mode the column-validation Feature exists to end: config that looks live and does nothing is worse than an absent feature, because it convinces the author their data is guarded when it is not.

Since column-validation, `decodeCollectionDef` uses `yaml.NewDecoder(...).KnownFields(true)` (`validator/def_validator.go`), so an unmodelled key is now a hard load error rather than silent drop. That closed the "silently discarded" hole but left the constraint unimplemented: geo's keys were stripped so it would load under strict decoding, and ingitdb-go#8 preserves the intent. This Feature implements it — adding the two keys to the schema so they decode, validating the bounds at load, and enforcing the count during whole-database validation.

Unlike a column constraint, a record-count bound has no per-record check point; it needs a defined collection-level one. The existing whole-database validator (`datavalidator.simpleValidator.Validate`) already computes each root collection's record count as it runs its per-collection schema pass, so that count is the natural place to enforce the bound.

## Behavior

### REQ: record-count-bounds-declarable

`CollectionDef` MUST model `min_records_count` and `max_records_count` so that a definition declaring either decodes cleanly under the strict `KnownFields(true)` decoder rather than being rejected as an unknown key.

Both MUST be pointer-typed (`*int`, `yaml:"min_records_count,omitempty"` / `yaml:"max_records_count,omitempty"`), following the `MinValue`/`MaxValue` and `MinLength`/`MaxLength` precedent (`column_def.go`) and for the same reason: a declared zero must be distinguishable from "not declared". `max_records_count: 0` is meaningful — "this collection must be empty" (e.g. a placeholder collection, or one temporarily locked) — and with a plain `int` the natural `!= 0` guard would read that declared zero as unset and enforce nothing. Pointer typing also makes "a negative bound" a detectable definition-load error rather than an invisible value.

### REQ: reject-invalid-record-count-bounds

A record-count bound that cannot describe any valid collection MUST be rejected at definition-load time (in `CollectionDef.Validate`), naming the offending key:

- A negative `min_records_count` or `max_records_count` MUST be rejected — a collection can never hold a negative number of records.
- `min_records_count` greater than `max_records_count`, when both are declared, MUST be rejected — no record count can satisfy both.

Load-time rejection (rather than a validation-time error) is deliberate: an impossible bound is an author mistake in the schema itself, discoverable the moment the definition loads, exactly as the column-validation Feature rejects an inverted `min_value`/`max_value` range at load.

### REQ: record-count-enforced

During whole-database validation, each collection's record count MUST be checked against its declared bounds. When the count is below `min_records_count`, or above `max_records_count`, validation MUST emit a **collection-level** validation error (no `FieldName`, no `RecordKey`) that names the collection, states which bound was violated, and reports the actual count.

The count enforced MUST be the collection's **record count** — the number of distinct records — not a raw count of files in the collection directory. For a single-record-per-file collection these coincide, but for a `map[...]`- or `list`-of-records collection every record lives in one backing file, so a directory scan reports 1 regardless of how many records the file holds. The check therefore reuses the record count the schema pass already computes per collection (`validateCollectionRecords` returns it as `total`; it is derived from `countRecords` for single-record collections and from the parsed entry/row count for map/list collections), so no collection is re-read and the enforced number matches the number an author means by "records count".

An error already reported for a collection (a parse failure, a missing record file) does not suppress the record-count check; the bound is evaluated against whatever count the pass produced.

### Root-vs-subcollection scope

`datavalidator.simpleValidator.Validate` iterates only `def.Collections` — the **root** collections — for its per-collection schema pass; it never walks subcollection *records* (an open finding: the mainline validator does not validate subcollection records at all). This Feature enforces record-count for **root collections only**, which is the scope where the count is already computed and reliable.

This is a deliberate limitation, called out here rather than hidden:

- The motivating `geo-ingitdb` declarations were on *subcollections*, so this Feature does not yet enforce the exact original sites. Those declarations were already stripped from geo (which, separately, does not even load under the current layout — ingitdb-go#9), so nothing regresses; the intent is preserved and enforced for root collections, and extends to subcollections for free once the validator walks subcollection records.
- Enforcing record-count on subcollections while the rest of subcollection record validation remains unwalked would give a false impression that subcollections are validated. It is left to the broader open finding that owns walking subcollection records (the foreign-key pass already walks subcollections separately, so the machinery exists; unifying the two is out of scope here).

## Acceptance Criteria

### AC: min-records-count-rejects-too-few

**Requirements:** record-count-constraints#req:record-count-enforced

**Given** a root collection declaring `min_records_count: 2` and holding 1 record
**When** the database is validated
**Then** validation fails with a collection-level error naming the collection, the `min_records_count` bound `2`, and the actual count `1`

### AC: max-records-count-rejects-too-many

**Requirements:** record-count-constraints#req:record-count-enforced

**Given** a root collection declaring `max_records_count: 1` and holding 2 records
**When** the database is validated
**Then** validation fails with a collection-level error naming the collection, the `max_records_count` bound `1`, and the actual count `2`

### AC: record-count-within-bounds-passes

**Requirements:** record-count-constraints#req:record-count-enforced

**Given** a root collection declaring `min_records_count: 1` and `max_records_count: 5` and holding 3 records
**When** the database is validated
**Then** the record-count check contributes no error

### AC: max-records-count-zero-enforced

**Requirements:** record-count-constraints#req:record-count-bounds-declarable, record-count-constraints#req:record-count-enforced

**Given** a root collection declaring `max_records_count: 0` and holding 1 record
**When** the database is validated
**Then** validation fails — the declared zero is enforced as "must be empty", not read as unset

### AC: record-count-bounds-decode-under-strict-fields

**Requirements:** record-count-constraints#req:record-count-bounds-declarable

**Given** a collection definition declaring `min_records_count` and `max_records_count`
**When** the definition is decoded by the strict `KnownFields(true)` decoder
**Then** decoding succeeds and both bounds are present on the resulting `CollectionDef` — the keys are modelled, not rejected as unknown

### AC: negative-min-rejected-at-load

**Requirements:** record-count-constraints#req:reject-invalid-record-count-bounds

**Given** a collection definition declaring `min_records_count: -1`
**When** the definition is loaded
**Then** loading fails with an error naming `min_records_count` and stating it must not be negative

### AC: negative-max-rejected-at-load

**Requirements:** record-count-constraints#req:reject-invalid-record-count-bounds

**Given** a collection definition declaring `max_records_count: -1`
**When** the definition is loaded
**Then** loading fails with an error naming `max_records_count` and stating it must not be negative

### AC: min-exceeds-max-rejected-at-load

**Requirements:** record-count-constraints#req:reject-invalid-record-count-bounds

**Given** a collection definition declaring `min_records_count: 10` and `max_records_count: 5`
**When** the definition is loaded
**Then** loading fails with an error stating that `min_records_count` exceeds `max_records_count`

### AC: known-databases-validate-clean

**Requirements:** record-count-constraints#req:record-count-enforced

**Given** the loadable workspace databases (`demo-ingitdb`, `demo-commerce-ingitdb`, `e2e-test-ingitdb`, `bots-go-framework/can-i-use`), none of which declares a record-count bound
**When** each is loaded and validated with the new rules
**Then** the record-count check contributes no error to any of them

## Not Doing (and Why)

- **`record_labels`** — declared in `geo-ingitdb` and unimplemented, surfaced in the same ingitdb-go#8 sweep, but out of scope here. It is a display/labelling concern, not a count invariant, and warrants its own Feature.
- **`readme.sub_collections`** — the dropped readme table spec noted in ingitdb-go#8 (`CollectionReadmeDef` models only `hide_columns`, `hide_subcollections`, `hide_views`, `hide_triggers`, `data_preview`). Out of scope; a docs/readme concern unrelated to record counts.
- **Enforcing record-count on subcollections** — see *Root-vs-subcollection scope*. Deferred to the open finding that owns walking subcollection records; enforcing it in isolation would misrepresent how much of a subcollection is validated.
- **A materialize-time check point** — ingitdb-go#8 asked whether the bound should fire on validate, on materialize, or both. This Feature enforces it on **validate**, where the record count is already computed and where every other declared constraint (enum, FK, value-range) is checked. A materialize-time check can be added later without changing this contract.

## Rehearse Integration

No Rehearse stubs are scaffolded: `specscore.yaml` in this repo declares no rehearse configuration, and every AC here is directly executable as a Go test — a table test against `datavalidator` (record-count enforcement), against `CollectionDef.Validate` (load-time bound rejection), against the strict decoder (`record-count-bounds-decode-under-strict-fields`), and against the real reader + validator over a temp-dir fixture (`min-records-count-rejects-too-few` … `known-databases-validate-clean`). Recorded as an explicit skip rather than an omission.

## Dependent Modules

This Feature stacks on the column-validation stack (ingitdb-go#15, branch `integration/column-validation-stack`) for the strict `KnownFields(true)` decoder that makes a modelled key necessary. Adding two `*int` fields to `CollectionDef` is an additive, non-breaking Go API change (new nil-able fields), so no downstream compile site needs updating.

## Open Questions

- Should the bound also fire at materialize time, not only validate? (See *Not Doing*.) Deferred until an author asks.
- When the validator learns to walk subcollection records, record-count enforcement should extend to subcollections with no schema change here (the bounds already live on every `CollectionDef`, root or sub). Tracked with the subcollection-record-walking finding.

---
*This document follows the https://specscore.md/feature-specification*
