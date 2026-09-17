---
format: https://specscore.md/feature-specification
status: Draft
---

# Feature: Id-scoped subcollection definitions and persisted placeholder names

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/scoped-subcollection-definitions?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/scoped-subcollection-definitions?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/scoped-subcollection-definitions?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-go/spec/features/scoped-subcollection-definitions?op=request-change) |
**Status:** Draft
**Date:** 2026-09-17
**Owner:** alex
**Supersedes:** —
**Source Ideas:** —
**Tracking:** [ingitdb/ingitdb-go#26](https://github.com/ingitdb/ingitdb-go/issues/26)
**Related Features:** [`subcollection-record-validation`](../subcollection-record-validation/README.md), [`explicit-record-base-directory`](../explicit-record-base-directory/README.md), [`record-body-file`](../record-body-file/README.md), [`definition-inheritance`](../definition-inheritance/README.md)

## Summary

A subcollection definition is stored under the id position of its parent collection, not just under its name, so `ext/datatug/projects` and `ext/sneat/projects` are independent definitions and each placeholder name is persisted once.

Each id position is either:

- a **placeholder** such as `{projectID}`: the definition applies to the subcollection under every record of the parent collection;
- a **concrete id** such as `datatug`: the definition applies only under that one parent record.

The schema tree mirrors the schema path segment for segment. The definition of `ext/datatug/projects/{projectID}/queries` is the file:

```
ext/.collection/subcollections/datatug/projects/subcollections/{projectID}/queries/definition.yaml
```

This has three consequences:

- `ext/datatug/projects` and `ext/sneat/projects` are two independent definitions.
- The placeholder name is persisted exactly once for each parent definition, as the name of its single `{name}` directory. A different name is a conflict.
- A concrete-id scope and the placeholder scope of the same parent may not both define a collection with the same name, so every data path resolves to at most one definition.

Every implementation turns a record id into a directory or file name segment with one exported function, `ingitdb.RecordDirName`, aligned with `record.EscapeID`, and concrete scope directories use the same function. Reading is strict: every entry in the tree is either a known shape or a load error. `CollectionDef.SubCollections` changes from a name-keyed map to a list of scopes. One exported resolver replaces the per-driver `resolveScopedCollection` walks. Where a subcollection's data directory sits relative to its parent collection's `records_dir` is an open founder decision (Open Question `subcollection-data-directory-rule`); the REQs and ACs that depend on it are written for the recommended option A and marked provisional. A scoping parent record such as `ext/datatug` need not exist, and nothing creates it. The contract is YAML conformance vectors in `ingitdb/ingitdb` plus the JSON Schema in `ingitdb/ingitdb-schema`.

This Feature is the inGitDB side of dal-go/dalgo Feature `schema-subcollections-extensions` (REQ `ingitdb-format-dependency`), and it unblocks ingitdb/dalgo2ingitdb#16. The project is in private beta, so the old name-keyed layout is replaced, not kept alongside (founder, 2026-09-17: "Fix at source, no temporary solutions"; "We are still in private beta, don't worry about what already exists").

## Problem

dal-go/dalgo Feature `schema-subcollections-extensions` REQ `schema-path-scopes` requires every nesting driver to:

- keep a definition's scope, so `ext/datatug/projects` never applies under `ext/sneat`;
- let `ext/datatug/projects` and `ext/sneat/projects` coexist with different fields and extensions;
- refuse a definition that `Overlaps` an existing one;
- persist every placeholder name, and refuse a conflicting one with `dbschema.ErrPlaceholderNameConflict` (founder decision "A", 2026-09-17, which makes this mandatory);
- neither require nor create the scoping parent record.

The inGitDB definition format cannot express any of this:

- `CollectionDef.SubCollections` is `map[string]*CollectionDef` keyed by collection name (`ingitdb/collection_def.go:42`). `ext/datatug/projects` and `ext/sneat/projects` would share the key `projects`.
- On disk, a subcollection lives at `<root>/.collection/subcollections/<name>/definition.yaml`, recursively (`validator/def_validator.go` `loadSubCollections`, `:479`). The shared layout discovers any child directory of `.collections/<name>/` that holds a `definition.yaml` (`loadSubCollectionsShared`, `:446`). Neither has room for an id position.
- A definition file has no key for a placeholder name. The strict reader (`decodeCollectionDef`, `KnownFields(true)`, `:33-35`) rejects unknown keys, which is correct: a name must be modelled, not smuggled in.
- `dalgo2ingitdb` `resolveScopedCollection` (`scoped_collection.go:39`) and its copies in `dalgo2ingitdb4github` and `dalgo2ingitdb4local` look subcollections up by name only. `createSubCollection` (`schema_modifier.go:349`) writes `subcollections/<name>` and checks only that the root exists.
- Implementations turn a parent record id into a directory name differently. `dalgo2ingitdb` escapes only `/` and `\` (`record_io.go:22`, `recordKeyToFileName`); `dalgo2ingitdb4local` (`record_file.go:22`), `dalgo2ingitdb4github` and the `ingitdb-go` validator use the raw id; a schema path uses `record.EscapeID`. The id `v1.2` is therefore `v1.2` in the data tree and `v1%2E2` in a schema path, and `a/b` is `a%2Fb` in one driver and a nested directory or an error in the others.
- `dalgo2ingitdb4github` lists records with a regular expression built from the name template in which `{key}` becomes `(.*?)` and a repeated `{key}` becomes `.*` (`query.go:255-259`), over a recursive tree listing. Deeper files, such as a subcollection's records or another extension's files, can match and be returned as records.
- Two data-directory conventions disagree. `datavalidator.subCollectionDataDir` (`datavalidator/subcollection_validation.go:37`) inserts the parent's `RecordsBasePath()` (`$records` by default). `dalgo2ingitdb`'s `resolveScopedCollection` joins the parent record id directly onto the parent's `DirPath`. The same definition therefore reads subcollection records from two different places.

The end user is DataTug's project store (`datatug/datatug` Feature `dalgo-project-store`, REQ `extension-namespace`). Every DataTug record lives under `ext/datatug/`, whose `datatug` record is a scoping parent with no record file. Another extension's `ext/<other>/` subtree must coexist in the same store. Each DataTug subcollection has its own `record_file`, `{key}/{key}.<suffix>.json` with `records_dir: '.'`, and `queries` also declares `body_files` (Feature `record-body-file`).

## Design Principles

- **The tree is the schema path.** Directory names under `subcollections/` are exactly the id and collection segments of the dalgo schema path, so a reader can see the path in the tree, and the tree can only express valid paths.
- **One place for each fact.** A placeholder name exists once per parent definition, as a directory name. It is never repeated in child files that could disagree.
- **Refuse ambiguity instead of ranking it.** Overlapping scopes are refused at load and at create, so no precedence rule can silently pick one of two definitions.
- **Strict everywhere.** An entry the reader does not model is an error, except the two explicitly ignored kinds (REQ `strict-tree-reading`).
- **One name for one id.** An id becomes exactly the same path segment in the schema tree, in record file names and in data directories, on every implementation and every platform.
- **One resolver.** Data-path resolution and the data-directory convention are exported once by `ingitdb-go`. Drivers do not reimplement them.
- **One path grammar.** Segment classification, id escaping and the placeholder identifier come from `github.com/dal-go/record`, as dalgo REQ `path-grammar` requires.

## Behavior

### On-disk layout

#### REQ: scope-directory-tree

A collection's subcollection definitions MUST be stored under its schema directory in this tree:

```
<collection schema dir>/
  definition.yaml
  subcollections/                 # optional; absent means no subcollections
    <scope dir>/                  # one id position of this collection: {name} or a concrete id
      <collection name>/          # a subcollection definition in that scope
        definition.yaml
        subcollections/ ...       # the same tree, recursively
        views/ ...                # as today
```

The collection schema directory is `<dir>/.collection/` for a root collection. For a subcollection, it is its own `<collection name>/` directory in this tree.

The DataTug store's schema, for its root collection `ext` at directory `ext`, is:

```
ext/.collection/
  definition.yaml                              # ext
  subcollections/
    datatug/                                   # scope: the one ext record "datatug" (need not exist)
      projects/
        definition.yaml                        # ext/datatug/projects
        subcollections/
          {projectID}/                         # scope: every project record; persists the name "projectID"
            queries/definition.yaml            # ext/datatug/projects/{projectID}/queries
            credentials/definition.yaml
            entities/definition.yaml
            environments/
              definition.yaml                  # ext/datatug/projects/{projectID}/environments
              subcollections/
                {envID}/
                  servers/definition.yaml      # .../environments/{envID}/servers
                  catalogs/definition.yaml
            dbdrivers/
              definition.yaml
              subcollections/
                {dbdriverID}/dbservers/definition.yaml
    sneat/                                     # another extension, independent of datatug
      projects/definition.yaml                 # ext/sneat/projects
```

The DataTug store keeps this whole tree inside the one root collection `ext`. There is no second root collection and no invented root id.

Recommendation over the alternatives:

- **Scope written into `definition.yaml`** (`scope: datatug` on a name-keyed directory). Two scopes of `projects` would still need two directories, so the name must be decorated anyway (`projects@datatug`). The tree would then stop being the schema path, and overlap detection would need every file to be parsed.
- **Placeholder name as a key on the parent definition** (`id_placeholder: projectID` on `projects`). Creating the first child under a placeholder would then rewrite the parent's file, a two-file change with no atomicity, and a hand edit could drop the name while children remain.
- **A separate root collection whose `DirPath` is `ext/datatug/projects`.** dalgo REQ `ingitdb-format-dependency` evaluates and rejects this: it needs invented ids and cannot express placeholder scopes above the scoped level, placeholder names or overlap.
- **Path segments without `subcollections/` separators** (`.collection/datatug/projects/{projectID}/queries`). A concrete id could then collide with `definition.yaml`, `views` or `README.md` at the same level.

#### REQ: record-dir-names

`ingitdb-go` MUST export the one mapping between a record id and the path segment that names it on disk:

```go
// RecordDirName returns the portable path segment for a record id.
func RecordDirName(id string) (string, error)
// RecordIDFromDirName is the exact inverse; it rejects any segment RecordDirName never returns.
func RecordIDFromDirName(name string) (string, error)
var ErrInvalidRecordID = errors.New("ingitdb: record id cannot be stored as a path segment")
```

`RecordDirName(id)` MUST:

1. reject an empty id, and an id that fails `record.ValidateStringID` (which reserves a literal `%`);
2. reject an id containing a control character or any of `:` `*` `?` `"` `<` `>` `|`, or ending in a space;
3. return `record.EscapeID(id)` (dal-go/record, with the escape table of dalgo REQ `path-grammar`, dal-go/record#3), which escapes `/` `\` (`%5C`, added to `EscapeID` and `UnescapeID` by dal-go/record#3) `.` `$` `#` `[` `]` `{` `}` `,` `=`;
4. reject the result when it fails the **portable segment rule** below.

**Portable segment rule.** A path segment (a `RecordDirName` result, a scope directory name or a subcollection directory name) MUST NOT end in `.` or a space, and the part of it before the first `.`, with trailing spaces removed and compared case-insensitively, MUST NOT be a Windows reserved device name (`CON`, `PRN`, `AUX`, `NUL`, `COM0`-`COM9`, `LPT0`-`LPT9`, including the superscript-digit variants). Escaped ids contain no `.`, so for them the whole name is compared; collection names are not escaped, so `queries.`, `queries `, `CON.txt`, `con.fig` and `Aux` all fail. The rule applies on every platform.

Every failure MUST satisfy `errors.Is(err, ErrInvalidRecordID)` (vector kind `invalid-record-id`). The rules are the same on every platform, at load, at resolution and at write, so a database written on Linux reads identically on Windows and macOS.

`RecordIDFromDirName(name)` MUST return `record.UnescapeID(name)` only when `RecordDirName` of that id returns `name` byte for byte, and `ErrInvalidRecordID` otherwise.

Every reader and writer MUST use these two functions, and nothing else, wherever a record id meets a path:

- each `{key}` substituted into `record_file.name`, and the key extracted back when listing;
- the `<key>` part of a body file name (Feature `record-body-file`);
- the parent record directory of a subcollection instance (REQ `data-resolution`);
- a concrete-id scope directory (REQ `scope-directory-names`).

Examples, pinned by vectors: `a/b` is `a%2Fb`; `a\b` is `a%5Cb`; `v1.2` is `v1%2E2`; `50%` fails (`%` is reserved); `CON`, `con` and `a:b` fail; `a ` fails.

Implementations that MUST switch to them: `ingitdb-go` (`RecordFileDef.GetRecordFileName`, the record readers' key extraction, `datavalidator.subCollectionDataDir`); `dalgo2ingitdb` (`recordKeyToFileName`, `recordKeyFromFileName`, `validateRecordPathSegment`, `scoped_collection.go`); `dalgo2ingitdb4github` (`query.go` `buildKeyExtractor`, record paths, `scoped_collection.go`); `dalgo2ingitdb4local` (`record_file.go` `resolveRecordPath`, `tx_query.go` glob, `scoped_collection.go`); `ingitdb-cli` (`subcollection_path.go` `resolveFromCollection`, which MUST unescape each id segment of `--from` with `record.UnescapeID` before resolving); `ingitdb-ts` `client-fs` and `client-github`; `ingitdb-ws`.

#### REQ: listing-one-segment-per-key

When a reader lists the records of a collection instance from a name template, each `{key}` in the template MUST match exactly one path segment, that is, a non-empty run without `/`. A file whose relative path would need `{key}` to span several segments, or whose `{key}` occurrences differ, MUST NOT be returned as a record, and every extracted key MUST pass `RecordIDFromDirName`. A recursive listing MUST NOT return files that belong to a deeper subcollection instance or to `.collection/`. This replaces `(.*?)` and `.*` in `dalgo2ingitdb4github` `buildKeyExtractor`, and is pinned by a `list` vector in the reader profile.

#### REQ: scope-directory-names

A scope directory name MUST be one id segment of dalgo REQ `path-grammar`, classified with the grammar exported by `github.com/dal-go/record`:

- **Placeholder:** `{` + identifier + `}`, where the identifier satisfies the shared placeholder identifier rule (`^[A-Za-z_][A-Za-z0-9_]*$`, `record.ValidPlaceholderName`).
- **Concrete id:** a name for which `RecordIDFromDirName` succeeds (REQ `record-dir-names`). So `datatug` and `v1%2E2` are valid, and `v1.2`, `a%2fb` (lower case), `CON` and `{x` are not. `EscapeID` escapes `{`, so no concrete id directory can look like a placeholder, and a concrete scope directory has exactly the name of that parent record's data directory.
- **Composite key** (`{field=value,…}`): rejected with `reserved-key-segment`, as in dalgo.
- Anything else, including a name that fails the portable segment rule of REQ `record-dir-names`, is `invalid-scope-directory`.

A subcollection directory name MUST satisfy `ingitdb.ValidateSubCollectionName`, which is dalgo's collection-name rule (non-empty, not `.` or `..`, no leading or trailing whitespace, no control character, none of `/` `\` `{` `}` `,` `=` `%`) plus REQ `record-dir-names` step 2 (no `:` `*` `?` `"` `<` `>` `|`) and its portable segment rule (no trailing `.` or space; no reserved device name before the first `.`, case-insensitively), and additionally MUST NOT begin with `.` or `$` (reserved for ignored entries and for `$views`). Otherwise it is `invalid-collection-name`. dalgo accepts `-`, and so does this rule; the root-collection id rule `ValidateCollectionID` is unchanged. `dalgo2ingitdb` MUST map `invalid-collection-name` to an error that satisfies `errors.Is(err, dbschema.ErrInvalidCollectionPath)`, so a name dalgo accepts but inGitDB refuses (for example `.hidden`) fails as a typed path error.

Two sibling scope directories whose names are equal under Unicode simple case folding MUST be rejected at load as `scope-case-collision`, because they name one directory on case-insensitive filesystems (NTFS, default APFS). Two sibling collection directories are held to the same rule. `PlanCreateCollection` MUST refuse to introduce such a pair (REQ `definition-path-addressing`).

#### REQ: placeholder-name-persisted

A collection definition's placeholder name is the name of its single placeholder scope directory. It is persisted for as long as that directory holds at least one subcollection definition.

- A `subcollections/` directory MUST contain **at most one** placeholder scope directory. Two (for example `{projectID}` and `{pid}`) MUST be rejected at load with `placeholder-name-conflict`.
- A create MUST compare every placeholder position of the declared path with the stored name at that position, root first, after ancestors are resolved and before anything is written (REQ `definition-path-addressing`). If the declared name differs from a stored name, the create MUST fail with an error satisfying `errors.Is(err, ingitdb.ErrPlaceholderNameConflict)` and create nothing. The message MUST name the path of the collection whose id the placeholder stands for (for example `ext/datatug/projects`), the stored name and the declared name. `dalgo2ingitdb` MUST return an error that also satisfies `errors.Is(err, dbschema.ErrPlaceholderNameConflict)`.
- The first definition created under a placeholder scope creates the directory, and so persists the name. When a drop removes the last definition under a placeholder scope, the writer MUST remove that scope directory (REQ `definition-path-addressing`), and the name is no longer stored.
- Addressing an existing definition (lookup, alter, drop) MUST ignore placeholder names, matching dalgo `SameCollection`.

#### REQ: scopes-must-not-overlap

Within one `subcollections/` directory, a collection name MUST NOT be defined both under a concrete-id scope and under the placeholder scope. For example, `datatug/projects` and `{extID}/projects` overlap, while `datatug/projects` and `{extID}/settings` do not.

- At load, an overlap MUST be rejected with `scopes-overlap`, naming both schema paths.
- On create, a declaration that would introduce one MUST fail with an error satisfying `errors.Is(err, ingitdb.ErrSubCollectionScopesOverlap)` and create nothing, in both directions: a placeholder scope when a concrete scope exists, and a concrete scope when the placeholder scope exists.

This per-level rule is exactly dalgo's `Overlaps`, because ancestor definitions must exist before a create (dalgo REQ `one-collection-per-create`). If two schema paths differ first at id position *i*, with a concrete id on one side and a placeholder on the other, then their truncations after the collection at position *i* + 1 already overlap at that one level. The deeper pair can only exist if that pair does, and that pair is refused.

Concrete scopes under different ids never overlap: `datatug/projects` and `sneat/projects` are independent definitions with independent columns, `record_file`, views and subcollections.

#### REQ: strict-tree-reading

The definition reader MUST reject every entry it does not model, with the error kind named here. It MUST NOT skip a malformed entry.

| Location | Allowed entries | Otherwise |
|----------|-----------------|-----------|
| collection schema directory | `definition.yaml`, `subcollections/`, `views/`, `README.md` (as today), ignored entries | `unexpected-schema-entry` |
| `subcollections/` | scope directories (REQ `scope-directory-names`), `README.md`, ignored entries | the scope-name kinds, or `unexpected-schema-entry` for a non-directory |
| scope directory | collection directories that contain `definition.yaml`, `README.md`, ignored entries | `missing-definition` for a directory without `definition.yaml`, `unexpected-schema-entry` for any other entry |

Further rules:

- **Ignored entries** are entries whose name begins with `.` (for example `.DS_Store`). No valid scope directory or collection directory can begin with `.`, because `EscapeID` escapes `.` and collection ids start with an alphanumeric. `README.md` is allowed only as a regular file, and the reader never parses it. `docsbuilder` writes it.
- **Check order.** Inside `subcollections/`, the legacy-layout check runs first, before any scope-name check, so `subcollections/order_details/definition.yaml` is always reported as `legacy-subcollection-layout` and never as a concrete scope.
- **`$`-prefixed directories** were skipped by the shared-layout reader. Except `$views/` where REQ `shared-layout-uses-same-tree` allows it, they are now `unexpected-schema-entry` (a scope name starting with `$` is also impossible, because `EscapeID` escapes `$`).
- **Legacy layout.** A scope directory that directly contains `definition.yaml` is the replaced `subcollections/<name>/definition.yaml` layout. It MUST be rejected with `legacy-subcollection-layout`, whose message names the directory and says the definition moves to `subcollections/{<name>}/<collection>/`. It MUST NOT be read as a concrete id scope.
- **Empty scope directory.** A scope directory with no entries except ignored ones and `README.md` holds no definitions. It MUST be read as absent, and for a placeholder directory it persists no name. Git cannot store an empty directory, so this can only arise on a working tree. Writers MUST NOT leave one behind.
- **Definition files** keep today's strict decoding (`KnownFields(true)`), inheritance resolution and `Validate`. No key is added to `definition.yaml` by this Feature (REQ `definition-file-carries-no-scope`).
- A load error MUST name the offending path relative to the database root, and the vector error kind MUST be recoverable with `errors.Is` against an exported sentinel error per kind.

#### REQ: shared-layout-uses-same-tree

The shared-directory layout (`.collections/<name>/definition.yaml`) MUST use the same `subcollections/<scope>/<collection>/` tree inside `.collections/<name>/`. Its implicit discovery of any child directory holding a `definition.yaml` (`loadSubCollectionsShared`) is removed. In that layout, the allowed entries of a collection schema directory are `definition.yaml`, `subcollections/`, `$views/`, `README.md` and ignored entries.

Recommendation over keeping implicit discovery: implicit discovery is name-keyed by construction, so it cannot hold scopes. Keeping it would leave one layout that silently cannot express `ext/datatug/projects`. No database or writer found uses this layout for subcollections today (searched on 2026-09-17 across the `ingitdb`, `sneat-co`, `openvaultdb` and `datatug` checkouts on host `ai`). `sneat-co/ovdb` `onboarding4ovdb` and openvaultdb Feature `sneat-space-export` use it for root collections only, and are listed in REQ `dependents-move-together`.

#### REQ: scoping-parent-not-required

A concrete-id scope MUST NOT require its parent record. Loading `ext/datatug/projects`, resolving `ext/datatug/projects/p1/queries`, validating that instance and creating a definition under the `datatug` scope MUST all succeed when no `ext` record `datatug` exists. None of them MUST create that record, its record file or its directory. Only definition files, the directories that hold them, and the data files of records actually written are ever created.

The ancestor **definitions** still must exist (dalgo REQ `one-collection-per-create`): `ext/datatug/projects` requires the root collection `ext`.

### Go model and resolution

#### REQ: go-model

`github.com/ingitdb/ingitdb-go/ingitdb` MUST replace the name-keyed map with scopes:

```go
// ParentScope is the id position between a parent collection and a
// subcollection definition. Exactly one field is non-empty.
type ParentScope struct {
    ID          string // concrete parent record id, unescaped
    Placeholder string // placeholder name, e.g. "projectID"
}

func ScopeID(id string) ParentScope
func ScopePlaceholder(name string) ParentScope
func (s ParentScope) IsPlaceholder() bool
func (s ParentScope) DirName() string                  // record.EscapeID(ID) or "{" + Placeholder + "}"
func ParseScopeDirName(name string) (ParentScope, error)
func (s ParentScope) SameScope(o ParentScope) bool     // both placeholders (names ignored), or equal IDs

// SubCollectionScope holds the subcollection definitions of one id position.
type SubCollectionScope struct {
    Scope       ParentScope
    Collections map[string]*CollectionDef // keyed by collection name
}

type CollectionDef struct {
    ID          string      // collection name (leaf), as today
    SchemaPath  string      `yaml:"-" json:"-"` // canonical, e.g. "ext/datatug/projects/{projectID}/queries"
    ParentScope ParentScope `yaml:"-" json:"-"` // zero value for a root collection
    // SubCollections lists this collection's scopes: concrete scopes sorted by
    // ID, then at most one placeholder scope. Replaces map[string]*CollectionDef.
    SubCollections []*SubCollectionScope `yaml:"-" json:"-"`
    // ... all other fields unchanged
}

func (c *CollectionDef) PlaceholderName() string // "" when there is no placeholder scope

// DefinitionPath is a parsed schema path: a root collection id, then steps.
type DefinitionStep struct {
    Scope      ParentScope
    Collection string
}
type DefinitionPath struct {
    Root  string
    Steps []DefinitionStep
}

func ParseDefinitionPath(s string) (DefinitionPath, error) // dalgo REQ path-grammar, via github.com/dal-go/record
func (p DefinitionPath) String() string                     // canonical, no leading "/"

var (
    ErrPlaceholderNameConflict    = errors.New("ingitdb: placeholder name conflicts with the stored name")
    ErrSubCollectionScopesOverlap = errors.New("ingitdb: subcollection scopes overlap")
    ErrCollectionNotFound         = errors.New("ingitdb: collection not found in definition")
    ErrAncestorNotFound           = errors.New("ingitdb: ancestor collection definition not found")
    ErrCollectionExists           = errors.New("ingitdb: collection definition already exists")
    ErrScopeCaseCollision         = errors.New("ingitdb: name differs from a sibling only by letter case")
    // plus one sentinel per load error kind of REQ strict-tree-reading and REQ scope-directory-names
)
```

- `ingitdb-go` MUST NOT import `github.com/dal-go/dalgo`. `DefinitionPath` is built on the `github.com/dal-go/record` grammar, and `dalgo2ingitdb` maps it to and from `dbschema.SchemaPath` one step at a time.
- `ParseDefinitionPath(p.String())` MUST equal `p` for every valid `p`, and `String()` MUST equal `dbschema.SchemaPath.String()` for the same path.
- `CollectionDef.Validate` MUST recurse through every scope, and the load-time rules of REQ `placeholder-name-persisted` and REQ `scopes-must-not-overlap` MUST also hold for a `CollectionDef` built in memory and validated.
- `ValidateSubCollectionName` (REQ `scope-directory-names`), `RecordDirName` and `RecordIDFromDirName` (REQ `record-dir-names`) are exported from the same package.
- Every in-repo user of the old map MUST move to the new API in the same change: `foreign_key.go` (`ValidateForeignKeys` walk), `datavalidator/subcollection_validation.go`, `datavalidator/foreign_key_check.go`, `docsbuilder/collection_readme.go`, `docsbuilder/update.go` and `validator/def_validator.go`.

#### REQ: definition-path-addressing

`ingitdb-go` MUST export the addressing and pre-write checks that every writer uses, so no writer reimplements them:

```go
// CollectionAt returns the definition at p, matching concrete ids exactly and
// placeholder positions by scope only (names ignored), or ErrCollectionNotFound.
func (d *Definition) CollectionAt(p DefinitionPath) (*CollectionDef, error)

// PlanCreateCollection checks a new definition at p and returns the path, relative to
// the database root, of the definition.yaml to write. It creates nothing.
func (d *Definition) PlanCreateCollection(p DefinitionPath) (defFile string, err error)

// PlanDropCollection returns the directories to remove for the definition at p:
// its collection directory, and its scope directory when p is that scope's
// last definition. It removes nothing.
func (d *Definition) PlanDropCollection(p DefinitionPath) (removeDirs []string, err error)
```

`PlanCreateCollection` MUST check, in this order, and return the first failure:

1. `p` is valid (`ParseDefinitionPath` rules), every concrete id passes `RecordDirName`, and every collection name passes `ValidateSubCollectionName` (REQ `scope-directory-names`).
2. The root collection and every ancestor definition exist, addressed as in `CollectionAt`. Otherwise it returns `ErrAncestorNotFound`, naming the first missing ancestor path.
3. The placeholder names of `p` match the stored names at every position (`ErrPlaceholderNameConflict`).
4. No definition exists at `p` (`ErrCollectionExists`). A caller with `IfNotExists` treats this error as success.
5. No scope directory or collection directory the plan would create case-folds equal to an existing sibling with a different name (`ErrScopeCaseCollision`, vector kind `scope-case-collision`). For example `ext/DataTug/projects` when `ext/datatug/…` exists.
6. The new definition would not overlap a sibling (`ErrSubCollectionScopesOverlap`).

- **Alter** rewrites only the addressed `definition.yaml`. The tree is unchanged.
- **Drop** removes the directories from `PlanDropCollection`. What a drop may also delete (records, descendant definitions) is decided by the caller under dalgo REQ `drop-refuses-data-loss-by-default` before the plan is applied. A writer MUST NOT leave an empty scope directory (REQ `strict-tree-reading`), and a scope directory that holds only `README.md` counts as empty and is removed with it.

#### REQ: data-resolution

`ingitdb-go` MUST export the one resolver for data paths, and the data-directory convention with it:

```go
// DataStep is one ancestor record of a data path.
type DataStep struct {
    Collection string
    ID         string
}

// ResolveCollection returns a shallow copy of the definition that governs
// records of collection name under the ancestor record chain parents (root
// first; empty for a root collection). DirPath is set to that instance's data
// directory, and SchemaPath to the definition's path.
func (d *Definition) ResolveCollection(parents []DataStep, name string) (*CollectionDef, error)
```

Resolution MUST, level by level, look `name` (or the next ancestor collection) up in the scope whose `ID` equals the parent record id byte for byte, and then in the placeholder scope. By REQ `scopes-must-not-overlap` at most one of them defines it, so the order does not change the result. It only avoids a second lookup in the common case. If neither scope defines it, resolution MUST return `ErrCollectionNotFound`, naming the data path. It MUST NOT fall back to a same-named definition in another concrete scope. So `ext/other/projects/p1` is not found when only `ext/datatug/projects` is defined.

**Provisional on Open Question `subcollection-data-directory-rule`, option A.** The data directory of an instance MUST be:

```
<parent instance data dir>/<RecordDirName(parent record id)>/<name>
```

where a root collection's instance data dir is its `DirPath`. The parent collection's `record_file.records_dir` and `$records` default do **not** take part: they place the parent's own record files, not its subcollections. Consequences:

- `ext/datatug/projects/p1/queries` resolves to `ext/datatug/projects/p1/queries/` whatever record file `ext` declares, so creating `ext` with the dalgo2ingitdb default `{key}.yaml` fixes nothing for other extensions, and changing `ext`'s `records_dir` later moves no descendant data.
- With `projects` declaring `{key}/{key}.datatug-project.json` and `records_dir: '.'`, the project record file and its subcollections share the directory `ext/datatug/projects/p1/`, matching `dalgo-project-store`'s canonical layout.
- With the default `$records` base, `orders/o1/order_details` resolves to `orders/o1/order_details/`, while the order record stays at `orders/$records/o1.yaml`.
- An id that `RecordDirName` rejects fails resolution with `ErrInvalidRecordID` without touching the filesystem.
- The resolved directory MUST stay inside the root collection's directory.

Vectors and runners express every `data_dir` and finding `path` relative to the database root, with `/` separators.

Under option B, the formula would instead insert `<parent RecordFile.RecordsBasePath()>` after the parent instance data dir (today's `datavalidator.subCollectionDataDir`), and `ext` would have to declare `records_dir: '.'`.

`dalgo2ingitdb`, `dalgo2ingitdb4github`, `dalgo2ingitdb4local` and `ingitdb-cli` (`subcollection_path.go` `resolveFromCollection`) MUST call `ResolveCollection` instead of walking `SubCollections` themselves. This retires their `resolveScopedCollection` copies and removes the data-directory and escaping disagreements described in Problem.

#### REQ: validation-walks-scopes

**Provisional on Open Question `subcollection-data-directory-rule`, option A** (for the data directories it walks).

Whole-database validation (Feature `subcollection-record-validation`) MUST enumerate subcollection instances per scope, for each instance of the parent collection:

- a **placeholder scope** yields one instance per existing record of that parent instance, as today;
- a **concrete scope** yields exactly one instance **per parent collection instance**, for its id, whether or not that parent record exists (REQ `scoping-parent-not-required`). Under a root parent that is one instance; under a placeholder ancestor, such as a hypothetical `ext/datatug/projects/{projectID}/settings/defaults/items`, it is one per `projects` record. If the data directory is absent, the instance has zero records, so `min_records_count` still applies to it.

Each finding's `CollectionID`, and each record-count key, MUST be the definition's `SchemaPath` (for example `orders/{orderID}/order_details` or `ext/datatug/projects`). This supersedes the placeholder-free slash path (`orders/order_details`) that `subcollection-record-validation` specifies. The placeholder-free path can no longer tell `ext/datatug/projects` from `ext/sneat/projects`.

#### REQ: validation-finding-kinds

`ingitdb.ValidationError` MUST gain `Kind string`, a stable kebab-case finding kind compared by conformance runners in place of message text. This Feature defines the kinds its vectors use, and every finding the validator emits for them MUST set one:

| Kind | Emitted when |
|------|--------------|
| `field-type-mismatch` | a record field's value does not match its column type |
| `record-count-below-min` | an instance holds fewer records than `min_records_count` |
| `record-count-above-max` | an instance holds more records than `max_records_count` |
| `invalid-record-file-name` | a listed file's extracted key fails `RecordIDFromDirName` |

Other findings MAY leave `Kind` empty until a Feature defines their kind; a vector MUST NOT expect a finding without a kind. `FilePath` stays absolute in the Go API. A runner converts it to the vector `path` by making it relative to the database root and replacing the OS separator with `/`.

#### REQ: definition-file-carries-no-scope

Scopes and placeholder names MUST live only in the directory tree. `definition.yaml` gains no key for them. `ingitdb/ingitdb-schema`'s `ingitdb-collection.schema.json` MUST therefore:

- keep `"additionalProperties": false` at the top level, so a hand-written `scope`, `placeholder`, `parent` or `subcollections` key fails schema validation, just as it fails the strict reader;
- describe in its top-level `description` that subcollection definitions and their scopes are stored as the `subcollections/<scope>/<collection>/definition.yaml` tree, with a link to `conformance/subcollection-scopes/README.md`;
- model every key that any definition in `conformance/subcollection-scopes/` uses, so every accepted definition in the vectors validates against it. Where that needs a key ingitdb/ingitdb-schema#9 has not added yet (for example `records_dir`), the change is made there first.

No JSON Schema is added for the directory tree. JSON Schema describes one document, and the tree is pinned by the vectors.

### Contract and dependents

#### REQ: conformance-vectors

The normative contract for the tree MUST be YAML conformance vectors in `conformance/subcollection-scopes/` in `ingitdb/ingitdb`. This is a sibling of `conformance/record-layout/` (Feature `record-body-file`), because those vectors fix a single collection `queries`, while these need definition trees. Every vector MUST carry a YAML comment that explains it for humans (founder, 2026-09-17: "Yaml can have comments for humans.").

The directory holds:

- `README.md`: normative. It defines the format below and the complete error-kind table.
- `vectors.yaml`: behaviour vectors.
- `validation_vectors.yaml`: load rejections.

**File format.** Both files have the top-level keys `version` (`1`) and `vectors`. File entries (`{path, content?, mode?}`) follow the `conformance/record-layout/README.md` definition exactly, including `|` versus `|-` and `base64:`. Each vector in `vectors.yaml` has these keys:

| Key | Meaning |
|-----|---------|
| `name` | A unique kebab-case id. |
| `setup_files` | The database before the operation, including `.ingitdb/root-collections.yaml`. |
| `op` | `{kind, path?, parents?, name?, id?}`. `kind` is `load`, `resolve`, `list`, `validate`, `record-dir-name`, `plan-create` or `plan-drop`. `path` is a schema path for `plan-create` and `plan-drop`. `parents` (a list of `{collection, id}`, raw ids) and `name` are for `resolve` and `list`. `id` is for `record-dir-name`. |
| `expect_definitions` | For `load`: every definition, sorted by `schema_path`, each `{schema_path, schema_dir, placeholder_name?}`. |
| `expect_resolution` | For `resolve`: `{schema_path, data_dir}`, `data_dir` relative to the database root with `/` separators. |
| `expect_list` | For `list`: the sorted raw record keys. |
| `expect_dir_name` | For `record-dir-name`: the returned segment. |
| `expect_findings` | For `validate`: `{kind, collection_id, path}` entries, `kind` from REQ `validation-finding-kinds`, `path` relative to the database root with `/` separators. |
| `expect_plan` | For `plan-create`: `{def_file}`. For `plan-drop`: `{remove_dirs}`. Paths relative to the database root with `/` separators. |
| `expect_error` | An error kind. Kinds are compared, never message text. |

`validation_vectors.yaml` vectors have `name`, `setup_files`, `expect_error` and `target` (the offending path relative to the database root).

**Error kinds:**

- load: `invalid-scope-directory`, `reserved-key-segment`, `invalid-collection-name`, `scope-case-collision`, `placeholder-name-conflict`, `scopes-overlap`, `unexpected-schema-entry`, `missing-definition`, `legacy-subcollection-layout`;
- behaviour: `collection-not-found`, `ancestor-not-found`, `collection-exists`, `placeholder-name-conflict`, `scope-case-collision`, `scopes-overlap`, `invalid-collection-path`, `invalid-collection-name`, `invalid-record-id`.

**Profiles.**

- The **reader** profile is `load`, `resolve`, `list`, `validate` and `record-dir-name`, plus every validation vector.
- The **writer** profile is `plan-create` and `plan-drop`.

Implementations run them as follows:

- `ingitdb-go` runs both profiles against its exported functions.
- `dalgo2ingitdb` runs both profiles: the writer profile through `CreateCollection` and `DropCollection` on a temporary directory, comparing the resulting tree.
- `dalgo2ingitdb4github` and `dalgo2ingitdb4local` run the reader profile.
- `ingitdb-ts` runs the reader profile once it reads subcollections, gated like record-layout on ingitdb/ingitdb-ts#104.

**Ordering.** `conformance/record-layout/` (ingitdb/ingitdb#9) is not on `ingitdb/ingitdb` main yet. Its File-entry definition is the one this directory uses, so #9 lands first; if this directory is written first, it defines File entries itself and #9 then references it instead, with identical text. Either way, one definition exists.

**Vendoring** follows `record-body-file` REQ `conformance-vectors-are-the-contract`: a "re-sync from the standard, do not edit" header, and a CI check that fails on drift.

**Required coverage.** The vectors MUST cover at least:

- the DataTug tree of REQ `scope-directory-tree`, loaded with its schema paths and the placeholder names `projectID`, `envID` and `dbdriverID`;
- `ext/datatug/projects` and `ext/sneat/projects` with different columns and `record_file`, each resolving to its own definition, and `ext/other/projects/p1` resolving to `collection-not-found`;
- a placeholder scope `{extID}/settings` beside the concrete scope `datatug/projects`, loading cleanly and resolving `ext/datatug/settings` to the placeholder definition;
- a scoping parent with no record: `resolve` of `ext/datatug/projects/p1/queries` giving `ext/datatug/projects/p1/queries`, with `setup_files` holding no `ext` record, and a `validate` whose findings are empty;
- the data directory for a parent without `records_dir` (`orders/o1/order_details`, provisional on option A) and for a parent with `records_dir: '.'`;
- `record-dir-name` for `a/b` (`a%2Fb`), `a\b` (`a%5Cb`), `v1.2` (`v1%2E2`), `50%`, `CON`, `con`, `a:b` and `a ` (the last five `invalid-record-id`), and a `resolve` and a `plan-create` using the id `v1.2`, whose data directory and scope directory are both `v1%2E2`;
- a `list` of `ext/datatug/projects` whose tree also holds `p1/queries/q1/q1.query.json`, another extension's `ext/sneat/projects/s1/s1.yaml` and `.collection/definition.yaml`, returning only the project keys, and a GitHub-tree runner applying the same vector;
- a `validate` finding whose `collection_id` is a scoped schema path;
- every load error kind, including two placeholder directories, both overlap directions, a non-canonical concrete id (`v1.2`), a composite key directory, a case collision (`DataTug` beside `datatug`), a file inside `subcollections/`, a directory without `definition.yaml`, and the legacy `subcollections/order_details/definition.yaml`;
- `README.md` and `.DS_Store` entries at every level, ignored;
- `plan-create` success at depth 2 (`.../environments/{envID}/servers`), and each create failure in the check order of REQ `definition-path-addressing`: an invalid collection name (`.hidden`), a missing ancestor, a conflicting placeholder name (`{pid}/entities` when `{projectID}` exists), an existing definition, a case collision (`ext/DataTug/projects` beside `datatug`), and overlap in both directions;
- subcollection names failing the portable segment rule, at load and in `plan-create`: `queries.`, `queries `, `CON.txt`, `con.fig`, `Aux` and `nul` (all `invalid-collection-name`);
- a legacy `subcollections/order_details/definition.yaml` whose directory name is also a valid concrete id, reported as `legacy-subcollection-layout`;
- a `$`-prefixed directory in a `.collection/` schema directory, rejected with `unexpected-schema-entry`;
- `plan-drop` of the last definition under a placeholder scope, removing the scope directory, and of a non-last one, keeping it.

#### REQ: suffix-disjointness-follows-listing

`record-body-file` REQ `suffix-disjointness-covers-body-files` compares a body-bearing collection with every collection whose records base directory is the same directory **or an ancestor**, because the GitHub reader listed recursively. With scopes, the root `ext` is an ancestor of every extension's collections, so one extension's definition could reject another's.

With REQ `listing-one-segment-per-key` in force, no record matcher can reach a deeper directory. The ancestor clause MUST therefore be dropped: disjointness MUST compare D only with collections whose record files can sit in D's record-file directory at the same depth, which are D itself and collections whose instance data directory is D's. Collections in other scopes, and ancestors, are never compared. This needs a follow-up amendment of `record-body-file` REQ `suffix-disjointness-covers-body-files` and its `suffix-overlap` vectors, landed together with this Feature.

#### REQ: dependents-move-together

In line with `record-body-file` REQ `implementations-current-and-strict`, there are no compatibility readers, migration shims or version switches. Every implementation moves to the release of `ingitdb-go` that ships this Feature and passes the vectors. Every repository-held database or fixture in the old layout is moved to the new tree in the same wave. A database left in the old layout fails to load with `legacy-subcollection-layout` (REQ `strict-tree-reading`), which is loud and names the fix.

Known dependents (2026-09-17, checkouts on host `ai`):

- **Databases and fixtures:**
  - `demo-ingitdb/modules/commerce/collections/orders/.collection/subcollections/order_details/` moves to `subcollections/{orderID}/order_details/`. Provisional on option A, its data moves from `orders/$records/<id>/order_details/` to `orders/<id>/order_details/`;
  - the `spaces/.collection/subcollections/` format fixtures in `dalgo2ingitdb/testdata/format-fixtures`, `ingitdb-ts/packages/client-fs/src/__fixtures__/format-fixtures` and `ingitdb-ts/packages/client-github/src/__fixtures__/format-fixtures`;
  - any record whose key contains a character `RecordDirName` escapes (for example `.`) is renamed to the escaped name.
- **Writers of the shared layout:** `sneat-co/ovdb` `backend/onboarding4ovdb/profile.go:46` writes `sneat/.collections/spaces`, and `openvaultdb/openvaultdb` Feature `sneat-space-export` specifies that layout. Both MUST follow REQ `shared-layout-uses-same-tree` and REQ `record-dir-names`. `$`-prefixed directories, which the shared-layout reader skipped, are now `unexpected-schema-entry` except `$views/`.
- **Go code:** `ingitdb-go` (REQ `go-model`); `dalgo2ingitdb` `schema_modifier.go`, `scoped_collection.go`, `record_io.go` and `protected.go:454` (which walks the map); `dalgo2ingitdb4github` `scoped_collection.go` and `query.go`; `dalgo2ingitdb4local` `scoped_collection.go`, `record_file.go` and `tx_query.go`; `ingitdb-cli` `subcollection_path.go` (`resolveFromCollection`, which unescapes segments), `materialize.go`, `tui/collection_screen.go` and `tui/collection_schema_panel.go`.
- **TypeScript:** `ingitdb-ts` `packages/client-fs/src/client.ts` (shared-layout detection, `:50`) and `packages/client-github`; `ingitdb-ws` `app/composables/useCollection.ts`.
- **Documentation:** `ingitdb-specs` `docs/schema/subcollection.md` describes the replaced layout and is rewritten.

## Acceptance Criteria

### AC: datatug-tree-loads

**Requirements:** scoped-subcollection-definitions#req:scope-directory-tree, scoped-subcollection-definitions#req:placeholder-name-persisted, scoped-subcollection-definitions#req:go-model

**Given** a database whose root collection `ext` holds the tree of REQ `scope-directory-tree` and no `ext` record file, with `ext` declaring `record_file` `{key}.yaml` and no `records_dir`, and with every DataTug subcollection declaring `record_file` `{key}/{key}.<suffix>.json`, `format: json`, `type: map[string]any`, `records_dir: '.'`, and `queries` also declaring `body_files`
**When** it is read with `validator.ReadDefinition`
**Then** it loads without error; `ext`'s `SubCollections` holds the scopes `datatug` and `sneat` and no placeholder scope; `CollectionAt(ParseDefinitionPath("ext/datatug/projects/{projectID}/queries"))` returns a definition whose `SchemaPath` is exactly that string and whose `RecordFile` carries the declared values; the `projects` definition's `PlaceholderName()` is `projectID`, and `environments`' is `envID`

### AC: sibling-scopes-are-independent

**Requirements:** scoped-subcollection-definitions#req:scopes-must-not-overlap, scoped-subcollection-definitions#req:data-resolution

**Given** the database of AC `datatug-tree-loads`, where `ext/sneat/projects` declares different columns and `record_file` `{key}.yaml` from `ext/datatug/projects`
**When** `ResolveCollection` is called for `ext` `datatug` / `projects`, for `ext` `sneat` / `projects`, and for `ext` `other` / `projects`
**Then** the first returns the DataTug definition, the second returns the Sneat definition, each with its own `record_file`, and the third fails with `errors.Is(err, ingitdb.ErrCollectionNotFound)`; no call returns a definition from another scope

### AC: overlap-rejected

**Requirements:** scoped-subcollection-definitions#req:scopes-must-not-overlap, scoped-subcollection-definitions#req:definition-path-addressing

**Given** three databases: one with `ext/.collection/subcollections/datatug/projects/` and `{extID}/projects/`; one with `datatug/projects/` and `{extID}/settings/`; and one with only `datatug/projects/`
**When** each is loaded, and `PlanCreateCollection("ext/{extID}/projects")` is called on the third, and `PlanCreateCollection("ext/datatug/settings")` on the second
**Then** the first fails to load with `scopes-overlap` naming `ext/datatug/projects` and `ext/{extID}/projects`; the second loads; and both planned creates return an empty `defFile` and an error satisfying `errors.Is(err, ingitdb.ErrSubCollectionScopesOverlap)`

### AC: placeholder-name-conflict-refused

**Requirements:** scoped-subcollection-definitions#req:placeholder-name-persisted, scoped-subcollection-definitions#req:definition-path-addressing

**Given** the database of AC `datatug-tree-loads`, which has no `entities` definition, and a second database whose `projects` `subcollections/` holds both `{projectID}/queries/` and `{pid}/notes/`
**When** `PlanCreateCollection` is called on the first with `ext/datatug/projects/{pid}/entities` and then with `ext/datatug/projects/{projectID}/entities`, and `CollectionAt("ext/datatug/projects/{anything}/queries")` is called; and the second database is loaded
**Then** the first plan fails with `errors.Is(err, ingitdb.ErrPlaceholderNameConflict)` and a message naming `ext/datatug/projects`, `projectID` and `pid`; the second plan returns `ext/.collection/subcollections/datatug/projects/subcollections/{projectID}/entities/definition.yaml`; `CollectionAt` returns the `queries` definition; and the second database fails to load with `placeholder-name-conflict`

### AC: create-check-order

**Requirements:** scoped-subcollection-definitions#req:definition-path-addressing

**Given** the database of AC `datatug-tree-loads`, and a copy of it without the `environments` definition and its subtree
**When** `PlanCreateCollection` is called with `ext/datatug/projects/{pid}/environments/{envID}/servers` on the copy, with `ext/datatug/projects/{pid}/queries` on the original, and with `ext/datatug/projects/{projectID}/queries` on the original
**Then** the first fails with `errors.Is(err, ingitdb.ErrAncestorNotFound)` naming `ext/datatug/projects/{pid}/environments`, not with a placeholder conflict; the second fails with `ErrPlaceholderNameConflict`, not with `ErrCollectionExists`; the third fails with `ErrCollectionExists`; `PlanCreateCollection("ext/DataTug/projects")` on the original fails with `ErrScopeCaseCollision`; and `PlanCreateCollection("ext/datatug/projects/{projectID}/.hidden")` fails with `invalid-collection-name`, which `dalgo2ingitdb` reports as `dbschema.ErrInvalidCollectionPath`, as do `.../{projectID}/queries.`, `.../{projectID}/con.fig` and `.../{projectID}/queries ` (trailing space); every failing call returns an empty `defFile`

### AC: scoping-parent-need-not-exist

**Requirements:** scoped-subcollection-definitions#req:scoping-parent-not-required, scoped-subcollection-definitions#req:data-resolution, scoped-subcollection-definitions#req:validation-walks-scopes

Provisional on Open Question `subcollection-data-directory-rule`, option A.

**Given** the database of AC `datatug-tree-loads` with no `ext` records at all, and the project record file `ext/datatug/projects/p1/p1.datatug-project.json` plus a query record `ext/datatug/projects/p1/queries/q1/q1.query.json`
**When** `ResolveCollection([{ext, datatug}, {projects, p1}], "queries")` is called, whole-database validation runs, and `PlanCreateCollection("ext/datatug/projects/{projectID}/boards")` is called
**Then** resolution returns a `DirPath` whose root-relative form is `ext/datatug/projects/p1/queries` and `SchemaPath` `ext/datatug/projects/{projectID}/queries`; validation checks `p1` and `q1` against their definitions and reports no finding about a missing `datatug` record; the plan returns `ext/.collection/subcollections/datatug/projects/subcollections/{projectID}/boards/definition.yaml`; and after all three calls the set of files in the database is unchanged, in particular no `ext/datatug.*`, `ext/datatug/datatug.*` or `ext/$records/**` file exists

### AC: data-dir-independent-of-parent-records-dir

**Requirements:** scoped-subcollection-definitions#req:data-resolution, scoped-subcollection-definitions#req:record-dir-names

Provisional on Open Question `subcollection-data-directory-rule`, option A.

**Given** a root collection `orders` at `orders` whose `record_file` is `{key}.yaml` with no `records_dir`, and `orders/.collection/subcollections/{orderID}/order_details/definition.yaml`
**When** `ResolveCollection` is called for parent `{orders, o1}`, `{orders, "a/b"}`, `{orders, "v1.2"}` and `{orders, "CON"}` with name `order_details` in `ingitdb-go`, and the same keys are read through `dalgo2ingitdb`, `dalgo2ingitdb4github` and `dalgo2ingitdb4local`
**Then** every implementation resolves the root-relative data directories `orders/o1/order_details`, `orders/a%2Fb/order_details` and `orders/v1%2E2/order_details`, and fails `CON` with `invalid-record-id` without touching the filesystem, on every platform

### AC: record-dir-names-are-portable

**Requirements:** scoped-subcollection-definitions#req:record-dir-names, scoped-subcollection-definitions#req:scope-directory-names

**Given** the ids `a/b`, `a\b`, `v1.2`, `50%`, `CON`, `con`, `a:b` and `a `, and a database whose `ext` scope tree holds a concrete scope for `v1.2`
**When** `RecordDirName` is called on each, `RecordIDFromDirName` on each successful result, the database is loaded, and a record with key `v1.2` is written to `ext` through `dalgo2ingitdb`
**Then** the results are `a%2Fb`, `a%5Cb` and `v1%2E2`, the others fail with `ErrInvalidRecordID`; each successful result round-trips; the scope directory is `ext/.collection/subcollections/v1%2E2/` while a `v1.2/` scope directory fails with `invalid-scope-directory`; and the written record file is `ext/$records/v1%2E2.yaml`

### AC: listing-returns-only-own-records

**Requirements:** scoped-subcollection-definitions#req:listing-one-segment-per-key

**Given** the database of AC `datatug-tree-loads` with files `ext/datatug/projects/p1/p1.datatug-project.json`, `ext/datatug/projects/p1/queries/q1/q1.query.json`, `ext/datatug/projects/x/y/y.datatug-project.json` and `ext/sneat/projects/s1.yaml`
**When** records of `ext/datatug/projects` are listed through `dalgo2ingitdb`, `dalgo2ingitdb4github` (Git tree) and `dalgo2ingitdb4local`
**Then** each returns exactly `[p1]`

### AC: invalid-trees-rejected

**Requirements:** scoped-subcollection-definitions#req:strict-tree-reading, scoped-subcollection-definitions#req:scope-directory-names

**Given** databases whose `subcollections/` directory holds, one per database: `v1.2/x/`, `CON/x/`, `{a=b}/x/`, `{1x}/x/`, `{x/`, both `DataTug/x/` and `datatug/x/`, a regular file `notes.txt`, `{id}/x/` with no `definition.yaml`, `{id}/Bad:Name/definition.yaml`, `{id}/queries./definition.yaml`, `{id}/CON.txt/definition.yaml`, `order_details/definition.yaml` (whose name is also a valid concrete id), and, in a `.collection/` directory, a `$tmp/` directory
**When** each is loaded
**Then** they fail with `invalid-scope-directory`, `invalid-scope-directory`, `reserved-key-segment`, `invalid-scope-directory`, `invalid-scope-directory`, `scope-case-collision`, `unexpected-schema-entry`, `missing-definition`, `invalid-collection-name`, `invalid-collection-name`, `invalid-collection-name`, `legacy-subcollection-layout` and `unexpected-schema-entry` respectively; each error names its offending path and satisfies `errors.Is` with that kind's exported sentinel; and none loads with the entry skipped

### AC: ignored-entries-and-empty-scopes

**Requirements:** scoped-subcollection-definitions#req:strict-tree-reading, scoped-subcollection-definitions#req:placeholder-name-persisted

**Given** the database of AC `datatug-tree-loads` with `README.md` added in `subcollections/`, in a scope directory and in a collection directory, a `.DS_Store` at each of those levels, and an empty directory `ext/.collection/subcollections/{extID}/`
**When** it is loaded
**Then** it loads with exactly the definitions of AC `datatug-tree-loads`, and `ext`'s `PlaceholderName()` is `""`

### AC: drop-plan-removes-emptied-scope

**Requirements:** scoped-subcollection-definitions#req:definition-path-addressing, scoped-subcollection-definitions#req:placeholder-name-persisted

**Given** the database of AC `datatug-tree-loads`, where `dbdrivers/subcollections/{dbdriverID}/` holds only `dbservers/` and a `README.md`, and `environments/subcollections/{envID}/` holds `servers/` and `catalogs/`
**When** `PlanDropCollection` is called for `ext/datatug/projects/{x}/dbdrivers/{y}/dbservers` and for `ext/datatug/projects/{projectID}/environments/{envID}/servers`, and each plan is applied
**Then** the first plan lists the `dbservers` directory and its `{dbdriverID}` scope directory; the second lists only the `servers` directory; after applying both the database loads, `dbdrivers`' `PlaceholderName()` is `""` and `environments`' is still `envID`

### AC: shared-layout-uses-scope-tree

**Requirements:** scoped-subcollection-definitions#req:shared-layout-uses-same-tree

**Given** a shared-layout database with `.collections/orders/definition.yaml` and `.collections/orders/subcollections/{orderID}/order_details/definition.yaml`, and a second one with `.collections/orders/order_details/definition.yaml`
**When** both are loaded
**Then** the first loads `orders/{orderID}/order_details`, and the second fails with `unexpected-schema-entry` naming `.collections/orders/order_details`

### AC: findings-use-schema-paths

**Requirements:** scoped-subcollection-definitions#req:validation-walks-scopes, scoped-subcollection-definitions#req:validation-finding-kinds

Provisional on Open Question `subcollection-data-directory-rule`, option A.

**Given** `demo-ingitdb` moved to the new tree, with one `order_details` record given a wrong-typed value, and a scoped `ext/datatug/projects` definition declaring `min_records_count: 1` with no project records
**When** whole-database validation runs
**Then** the type finding has `Kind` `field-type-mismatch` and `CollectionID` `orders/{orderID}/order_details`; a finding with `Kind` `record-count-below-min`, `CollectionID` `ext/datatug/projects` and a `FilePath` whose root-relative form is `ext/datatug/projects` is reported; and the record counts are keyed by those schema paths

### AC: definition-path-round-trip

**Requirements:** scoped-subcollection-definitions#req:go-model, scoped-subcollection-definitions#req:scope-directory-names

**Given** the schema paths `ext/datatug/projects/{projectID}/queries`, `/ext/datatug/projects`, `ext/v1%2E2/x`, `ext/datatug` and `ext/{a=b}/x`
**When** each is parsed with `ParseDefinitionPath`, and every valid result is printed with `String()`, and each scope is converted with `DirName()` and `ParseScopeDirName`
**Then** the first three parse, and print as `ext/datatug/projects/{projectID}/queries`, `ext/datatug/projects` and `ext/v1%2E2/x`, equal to `dbschema.SchemaPath.String()` for the same paths; `ParseScopeDirName(s.DirName())` equals `s` for each scope; `ext/datatug` fails as a record path; and `ext/{a=b}/x` fails as a reserved key segment

### AC: model-migrated-in-repo

**Requirements:** scoped-subcollection-definitions#req:go-model, scoped-subcollection-definitions#req:dependents-move-together

**Given** the `ingitdb-go` module after this Feature
**When** `go build ./...` and `go test ./...` run
**Then** both pass; no code refers to `SubCollections` as a map; `ValidateForeignKeys`, `datavalidator` and `docsbuilder` walk every scope; and `validator.ReadDefinition` on the old-layout `demo-ingitdb` fails with `legacy-subcollection-layout`, while the moved `demo-ingitdb` loads

### AC: json-schema-rejects-scope-keys

**Requirements:** scoped-subcollection-definitions#req:definition-file-carries-no-scope

**Given** `ingitdb-collection.schema.json` after this Feature, and every accepted definition in `conformance/subcollection-scopes/`
**When** each accepted definition is validated against it, and so is a copy of one with a top-level `placeholder: projectID` key
**Then** every accepted definition validates, the copy fails schema validation, and the strict reader also rejects the copy

### AC: every-implementation-runs-scope-vectors

**Requirements:** scoped-subcollection-definitions#req:conformance-vectors, scoped-subcollection-definitions#req:dependents-move-together, scoped-subcollection-definitions#req:data-resolution

**Given** `conformance/subcollection-scopes/` in `ingitdb/ingitdb` with a normative `README.md`, `vectors.yaml` and `validation_vectors.yaml` covering every required case
**When** CI runs in `ingitdb-go`, `dalgo2ingitdb`, `dalgo2ingitdb4github` and `dalgo2ingitdb4local`
**Then** in each repository the vendored files carry the re-sync header, the re-sync check passes, and the repository's profiles pass (both in `ingitdb-go` and `dalgo2ingitdb`, reader in the other two); and no repository other than `ingitdb-go` defines its own subcollection-lookup walk

## Not Doing (and Why)

- **A precedence rule for overlapping scopes.** Concrete-over-placeholder precedence would make `ext/{extID}/projects` silently not apply under `datatug`. dalgo refuses overlap, and so does this Feature (REQ `scopes-must-not-overlap`).
- **Placeholder names in `definition.yaml`.** They would duplicate the directory name, or need a two-file write (REQ `scope-directory-tree`).
- **Compatibility with the name-keyed layout.** Private beta. The old layout fails loudly and the known databases move in the same wave (REQ `dependents-move-together`).
- **A migration command.** Moving directories is a one-off `git mv` for the few known databases.
- **Multi-field key scopes** (`{field=value,…}` directories). Reserved and rejected, as in dalgo.
- **Platform-specific name rules.** Portability rules apply everywhere (REQ `record-dir-names`), so no implementation relaxes them on Linux.
- **Discovering data under a missing parent record in a placeholder scope.** Placeholder-scope instances are still enumerated from existing parent records (REQ `validation-walks-scopes`). Only concrete scopes are enumerated without their parent record.
- **Foreign keys that target subcollections by schema path.** `ResolveForeignKey` keeps today's resolution. The walk now covers every scope, but no new target syntax is added.
- **The dalgo DDL surface** (`CreateCollection` options, extensions, drop flags). Owned by dal-go/dalgo `schema-subcollections-extensions` and implemented in ingitdb/dalgo2ingitdb#16.
- **Writing the vectors or the schema change here.** They are owned by `ingitdb/ingitdb` and `ingitdb/ingitdb-schema`. This Feature states what they must contain.

## Rehearse Integration

No Rehearse stubs are scaffolded: `specscore.yaml` declares no rehearse configuration. Every AC is directly executable as a Go test: loader table tests over temporary trees through `validator.ReadDefinition`, pure tests of `ParseDefinitionPath`, `CollectionAt`, `PlanCreateCollection`, `PlanDropCollection` and `ResolveCollection`, `datavalidator` runs over temporary databases, and the vendored-vector runners in each implementation. This is recorded as an explicit skip rather than an omission.

## Dependent Modules

- **`github.com/dal-go/record`** provides the grammar this Feature consumes (`EscapeID` with `{ } , =`, `UnescapeID` from dal-go/record#3, the placeholder identifier and segment classification), per dal-go/dalgo Feature `schema-subcollections-extensions` REQ `path-grammar` (dal-go/dalgo#163). `ingitdb-go` moves to that release first.
- **`ingitdb/ingitdb`** owns `conformance/subcollection-scopes/` (tracking issue to be opened when this Feature is approved), ordered after `conformance/record-layout/` (#9) as REQ `conformance-vectors` states.
- **`record-body-file`** (this repository) is amended per REQ `suffix-disjointness-follows-listing`.
- **`ingitdb/ingitdb-schema`** updates `ingitdb-collection.schema.json` per REQ `definition-file-carries-no-scope`.
- **`ingitdb/dalgo2ingitdb`** (#16):
  - creates, alters and drops through `PlanCreateCollection` and `PlanDropCollection`;
  - wraps `ErrPlaceholderNameConflict` so that `dbschema.ErrPlaceholderNameConflict` also matches;
  - maps `dbschema.SchemaPath` to `DefinitionPath`;
  - replaces `resolveScopedCollection` and `createSubCollection` with the exported functions;
  - replaces `recordKeyToFileName`, `recordKeyFromFileName` and `validateRecordPathSegment` with `RecordDirName` and `RecordIDFromDirName`;
  - moves `protected.go:454` off the map;
  - maps `invalid-collection-name` to `dbschema.ErrInvalidCollectionPath`;
  - moves `testdata/format-fixtures`;
  - vendors and runs both profiles.
- **`ingitdb/dalgo2ingitdb4github`** and **`ingitdb/dalgo2ingitdb4local`** replace their `resolveScopedCollection` and record-path code, list with one segment per `{key}` (4github `query.go` `buildKeyExtractor`), and vendor and run the reader profile.
- **`ingitdb/ingitdb-cli`** moves `subcollection_path.go` (unescaping each id segment), `materialize.go` and the TUI (`collection_screen.go`, `collection_schema_panel.go`) to scopes, showing scope directory names as path segments.
- **`ingitdb/demo-ingitdb`** moves `orders/.collection/subcollections/order_details/` to `{orderID}/order_details/`.
- **`ingitdb/ingitdb-ts`** moves its format fixtures, updates `client-fs` (`client.ts`) and `client-github`, and, after ingitdb/ingitdb-ts#104, reads scopes and runs the reader profile.
- **`ingitdb/ingitdb-ws`** updates `app/composables/useCollection.ts`.
- **`ingitdb/ingitdb-specs`** rewrites `docs/schema/subcollection.md`.
- **`sneat-co/ovdb`** (`onboarding4ovdb/profile.go`) and **`openvaultdb/openvaultdb`** (Feature `sneat-space-export`) follow the shared-layout tree and naming rules.
- **`datatug/datatug`** (`dalgo-project-store`) creates the DataTug tree through DALgo DDL. Under option A its `ext` root collection needs no `records_dir`; under option B it must declare `records_dir: '.'`, and dal-go/dalgo AC `dalgo2ingitdb-creates-datatug-queries` must create `ext` with that extension.

## Open Questions

### Q: subcollection-data-directory-rule

Where does a subcollection instance's data live relative to its parent collection's `record_file.records_dir`?

Today the two rules disagree (see Problem). With the validator's rule, the root `ext` collection's record file decides where every extension's data lives: dal-go/dalgo AC `dalgo2ingitdb-creates-datatug-queries` creates `ext` with no extension, so `dalgo2ingitdb` writes the default `{key}.yaml` and all DataTug data would sit under `ext/$records/datatug/…`. Whichever extension creates `ext` first fixes the layout for all others, and a later `records_dir` change on `ext` moves all descendant data.

- **A (recommended):** `dalgo2ingitdb`'s rule. The data directory is `<parent instance data dir>/<RecordDirName(parent id)>/<name>`, independent of the parent's `records_dir`. DataTug's layout is unchanged, `ext` needs no extension, and no extension can move another's data. The cost is that `demo-ingitdb`'s `order_details` data moves out of `orders/$records/`.
- **B:** the validator's rule, `<parent instance data dir>/<parent RecordsBasePath()>/<RecordDirName(parent id)>/<name>`. `ext` must then declare `records_dir: '.'`, and dal-go/dalgo AC `dalgo2ingitdb-creates-datatug-queries` and `datatug/datatug` `dalgo-project-store` are bound to that.

REQ `data-resolution`, REQ `validation-walks-scopes` and ACs `scoping-parent-need-not-exist`, `data-dir-independent-of-parent-records-dir` and `findings-use-schema-paths` are written for A and are provisional until this is decided.

Decisions this Feature relies on, already made by the founder on 2026-09-17:

- placeholder-name enforcement is mandatory, with no optional path (dal-go/dalgo `schema-subcollections-extensions`, decision "A");
- private beta, so there are no compatibility constraints, and fixes are made at source;
- the contract is YAML conformance vectors plus JSON Schema.

---
*This document follows the https://specscore.md/feature-specification*
