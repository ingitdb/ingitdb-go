# Collection Definition File (`.ingitdb-collection.yaml`)

Each collection directory contains an `.ingitdb-collection.yaml` file that describes
how records are stored and what columns (fields) they have.

## Top-level fields

| Field           | Type                        | Description |
|-----------------|-----------------------------|-------------|
| `titles`        | `map[locale]string`         | Human-readable collection name, keyed by locale |
| `record_file`   | `RecordFileDef`             | How records are stored on disk (required) |
| `data_dir`      | `string`                    | Custom data directory (optional) |
| `columns`       | `map[string]ColumnDef`      | Column (field) definitions |
| `columns_order` | `[]string`                  | Display order for columns |
| `default_view`  | `string`                    | Default view name (optional) |

---

## `record_file`

Controls how records are physically stored.

| Field    | Type           | Description |
|----------|----------------|-------------|
| `name`   | `string`       | File name or pattern. May contain `{key}` and `{fieldName}` placeholders. |
| `format` | `string`       | File format: `json`, `yaml`, or `yml` |
| `type`   | `RecordType`   | Layout of records within the file (see below) |

### `record_file.type` values

#### `map[string]any` — one record per file

Each record is a separate file. The file name typically contains `{key}`.

```yaml
record_file:
  name: "$records/{key}.json"
  type: "map[string]any"
  format: json
```

File `$records/ireland.json`:
```json
{
  "title": "Ireland",
  "population": 5000000
}
```

#### `[]map[string]any` — list of records in one file

All records are stored as an array (or YAML list) in a single file.

```yaml
record_file:
  name: "statuses.yaml"
  type: "[]map[string]any"
  format: yaml
```

File `statuses.yaml`:
```yaml
- id: new
  title: New
- id: in_progress
  title: In Progress
```

#### `map[string]map[string]any` — dictionary of records

All records are stored in one file as a dictionary where the top-level key is
the record ID.

```yaml
record_file:
  name: "statuses.yaml"
  type: "map[string]map[string]any"
  format: yaml
```

File `statuses.yaml`:
```yaml
new:
  titles: {en: New, ru: Новая}
in_progress:
  titles: {en: In Progress, ru: В работе}
```

#### `map[id]map[field]any` — all records in one file keyed by ID

Similar to `map[string]map[string]any` but used when the file name is static
(no `{key}` placeholder). Top-level keys are record IDs, second-level keys are
field names.

This is the preferred type when you want a single, human-readable file for a
small reference collection (e.g. tags, categories).

```yaml
record_file:
  name: "tags.json"
  type: "map[id]map[field]any"
  format: json
```

File `tags.json`:
```json
{
  "work": {
    "titles": {"en": "Work", "ru": "Работа", "es": "Trabajo"}
  },
  "home": {
    "titles": {"en": "Home", "ru": "Дом", "es": "Casa"}
  }
}
```

---

## `columns`

Each entry under `columns` is a **ColumnDef** keyed by the field name.

| Field         | Type                  | Description |
|---------------|-----------------------|-------------|
| `type`        | `ColumnType`          | Field type (required) |
| `required`    | `bool`                | Whether the field must be present |
| `title`       | `string`              | Display label |
| `titles`      | `map[locale]string`   | Localized display labels |
| `length`      | `int`                 | Exact length constraint |
| `min_length`  | `int`                 | Minimum length |
| `max_length`  | `int`                 | Maximum length |
| `foreign_key` | `string`              | Reference to another collection |
| `locale`      | `string`              | Locale shortcut (see below) |

### Column types

| Type                | Description |
|---------------------|-------------|
| `string`            | Plain text |
| `int`               | Integer number |
| `float`             | Floating-point number |
| `bool`              | Boolean flag |
| `date`              | Date (ISO 8601) |
| `time`              | Time |
| `datetime`          | Date and time |
| `any`               | Untyped value |
| `map[locale]string` | Localized string map (see Locale columns below) |

---

## Locale columns and the `locale` field

A column with `locale` set is a **locale shortcut** — a convenient single-value
alias for one locale within a paired `map[locale]string` column.

The pairing rule is: the pair column name is `<shortcut_column_name> + "s"`.
For example, column `title` (with `locale: en`) is paired with column `titles`
(of type `map[locale]string`).

```yaml
columns:
  title:
    type: string
    locale: en          # shortcut for titles["en"]
  titles:
    type: "map[locale]string"
    required: true
```

### Read behaviour

When a record is read from the file, the locale value is **extracted** from the
pair column and exposed via the shortcut column. The locale key is **removed**
from the pair column to avoid duplication. Example:

File data:
```json
{"titles": {"en": "Work", "ru": "Работа"}}
```

Data presented to the application:
```json
{"title": "Work", "titles": {"ru": "Работа"}}
```

The `en` entry is not stored in `titles` when `title` already carries it.

### Write behaviour

When a record is written, the shortcut column value is **merged** into the pair
column under the configured locale key, and the shortcut column itself is
**not stored** in the file. Example:

Data from the application:
```json
{"title": "Work", "titles": {"ru": "Работа"}}
```

Data written to the file:
```json
{"titles": {"en": "Work", "ru": "Работа"}}
```

This keeps the on-disk representation clean and avoids redundant storage of the
same value in two places.

---

## Full example

`todo/tags/.ingitdb-collection.yaml`:

```yaml
titles:
  en: Tags
  ru: Теги
record_file:
  name: "tags.json"
  type: "map[id]map[field]any"
  format: json
columns:
  title:
    type: string
    locale: en
  titles:
    type: "map[locale]string"
    required: true
```

`todo/tags/tags.json`:

```json
{
  "work":     {"titles": {"en": "Work",     "ru": "Работа"}},
  "home":     {"titles": {"en": "Home",     "ru": "Дом"}},
  "personal": {"titles": {"en": "Personal", "ru": "Личное"}}
}
```
