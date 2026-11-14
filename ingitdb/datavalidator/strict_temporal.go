package datavalidator

// specscore: feature/column-validation

import (
	"fmt"
	"maps"
	"slices"

	ingitdb "github.com/ingitdb/ingitdb-go/ingitdb"
)

// StrictTemporalFindings is an opt-in check, not part of Validate: it reports
// every text value of a top-level `date`, `time` or `datetime` column that is
// not a valid value of that type, using the parser that checks a typed `$ID`
// (column-validation#req:record-key-type-checked). Validate accepts any text
// in such a column and keeps doing so, so no existing database newly fails;
// a caller that wants the strict check (for example `2026-02-30` in a `date`
// column) runs this as well. Non-text values (a YAML timestamp decoded as a
// time) are left to Validate. root is the database root, as for Validate.
func StrictTemporalFindings(root string, def *ingitdb.Definition) []ingitdb.ValidationError {
	if def == nil {
		return nil
	}
	var findings []ingitdb.ValidationError
	check := func(fullID string, colDef *ingitdb.CollectionDef) {
		records, _ := loadCollectionRecords(colDef) // a read/parse failure is reported by Validate
		for _, rec := range records {
			for _, name := range slices.Sorted(maps.Keys(colDef.Columns)) {
				columnType := colDef.Columns[name].Type
				if name == reservedIDField || !isTemporalType(columnType) {
					continue
				}
				text, isText := rec.Data[name].(string)
				if !isText {
					continue
				}
				if _, ok := parseRecordKey(text, columnType); !ok {
					message := fmt.Sprintf("strict %s check for field %q: %q is not a valid %s (%s)", columnType, name, text, columnType, keyTypeFormats[columnType])
					findings = append(findings, newValidationError(fullID, rec.FilePath, rec.Key, name, message, nil))
				}
			}
		}
	}
	for _, rootID := range slices.Sorted(maps.Keys(def.Collections)) {
		col := def.Collections[rootID]
		check(rootID, col)
		instanceWalk{root: root}.walk(rootID, col, func(inst subCollectionInstance) {
			check(inst.fullID, inst.colDef)
		})
	}
	return findings
}

func isTemporalType(columnType ingitdb.ColumnType) bool {
	return columnType == ingitdb.ColumnTypeDate || columnType == ingitdb.ColumnTypeTime || columnType == ingitdb.ColumnTypeDateTime
}
