# ingitdb Command Line Interface

`--path` defaults to the current working directory when omitted. `~` in paths is expanded to the user's home directory.

## Commands

```
ingitdb validate [--path=PATH] [--from-commit=SHA] [--to-commit=SHA]
```
Validates schema and data. With `--from-commit` / `--to-commit`, validates only files changed between the two commits (see [Validator docs](components/validator/README.md)).

```
ingitdb query --collection=<key> [--path=PATH] [--format=CSV|JSON|YAML]
```
Returns records from a collection. Default format is JSON.

```
ingitdb materialize [--path=PATH] [--views=<view1,view2>]
```
Builds materialized views into `$views/`. Without `--views`, materializes all views.

## Global flags

```
ingitdb --version
```
Prints build version, commit hash, and build date.
