package ingitdb

// specscore: feature/column-validation

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

type ColumnDef struct {
	Type       ColumnType        `yaml:"type"`
	Title      string            `yaml:"title,omitempty"`
	Titles     map[string]string `yaml:"titles,omitempty"`
	ValueTitle string            `yaml:"valueTitle,omitempty"`
	Required   bool              `yaml:"required,omitempty"`
	// RequiredWhen makes the column required only when the expression evaluates
	// to Starlark True. It is a single Starlark expression over the record's
	// stored sibling fields, and reuses Formula's parser and evaluator rather
	// than introducing a second expression dialect: one grammar, one evaluator,
	// one set of edge cases.
	//
	// Siblings the record omits bind as None, so `state != "absent"` is a valid
	// question to ask of a record that has no state. Computed siblings are not
	// visible; referencing one is a definition-load error, as is an undeclared
	// identifier.
	//
	// The expression MUST evaluate to True or False. Anything else is an error
	// rather than a truthiness coercion, so `required_when: 'name'` does not
	// silently mean "required when name is non-empty".
	//
	// Declaring both Required and RequiredWhen on one column is a
	// definition-load error: the two would otherwise contradict each other with
	// no defined precedence.
	RequiredWhen string `yaml:"required_when,omitempty"`
	// Length, MinLength and MaxLength constrain a value's length: character
	// count (Unicode code points) for a string, element count for a list,
	// entry count for a map.
	//
	// Pointer-typed on purpose, for the same reason as MinValue/MaxValue: a
	// declared zero must be distinguishable from "not declared". min_length: 0
	// is meaningless, but max_length: 0 (forbid any content) is not — and with
	// a plain int the natural "!= 0" guard reads it as unset and enforces
	// nothing. It also makes "a length constraint declared on a bool column" a
	// detectable definition-load error rather than an invisible no-op.
	Length     *int   `yaml:"length,omitempty"`
	MinLength  *int   `yaml:"min_length,omitempty"`
	MaxLength  *int   `yaml:"max_length,omitempty"`
	ForeignKey string `yaml:"foreign_key,omitempty"`
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

	// Columns, Values and Items declare the shape of a structured value with
	// the same column definition, recursively. All three are optional; a
	// column without them is validated exactly as before (its map or list
	// content unchecked).
	//
	//   - Columns: the fields of a nested object. Allowed on `any` and map
	//     types other than map[locale]string. Every declared field is checked,
	//     and a field the object holds but Columns does not declare is an
	//     undeclared-field finding, as at the top level.
	//   - Values: every value of a map (`map[K]V`), e.g. a map keyed by id
	//     whose values are objects.
	//   - Items: every element of a list (`[]T`).
	//
	// A nested definition supports type, required, enum, length, value-range
	// and its own Columns/Values/Items. formula, required_when, foreign_key and
	// locale are top-level only and are a definition-load error when nested.
	Columns map[string]*ColumnDef `yaml:"columns,omitempty"`
	Values  *ColumnDef            `yaml:"values,omitempty"`
	Items   *ColumnDef            `yaml:"items,omitempty"`
}

func (v *ColumnDef) Validate() error {
	if err := ValidateColumnType(v.Type); err != nil {
		if errors.Is(err, errMissingRequiredField) {
			return errors.New("missing 'type' in column definition")
		}
		return err
	}
	return v.validateShape()
}

// validateShape checks Columns, Values and Items against the column type and
// validates each nested definition recursively.
func (v *ColumnDef) validateShape() error {
	isMap := strings.HasPrefix(string(v.Type), "map[") && v.Type != ColumnTypeL10N
	elemType, isList := ListElementType(v.Type)
	if len(v.Columns) > 0 && !isMap && v.Type != ColumnTypeAny {
		return fmt.Errorf("'columns' requires a map type or any, got %s", v.Type)
	}
	if v.Values != nil && !isMap {
		return fmt.Errorf("'values' requires a map type, got %s", v.Type)
	}
	if v.Values != nil && v.Values.Type != "" {
		if valueType := string(v.Type)[strings.Index(string(v.Type), "]")+1:]; valueType != "" && valueType != string(ColumnTypeAny) && valueType != string(v.Values.Type) {
			return fmt.Errorf("'values' has type %s, but %s holds %s values", v.Values.Type, v.Type, valueType)
		}
	}
	if v.Items != nil && !isList {
		return fmt.Errorf("'items' requires a list type, got %s", v.Type)
	}
	if v.Items != nil && v.Items.Type != "" && elemType != ColumnTypeAny && elemType != v.Items.Type {
		return fmt.Errorf("'items' has type %s, but %s holds %s elements", v.Items.Type, v.Type, elemType)
	}
	for _, name := range slices.Sorted(maps.Keys(v.Columns)) {
		if err := v.Columns[name].validateNested(); err != nil {
			return fmt.Errorf("invalid nested column '%s': %w", name, err)
		}
	}
	if v.Values != nil {
		if err := v.Values.validateNested(); err != nil {
			return fmt.Errorf("invalid 'values': %w", err)
		}
	}
	if v.Items != nil {
		if err := v.Items.validateNested(); err != nil {
			return fmt.Errorf("invalid 'items': %w", err)
		}
	}
	return nil
}

// validateNested validates a definition used inside Columns, Values or Items.
func (v *ColumnDef) validateNested() error {
	if v == nil {
		return errors.New("missing definition")
	}
	switch {
	case v.Formula != "":
		return errors.New("'formula' is not supported in a nested column")
	case v.RequiredWhen != "":
		return errors.New("'required_when' is not supported in a nested column")
	case v.ForeignKey != "":
		return errors.New("'foreign_key' is not supported in a nested column")
	case v.Locale != "":
		return errors.New("'locale' is not supported in a nested column")
	}
	return v.Validate()
}

type ColumnDefWithID struct {
	ID string `yaml:"id"`
	ColumnDef
}
