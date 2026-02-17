package ingitdb

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

func ValidateColumnType(ct ColumnType) error {
	if ct == "" {
		return errMissingRequiredField
	}
	if slices.Contains(knownColumnTypes, ct) {
		return nil
	}
	if strings.HasPrefix(string(ct), "map[") {
		i := strings.Index(string(ct)[4:], "]")
		keyType := string(ct[4 : i+4])
		switch keyType {
		case "":
			return fmt.Errorf("missing key type for column type: %s", ct)
		case "locale", "string", "int", "number", "bool", "date": // OK
		default:
			return fmt.Errorf("unsupported key type for column type '%s', supported types are: string, int, number, bool, date", ct)
		}
	}
	return nil
}
