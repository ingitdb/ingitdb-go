---
format: https://specscore.md/feature-specification
status: Approved
---

# Feature: Materialized view Markdown extension

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/materialized-view-md-extension?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/materialized-view-md-extension?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/materialized-view-md-extension?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/materialized-view-md-extension?op=request-change) |
**Status:** Approved
**Date:** 2026-07-17
**Owner:** alex
**Supersedes:** —
**Source Ideas:** —

## Summary

A template-less named view whose `formats` list contains `md` renders a Markdown table (via the built-in renderer added for ingitdb-go#4/#6), but its output file is given a `.ingr` extension because the extension is derived from the unrelated `format` (singular, INGR-defaulting) field. The result is a `.ingr` file full of Markdown — which GitHub will not render as a table, defeating the whole point of the feature. This Feature makes the output extension follow the actual rendered format: when the built-in Markdown renderer will run, the file gets a `.md` extension. It also adds a regression test that a database shipping a view definition materializes at least one file, which the demo would have caught. Closes ingitdb-go#3 (the remaining gap after #4/#6).

## Problem

`materialize --views` was reported as a silent no-op (ingitdb-go#3): it rendered nothing even on `demo-ingitdb`, which ships `modules/todo/tasks/.collection/views/status_{status}.yaml`. The underlying causes — a hard-required `template` (ingitdb-go#4) and parameterized views that never partitioned (ingitdb-go#6) — were fixed in a parallel effort (commit `6659c39`, "add built-in md renderer and parameterized view partitioning"). Verifying against the real reader confirms views now materialize: `demo-ingitdb`'s `status_{status}` view produces one partition (`status=new`, the sole task record).

A gap remains. The demo view declares `formats: [md]` and no `format:`, and the built-in renderer (`renderBuiltinView`, `materializer/view_writer.go`) correctly emits a Markdown pipe table because `formats` contains `md`. But `resolveViewOutputPath` (`materializer/view_builder.go`) computes the file extension from `view.Format` (the singular data-export field, default `ingr`) — it never consults `view.Formats`. So the demo view materializes to `$ingitdb/modules/todo/tasks/status_new.ingr` containing a Markdown table. The extension and the content disagree.

This matters because the pitch of parameterized views is browsable, diffable Markdown on GitHub with no build step (`ingitdb-specs/docs/features/materialized-views.md`). A `.ingr` file is not rendered by GitHub as a table; it reads as raw text. For the named-view path there is no data-export renderer at all — `renderBuiltinView` produces Markdown when `formats` contains `md` and otherwise errors ("view template is required") — so `view.Format` has no bearing on what is actually written. The extension must reflect what the renderer produces, which for a template-less named view is Markdown.

The output *location* is already correct: named views materialize under `$ingitdb/` mirroring the collection tree, which matches the current spec (`materialized-views.md`: "There is no `$views/` subfolder inside individual collection directories"). Only the extension is wrong.

## Behavior

### REQ: md-formats-yield-md-extension

`resolveViewOutputPath` MUST give a template-less named view a `.md` file extension when the view has no explicit `file_name` and its `formats` list contains `md` (case-insensitive). This is the format the built-in renderer produces for such a view, so the extension MUST match it.

### REQ: non-md-extension-unchanged

When a template-less named view has no explicit `file_name` and its `formats` list does not contain `md`, the output extension MUST remain the data-export extension derived from `view.Format` (`ingr` by default), unchanged from prior behaviour. A view with an explicit `file_name`, and a template-rendered or default/export view, MUST keep their existing output paths.

### REQ: view-definition-materializes-output

A collection that ships a valid named view definition with `formats: [md]` MUST, when materialized through the standard view builder, produce at least one output file, and that file MUST carry the `.md` extension and contain the rendered Markdown table.

## Acceptance Criteria

### AC: md-formats-view-gets-md-extension

**Requirements:** materialized-view-md-extension#req:md-formats-yield-md-extension

**Given** a template-less named view with `formats: ["md"]`, no `file_name`, and no `format`
**When** `resolveViewOutputPath` resolves its output path
**Then** the path ends in `.md`

### AC: non-md-view-keeps-ingr-extension

**Requirements:** materialized-view-md-extension#req:non-md-extension-unchanged

**Given** a template-less named view with no `formats`, no `file_name`, and no `format`
**When** `resolveViewOutputPath` resolves its output path
**Then** the path ends in `.ingr`, unchanged from prior behaviour

### AC: parameterized-md-view-materializes-md-files

**Requirements:** materialized-view-md-extension#req:md-formats-yield-md-extension, materialized-view-md-extension#req:view-definition-materializes-output

**Given** a collection with a parameterized view `status_{status}.yaml` declaring `formats: [md]` and records spanning two `status` values, loaded via the real reader
**When** the standard view builder materializes its views
**Then** it writes one `.md` file per distinct `status` value, each containing a Markdown table, and reports no errors

### AC: view-db-materializes-at-least-one-file

**Requirements:** materialized-view-md-extension#req:view-definition-materializes-output

**Given** a database whose collection ships a single `formats: [md]` view definition and one matching record, read with `validator.ReadDefinition(db, ingitdb.Validate())`
**When** the standard view builder materializes that collection's views
**Then** at least one file is created and no error is reported — the regression the demo would have caught

## Not Doing (and Why)

- **Multi-format generation** — `formats` is a `[]string` and the schema hints a view could emit several formats at once. The built-in renderer only supports `md`; generating one file per listed format is a larger feature. This change aligns the single supported case (Markdown) and leaves multi-format for later without contradicting this contract.
- **Changing the output directory** — the `$ingitdb/`-mirrored location is already spec-correct; only the extension was wrong. No relocation.
- **The CLI `list views` "Not yet implemented" and the silent `0 created` summary** — those live in `ingitdb-cli`, out of scope for this library repo. They are the CLI-side half of ingitdb-go#3 and are noted for a follow-up in that repo; this Feature makes the library actually produce correct `.md` output so that once the CLI surfaces results honestly, there is real output to show.

## Rehearse Integration

No Rehearse stubs are scaffolded: `specscore.yaml` in this repo declares no rehearse configuration. Every AC is executable as a Go test — a unit test against `resolveViewOutputPath`, and an end-to-end test that builds a temp-dir fixture, reads it with the real `validator.ReadDefinition`, runs `SimpleViewBuilder.BuildViews`, and asserts the written `.md` files. Recorded as an explicit skip rather than an omission.

## Dependent Modules

Builds on the built-in Markdown renderer and parameterized-view partitioning delivered for ingitdb-go#4/#6 (commit `6659c39`). The change is confined to `resolveViewOutputPath`; existing tests that assert `.ingr` for views without `formats: [md]` are preserved by REQ non-md-extension-unchanged.

## Open Questions

- When multi-format generation lands, should a view with `formats: [md, csv]` emit both `status_new.md` and `status_new.csv`? Deferred with the multi-format work above.

---
*This document follows the https://specscore.md/feature-specification*
