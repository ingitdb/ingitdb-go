package ingitdb

import "errors"

type ColumnDef struct {
	Type       ColumnType        `yaml:"type"`
	Title      string            `yaml:"title,omitempty"`
	Titles     map[string]string `yaml:"titles,omitempty"`
	ValueTitle string            `yaml:"valueTitle,omitempty"`
	Required   bool              `yaml:"required,omitempty"`
	Length     int               `yaml:"length,omitempty"`
	MinLength  int               `yaml:"min_length,omitempty"`
	MaxLength  int               `yaml:"max_length,omitempty"`
	ForeignKey string            `yaml:"foreign_key,omitempty"`
	// MinValue and MaxValue constrain a numeric column's value inclusively.
	//
	// Pointer-typed on purpose: a declared zero must be distinguishable from
	// "not declared". `min_value: 0` is not hypothetical — geo-ingitdb declares
	// exactly that on population and area, and with a plain float64 the natural
	// "!= 0" guard would read the declared bound as unset and silently enforce
	// nothing, which is the failure this whole feature exists to end.
	//
	// float64 rather than *int so `min_value: 0.5` is expressible on a float
	// column; a fractional bound on an int column is a definition-load error.
	MinValue *float64 `yaml:"min_value,omitempty"`
	MaxValue *float64 `yaml:"max_value,omitempty"`
	// Enum, when non-empty, restricts the column's value to one of the listed
	// members. A record value outside the list is a validation error naming the
	// field, the offending value, and the permitted set.
	//
	// Declaring an empty enum, duplicate members, or a member not assignable to
	// the column's declared Type is a definition-load error: each is a mistake
	// that would otherwise silently constrain nothing or nothing at all.
	Enum []any `yaml:"enum,omitempty"`
	// Locale pairs this column with a map[locale]string column named <this_column_name>+"s".
	// For example, column "title" with locale "en" is paired with column "titles".
	// When reading, the locale value is extracted from the pair column and exposed as this column.
	// When writing, this column is stored as-is in the file; if the pair column contains an entry
	// for the primary locale key, that entry is promoted here and removed from the pair column.
	Locale string `yaml:"locale,omitempty"`
	// Format is an optional, free-form hint about the column's logical content
	// type. Well-known values include `markdown`, `html`, `json`, `jsonl`,
	// `yaml`, `uri`, `email`, `pdf`. inGitDB does not validate the value;
	// tooling may use it to choose a renderer or preview strategy.
	Format string `yaml:"format,omitempty"`
	// Formula declares this column as a computed (virtual) column. When set,
	// it must be a single Starlark expression that references only stored
	// (non-computed) sibling fields. Computed columns support only the
	// string, int, float, bool, and any declared types.
	//
	// Note: Starlark's `/` operator is float division, so for an int column
	// use integer division `//` (e.g. `total // count`) — `a / b` yields a
	// float and fails coercion into an int column unless the result is whole.
	Formula string `yaml:"formula,omitempty"`
}

func (v *ColumnDef) Validate() error {
	if err := ValidateColumnType(v.Type); err != nil {
		if errors.Is(err, errMissingRequiredField) {
			return errors.New("missing 'type' in column definition")
		}
		return err
	}
	return nil
}

type ColumnDefWithID struct {
	ID string `yaml:"id"`
	ColumnDef
}
