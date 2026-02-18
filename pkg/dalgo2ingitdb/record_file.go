package dalgo2ingitdb

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/dal-go/dalgo/dal"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"gopkg.in/yaml.v3"
)

// CollectionForKey finds the collection and record key for a given ID string.
// id format: "collection/path/recordKey" where the collection part uses "/" as separator.
// Collection IDs in the definition use "." as separator, which is normalized to "/" for matching.
// The longest matching collection prefix wins.
func CollectionForKey(def *ingitdb.Definition, id string) (*ingitdb.CollectionDef, string, error) {
	var bestColDef *ingitdb.CollectionDef
	var bestKey string
	var bestLen int

	for colID, colDef := range def.Collections {
		normalizedColID := strings.ReplaceAll(colID, ".", "/")
		prefix := normalizedColID + "/"
		if !strings.HasPrefix(id, prefix) {
			continue
		}
		if len(normalizedColID) <= bestLen {
			continue
		}
		bestLen = len(normalizedColID)
		bestColDef = colDef
		bestKey = id[len(prefix):]
	}

	if bestColDef == nil {
		return nil, "", fmt.Errorf("collection not found for ID %q", id)
	}
	if bestKey == "" {
		return nil, "", fmt.Errorf("no record key in ID %q", id)
	}
	return bestColDef, bestKey, nil
}

// resolveRecordPath replaces all {key} occurrences in the record file name template and joins with the collection dir.
func resolveRecordPath(colDef *ingitdb.CollectionDef, recordKey string) string {
	name := strings.ReplaceAll(colDef.RecordFile.Name, "{key}", recordKey)
	return filepath.Join(colDef.DirPath, name)
}

// readRecordFromFile reads a YAML or JSON file and returns its content as a map.
// Returns (nil, false, nil) if the file does not exist.
func readRecordFromFile(path string, format ingitdb.RecordFormat) (map[string]any, bool, error) {
	fileContent, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	var data map[string]any
	switch format {
	case "yaml", "yml":
		if err = yaml.Unmarshal(fileContent, &data); err != nil {
			return nil, false, fmt.Errorf("failed to parse YAML file %s: %w", path, err)
		}
	case "json":
		if err = yaml.Unmarshal(fileContent, &data); err != nil {
			return nil, false, fmt.Errorf("failed to parse JSON file %s: %w", path, err)
		}
	default:
		return nil, false, fmt.Errorf("unsupported record format %q", format)
	}
	return data, true, nil
}

// writeRecordToFile marshals data to the specified format and writes it to path.
// Intermediate directories are created as needed.
func writeRecordToFile(path string, format ingitdb.RecordFormat, data map[string]any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	var (
		content []byte
		err     error
	)
	switch format {
	case "yaml", "yml":
		content, err = yaml.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal data as YAML: %w", err)
		}
	case "json":
		content, err = json.MarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal data as JSON: %w", err)
		}
		content = append(content, '\n')
	default:
		return fmt.Errorf("unsupported record format %q", format)
	}
	if err = os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}
	return nil
}

// deleteRecordFile removes a record file. Returns dal.ErrRecordNotFound if it does not exist.
func deleteRecordFile(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return dal.ErrRecordNotFound
	}
	return err
}

// readMapOfIDRecordsFile reads a file whose top-level keys are record IDs and whose
// values are field maps (map[id]map[field]any layout).
// Returns (nil, false, nil) if the file does not exist.
func readMapOfIDRecordsFile(path string, format ingitdb.RecordFormat) (map[string]map[string]any, bool, error) {
	raw, found, err := readRecordFromFile(path, format)
	if err != nil || !found {
		return nil, found, err
	}
	result := make(map[string]map[string]any, len(raw))
	for id, val := range raw {
		fields, ok := val.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("record %q in %s is not a map", id, path)
		}
		result[id] = fields
	}
	return result, true, nil
}

// writeMapOfIDRecordsFile writes a map[id]map[field]any dataset back to a file.
func writeMapOfIDRecordsFile(path string, format ingitdb.RecordFormat, data map[string]map[string]any) error {
	raw := make(map[string]any, len(data))
	for id, fields := range data {
		raw[id] = fields
	}
	return writeRecordToFile(path, format, raw)
}

// applyLocaleToRead transforms record data from file representation to application representation.
// For each column that has a Locale value set (e.g. column "title" with locale "en"),
// the paired map column (e.g. "titles") is inspected: the locale entry is extracted and
// exposed as the shortcut column ("title"), and that locale key is removed from the pair map
// to avoid duplication. The caller receives e.g. {"title": "Work", "titles": {"ru": "Работа"}}.
func applyLocaleToRead(data map[string]any, cols map[string]*ingitdb.ColumnDef) map[string]any {
	if len(cols) == 0 {
		return data
	}
	result := maps.Clone(data)
	for colName, colDef := range cols {
		if colDef.Locale == "" {
			continue
		}
		pairField := colName + "s"
		pairVal, ok := result[pairField]
		if !ok {
			continue
		}
		pairMap, ok := pairVal.(map[string]any)
		if !ok {
			continue
		}
		localeVal, exists := pairMap[colDef.Locale]
		if !exists {
			continue
		}
		result[colName] = localeVal
		newPairMap := maps.Clone(pairMap)
		delete(newPairMap, colDef.Locale)
		result[pairField] = newPairMap
	}
	return result
}

// applyLocaleToWrite normalises record data before writing to file.
// For each column that has a Locale value set (e.g. column "title" with locale "en"):
//   - The shortcut column ("title") is stored as-is in the file.
//   - If the paired map column ("titles") contains an entry for the primary locale key ("en"),
//     that entry is promoted to the shortcut column and removed from the map, so the value is
//     never duplicated across both fields.
//   - If the paired map becomes empty after removing the primary locale entry, it is dropped
//     from the result to avoid writing a redundant empty map.
func applyLocaleToWrite(data map[string]any, cols map[string]*ingitdb.ColumnDef) map[string]any {
	if len(cols) == 0 {
		return data
	}
	result := maps.Clone(data)
	for colName, colDef := range cols {
		if colDef.Locale == "" {
			continue
		}
		pairField := colName + "s"
		pairVal, hasPair := result[pairField]
		if !hasPair {
			continue
		}
		pairMap, ok := pairVal.(map[string]any)
		if !ok {
			continue
		}
		// If the primary locale entry is in the pair map, promote it to the shortcut column.
		if localeVal, exists := pairMap[colDef.Locale]; exists {
			result[colName] = localeVal
			newPairMap := maps.Clone(pairMap)
			delete(newPairMap, colDef.Locale)
			pairMap = newPairMap
		}
		// Drop the pair map if it is now empty.
		if len(pairMap) == 0 {
			delete(result, pairField)
		} else {
			result[pairField] = pairMap
		}
	}
	return result
}
