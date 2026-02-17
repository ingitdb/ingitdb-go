package ingitdb

type ViewDef struct {
	ID      string         `yaml:"-"`
	Titles  map[int]string `yaml:"titles"`
	OrderBy string         `yaml:"order_by"`
	Formats []string       `yaml:"formats"`
	Select  []string       `yaml:"select"`
	// How many records to get
	Top      int    `yaml:"top,omitempty"`
	FileName string `yaml:"file_name"`
}
