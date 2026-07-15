---
format: https://specscore.md/feature-specification
status: Draft
---

# Feature: Column validation primitives

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/column-validation?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/column-validation?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/column-validation?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/column-validation?op=request-change) |
**Status:** Draft
**Source Ideas:** —

## Summary

Enforce the validation constraints a collection definition already declares, add the primitives it lacks (enum, list type, conditional-required), and fail loudly on config that is currently accepted and silently ignored.

## Problem

A collection definition can declare validation constraints that inGitDB accepts, reports as valid, and then never enforces. This is worse than not offering the constraint at all: a missing feature is discoverable, but config that silently does nothing actively convinces the author their data is guarded when it is not.

Verified against v1.31.12:

- `min_length`, `max_length` and `length` exist on `ColumnDef` (`ingitdb/column_def.go`) and have **zero readers** anywhere outside their own declaration and tests. Declaring them changes nothing.
- `foreign_key` is read only by `materializer/view_builder.go` to build FK views. `ForeignKeyIndex` (`datavalidator/interfaces.go`) is declared as "a pre-built read-only lookup for FK validation" and is referenced by no other code. Referential integrity is never checked.
- Unknown keys in a column definition are accepted and discarded — adding `enum:`, `values:` or `one_of:` still reports *"Definition of collection is valid"*, so an author reasonably concludes the constraint is live.
- `validateRecordData` (`datavalidator/validator.go:273`) iterates `colDef.Columns`, never the record's own keys, so a record may carry arbitrary undeclared fields that are never examined.
- `valueMatchesColumnType`'s `default:` branch returns `true`, so any column type the validator does not recognise matches every value.

The practical result: a definition declaring `state` with `enum`, and `docs` with `min_length: 1`, validates a record with `state: emulatable` and `docs: []` as clean. Unconditional `required` is currently the only column constraint that genuinely works.

Beyond enforcing what is declared, three primitives are missing outright: there is no enum, no list column type at all (`knownColumnTypes` has none), and `Required` is a plain `bool` with no way to condition it on another field.

## Behavior

### Enforcing declared constraints

#### REQ: length-constraints-enforced

`min_length`, `max_length` and `length` MUST be enforced during record validation. For a string value they constrain character count; for a list value they constrain element count; for a map value they constrain entry count. A value violating any of them MUST produce a validation error naming the field, the constraint, and the actual length. Declaring a length constraint on a column type where length is undefined (e.g. `bool`) MUST be rejected at definition-load time rather than silently ignored.

#### REQ: foreign-key-enforced

When a column declares `foreign_key: <collection>`, record validation MUST verify the value exists as a key in that collection, via the already-declared `ForeignKeyIndex`. A value with no matching key MUST produce a validation error naming the field, the value, and the referenced collection. A `foreign_key` naming a collection absent from the definition MUST be rejected at definition-load time.

### New primitives

#### REQ: enum-membership

A column MAY declare `enum` as a non-empty list of permitted values. A record value outside that list MUST produce a validation error naming the field, the offending value, and the permitted values. `enum` MUST be rejected at definition-load time when empty, when it contains duplicates, or when any member is not assignable to the column's declared `type`.

#### REQ: list-column-type

The column type grammar MUST support lists, spelled `[]<element-type>` where `<element-type>` is any existing scalar type (e.g. `[]string`). A value that is not a list, or whose elements do not all match `<element-type>`, MUST produce a validation error. `min_length`/`max_length` on a list column constrain element count per `length-constraints-enforced`, which is what makes "at least one entry" expressible.

#### REQ: conditional-required

A column MAY declare `required_when` as a single condition over sibling fields, making the column required only when the condition holds. When `required_when` is present, `required` MUST NOT also be set on the same column — declaring both is a definition-load error, since they would contradict each other. A `required_when` referencing an undeclared sibling MUST be rejected at definition-load time.

### Failing loudly instead of silently

#### REQ: reject-unknown-column-keys

An unrecognised key in a column definition MUST be rejected at definition-load time, naming the key and the column. Silently discarding unknown keys is what allows a plausible-looking `enum:` or `one_of:` to appear enforced while doing nothing.

#### REQ: reject-undeclared-record-fields

A record field with no corresponding column in the collection definition MUST produce a validation error naming the field. Validation MUST examine the record's keys, not only the schema's columns.

#### REQ: reject-unknown-column-type

A column type that is not recognised MUST be rejected at definition-load time and MUST NOT fall through to matching every value. The validator MUST NOT treat an unrecognised type as permissive.

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

**Given** a collection where column `name` declares `required_when` a condition on sibling `state` not being `absent`
**When** a record is validated with `state: "native"` and no `name`
**Then** validation fails naming `name` as required

### AC: conditional-required-silent-when-condition-fails

**Requirements:** column-validation#req:conditional-required

**Given** the same collection
**When** a record is validated with `state: "absent"` and no `name`
**Then** validation passes

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
**Then** loading fails naming the unrecognised type, and no record validates against it

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/feature-specification*
