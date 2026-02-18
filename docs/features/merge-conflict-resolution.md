# Git Merge Conflict Resolution

When a `git merge` or `git rebase` produces conflicts inside an inGitDB database, `ingitdb` can resolve them automatically or interactively depending on the file type.

## Two resolution modes

### Generated files — automatic resolution

Materialized view files (`$views/**`) and collection `README.md` files are fully generated from source data. When they conflict, the correct resolution is always to regenerate them. `ingitdb` is registered as a [git merge driver](https://git-scm.com/docs/gitattributes#_defining_a_custom_merge_driver) for these files and silently regenerates the output, producing no conflict markers.

### Source data files — interactive resolution

Record files in `$records/` contain hand-authored data. When they conflict, `ingitdb` opens a TUI showing the conflicting fields side-by-side in a data table format. The user resolves each field conflict by choosing the incoming, current, or a manually edited value.

## Setup

Register `ingitdb` as a merge driver for the database:

```shell
ingitdb setup [--path=PATH]
```

This writes the necessary merge driver entries to `.gitattributes`:

```
$views/**   merge=ingitdb-generated
README.md   merge=ingitdb-generated
$records/*  merge=ingitdb-data
```

And registers the drivers in the local git config.

## Usage

After `git merge` or `git rebase` leaves conflicts:

```shell
ingitdb resolve [--path=PATH]
```

- Generated file conflicts are resolved silently.
- For each conflicted data file a TUI session opens, one file at a time.

Or resolve a single file:

```shell
ingitdb resolve --file=<path>
```
