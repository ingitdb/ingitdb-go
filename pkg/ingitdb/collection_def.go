package ingitdb

import "fmt"

const CollectionDefFileName = ".ingitdb-collection.yaml"

type CollectionDef struct {
	ID           string                `json:"-"`
	Titles       map[string]string     `yaml:"titles,omitempty"`
	RecordFile   *RecordFileDef        `yaml:"record_file"`
	DataDir      string                `yaml:"data_dir,omitempty"`
	Columns      map[string]*ColumnDef `yaml:"columns"`
	ColumnsOrder []string              `yaml:"columns_order,omitempty"`
	DefaultView  string                `yaml:"default_view,omitempty"`
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
	if v.RecordFile == nil {
		return fmt.Errorf("missing 'record_file' in collection definition")
	}
	if err := v.RecordFile.Validate(); err != nil {
		return fmt.Errorf("invalid record_file definition: %w", err)
	}
	return nil
}
