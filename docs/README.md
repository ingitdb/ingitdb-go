# inGitDB Documentation

inGitDB turns a Git repository into a schema-validated, queryable database where every record is
a plain YAML or JSON file. There is no server to run and no binary format to decode — the data
lives in files you can read, edit, and diff with any text editor or git client. Git itself
provides the history, branching, and merge story for free. If that sounds like the right
trade-off for your project, read on — and consider contributing.

## Index

### User docs

- [CLI](CLI.md) — command reference: flags, subcommands, exit codes
- [Configuration](configuration/README.md) — `.ingitdb.yaml` and collection config files
- [Features](features/README.md) — what inGitDB can do (and what is coming)

### Developer docs

- [Architecture](ARCHITECTURE.md) — data model, package map, key design decisions
- [Components](components/README.md) — building blocks: Scanner, Validator, Views Builder, Watcher, and more
- [Contributing](CONTRIBUTING.md) — how to open issues and submit pull requests
- [Coding Standards](CODING_STANDARDS.md) — conventions every contributor should follow
- [Guidelines](GUIDELINES.md) — broader design and process guidelines
- [CI](CI.md) — continuous integration setup and checks

### Project docs

- [Roadmap](ROADMAP.md) — nine delivery phases from Validator to GraphQL
- [Backlog](BACKLOG.md) — concrete tasks with acceptance criteria, ordered by dependency
- [Competitors](COMPETITORS.md) — honest feature comparison with related projects

---

## Documentation Map

A one-line description of each document, grouped by purpose.

### User docs

| Document | What it covers |
|---|---|
| [CLI](CLI.md) | Every subcommand, flag, and exit code for the `ingitdb` binary |
| [Configuration](configuration/README.md) | Root DB config (`.ingitdb.yaml`) and per-collection schema files |
| [Features](features/README.md) | Full feature list with status (WIP / pending) and links to detail pages |

### Developer docs

| Document | What it covers |
|---|---|
| [Architecture](ARCHITECTURE.md) | Filesystem layout, package responsibilities, and key design decisions |
| [Components](components/README.md) | Scanner, Validator, Views Builder, Watcher, Triggers, MCP Server, and more |
| [Contributing](CONTRIBUTING.md) | Bug reports, feature suggestions, pull request workflow |
| [Coding Standards](CODING_STANDARDS.md) | Go conventions enforced in this codebase |
| [Guidelines](GUIDELINES.md) | Design and process guidelines that apply project-wide |
| [CI](CI.md) | What the CI pipeline runs and how to satisfy it locally |

### Project docs

| Document | What it covers |
|---|---|
| [Roadmap](ROADMAP.md) | Nine delivery phases from schema validation to GraphQL and migration tooling |
| [Backlog](BACKLOG.md) | Actionable tasks with acceptance criteria and implementation notes |
| [Competitors](COMPETITORS.md) | Feature matrix comparing inGitDB to related tools |

---

## Architecture at a Glance

The diagram below shows the write path: a CLI command loads the database definition, validates
records, rebuilds materialized views, and leaves every change as a normal git commit.

```mermaid
flowchart LR
    A([ingitdb CLI]) --> B[validator.ReadDefinition]
    B --> C{Schema valid?}
    C -- yes --> D[DataValidator\nwalks $records/]
    C -- no --> E([Exit 1 — schema error])
    D --> F{Data valid?}
    F -- no --> G([Exit 1 — data errors])
    F -- yes --> H[ViewsBuilder\ngenerates $views/]
    H --> I[(Git repository\non disk)]
```

The same Git repository is also exposed to AI agents via an MCP server (Phase 6), and to any
Go program through the [DALgo](https://github.com/dal-go/dalgo) abstraction layer implemented
in `pkg/dalgo2ingitdb`.

---

## Get Involved

The best way to understand inGitDB is to point it at a directory of YAML files and run
`ingitdb validate`. From there, check the [Roadmap](ROADMAP.md) to see what is being built and
the [Backlog](BACKLOG.md) for tasks that are ready to be picked up.

To contribute:

1. Read [CONTRIBUTING.md](CONTRIBUTING.md) for the pull-request workflow.
2. Read [CODING_STANDARDS.md](CODING_STANDARDS.md) before writing any Go.
3. Open or comment on an [issue](https://github.com/ingitdb/ingitdb-go/issues) to discuss your
   idea before investing time in a large change.

Bug reports, documentation improvements, and questions are all welcome. The project is small
enough that every contribution makes a visible difference.
