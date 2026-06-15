package ingitdb

import (
	"encoding/csv"
	"io"

	"github.com/ingr-io/ingr-go/ingr"
	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// csvWriter captures the encoding/csv.Writer methods used by
// encodeCSVForCollection. *csv.Writer satisfies it.
type csvWriter interface {
	Write(record []string) error
	Flush()
	Error() error
}

var (
	yamlMarshal  = yaml.Marshal
	tomlMarshal  = toml.Marshal
	newCSVWriter = func(w io.Writer) csvWriter { return csv.NewWriter(w) }
	newRecordsWriter = func(w io.Writer) ingr.RecordsWriter { return ingr.NewRecordsWriter(w) }
)
