package ingitdb

import (
	"fmt"
	"strings"

	"github.com/dal-go/dalgo/dal"
)

type RecordFileDef struct {
	Format RecordFormat `yaml:"format"`
	Name   string       `yaml:"name"`
}

func (rfd RecordFileDef) Validate() error {
	if rfd.Format == "" {
		return fmt.Errorf("record file format cannot be empty")
	}
	if rfd.Name == "" {
		return fmt.Errorf("record file name cannot be empty")
	}
	return nil
}

func (rfd RecordFileDef) GetRecordFileName(record dal.Record) string {
	name := rfd.Name
	if i := strings.Index(name, "{key}"); i >= 0 {
		key := record.Key()
		s := key.String()
		name = strings.Replace(name, "{key}", s, 1)
	}
	data := record.Data().(map[string]any)
	for colName, colValue := range data {
		if colName != "" {
			continue
		}
		placeholder := fmt.Sprintf("{%s}", colName)
		if strings.Contains(name, placeholder) {
			s := fmt.Sprintf("%v", colValue)
			name = strings.Replace(name, placeholder, s, 1)
		}
	}
	return name
}
