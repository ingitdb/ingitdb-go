# inGitDB Validator

Validates inGitDB schema and data.

Consists of two subcomponents:

- [Schema validator](schema-validator.md)
- [Data validator](data-validator.md)

There are two validation modes:

- [Full validation](full-validation.md) — performs a full DB scan and validates everything.
- [Change validation](change-validation.md) — validates only the difference between two commits.