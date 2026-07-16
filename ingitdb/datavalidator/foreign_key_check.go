package datavalidator

// specscore: feature/column-validation

import (
	"fmt"
	"os"
	"path/filepath"

	ingitdb "github.com/ingitdb/ingitdb-go/ingitdb"
)

// foreignKeyIndex maps a root collection id to the set of its record keys. It
// implements ingitdb's ForeignKeyIndex shape (Contains) and is built once per
// validation run, then read concurrently.
type foreignKeyIndex map[string]map[string]bool

// Contains reports whether collectionID has a record with the given key.
func (idx foreignKeyIndex) Contains(collectionID, key string) bool {
	keys, ok := idx[collectionID]
	return ok && keys[key]
}

// loadedRecord is a record's key paired with its data, used by the FK pass.
type loadedRecord struct {
	Key  string
	Data map[string]any
}

// validateForeignKeyReferences checks that every foreign_key value in every
// record points at an existing key in the resolved target collection.
//
// This is a separate pass rather than part of validateRecordData because it
// needs the whole definition: a target is a root collection resolved
// module-relative (ingitdb.ResolveForeignKey), and the index of valid keys can
// only be built once every collection's records are known. The record-count
// this re-reads is small, and keeping it out of validateRecordData avoids
// threading the index through every per-record call site.
//
// Definition load already rejects a foreign_key that resolves to no collection
// (ingitdb.ValidateForeignKeys), so here the target always resolves; what is
// checked is the value's existence as a key.
func validateForeignKeyReferences(def *ingitdb.Definition) []ingitdb.ValidationError {
	if def == nil {
		return nil
	}
	// The index holds root collections only: a foreign_key resolves to a root
	// collection, never a subcollection.
	idx := make(foreignKeyIndex, len(def.Collections))
	for id, col := range def.Collections {
		records, err := loadCollectionRecords(col)
		if err != nil {
			continue // a read/parse failure is already reported by the schema pass
		}
		keys := make(map[string]bool, len(records))
		for _, r := range records {
			keys[r.Key] = true
		}
		idx[id] = keys
	}

	var errors []ingitdb.ValidationError
	var walk func(fullID string, col *ingitdb.CollectionDef)
	walk = func(fullID string, col *ingitdb.CollectionDef) {
		errors = append(errors, checkCollectionForeignKeys(fullID, col, def, idx)...)
		for subID, sub := range col.SubCollections {
			walk(fullID+"/"+subID, sub)
		}
	}
	for id, col := range def.Collections {
		walk(id, col)
	}
	return errors
}

// checkCollectionForeignKeys checks one collection's records against the index.
func checkCollectionForeignKeys(fullID string, col *ingitdb.CollectionDef, def *ingitdb.Definition, idx foreignKeyIndex) []ingitdb.ValidationError {
	fkColumns := make(map[string]string) // column name -> resolved target collection
	for name, colDef := range col.Columns {
		if colDef.ForeignKey == "" {
			continue
		}
		target, ok := ingitdb.ResolveForeignKey(fullID, colDef.ForeignKey, def.Collections)
		if !ok {
			continue // unresolved targets are a load-time error, handled elsewhere
		}
		fkColumns[name] = target
	}
	if len(fkColumns) == 0 {
		return nil
	}

	records, err := loadCollectionRecords(col)
	if err != nil {
		return nil // read/parse failure already reported by the schema pass
	}

	var errors []ingitdb.ValidationError
	for _, r := range records {
		for name, target := range fkColumns {
			raw, present := r.Data[name]
			if !present || raw == nil {
				continue // an absent FK value is a required/optional concern, not integrity
			}
			value := fmt.Sprintf("%v", raw)
			if value == "" {
				continue
			}
			if !idx.Contains(target, value) {
				message := fmt.Sprintf("foreign key %q = %q has no matching record in collection %q", name, value, target)
				errors = append(errors, newValidationError(fullID, "", r.Key, name, message, nil))
			}
		}
	}
	return errors
}

// loadCollectionRecords reads a collection's records as key/data pairs, reusing
// the same parse helpers the schema pass uses. It mirrors
// validateCollectionRecords' dispatch on record type but collects rather than
// validates.
func loadCollectionRecords(colDef *ingitdb.CollectionDef) ([]loadedRecord, error) {
	if shouldSkipRecordParsing(colDef) {
		return nil, nil
	}
	switch colDef.RecordFile.RecordType {
	case ingitdb.SingleRecord:
		return loadSingleRecords(colDef)
	case ingitdb.MapOfRecords:
		return loadMapRecords(colDef)
	case ingitdb.ListOfRecords:
		return loadListRecords(colDef)
	default:
		return nil, nil
	}
}

func loadSingleRecords(colDef *ingitdb.CollectionDef) ([]loadedRecord, error) {
	pattern, err := singleRecordGlobPattern(colDef)
	if err != nil {
		return nil, err
	}
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	var records []loadedRecord
	for _, filePath := range matches {
		if skipRecordPath(filePath, colDef.RecordFile) {
			continue
		}
		info, statErr := os.Stat(filePath)
		if statErr != nil || info.IsDir() {
			continue
		}
		content, readErr := os.ReadFile(filePath)
		if readErr != nil {
			continue
		}
		data, parseErr := ingitdb.ParseRecordContentForCollection(content, colDef)
		if parseErr != nil {
			continue
		}
		records = append(records, loadedRecord{Key: recordKeyFromFilePath(filePath), Data: data})
	}
	return records, nil
}

func loadMapRecords(colDef *ingitdb.CollectionDef) ([]loadedRecord, error) {
	filePath := collectionRecordFilePath(colDef)
	content, ok, _ := readRecordsFile("", filePath)
	if !ok {
		return nil, nil
	}
	parsed, err := ingitdb.ParseMapOfRecordsContent(content, colDef.RecordFile.Format)
	if err != nil {
		return nil, err
	}
	records := make([]loadedRecord, 0, len(parsed))
	for key, data := range parsed {
		records = append(records, loadedRecord{Key: key, Data: data})
	}
	return records, nil
}

func loadListRecords(colDef *ingitdb.CollectionDef) ([]loadedRecord, error) {
	filePath := collectionRecordFilePath(colDef)
	content, ok, _ := readRecordsFile("", filePath)
	if !ok {
		return nil, nil
	}
	rows, err := parseListRows(content, colDef)
	if err != nil {
		return nil, err
	}
	var records []loadedRecord
	for _, row := range rows {
		key, keyOK := ingitdb.ResolveListRecordKey(row, colDef)
		if !keyOK {
			continue
		}
		records = append(records, loadedRecord{Key: key, Data: row})
	}
	return records, nil
}
