package ingitdb

import "fmt"

const CollectionDefFileName = ".ingitdb-collection.yaml"

type CollectionDef struct {
	ID           string                `json:"-"`
	Titles       map[string]string     `yaml:"titles,omitempty"`
	DataFormat   string                `yaml:"data_format,omitempty"`
	DataDir      string                `yaml:"data_dir,omitempty"`
	Columns      map[string]*ColumnDef `yaml:"columns"`
	ColumnsOrder []string              `yaml:"columns_order,omitempty"`
}

func (v *CollectionDef) Validate() error {
	if len(v.Columns) == 0 {
		return fmt.Errorf("missing 'columns' in collection definition")
	}
	for id, col := range v.Columns {
		if err := col.Validate(); err != nil {
			return fmt.Errorf("invalid column '%s': %w", id, err)
		}
	}
	for i, colName := range v.ColumnsOrder {
		if _, ok := v.Columns[colName]; !ok {
			return fmt.Errorf("columns_order[%d] references unspecified column: %s", i, colName)
		}
		for j, prevCol := range v.ColumnsOrder[:i] {
			if prevCol == colName {
				return fmt.Errorf("duplicate value in columns_order at indexes %d and %d: %s", j, i, colName)
			}
		}
	}
	return nil
}

type ColumnDef struct {
	Type       string            `yaml:"type"`
	Title      string            `yaml:"title,omitempty"`
	Titles     map[string]string `yaml:"titles,omitempty"`
	ValueTitle string            `yaml:"valueTitle,omitempty"`
	Required   bool              `yaml:"required,omitempty"`
	Length     int               `yaml:"length,omitempty"`
	MinLength  int               `yaml:"min_length,omitempty"`
	MaxLength  int               `yaml:"max_length,omitempty"`
	ForeignKey string            `yaml:"foreign_key,omitempty"`
}

func (v *ColumnDef) Validate() error {
	if v.Type == "" {
		return fmt.Errorf("missing 'type' in column definition")
	}
	return nil
}

type ColumnDefWithID struct {
	ID string `yaml:"id"`
	ColumnDef
}
