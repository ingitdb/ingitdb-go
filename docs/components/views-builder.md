# 📘 inGitDB Views Builder

Responsible for creating and updating materialized views. The views builder runs as part of the `ingitdb validate` command, after successful validation.

## 📂 View definition files

Each view is defined in a file named `.ingitdb-view.<name>.yaml` in the collection directory. The `<name>` is the view's name pattern and becomes the name of the output directory under `$views/`.

**Field references in the name pattern** use `{field}` syntax. The field value from each record is substituted to determine which output partition file the record belongs to.

Example: `.ingitdb-view.status_{status}.yaml` → output in `$views/status_{status}/`, one file per distinct status value (e.g. `status_in_progress.md`, `status_done.md`).

## 📂 View definition format

```yaml
# 📘 titles: i18n display names for the view.
# {field} placeholders are substituted with the partition value.
titles:
  en: "Status: {status}"
  fr: "Statut: {status}"

# 📘 order_by: field name to sort by, followed by asc or desc.
# 📘 Use $last_modified to sort by the record file's modification time.
order_by: $last_modified desc

# 📘 formats: output file formats to generate. Currently supported: md
formats:
  - md

# 📘 columns: ordered list of column IDs to include in the output.
# 📘 Must match column IDs defined in .ingitdb-collection.yaml.
columns:
  - title
  - status

# 📘 top: limit output to the first N records after sorting. 0 (default) means all records.
top: 0
```

All fields are optional except where a default is not meaningful (e.g. `columns` defaults to all columns in `columns_order` if omitted).

## 📂 Output

For each view, the builder creates one output file per distinct partition value under `$views/<view-name>/`:

```
$views/
└── status_{status}/
    ├── status_in_progress.md
    ├── status_done.md
    └── status_new.md
```

Output files for partition values that no longer exist in the data are deleted.

## 📂 README builder

`README.md` files for collections are a built-in specialization of materialized views. See `docs/components/readme-builders/README.md`.
