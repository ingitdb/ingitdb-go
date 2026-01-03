package ingitdb

type ViewDef struct {
	ID      string         `yaml:"-"`
	Titles  map[int]string `yaml:"titles"`
	OrderBy string         `yaml:"order_by"`
	Formats []string       `yaml:"formats"`
	Columns []string       `yaml:"columns"`
}
