---
format: https://specscore.md/features-index-specification
---

# Features

Feature specifications for this project.

## Index

| Feature | Status | Description |
|---------|--------|-------------|
| [column-validation](column-validation/README.md) | Approved | Enforce the validation constraints a collection definition already declares, add the primitives it lacks (enum, list type, conditional-required), and fail loudly on config that is currently accepted and silently ignored. |
| [record-count-constraints](record-count-constraints/README.md) | Draft | Enforce the min_records_count and max_records_count bounds a collection definition may declare: reject an invalid bound at load, and fire a collection-level validation error when a collection holds too few or too many records. |

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/features-index-specification*
