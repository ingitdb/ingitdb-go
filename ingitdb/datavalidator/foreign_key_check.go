package datavalidator

// specscore: feature/column-validation
// specscore: feature/subcollection-record-validation

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

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
	Key      string
	Data     map[string]any
	FilePath string // the file the record was read from
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
func validateForeignKeyReferences(root string, def *ingitdb.Definition) []ingitdb.ValidationError {
	if def == nil {
		return nil
	}
	// The index holds root collections. A foreign_key naming a subcollection
	// by path is checked against the one target instance it gives per record
	// (checkPathForeignKey, instanceKeyCache).
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

	instances := instanceKeyCache{}
	var errors []ingitdb.ValidationError
	for _, id := range slices.Sorted(maps.Keys(def.Collections)) {
		col := def.Collections[id]
		errors = append(errors, checkCollectionForeignKeys(id, col, nil, def, idx, instances)...)
		// Subcollection records live once per parent record, not at the
		// subcollection's loaded DirPath; walkSubCollectionInstances repoints each
		// instance at its on-disk data so a foreign_key on a subcollection column
		// is actually checked. This shares one instance-enumeration with the
		// schema pass (validateSubCollections), so the two passes cannot disagree
		// on where a subcollection's records live.
		instanceWalk{root: root}.walk(id, col, func(inst subCollectionInstance) {
			errors = append(errors, checkCollectionForeignKeys(inst.fullID, inst.colDef, inst.ancestorKeys, def, idx, instances)...)
		})
	}
	return errors
}

// checkCollectionForeignKeys checks one collection's records against the index.
// ancestorKeys are the keys of the records above col's records (nil for a root
// collection); a path foreign_key needs them for its $same segments.
func checkCollectionForeignKeys(
	fullID string,
	col *ingitdb.CollectionDef,
	ancestorKeys []string,
	def *ingitdb.Definition,
	idx foreignKeyIndex,
	instances instanceKeyCache,
) []ingitdb.ValidationError {
	fkColumns := make(map[string]string)                    // column name -> resolved root target collection
	pathColumns := make(map[string]*ingitdb.ForeignKeyPath) // column name -> subcollection target
	for name, colDef := range col.Columns {
		if colDef.ForeignKey == "" {
			continue
		}
		target, ok := ingitdb.ResolveForeignKey(fullID, colDef.ForeignKey, def.Collections)
		if ok {
			fkColumns[name] = target
			continue
		}
		if path, err := ingitdb.ResolveForeignKeyPath(fullID, colDef.ForeignKey, col, def.Collections); err == nil {
			pathColumns[name] = path
		}
		// unresolved targets are a load-time error, handled elsewhere
	}
	if len(fkColumns) == 0 && len(pathColumns) == 0 {
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
			// A `type: any` or list-typed FK column (e.g. event_ids: [a, b]) is
			// checked element by element via ingitdb.ForeignKeyElements — a
			// scalar value still yields itself, unchanged from before.
			values, elementErrs := ingitdb.ForeignKeyElements(raw)
			for _, badElem := range elementErrs {
				message := fmt.Sprintf("foreign key %q has a list element that is not a scalar (%s): foreign key element must be a scalar", name, badElem)
				errors = append(errors, newValidationError(fullID, "", r.Key, name, message, nil))
			}
			for _, value := range values {
				if !idx.Contains(target, value) {
					message := fmt.Sprintf("foreign key %q = %q has no matching record in collection %q", name, value, target)
					errors = append(errors, newValidationError(fullID, "", r.Key, name, message, nil))
				}
			}
		}
		recordKeys := append(slices.Clone(ancestorKeys), r.Key)
		for _, name := range slices.Sorted(maps.Keys(pathColumns)) {
			errors = append(errors, checkPathForeignKey(fullID, name, pathColumns[name], r, recordKeys, def, instances)...)
		}
	}
	return errors
}

// instanceKeyCache holds the record keys of each subcollection instance a path
// foreign_key pointed at, by instance data directory, so each is read once.
type instanceKeyCache map[string]map[string]bool

func (c instanceKeyCache) keys(colDef *ingitdb.CollectionDef) map[string]bool {
	if keys, ok := c[colDef.DirPath]; ok {
		return keys
	}
	records, _ := loadCollectionRecords(colDef) // a parse failure is reported by the schema pass
	keys := make(map[string]bool, len(records))
	for _, rec := range records {
		keys[rec.Key] = true
	}
	c[colDef.DirPath] = keys
	return keys
}

// checkPathForeignKey checks one record's value of a foreign_key that names a
// subcollection: it finds the target instance from the path (literal keys, the
// record's own keys for $same, the record's fields for {field}) and checks
// every value against that instance's record keys.
func checkPathForeignKey(
	fullID, name string,
	path *ingitdb.ForeignKeyPath,
	r loadedRecord,
	recordKeys []string,
	def *ingitdb.Definition,
	instances instanceKeyCache,
) []ingitdb.ValidationError {
	raw, present := r.Data[name]
	if !present || raw == nil {
		return nil
	}
	values, elementErrs := ingitdb.ForeignKeyElements(raw)
	var errors []ingitdb.ValidationError
	for _, badElem := range elementErrs {
		message := fmt.Sprintf("foreign key %q has a list element that is not a scalar (%s): foreign key element must be a scalar", name, badElem)
		errors = append(errors, newValidationError(fullID, "", r.Key, name, message, nil))
	}
	if len(values) == 0 {
		return errors
	}
	target := *def.Collections[path.Root]
	instancePath := []string{path.Root}
	for level, step := range path.Steps {
		parentKey := step.Literal
		switch {
		case step.Same:
			parentKey = recordKeys[level]
		case step.Field != "":
			key, problem := parentKeyFromField(r.Data[step.Field])
			if problem != "" {
				message := fmt.Sprintf("foreign key %q = %q: field %q, which gives the parent key in %q, %s", name, values[0], step.Field, path.String(), problem)
				return append(errors, newValidationError(fullID, "", r.Key, name, message, nil))
			}
			parentKey = key
		}
		sub := *target.SubCollections[step.Collection]
		sub.DirPath = subCollectionDataDir(&target, parentKey, step.Collection)
		instancePath = append(instancePath, parentKey, step.Collection)
		target = sub
	}
	keys := instances.keys(&target)
	for _, value := range values {
		if !keys[value] {
			message := fmt.Sprintf("foreign key %q = %q has no matching record in collection %q", name, value, strings.Join(instancePath, "/"))
			errors = append(errors, newValidationError(fullID, "", r.Key, name, message, nil))
		}
	}
	return errors
}

// parentKeyFromField turns the value of a {field} path segment into a parent
// record key: a string as is, a number as written (a whole JSON number 1000000
// is "1000000", not "1e+06"), a bool as true/false. It returns a problem
// (completing "field X, which gives the parent key in P, ...") when the value
// is empty, not a scalar, or not a valid record key (e.g. "..", "a/b").
func parentKeyFromField(raw any) (string, string) {
	var key string
	switch v := raw.(type) {
	case nil:
		return "", "is empty"
	case string:
		key = v
	case float64:
		key = strconv.FormatFloat(v, 'f', -1, 64)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, bool:
		key = fmt.Sprintf("%v", v)
	default:
		return "", "must be a scalar"
	}
	if key == "" {
		return "", "is empty"
	}
	if !ingitdb.IsPlainKeySegment(key) {
		return "", fmt.Sprintf("is %q, which is not a valid record key", key)
	}
	return key, ""
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
	pattern := singleRecordGlobPattern(colDef)
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
		records = append(records, loadedRecord{Key: recordKeyFromCollectionFilePath(colDef, filePath), Data: data, FilePath: filePath})
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
		records = append(records, loadedRecord{Key: key, Data: data, FilePath: filePath})
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
		records = append(records, loadedRecord{Key: key, Data: row, FilePath: filePath})
	}
	return records, nil
}
