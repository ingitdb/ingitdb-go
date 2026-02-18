package dalgo2ingitdb

import (
	"encoding/json"
	"errors"
	"fmt"
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

// resolveRecordPath replaces {key} in the record file name template and joins with the collection dir.
func resolveRecordPath(colDef *ingitdb.CollectionDef, recordKey string) string {
	name := strings.Replace(colDef.RecordFile.Name, "{key}", recordKey, 1)
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
