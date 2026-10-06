package datavalidator

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	ingitdb "github.com/ingitdb/ingitdb-go/ingitdb"
)

type storageClassEntry struct {
	ID      string            `json:"id"`
	Classes map[string]string `json:"classes"`
}

// validateStorageClassFiles binds typed-transport sidecar entries to actual
// record IDs. It rejects dangling, duplicate, incomplete, or invalid entries.
func validateStorageClassFiles(collectionID string, col *ingitdb.CollectionDef) []ingitdb.ValidationError {
	if col.SourceSchema == nil || len(col.SourceSchema.StorageClassFiles) == 0 {
		return nil
	}
	rows, err := loadCollectionRecords(col)
	if err != nil {
		return nil
	} // ordinary record parsing reports the failure
	records := make(map[string]map[string]any, len(rows))
	for _, row := range rows {
		records[row.Key] = row.Data
	}
	seen := map[string]bool{}
	seenClasses := map[string]map[string]bool{}
	var findings []ingitdb.ValidationError
	add := func(path, id, field, message string) {
		findings = append(findings, newValidationError(collectionID, path, id, field, message, nil))
	}
	for _, name := range col.SourceSchema.StorageClassFiles {
		path := filepath.Join(col.DirPath, name)
		file, err := os.Open(path)
		if err != nil {
			add(path, "", "", fmt.Sprintf("cannot read source storage classes: %v", err))
			continue
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			var entry storageClassEntry
			if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
				add(path, "", "", fmt.Sprintf("invalid source storage class line: %v", err))
				continue
			}
			if entry.ID == "" || seen[entry.ID] {
				add(path, entry.ID, "", "duplicate or empty source storage class record ID")
				continue
			}
			seen[entry.ID] = true
			seenClasses[entry.ID] = map[string]bool{}
			row, exists := records[entry.ID]
			if !exists {
				add(path, entry.ID, "", "source storage class ID has no matching record")
				continue
			}
			for field, class := range entry.Classes {
				seenClasses[entry.ID][field] = true
				if logicalSourceType(col.SourceSchema, field) != "decimal" {
					add(path, entry.ID, field, "source storage class names a non-decimal field")
					continue
				}
				if row[field] == nil {
					add(path, entry.ID, field, "source storage class names a null or absent value")
					continue
				}
				switch class {
				case "integer", "real", "text", "blob":
				default:
					add(path, entry.ID, field, fmt.Sprintf("invalid source storage class %q", class))
				}
			}
		}
		if err := scanner.Err(); err != nil {
			add(path, "", "", fmt.Sprintf("read source storage classes: %v", err))
		}
		_ = file.Close()
	}
	decimalFields := []string{}
	for _, field := range col.SourceSchema.Fields {
		if field.Type == "decimal" {
			decimalFields = append(decimalFields, field.Name)
		}
	}
	for id, row := range records {
		for _, field := range decimalFields {
			if row[field] != nil && !seenClasses[id][field] {
				add("", id, field, "missing source storage class for decimal value")
			}
		}
	}
	return findings
}
