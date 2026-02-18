package dalgo2ghingitdb

import (
	"encoding/json"
	"fmt"
	"maps"
	"path"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"gopkg.in/yaml.v3"
)

func resolveRecordPath(colDef *ingitdb.CollectionDef, recordKey string) string {
	recordName := strings.ReplaceAll(colDef.RecordFile.Name, "{key}", recordKey)
	recordPath := path.Join(colDef.DirPath, recordName)
	return path.Clean(recordPath)
}

func parseRecordContent(content []byte, format ingitdb.RecordFormat) (map[string]any, error) {
	var data map[string]any
	switch format {
	case "yaml", "yml":
		err := yaml.Unmarshal(content, &data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse YAML record: %w", err)
		}
	case "json":
		err := json.Unmarshal(content, &data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse JSON record: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported record format %q", format)
	}
	return data, nil
}

func parseMapOfIDRecordsContent(content []byte, format ingitdb.RecordFormat) (map[string]map[string]any, error) {
	raw, err := parseRecordContent(content, format)
	if err != nil {
		return nil, err
	}
	result := make(map[string]map[string]any, len(raw))
	for id, value := range raw {
		recordFields, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("record %q is not a map", id)
		}
		result[id] = recordFields
	}
	return result, nil
}

func applyLocaleToRead(data map[string]any, cols map[string]*ingitdb.ColumnDef) map[string]any {
	if len(cols) == 0 {
		return data
	}
	result := maps.Clone(data)
	for colName, colDef := range cols {
		if colDef.Locale == "" {
			continue
		}
		pairFieldName := colName + "s"
		pairValue, ok := result[pairFieldName]
		if !ok {
			continue
		}
		pairMap, ok := pairValue.(map[string]any)
		if !ok {
			continue
		}
		localeValue, exists := pairMap[colDef.Locale]
		if !exists {
			continue
		}
		result[colName] = localeValue
		newPairMap := maps.Clone(pairMap)
		delete(newPairMap, colDef.Locale)
		result[pairFieldName] = newPairMap
	}
	return result
}
