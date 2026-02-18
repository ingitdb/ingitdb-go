# Merge Conflict Resolver

Resolves git merge conflicts inside an inGitDB database. Operates in two distinct modes depending on whether the conflicted file is generated or hand-authored.

## Git merge driver integration

`ingitdb setup` writes two merge driver definitions to `.gitattributes` and the local `.git/config`:

| Driver name | Applies to | Mode |
|---|---|---|
| `ingitdb-generated` | `$views/**`, `README.md` | Automatic — regenerate from source |
| `ingitdb-data` | `$records/*` | Interactive — TUI field-by-field |

Git calls the driver as:
```
ingitdb merge-driver --type=<generated|data> %O %A %B %P
```
where `%O` = base, `%A` = current (ours), `%B` = incoming (theirs), `%P` = result path.

## Generated file resolver

Ignores all three versions entirely. Reads the current source records from disk (already at their merged state or with conflicts in `$records/`), calls the Views Builder to regenerate the output, and writes the result to the result path. Exits 0 so git marks the conflict resolved.

**Dependency:** Views Builder (Phase 1).

## Data file resolver

Parses all three versions of the record file (base, ours, theirs) as JSON or YAML. For each field:

- If ours == theirs → take either (no conflict)
- If base == ours → take theirs (clean incoming change)
- If base == theirs → take ours (clean local change)
- If all three differ → present to user in the TUI

The TUI displays conflicting fields in a table:

```
Collection: todo.tasks  |  File: $records/2025-12-20T23-34-56.json
────────────────────────────────────────────────────────────────────
  Field     │ Current (ours)   │ Incoming (theirs)
────────────────────────────────────────────────────────────────────
  status    │ in_progress      │ done
  title     │ Buy milk         │ Buy oat milk
────────────────────────────────────────────────────────────────────
[↑↓] navigate  [c] keep current  [i] keep incoming  [e] edit  [s] save
```

After the user resolves all fields, the merged record is written back to the result path. Exits 0 on save, 1 if the user aborts.

## `ingitdb resolve` command

Scans the database root for all files with conflict markers (using `git status --porcelain`). Delegates each to the appropriate resolver. Generated files are processed first (silently), then data files are presented one at a time in the TUI.

## Package location

Implement in `pkg/ingitdb/resolver/`. Two sub-packages:
- `pkg/ingitdb/resolver/generated/` — generated file resolver
- `pkg/ingitdb/resolver/data/` — data file resolver and TUI

The TUI must write to the terminal directly (not stdout) so it does not interfere with piped output.
