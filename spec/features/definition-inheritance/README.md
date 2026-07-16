---
format: https://specscore.md/feature-specification
status: Approved
---

# Feature: Definition inheritance

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/definition-inheritance?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/definition-inheritance?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/definition-inheritance?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/definition-inheritance?op=request-change) |
**Status:** Approved
**Date:** 2026-07-16
**Owner:** alex
**Supersedes:** —
**Source Ideas:** —

## Summary

A collection or subcollection definition may declare `inherits: <path>` to overlay configuration from a base *partial* definition, so a family of related definitions can share columns and settings instead of repeating them. The base is loaded, merged under the child (child wins), and only the merged result is validated. A missing base or an inheritance cycle is a load-time error.

## Problem

`inherits:` is a designed-but-never-implemented feature. `geo-ingitdb` declared it across four subcollection definitions — for example `countries/.ingitdb-subcol.states.yaml` opened with `inherits: .ingitdb-subcol.$admin_divisions.yaml`, and a diamond `$admin_divisions_base` → `$admin_divisions` → `states`/`provinces`/`oblasts` sat behind a `$`-prefixed naming convention for abstract bases. **`inherits` has zero references anywhere in the Go code.** The key was read and silently discarded, so the entire hierarchy was inert: the `$`-prefixed bases declared columns and constraints that never applied to any record, which is the puzzle recorded in ingitdb-go#7.

Silent discard is the specific harm. An author writes `inherits: base.yaml`, inGitDB reports the definition valid, and none of the base's columns or constraints take effect — the definition looks live and does nothing. This is the same failure mode the `column-validation` Feature exists to end, and the strict `decodeCollectionDef` (`KnownFields(true)`) it introduced now *rejects* `inherits:` outright, because `CollectionDef` has no such field. So today the key is not merely inert — it makes the definition unloadable. The choice recorded in ingitdb-go#7 was "implement it or delete the concept." This Feature implements it.

Verified against the branch this stacks on (`integration/column-validation-stack`): `grep -rn inherits ingitdb/*.go` returns nothing outside test fixtures, and `decodeCollectionDef` rejects any document carrying an `inherits:` key (`validator/strict_fields_test.go:57`).

## Behavior

### REQ: inherits-resolves-base-partial

A `CollectionDef` MAY declare `inherits: <path>`. The value is a **filesystem path resolved relative to the directory that contains the inheriting definition file** (the `.collection/` schema directory in the per-collection layout, or the `.collections/<name>/` directory in the shared layout). It points to a YAML file holding a *partial* `CollectionDef` — the base.

The base MUST be decoded with the same strict rules as any definition (`decodeCollectionDef`, `KnownFields(true)`): an unrecognised key in a base partial is a load-time error naming the key, exactly as it is in a top-level definition. This keeps a base from becoming a place where silently-discarded config hides.

Path resolution is deliberately filesystem-based rather than by-collection-ID. A base is a partial — it need not be a loadable collection on its own (a base commonly declares only shared columns and no `record_file`) — so it cannot be addressed as an entry in the resolved collection tree, and resolving by path avoids a chicken-and-egg dependency on the very tree that inheritance helps build. A `$`-prefixed base filename (e.g. `$base.yaml`) is the recommended convention: the shared-layout subcollection scanner already skips `$`-prefixed directory names, and a loose `.yaml` file is never mistaken for a collection or a view in either layout, so a base file never accidentally becomes a collection.

### REQ: column-merge-child-wins

Columns merge **by name**. A column the child does not declare is inherited from the base unchanged. A column the child *does* declare **wholly replaces** the base's column of the same name — the child's `ColumnDef` is used in full, not deep-merged field-by-field.

Whole-column replacement (rather than per-field merge) is chosen for unambiguity: to change one constraint on an inherited column, the child redeclares the column. A per-field merge would make *removing* an inherited constraint (e.g. dropping a base's `max_value`) impossible to express, and would introduce order- and presence-dependent surprises. This matches the original `geo-ingitdb` intent, where `states.yaml` redeclared `population` in full with its own bounds rather than tweaking the base's.

### REQ: scalar-and-map-field-inheritance

For non-column fields, the base contributes only what the child leaves unset:

- `titles` — merged **by locale key**; the child's entry wins for any locale it declares, and the base supplies locales the child omits.
- `record_file` — the child's wins if present (non-nil); otherwise inherited from the base.
- `data_dir`, `columns_order`, `primary_key` — the child's wins if non-empty; otherwise inherited.
- `default_view`, `readme`, `conflict_resolution` — the child's wins if present (non-nil); otherwise inherited.

The following are **never** inherited, because they are identity or are populated from the filesystem after the definition file is decoded, not from its content: `id`, the resolved `DirPath`, `SubCollections`, and `Views`. In particular, inheritance does **not** copy subcollection or view topology from a base; those are discovered from the child's own directory. (The original `geo-ingitdb` base `$admin_divisions` declared a `subCollections:` list; that shape is out of scope — see *Not Doing*.)

### REQ: inheritance-chains-supported

A base partial MAY itself declare `inherits:`, forming a chain (`child` → `base` → `base-of-base` → …). The chain MUST be resolved transitively: each level's `inherits` is resolved relative to **that level's own file**, the far end is merged first, and each nearer definition overlays it, so a nearer definition always wins over a farther one. This supports the diamond `$admin_divisions_base` → `$admin_divisions` → `states` that `geo-ingitdb` designed.

### REQ: missing-base-is-load-error

If `inherits` resolves to a path that cannot be read, loading MUST fail with an error naming the inheriting definition file and the unresolvable base path. Failing loudly is the entire point of the Feature: the bug being fixed is that an unresolvable (or any) `inherits` was silently discarded.

### REQ: inheritance-cycles-rejected

A cycle — a definition that inherits itself, two definitions that inherit each other, or any longer loop — MUST be a load-time error naming the cycle, never an infinite loop or a stack overflow. Cycle detection MUST track the set of files already visited on the current chain (by resolved absolute path) and error when a file is about to be visited twice.

### REQ: merged-definition-is-validated

Definition validation MUST run on the **merged** result, not on the child or base in isolation. Two consequences follow, both intended:

- A base partial is **not** validated as a standalone collection. A base that declares only shared columns and no `record_file` is legal, because it is never validated alone — only after a child (or a nearer base) has supplied the missing pieces.
- A merged definition that is *still* incomplete — e.g. neither child nor any base supplies `record_file`, or the merged column set is empty — MUST fail validation loudly, exactly as a non-inheriting definition with the same gap would. Inheritance fills gaps; it never suppresses the checks for gaps it did not fill.

Inheritance is resolved during definition load in `validator/def_validator.go`, before `DirPath`/`data_dir` resolution and before subcollections and views are loaded, so the merge is layout-agnostic and every downstream reader sees the fully-merged `CollectionDef`.

## Acceptance Criteria

### AC: base-only-column-is-inherited

**Requirements:** definition-inheritance#req:inherits-resolves-base-partial, definition-inheritance#req:column-merge-child-wins

**Given** a base partial declaring column `population` (type `int`) and a child definition that declares `inherits: $base.yaml`, its own column `name`, and a `record_file`, but not `population`
**When** the database is loaded with validation
**Then** loading succeeds and the merged collection has both `name` and `population` columns

### AC: child-column-overrides-base-column

**Requirements:** definition-inheritance#req:column-merge-child-wins

**Given** a base partial declaring column `code` (type `string`, `max_length: 2`) and a child that declares `inherits: $base.yaml` and its own column `code` (type `string`, `max_length: 5`)
**When** the database is loaded and a record with `code: "abcd"` is validated
**Then** validation passes — the child's `max_length: 5` is in force, the base's `max_length: 2` was wholly replaced

### AC: scalar-field-inherited-when-child-omits-it

**Requirements:** definition-inheritance#req:scalar-and-map-field-inheritance, definition-inheritance#req:merged-definition-is-validated

**Given** a base partial declaring a `record_file` and a child that declares `inherits: $base.yaml` and columns but **no** `record_file`
**When** the database is loaded with validation
**Then** loading succeeds — the child inherited the base's `record_file`

### AC: titles-merge-by-locale

**Requirements:** definition-inheritance#req:scalar-and-map-field-inheritance

**Given** a base partial with `titles: {en: Divisions}` and a child with `titles: {ru: Штаты}` that inherits it
**When** the database is loaded
**Then** the merged collection's titles contain both `en: Divisions` and `ru: Штаты`

### AC: missing-base-is-load-error

**Requirements:** definition-inheritance#req:missing-base-is-load-error

**Given** a definition declaring `inherits: $nonexistent.yaml` with no such file present
**When** the database is loaded
**Then** loading fails with an error naming the inheriting file and the unresolvable base — it is not silently discarded

### AC: self-inheritance-is-a-cycle

**Requirements:** definition-inheritance#req:inheritance-cycles-rejected

**Given** a base partial `$a.yaml` that declares `inherits: $a.yaml` and a child that inherits `$a.yaml`
**When** the database is loaded
**Then** loading fails with an inheritance-cycle error, not an infinite loop

### AC: mutual-inheritance-is-a-cycle

**Requirements:** definition-inheritance#req:inheritance-cycles-rejected

**Given** base partials `$a.yaml` (`inherits: $b.yaml`) and `$b.yaml` (`inherits: $a.yaml`), and a child that inherits `$a.yaml`
**When** the database is loaded
**Then** loading fails with an inheritance-cycle error naming the loop

### AC: multi-level-chain-merges-far-base

**Requirements:** definition-inheritance#req:inheritance-chains-supported, definition-inheritance#req:column-merge-child-wins

**Given** `$grandbase.yaml` declaring column `area`, `$base.yaml` declaring `inherits: $grandbase.yaml` plus column `population`, and a child declaring `inherits: $base.yaml` plus column `name` and a `record_file`
**When** the database is loaded with validation
**Then** loading succeeds and the merged collection has all of `area`, `population`, and `name`

### AC: unknown-key-in-base-partial-rejected

**Requirements:** definition-inheritance#req:inherits-resolves-base-partial

**Given** a base partial that declares an unrecognised top-level key `min_records_count: 1` and a child that inherits it
**When** the database is loaded
**Then** loading fails naming the unrecognised key — strict decoding applies to base partials, not only to top-level definitions

### AC: merged-definition-still-missing-record-file-fails

**Requirements:** definition-inheritance#req:merged-definition-is-validated

**Given** a base partial with columns but no `record_file`, and a child that inherits it and also declares no `record_file`
**When** the database is loaded with validation
**Then** loading fails with the usual "missing 'record_file'" error — inheritance did not mask the gap because nothing supplied it

### AC: nearer-base-wins-over-farther-base

**Requirements:** definition-inheritance#req:inheritance-chains-supported, definition-inheritance#req:column-merge-child-wins

**Given** `$grandbase.yaml` declaring column `code` (`max_length: 2`) and `$base.yaml` declaring `inherits: $grandbase.yaml` and its own column `code` (`max_length: 5`), and a child that inherits `$base.yaml` and does not declare `code`
**When** the database is loaded and a record with `code: "abcd"` is validated
**Then** validation passes — the nearer base's `code` (`max_length: 5`) overrode the farther base's, and the child inherited the nearer one

## Not Doing (and Why)

- **Inheriting subcollection or view topology.** The original `geo-ingitdb` base `$admin_divisions` declared a `subCollections:` list under `inherits`. In the current layouts, subcollections and views are discovered from the filesystem, not from the definition file, so inheriting them would mean re-introducing an in-file topology concept that inGitDB deliberately does not have. Scoped out; see Open Questions.
- **Resolving `inherits` by collection ID.** A base is a partial and need not be a loadable collection, so it has no place in the resolved collection tree, and resolving by ID would depend on the tree that inheritance helps construct. Filesystem-path resolution avoids the cycle and matches the original design (a sibling file reference).
- **Per-field (deep) column merge.** Whole-column replacement is unambiguous and lets a child *remove* an inherited constraint; deep merge cannot express removal and adds order-dependent surprises. See `column-merge-child-wins`.
- **A deprecation window.** `inherits` never worked, so there is no working behaviour to deprecate. It stacks on the hard-break `column-validation` Feature.

## Open Questions

- Should inheritance eventually cover subcollection topology (the original `$admin_divisions` intent), and if so via an explicit in-file mechanism rather than filesystem discovery? Deferred until an author needs a shared subcollection shape.
- Should a base partial be lintable/verifiable on its own (e.g. a `specscore`-style check that a `$base.yaml` is a well-formed partial) independent of any child that inherits it? Out of scope here.

## Rehearse Integration

No Rehearse stubs are scaffolded: `specscore.yaml` in this repo declares no rehearse configuration, and every AC here is directly executable as a Go test that writes a fixture database to a temp directory and drives the real reader (`validator.ReadDefinition(dir, ingitdb.Validate())`) plus `datavalidator.NewValidator().Validate(...)`, which is where this project's coverage lives. Recorded as an explicit skip, not an omission.

## Dependent Modules

None. Adding `Inherits string` to `CollectionDef` is an additive struct change; no existing database in the workspace declares `inherits:` (the `geo-ingitdb` occurrences were stripped when it was rebuilt to the current layout), so nothing breaks. This Feature stacks on ingitdb-go#15 (`integration/column-validation-stack`) and closes ingitdb-go#7.

---
*This document follows the https://specscore.md/feature-specification*
