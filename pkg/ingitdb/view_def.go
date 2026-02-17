package ingitdb

type ViewDef struct {
	ID      string            `yaml:"-"`
	Titles  map[string]string `yaml:"titles,omitempty"`
	OrderBy string            `yaml:"order_by,omitempty"`
	Formats []string          `yaml:"formats,omitempty"`
	Columns []string          `yaml:"columns,omitempty"`
	// How many records to include; 0 means all
	Top int `yaml:"top,omitempty"`
}
