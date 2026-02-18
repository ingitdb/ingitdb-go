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
	// Locale pairs this column with a map[locale]string column named <this_column_name>+"s".
	// For example, column "title" with locale "en" is paired with column "titles".
	// When reading, the locale value is extracted from the pair column and exposed as this column.
	// When writing, this column is stored as-is in the file; if the pair column contains an entry
	// for the primary locale key, that entry is promoted here and removed from the pair column.
	Locale string `yaml:"locale,omitempty"`
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
