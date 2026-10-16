# Package `ingitdb`

inGitDB schema & config types.

## DataTug project-content integration

[DataTug](https://datatug.app) is exploring inGitDB for project definitions and version-controlled content; integration work is ongoing. This repository defines inGitDB’s Go schema and configuration types. Read [how DataTug’s development tools were built](https://datatug.app/en/under-the-hood/tools-we-built/).

## Schema types

- type [Definition](definition.go) — top-level schema definition; holds all collection definitions
    - type [CollectionDef](collection_def.go) — schema for one collection
        - type [RecordFileDef](record_file_def.go) — describes the record file format and naming pattern
        - type [ColumnDef](column_def.go) — schema for a single field; see [ColumnType](column_type.go)
    - type [ViewDef](view_def.go) — materialized view definition (ordering, columns, format, top-N limit)

## Configuration types

- package [config](config)
    - type [RootConfig](config/root_config.go) — parsed from `.ingitdb.yaml`; maps collection keys to paths
