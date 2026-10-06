package datavalidator

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	ingitdb "github.com/ingitdb/ingitdb-go/ingitdb"
)

// validateSourceForeignKeys checks relational references by the declared
// target columns, including composite keys. Source referential actions are
// retained as metadata; this pass validates data, not mutation cascades.
func validateSourceForeignKeys(def *ingitdb.Definition) []ingitdb.ValidationError {
	if def == nil {
		return nil
	}
	var findings []ingitdb.ValidationError
	for collectionID, collection := range def.Collections {
		if collection.SourceSchema == nil {
			continue
		}
		rows, err := loadCollectionRecords(collection)
		if err != nil {
			continue
		} // the ordinary record validator reports parse errors
		for _, fk := range collection.SourceSchema.ForeignKeys {
			// Audit declared relationships even when the original SQL connection
			// had enforcement disabled. This is a snapshot validation, not a
			// claim that native inGitDB writes implement SQL referential actions.
			target, ok := def.Collections[fk.ReferencedCollection]
			if !ok {
				findings = append(findings, newValidationError(collectionID, "", "", strings.Join(fk.Fields, ","), fmt.Sprintf("source foreign key targets missing collection %q", fk.ReferencedCollection), nil))
				continue
			}
			targetFields := fk.ReferencedFields
			if len(targetFields) == 0 {
				targetFields = target.PrimaryKey
			}
			if len(targetFields) != len(fk.Fields) || len(targetFields) == 0 {
				findings = append(findings, newValidationError(collectionID, "", "", strings.Join(fk.Fields, ","), "source foreign key has no resolvable target column tuple", nil))
				continue
			}
			targetRows, err := loadCollectionRecords(target)
			if err != nil {
				continue
			}
			index := make(map[string]bool, len(targetRows))
			for _, row := range targetRows {
				tuple, complete := relationalTuple(row.Data, targetFields, target.SourceSchema)
				if complete {
					index[tuple] = true
				}
			}
			for _, row := range rows {
				tuple, complete := relationalTuple(row.Data, fk.Fields, collection.SourceSchema)
				if complete && !index[tuple] {
					findings = append(findings, newValidationError(collectionID, "", row.Key, strings.Join(fk.Fields, ","), fmt.Sprintf("source foreign key %q has no matching %q tuple", fk.Name, fk.ReferencedCollection), nil))
				}
			}
		}
	}
	return findings
}

func relationalTuple(data map[string]any, fields []string, schema *ingitdb.SourceSchemaDef) (string, bool) {
	parts := make([]string, len(fields))
	for i, field := range fields {
		value, ok := data[field]
		if !ok || value == nil {
			return "", false
		} // SQL MATCH SIMPLE null semantics
		if logicalSourceType(schema, field) == "decimal" {
			parts[i] = "n:" + fmt.Sprint(value)
			continue
		}
		switch v := value.(type) {
		case int:
			parts[i] = "n:" + strconv.FormatInt(int64(v), 10)
		case int64:
			parts[i] = "n:" + strconv.FormatInt(v, 10)
		case float64:
			parts[i] = "n:" + strconv.FormatFloat(v, 'g', -1, 64)
		default:
			encoded, err := json.Marshal(v)
			if err != nil {
				return "", false
			}
			parts[i] = "v:" + string(encoded)
		}
	}
	encoded, _ := json.Marshal(parts)
	return string(encoded), true
}

func logicalSourceType(schema *ingitdb.SourceSchemaDef, field string) string {
	if schema == nil {
		return ""
	}
	for _, sourceField := range schema.Fields {
		if sourceField.Name == field {
			return sourceField.Type
		}
	}
	return ""
}
