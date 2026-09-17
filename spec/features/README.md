---
format: https://specscore.md/features-index-specification
---

# Features

Feature specifications for this project.

## Index

| Feature | Status | Description |
|---------|--------|-------------|
| [Column validation primitives](column-validation/README.md) | Approved | Enforce the validation constraints a collection definition already declares, add the primitives it lacks (enum, list type, conditional-required), and fail loudly on config that is currently accepted and silently ignored. |
| [Record-count constraints](record-count-constraints/README.md) | Approved | Enforce the `min_records_count` and `max_records_count` bounds a collection definition may declare: reject an invalid bound at definition-load time, and fire a collection-level validation error when a collection holds fewer than `min_records_count` or more than `max_records_count` records. Closes ingitdb-go#8. |
| [Definition inheritance](definition-inheritance/README.md) | Approved | A collection or subcollection definition may declare `inherits: <path>` to overlay configuration from a base *partial* definition, so a family of related definitions can share columns and settings instead of repeating them. The base is loaded, merged under the child (child wins), and only the merged result is validated. A missing base or an inheritance cycle is a load-time error. |
| [Subcollection record validation](subcollection-record-validation/README.md) | Approved | Make whole-database validation recurse into subcollection *records*. Today `datavalidator.simpleValidator.Validate` iterates only `def.Collections` — the root collections — for its per-record schema pass, so a record stored in a subcollection is never checked against its declared schema. Every constraint an author writes on a subcollection column (type, `enum`, `required`, `required_when`, length, value-range, `foreign_key`, `min_records_count`/`max_records_count`) is silently inert. This Feature walks each collection's `SubCollections` recursively, validates their records with the same per-record checks the root pass uses, pins down and documents the on-disk storage convention for nested subcollection data, and reconciles the change with the foreign-key pass (which already descends into subcollections but, given the same undefined data path, never actually reached their records). |
| [Record-file name placeholder substitution and path-separator safety](record-file-name-placeholders/README.md) | Approved | `RecordFileDef.GetRecordFileName` computes a record's on-disk file name from the `record_file.name` pattern by substituting `{key}` and `{fieldName}` placeholders. Two defects make it produce data-losing or wrong file names: the `{fieldName}` substitution loop is guarded by an inverted condition and never runs, and a substituted value containing a path separator is written literally into the name, nesting the record file where the reader can never glob it back. This Feature makes `{fieldName}` substitution work and makes the function reject any substituted value that contains a path separator, turning silent data loss into a loud, actionable error. Closes ingitdb-go#1 and ingitdb-go#2. |
| [Materialized view Markdown extension](materialized-view-md-extension/README.md) | Approved | A template-less named view whose `formats` list contains `md` renders a Markdown table (via the built-in renderer added for ingitdb-go#4/#6), but its output file is given a `.ingr` extension because the extension is derived from the unrelated `format` (singular, INGR-defaulting) field. The result is a `.ingr` file full of Markdown — which GitHub will not render as a table, defeating the whole point of the feature. This Feature makes the output extension follow the actual rendered format: when the built-in Markdown renderer will run, the file gets a `.md` extension. It also adds a regression test that a database shipping a view definition materializes at least one file, which the demo would have caught. Closes ingitdb-go#3 (the remaining gap after #4/#6). |
| [Explicit record base directory](explicit-record-base-directory/README.md) | Draft | Allow a collection to set `record_file.records_dir` explicitly, including `.` to place keyed records directly under `data_dir`, while preserving the existing implicit `$records` directory when the field is omitted. |
| [Record companion body files](record-body-file/README.md) | Draft | A single-record collection may declare `record_file.body_files`: one or more string fields, each stored as a raw-text file beside the record file. The record file holds an explicit reference in place of the value, `{"$file": "<name>"}`. It holds `null` for a null value, and omits the key for an absent field. |

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/features-index-specification*
