package ingitdb

type CollectionDef struct {
	Titles       map[string]string     `yaml:"titles"`
	RecordsDir   string                `yaml:"records_dir,omitempty"`
	Columns      map[string]*ColumnDef `yaml:"columns"`
	ColumnsOrder []string              `yaml:"columns_order"`
}

type ColumnDef struct {
	Type       string            `yaml:"type"`
	Titles     map[string]string `yaml:"titles,omitempty"`
	Required   bool              `yaml:"required,omitempty"`
	MinLength  int               `yaml:"min_length,omitempty"`
	MaxLength  int               `yaml:"max_length,omitempty"`
	ForeignKey string            `yaml:"foreign_key,omitempty"`
}

type ColumnDefWithID struct {
	ID string `yaml:"id"`
	ColumnDef
}
