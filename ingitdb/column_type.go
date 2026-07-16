package ingitdb

// specscore: feature/column-validation

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

type ColumnType string

const (
	ColumnTypeL10N     ColumnType = "map[locale]string"
	ColumnTypeString   ColumnType = "string"
	ColumnTypeInt      ColumnType = "int"
	ColumnTypeFloat    ColumnType = "float"
	ColumnTypeBool     ColumnType = "bool"
	ColumnTypeDate     ColumnType = "date"
	ColumnTypeTime     ColumnType = "time"
	ColumnTypeDateTime ColumnType = "datetime"
	ColumnTypeAny      ColumnType = "any"
)

var knownColumnTypes = []ColumnType{
	ColumnTypeL10N,
	ColumnTypeString,
	ColumnTypeInt,
	ColumnTypeFloat,
	ColumnTypeBool,
	ColumnTypeDate,
	ColumnTypeTime,
	ColumnTypeDateTime,
	ColumnTypeAny,
}

var errMissingRequiredField = errors.New("missing required field")

// ValidateColumnType reports whether ct is a column type inGitDB understands.
//
// The grammar is closed: a scalar from knownColumnTypes, a []T list, or a
// map[K]V. Anything else is an error. It previously ended in a bare `return
// nil`, so every unrecognised spelling was accepted at load and then matched
// every value at validation — `type: number` rode that path in
// e2e-test-ingitdb, leaving two columns entirely unvalidated. `number` is a
// legitimate map *key* type and was never a column type; it is not added as an
// alias for int/float, because two spellings for one type is how the
// inconsistency started.
func ValidateColumnType(ct ColumnType) error {
	if ct == "" {
		return errMissingRequiredField
	}
	if slices.Contains(knownColumnTypes, ct) {
		return nil
	}
	if strings.HasPrefix(string(ct), "map[") {
		rest := string(ct)[4:]
		i := strings.Index(rest, "]")
		if i < 0 {
			return fmt.Errorf("malformed map column type: %s", ct)
		}
		keyType := rest[:i]
		switch keyType {
		case "":
			return fmt.Errorf("missing key type for column type: %s", ct)
		case "locale", "string", "int", "number", "bool", "date":
			// The value type is deliberately unconstrained: map[string]any is
			// the declared escape hatch for per-record shapes.
			return nil
		default:
			return fmt.Errorf("unsupported key type for column type '%s', supported types are: string, int, number, bool, date", ct)
		}
	}
	if strings.HasPrefix(string(ct), "[]") {
		if _, ok := ListElementType(ct); ok {
			return nil
		}
		return fmt.Errorf("unsupported element type for list column type '%s', supported element types are: string, int, float, bool, date, time, datetime, any", ct)
	}
	return fmt.Errorf("unknown column type '%s'", ct)
}

// listElementTypes are the element types permitted inside a []T column.
// Deliberately closed: map[...] and nested lists are out of scope, so the
// grammar cannot grow ambiguous spellings by accident.
var listElementTypes = []ColumnType{
	ColumnTypeString,
	ColumnTypeInt,
	ColumnTypeFloat,
	ColumnTypeBool,
	ColumnTypeDate,
	ColumnTypeTime,
	ColumnTypeDateTime,
	ColumnTypeAny,
}

// ListElementType reports the element type of a []T column type, and whether
// ct is a list type at all. The element type must be one of listElementTypes;
// "[]map[locale]string" and "[][]string" are not list types by this rule.
func ListElementType(ct ColumnType) (ColumnType, bool) {
	const prefix = "[]"
	if !strings.HasPrefix(string(ct), prefix) {
		return "", false
	}
	elem := ColumnType(strings.TrimPrefix(string(ct), prefix))
	if slices.Contains(listElementTypes, elem) {
		return elem, true
	}
	return "", false
}
