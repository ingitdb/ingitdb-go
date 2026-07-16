---
format: https://specscore.md/features-index-specification
---

# Features

Feature specifications for this project.

## Index

| Feature | Status | Description |
|---------|--------|-------------|
| [column-validation](column-validation/README.md) | Approved | Enforce the validation constraints a collection definition already declares, add the primitives it lacks (enum, list type, conditional-required), and fail loudly on config that is currently accepted and silently ignored. |
| [record-count-constraints](record-count-constraints/README.md) | Approved | Enforce the min_records_count and max_records_count bounds a collection definition may declare: reject an invalid bound at load, and fire a collection-level validation error when a collection holds too few or too many records. |
| [definition-inheritance](definition-inheritance/README.md) | Approved | A collection or subcollection definition may declare inherits: to overlay configuration from a base partial definition, so related definitions do not repeat shared columns and settings. |
| [subcollection-record-validation](subcollection-record-validation/README.md) | Approved | Recurse whole-database validation into subcollection records and pin down the on-disk storage convention for nested subcollection data. |
| [record-file-name-placeholders](record-file-name-placeholders/README.md) | Approved | Fix {fieldName} placeholder substitution and reject path separators in substituted record-file name segments (closes ingitdb-go#1, #2). |
| [materialized-view-md-extension](materialized-view-md-extension/README.md) | Approved | A template-less named view whose formats include md materializes a .md file (not .ingr), so parameterized views render browsable Markdown (closes ingitdb-go#3). |

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/features-index-specification*
