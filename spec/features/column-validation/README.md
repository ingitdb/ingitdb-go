---
format: https://specscore.md/feature-specification
status: Approved
---

# Feature: Column validation primitives

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/column-validation?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/column-validation?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/column-validation?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/column-validation?op=request-change) |
**Status:** Approved
**Date:** 2026-07-15
**Owner:** alex
**Supersedes:** —
**Source Ideas:** —
**Grade:** B

## Summary

Enforce the validation constraints a collection definition already declares, add the primitives it lacks (enum, list type, conditional-required), and fail loudly on config that is currently accepted and silently ignored.

## Problem

A collection definition can declare validation constraints that inGitDB accepts, reports as valid, and then never enforces. This is worse than not offering the constraint at all: a missing feature is discoverable, but config that silently does nothing actively convinces the author their data is guarded when it is not.

Verified at commit `ee5aa30` of this repo (the `ingitdb` module; note `v1.31.x` tags belong to `ingitdb-cli`, not here):

- `min_length`, `max_length` and `length` exist on `ColumnDef` (`ingitdb/column_def.go:11-13`) and have **zero readers** anywhere in the repo, including tests. Declaring them changes nothing.
- `foreign_key` is read by `materializer/view_builder.go:407` (to build FK views) and `docsbuilder/collection_readme.go:124` (to render docs) — but by no validation code. `ForeignKeyIndex` (`datavalidator/interfaces.go:11`) is declared as "a pre-built read-only lookup for FK validation" and is its own only occurrence: no implementation, no caller. Referential integrity is never checked.
- Unknown keys in a column definition are accepted and discarded — `validator/def_validator.go:122,204` use bare `yaml.Unmarshal` without `KnownFields`. Adding `enum:`, `values:` or `one_of:` still reports *"Definition of collection is valid"*, so an author reasonably concludes the constraint is live.
- `validateRecordData` (`datavalidator/validator.go:281`) iterates `colDef.Columns`, never the record's own keys, so a record may carry arbitrary undeclared fields that are never examined.
- `valueMatchesColumnType`'s `default:` branch returns `true` (`datavalidator/validator.go:330`), so any column type the validator does not recognise matches every value.

Type checking works for recognised types, and the computed-column "must not be stored" rule works. What does not work is every *constraint* beyond unconditional `required`: a definition declaring `state` with `enum`, and `docs` with `min_length: 1`, validates a record with `state: emulatable` and `docs: []` as clean.

This is not hypothetical drift — authors are already writing constraints that do nothing. `geo-ingitdb/countries/.ingitdb-subcol.states.yaml:27-28` declares `min_value: 0` / `max_value: 99 000 000` on `population`, silently inert because no such field exists. `demo-ingitdb`'s `order_details/definition.yaml:21-22,26-27` carries literal `# TODO: min value constraint needs implementation in ColumnDef`. `geo-ingitdb/countries/.ingitdb-subcol.provinces.yaml:10` declares `counties: "[]string"`, a type the validator does not know and therefore accepts unconditionally.

Beyond enforcing what is declared, primitives are missing outright: no enum, no list column type (`knownColumnTypes`, `column_type.go:24-34`, has none), no value-range constraint, and `Required` is a plain `bool` with no way to condition it on a sibling.

## Behavior

### Enforcing declared constraints

#### REQ: length-constraints-enforced

`min_length`, `max_length` and `length` MUST be enforced during record validation. For a string value they constrain character count (Unicode code points, not bytes); for a list value they constrain element count; for a map value they constrain entry count. A value violating any of them MUST produce a validation error naming the field, the constraint, and the actual length. Declaring a length constraint on a column type where length is undefined (`bool`, `int`, `float`, the temporal types) MUST be rejected at definition-load time rather than silently ignored.

Because `Length`, `MinLength` and `MaxLength` are `int` with `omitempty` (`column_def.go:11-13`), a declared `0` is today indistinguishable from absent. These fields MUST become pointer-typed (`*int`) so "declared zero" and "not declared" are distinguishable — `min_length: 0` is meaningless but `max_length: 0` (forbid any content) is not, and load-time rejection of a length constraint on `bool` cannot otherwise detect `length: 0`.

#### REQ: value-range-constraints

A column MAY declare `min_value` and/or `max_value`, constraining a numeric value inclusively. A value outside the range MUST produce a validation error naming the field, the bound, and the actual value. Both MUST be rejected at definition-load time on a non-numeric column type, and when `min_value` exceeds `max_value`.

Both MUST be pointer-typed (`*float64`) for the same reason `length-constraints-enforced` requires `*int`: `min_value: 0` is not a hypothetical — it is exactly what `geo-ingitdb` declares on `population` and `area` (`states.yaml:27,32`). With a plain numeric and the natural `!= 0` guard, that declared zero reads as unset, the constraint silently vanishes, and the Feature's own flagship migration fixture keeps lying — the precise harm this Feature exists to end. `*float64` (rather than `*int`) is required so `min_value: 0.5` is expressible on a `float` column; on an `int` column a bound with a fractional part MUST be rejected at definition-load time.

This is the one constraint with demonstrated demand: `geo-ingitdb` already declares it (silently inert), and `demo-ingitdb` carries TODO comments requesting it. Implementing it also converts `geo-ingitdb`'s existing declaration from a lie into an enforced constraint, rather than breaking it under `reject-unknown-column-keys`.

#### REQ: foreign-key-enforced

When a column declares `foreign_key: <collection>`, record validation MUST verify the value exists as a key in that collection, via the already-declared `ForeignKeyIndex`. A value with no matching key MUST produce a validation error naming the field, the value, and the referenced collection. A `foreign_key` naming a collection absent from the definition MUST be rejected at definition-load time.

### New primitives

#### REQ: enum-membership

A column MAY declare `enum` as a non-empty list of permitted values. A record value outside that list MUST produce a validation error naming the field, the offending value, and the permitted values. `enum` MUST be rejected at definition-load time when empty, when it contains duplicates, or when any member is not assignable to the column's declared `type`.

#### REQ: list-column-type

The column type grammar MUST support lists, spelled `[]<element-type>` where `<element-type>` is exactly one of `string`, `int`, `float`, `bool`, `date`, `time`, `datetime`, or `any`. `map[...]` and nested list element types are out of scope. A value that is not a list, or whose elements do not all match `<element-type>`, MUST produce a validation error; `[]any` accepts any element but still requires the value to be a list. `min_length`/`max_length` on a list column constrain element count per `length-constraints-enforced`, which is what makes "at least one entry" expressible.

`[]string` is already declared in the wild (`geo-ingitdb/.../provinces.yaml:10`) and currently validates nothing, so this REQ retrofits enforcement onto existing data. That data MUST be verified against the new rule as part of this Feature.

#### REQ: conditional-required

A column MAY declare `required_when` as a **Starlark expression over sibling fields**, making the column required only when the expression evaluates to Starlark `True`. It MUST reuse the parsing and evaluation already implemented for `ColumnDef.Formula` (`ingitdb/column_formula.go`, `column_formula_eval.go`) — `go.starlark.net` is already a dependency. Introducing a second expression dialect is expressly rejected: one grammar, one evaluator, one set of edge cases.

**Identifier resolution MUST be specified, because `Formula`'s existing check is narrower than it appears.** `column_formula.go:45-50` rejects a reference to a *computed* column but lets an **undeclared** identifier pass silently (`sibling, exists := columns[ident.Name]; if exists && sibling.Formula != ""`), and `column_formula_eval.go:55-56,164-169` deliberately predeclares `starlark.Universe` plus `abs`/`round`/`floor`/`ceil` — so `len`, `True` and `abs` are all `*syntax.Ident` under `syntax.Walk`. A rule of "every identifier must be a declared column" would therefore reject `required_when: 'len(tags) > 0'`.

A free identifier MUST resolve as either a declared non-computed sibling column or a predeclared builtin; one that is neither MUST be rejected at definition-load time. A reference to a computed column MUST be rejected, as it already is.

**This MUST be enforced via Starlark's own resolver, NOT a hand-rolled walk over `*syntax.Ident` nodes.** `compileFormula` (`column_formula_eval.go:92`) currently passes `func(string) bool { return true }` as its is-predeclared predicate, which is why undeclared identifiers survive load today. Supplying a real predicate — stored siblings ∪ the evaluator's universe — makes the resolver reject `nosuchfield == 1` with `undefined: nosuchfield` at load, for both `Formula` and `required_when`, with no bespoke traversal.

A manual `syntax.Walk` over `*syntax.Ident` nodes is expressly forbidden, because it over-rejects: `Walk` visits `DotExpr.Name` and `ForClause.Vars` as well as free variables, so `name.startswith("x")` and `[c for c in counties]` would be rejected on `startswith` and `c` — neither a sibling nor a universe entry (string methods resolve dynamically via `Attr()`). That would break capabilities the evaluator documents as intentional: `column_formula_eval.go:52-54` advertises "native string methods", and the `maxFormulaSteps` ceiling (`column_formula_eval.go:13-19`) exists precisely because comprehensions are expected to be usable. The resolver distinguishes free from bound variables and ignores attribute names by construction; a walk cannot. (Cited by AST node name deliberately — line numbers in a third-party module shift between releases.)

The **computed-column check MUST also come from the resolver**, and the existing `syntax.Walk` (`column_formula.go:45-50`) MUST be deleted rather than patched. Because the predeclared set contains only *stored* siblings, a reference to a computed column is already undefined to the resolver: it reports `undefined: total`, which MUST be mapped to the computed-column error message when `columns[total]` is computed. This is strictly better than the walk, which reports a false positive on `[total for total in tags]` — a bound variable that merely shares a computed column's name. Deleting the walk also removes the `DotExpr.Name` false positive (`x.total`), the identical `ForClause.Vars`/`LambdaExpr.Params` cases, and any question of walk-vs-compile ordering.

#### REQ: computed-column-name-not-builtin

A **computed** column (one declaring `formula`) whose name collides with a predeclared builtin — any entry in `starlark.Universe`, plus the evaluator's own `abs`/`round`/`floor`/`ceil` (`column_formula_eval.go:164-169`) — MUST be rejected at definition-load time. `type`, `len`, `min`, `max`, `list`, `str`, `int`, `float`, `bool`, `range`, `hash`, `sorted`, `any` and `all` are all universe entries, and a computed column named `type` is entirely ordinary.

This rule is what keeps `conditional-required`'s resolver-only approach sound. That approach detects a computed-column reference by the resolver reporting `undefined: X` — but a name that lives in the universe is never undefined, so the check silently never fires. It cannot be fixed by tightening the is-predeclared predicate: `starlark.FileProgram` (`eval.go:401-402`) hardwires `resolve.File(f, isPredeclared, Universe.Has)`, and `resolve.go:132-134` states the `isUniversal` parameter exists "to avoid a cyclic dependency upon starlark.Universe, not because users should ever need to redefine it".

The failure it prevents is silent, which is why it is a load-time error rather than a lint: `EvaluateFormula` binds only *stored* fields (`column_formula_eval.go:57-63`), so a computed `type` is never bound, and `type == "x"` compares the Starlark builtin *function* to a string and yields `False` — a wrong value, never an error.

The restriction costs nothing real: such a column is unreferenceable anyway, since computed columns are never bound into `fields`, so every reference to it already resolves silently to the builtin instead. **Stored** columns need no such rule — they are predeclared via the stored half of the predicate, and `fields` shadows the builtin at evaluation **whenever the field is present in the record**. Forbidding a stored column named `type` would be absurd; it resolves correctly every time it is populated.

#### REQ: formula-cache-key-includes-predeclared-set

`compileFormula` (`column_formula_eval.go:83-98`) memoises compiled programs on **formula source alone** (`formulaProgramCache.Load(formula)`, `LoadOrStore(formula, prog)`), and `column_formula_eval.go:83-85` justifies that key explicitly: *"Every free identifier is treated as predeclared, making the compiled program independent of any particular record's fields and therefore safe to cache by source."*

`conditional-required` voids that invariant: once the predicate depends on the collection's column set, the compiled program is no longer independent of it, while the cache key still is. Only successes are cached (an error returns before `LoadOrStore`), so a collection that MUST fail can take a cache hit from a *different* collection that compiled the same formula text successfully — silently, and dependent on load order.

The memo key MUST therefore incorporate the resolved predeclared set (formula source plus a column-set fingerprint), or load-time resolution MUST bypass the memo entirely.

Under the **fingerprint** option, the comment at `column_formula_eval.go:83-85` MUST be updated in the same change, since its stated invariant no longer holds. Under the **bypass** option it still holds and MUST be left alone: `compileFormula` keeps `func(string) bool { return true }` for the evaluation path, so its program remains genuinely independent of any collection's column set and the source-only key stays sound, while strict per-collection resolution happens separately at load, uncached.

#### REQ: formula-load-time-resolution

Supplying the predicate is a deliberate behaviour change to `Formula`: an undeclared identifier becomes a load-time error where today it passes load and fails at evaluation. That is a bug fix rather than a regression — no database in this workspace declares any `formula:` — but it is a change to shipped behaviour and MUST be stated in the release notes.

An expression evaluating to anything other than Starlark `True`/`False` MUST produce a validation error rather than a silent truthiness coercion — `starlarkToGo` (`column_formula_eval.go:139-158`) can return `bool`, `string`, `int64` or `float64`, so `required_when: 'name'` is an error, not "required when name is non-empty".

When `required_when` is present, `required` MUST NOT also be set on the same column — declaring both is a definition-load error.

### Failing loudly instead of silently

#### REQ: reject-unknown-column-keys

An unrecognised key in a column definition MUST be rejected at definition-load time, naming the key and the column. Silently discarding unknown keys is what allows a plausible-looking `enum:` or `one_of:` to appear enforced while doing nothing. The implementation MUST use `decoder.KnownFields(true)`, already the house pattern at `config/root_config.go:331,393` and `validator/subscribers_validator.go:44`; `validator/def_validator.go` simply diverged from it.

#### REQ: reject-undeclared-record-fields

A record field with no corresponding column in the collection definition MUST produce a validation error naming the field. Validation MUST examine the record's keys, not only the schema's columns.

Synthetic keys injected by the library itself are exempt: `datavalidator/validator.go:254` sets `record["$ID"] = key` for INGR input and `parse.go:186` sets `row["$ID"] = id` for CSV. Any key prefixed `$` MUST be treated as library-reserved and skipped by this check, whether or not the definition declares it — otherwise every list/INGR collection fails validation on a field the library added. A definition MAY still declare a `$`-prefixed column (as `demo-ingitdb/.../order_details/definition.yaml:10` declares `"$ID"`), and doing so MUST NOT change the outcome.

#### REQ: reject-unknown-column-type

A column type that is not recognised MUST be rejected at definition-load time and MUST NOT fall through to matching every value. The validator MUST NOT treat an unrecognised type as permissive.

`number` is currently accepted as a map *key* type (`column_type.go:51`) but is not a column type, which is a live grammar inconsistency: `e2e-test-ingitdb/countries/.collection/definition.yaml:12,14` declares `type: number` and passes today only because unknown types match everything. This Feature does NOT add `number` as a column-type alias — `int` and `float` already cover it, and two spellings for one type is how the inconsistency started. The fixture is migrated instead, per `migrate-known-databases`.

### Migration

#### REQ: migrate-known-databases

This Feature is a deliberate breaking change with no deprecation window, so every database in this workspace MUST load and validate cleanly against the new rules before it ships. Two are known to break today and MUST be migrated as part of this Feature:

- `e2e-test-ingitdb/countries/.collection/definition.yaml:12,14` — `type: number` becomes `int` or `float` per the column's actual data.
- `geo-ingitdb` — `min_value`/`max_value` become enforced rather than unknown, so their declared values MUST be checked to be well-formed numbers. `max_value: 99 000 000` (`.ingitdb-subcol.states.yaml:28`) is not a YAML number and MUST be corrected.

`demo-ingitdb`, `demo-commerce-ingitdb` and any other database in the workspace MUST be verified against the new rules, and their TODO comments requesting value constraints resolved. Databases outside this workspace are out of scope and are addressed by the release notes, not by code.

## Acceptance Criteria

### AC: min-length-rejects-empty-list

**Requirements:** column-validation#req:length-constraints-enforced, column-validation#req:list-column-type

**Given** a collection with column `docs` of type `[]string` and `min_length: 1`
**When** a record is validated with `docs: []`
**Then** validation fails with an error naming `docs`, the `min_length` constraint, and the actual length `0`

### AC: max-length-rejects-long-string

**Requirements:** column-validation#req:length-constraints-enforced

**Given** a collection with column `title` of type `string` and `max_length: 5`
**When** a record is validated with `title: "far too long"`
**Then** validation fails with an error naming `title`, the `max_length` constraint, and the actual length

### AC: length-on-bool-rejected-at-load

**Requirements:** column-validation#req:length-constraints-enforced

**Given** a collection definition with column `active` of type `bool` and `min_length: 1`
**When** the definition is loaded
**Then** loading fails with an error stating that length constraints are undefined for type `bool`

### AC: foreign-key-rejects-dangling-reference

**Requirements:** column-validation#req:foreign-key-enforced

**Given** collections `statuses` (containing key `open`) and `tasks` whose column `status` declares `foreign_key: statuses`
**When** a `tasks` record is validated with `status: "nonexistent"`
**Then** validation fails with an error naming `status`, the value `nonexistent`, and the collection `statuses`

### AC: foreign-key-accepts-live-reference

**Requirements:** column-validation#req:foreign-key-enforced

**Given** the same two collections
**When** a `tasks` record is validated with `status: "open"`
**Then** validation passes

### AC: enum-rejects-non-member

**Requirements:** column-validation#req:enum-membership

**Given** a collection with column `state` of type `string` and `enum: [native, partial, absent, unknown]`
**When** a record is validated with `state: "emulatable"`
**Then** validation fails with an error naming `state`, the value `emulatable`, and the four permitted values

### AC: empty-enum-rejected-at-load

**Requirements:** column-validation#req:enum-membership

**Given** a collection definition with column `state` declaring `enum: []`
**When** the definition is loaded
**Then** loading fails with an error stating that `enum` must be non-empty

### AC: list-rejects-wrong-element-type

**Requirements:** column-validation#req:list-column-type

**Given** a collection with column `tags` of type `[]string`
**When** a record is validated with `tags: ["ok", 42]`
**Then** validation fails with an error naming `tags` and the non-string element

### AC: list-rejects-non-list

**Requirements:** column-validation#req:list-column-type

**Given** a collection with column `docs` of type `[]string`
**When** a record is validated with `docs: "not-a-list"`
**Then** validation fails with an error naming `docs` and the expected list type

### AC: conditional-required-fires-when-condition-holds

**Requirements:** column-validation#req:conditional-required

**Given** a collection where column `name` declares `required_when: 'state != "absent"'`
**When** a record is validated with `state: "native"` and no `name`
**Then** validation fails naming `name` as required

### AC: conditional-required-silent-when-condition-fails

**Requirements:** column-validation#req:conditional-required

**Given** the same collection
**When** a record is validated with `state: "absent"` and no `name`
**Then** validation passes

### AC: required-when-unknown-sibling-rejected-at-load

**Requirements:** column-validation#req:conditional-required

**Given** a collection definition where column `name` declares `required_when: 'nosuchfield == 1'`
**When** the definition is loaded
**Then** loading fails naming the undeclared sibling `nosuchfield`

### AC: min-value-rejects-below-bound

**Requirements:** column-validation#req:value-range-constraints

**Given** a collection with column `population` of type `int` and `min_value: 0`
**When** a record is validated with `population: -5`
**Then** validation fails with an error naming `population`, the `min_value` bound, and the value `-5` — the declared zero bound is enforced, not read as unset

### AC: max-value-rejects-above-bound

**Requirements:** column-validation#req:value-range-constraints

**Given** a collection with column `percent` of type `int` and `max_value: 100`
**When** a record is validated with `percent: 101`
**Then** validation fails naming `percent`, the `max_value` bound, and the value `101`

### AC: inverted-range-rejected-at-load

**Requirements:** column-validation#req:value-range-constraints

**Given** a collection definition with column `n` of type `int`, `min_value: 10` and `max_value: 5`
**When** the definition is loaded
**Then** loading fails stating that `min_value` exceeds `max_value`

### AC: required-when-builtin-identifier-allowed

**Requirements:** column-validation#req:conditional-required

**Given** a collection with column `tags` of type `[]string` and column `summary` declaring `required_when: 'len(tags) > 0'`
**When** the definition is loaded
**Then** loading succeeds — `len` resolves as a predeclared builtin, not as an undeclared sibling

### AC: required-when-method-call-and-comprehension-allowed

**Requirements:** column-validation#req:conditional-required

**Given** a collection with `string` column `name`, `[]string` column `counties`, and a column declaring `required_when: 'name.startswith("x") or [c for c in counties if c]'`
**When** the definition is loaded
**Then** loading succeeds — the method name `startswith` and the comprehension variable `c` are neither siblings nor builtins, and MUST NOT be mistaken for undeclared identifiers

### AC: formula-undeclared-identifier-rejected-at-load

**Requirements:** column-validation#req:formula-load-time-resolution, column-validation#req:conditional-required

**Given** a collection where column `total` declares `formula: 'nosuchfield * 2'`
**When** the definition is loaded
**Then** loading fails with `undefined: nosuchfield` — the shared predeclared predicate applies to `formula`, not only to `required_when`, where today it would pass load and fail only at evaluation

### AC: required-when-non-boolean-rejected

**Requirements:** column-validation#req:conditional-required

**Given** a collection where column `note` declares `required_when: 'name'` and `name` is a `string` column
**When** a record is validated with `name: "anything"`
**Then** validation fails stating the expression evaluated to a non-boolean — it is not coerced to truthy

### AC: required-when-computed-column-rejected-at-load

**Requirements:** column-validation#req:conditional-required

**Given** a collection where column `total` declares a `formula` and column `note` declares `required_when: 'total > 0'`
**When** the definition is loaded
**Then** loading fails stating that `required_when` may not reference a computed column

### AC: bound-variable-shadowing-computed-column-allowed

**Requirements:** column-validation#req:conditional-required

**Given** a collection with a computed column `total` and a `[]string` column `tags`, where another column declares `required_when: '[total for total in tags]'`
**When** the definition is loaded
**Then** loading succeeds — `total` here is a comprehension-bound variable, not a reference to the computed column

### AC: computed-column-named-after-builtin-rejected-at-load

**Requirements:** column-validation#req:computed-column-name-not-builtin, column-validation#req:conditional-required

**Given** a collection declaring a computed column named `type` (a `starlark.Universe` entry) with any `formula`
**When** the definition is loaded
**Then** loading fails naming `type` as colliding with a predeclared builtin — without this, `required_when: 'type == "x"'` would resolve `type` to the builtin function, compare it to a string, and silently yield `False` instead of being rejected

### AC: stored-column-named-after-builtin-allowed

**Requirements:** column-validation#req:computed-column-name-not-builtin

**Given** a collection declaring a **stored** (non-computed) column named `type`, and another column declaring `required_when: 'type == "x"'`
**When** the definition is loaded and a record is validated
**Then** loading succeeds and `type` resolves to the record's stored field, not the builtin — the restriction applies only to computed columns

### AC: formula-cache-does-not-leak-across-collections

**Requirements:** column-validation#req:formula-cache-key-includes-predeclared-set, column-validation#req:conditional-required

**Given** collection A declaring a stored column `population` with a computed column using `formula: 'population * 2'`, and collection B declaring no `population` column but the identical formula text
**When** collection A is loaded first (compiling and caching that formula successfully), then collection B is loaded
**Then** collection B still fails with `undefined: population` — the memoised program from A MUST NOT satisfy B, and the outcome MUST NOT depend on load order

### AC: list-any-still-requires-a-list

**Requirements:** column-validation#req:list-column-type

**Given** a collection with column `misc` of type `[]any`
**When** a record is validated with `misc: "not-a-list"`
**Then** validation fails — `[]any` accepts any element but still requires the value to be a list

### AC: value-range-on-string-rejected-at-load

**Requirements:** column-validation#req:value-range-constraints

**Given** a collection definition with column `title` of type `string` and `min_value: 0`
**When** the definition is loaded
**Then** loading fails stating that value-range constraints require a numeric column type

### AC: exact-length-enforced

**Requirements:** column-validation#req:length-constraints-enforced

**Given** a collection with column `code` of type `string` and `length: 3`
**When** a record is validated with `code: "ab"`
**Then** validation fails naming `code`, the `length` constraint, and the actual length `2`

### AC: declared-zero-max-length-distinguishable-from-absent

**Requirements:** column-validation#req:length-constraints-enforced

**Given** a collection with column `note` of type `string` and `max_length: 0`
**When** a record is validated with `note: "x"`
**Then** validation fails — the declared zero is enforced rather than read as unset

### AC: foreign-key-to-absent-collection-rejected-at-load

**Requirements:** column-validation#req:foreign-key-enforced

**Given** a collection definition whose column `status` declares `foreign_key: nosuchcollection`
**When** the definition is loaded
**Then** loading fails naming the missing collection `nosuchcollection`

### AC: duplicate-enum-members-rejected-at-load

**Requirements:** column-validation#req:enum-membership

**Given** a collection definition with column `state` declaring `enum: [native, native, absent]`
**When** the definition is loaded
**Then** loading fails naming the duplicated member `native`

### AC: enum-member-type-mismatch-rejected-at-load

**Requirements:** column-validation#req:enum-membership

**Given** a collection definition with column `count` of type `int` declaring `enum: [1, "two"]`
**When** the definition is loaded
**Then** loading fails naming the member `"two"` as not assignable to type `int`

### AC: synthetic-id-field-not-rejected

**Requirements:** column-validation#req:reject-undeclared-record-fields

**Given** an INGR or CSV collection that does not declare a `$ID` column
**When** a record is validated after the library injects `$ID`
**Then** validation passes — the injected `$` -prefixed key is exempt, not reported as undeclared

### AC: known-databases-validate-clean

**Requirements:** column-validation#req:migrate-known-databases, column-validation#req:reject-unknown-column-type, column-validation#req:value-range-constraints

**Given** the migrated `e2e-test-ingitdb` and `geo-ingitdb` databases
**When** each is loaded and validated with the new rules
**Then** both report zero violations, and `e2e-test-ingitdb` declares no `type: number`

### AC: required-and-required-when-together-rejected

**Requirements:** column-validation#req:conditional-required

**Given** a collection definition where column `name` declares both `required: true` and a `required_when` condition
**When** the definition is loaded
**Then** loading fails with an error stating the two are mutually exclusive

### AC: unknown-column-key-rejected

**Requirements:** column-validation#req:reject-unknown-column-keys

**Given** a collection definition with column `state` declaring `one_of: [a, b]`
**When** the definition is loaded
**Then** loading fails with an error naming the unrecognised key `one_of` on column `state`

### AC: undeclared-record-field-rejected

**Requirements:** column-validation#req:reject-undeclared-record-fields

**Given** a collection declaring only column `state`
**When** a record is validated carrying `state: "native"` and `junk: "value"`
**Then** validation fails with an error naming the undeclared field `junk`

### AC: unknown-column-type-rejected-at-load

**Requirements:** column-validation#req:reject-unknown-column-type

**Given** a collection definition with column `state` of type `strng`
**When** the definition is loaded
**Then** loading fails with an error naming the unrecognised type `strng` and listing the recognised types

## Not Doing (and Why)

- **A deprecation window for the strictness rules** — the owner chose a hard break with the two known-broken fixtures migrated in the same change (see `migrate-known-databases`). A warn-then-error cycle was considered and rejected as ceremony for a pre-1.0 module whose only known consumers are in this workspace. Databases outside it are covered by release notes.
- **Splitting into three Features** (enforce-declared / new-primitives / fail-loudly) — the reviewer's decomposition is sound in principle, but the hard-break decision couples them: strictness lands with the fixture migration, and `value-range-constraints` must land with `reject-unknown-column-keys` or `geo-ingitdb` breaks instead of being fixed. Kept as one Feature by owner decision.
- **`number` as a column-type alias** — `int` and `float` already cover it; two spellings for one type is what produced the grammar inconsistency in the first place.
- **Nested and `map[...]` list element types** — only the closed element set enumerated in `list-column-type`; nested lists have no demonstrated demand.

## Rehearse Integration

No Rehearse stubs are scaffolded: `specscore.yaml` in this repo declares no rehearse configuration, and every AC here is directly executable as a Go table test against `datavalidator` and the definition loader, which is where this project's existing coverage lives. Recorded as an explicit skip rather than an omission.

## Dependent Modules

Making `Length`/`MinLength`/`MaxLength` pointer-typed is a Go API break. One known downstream compile site exists in this workspace: `ingitdb-cli/cmd/ingitdb/tui/lazy_eval_test.go:213` (`Length: 12`). `migrate-known-databases` covers databases only; this module change MUST be landed together with an `ingitdb-cli` update.

## Open Questions

- How should `enum` interact with a `[]string` column — membership per element, or over the whole list? `geo-ingitdb`'s `counties: "[]string"` is exactly this shape, so the question is live rather than theoretical. Per-element is the intuitive reading; unresolved until an author asks for it.
- How should length and value-range constraints behave on a `type: any` column, where the runtime value could be a string, a list, or a number? Load-time rejection is undecidable, so the choice is runtime-check-if-applicable or forbid the combination.
- `column_type.go:45-47` has a latent slice-bounds panic: for `ct == "map["`, `strings.Index` returns `-1`, yielding `ct[4:3]`. `reject-unknown-column-type` puts an implementer directly in this function; fix in scope or file separately?
- A **stored** column named after a builtin behaves asymmetrically when *omitted* from a record: `EvaluateFormula` leaves it unbound, so `type` falls through to the Starlark builtin and the expression silently yields a wrong value. Pre-existing `EvaluateFormula` behaviour, untouched by this Feature and not worth forbidding a legitimate column name over — but the silent failure is the same family as the `column_type.go:47` panic above. Worth a fix?

  **Do not "fix" this by making an omitted stored column error.** An omitted *non-builtin* optional column currently binds to `None` and evaluates cleanly, and a downstream contract already depends on that: `bots-go-framework/can-i-use`'s `capability-record` conditions `required_when` on `equivalenceClass != None`, where `equivalenceClass` is optional and frequently absent. Verified against the real evaluator: the absent column arrives as `None` and the expression resolves. Making absence an error would break that mechanism. The asymmetry to close, if any, is the *builtin-named* case falling through to the builtin rather than to `None` — not the `None` binding itself.
- `*float64` cannot exactly represent integer bounds above 2^53. `geo-ingitdb`'s bounds (`0`, `99000000`) are orders of magnitude below that, so it does not block — but an `int` column with a bound near `math.MaxInt64` would round. Accept the limit, or carry the bound as a decimal/`json.Number`?
- `foreign-key-enforced` moves an existing check's timing — `view_builder.go:413` already reports a missing FK collection at materialize time. Should the load-time check replace it, or both fire?

---
*This document follows the https://specscore.md/feature-specification*
