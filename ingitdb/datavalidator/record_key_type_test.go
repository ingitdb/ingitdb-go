package datavalidator

// specscore: feature/column-validation

import (
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

func keyTypedColDef(keyType ingitdb.ColumnType) *ingitdb.CollectionDef {
	return &ingitdb.CollectionDef{
		Columns: map[string]*ingitdb.ColumnDef{
			"$ID": {Type: keyType, Required: true},
		},
	}
}

// Verifies column-validation#ac:typed-record-key-checked.
func TestRecordKeyType_CheckedAgainstDeclaredType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		keyType ingitdb.ColumnType
		key     string
		valid   bool
	}{
		{ingitdb.ColumnTypeDate, "2026-11-24", true},
		{ingitdb.ColumnTypeDate, "notadate", false},
		{ingitdb.ColumnTypeDate, "2026-02-30", false},
		{ingitdb.ColumnTypeDateTime, "2026-11-18T07:30:00Z", true},
		{ingitdb.ColumnTypeDateTime, "2026-11-18T07:30:00.5+01:00", true},
		{ingitdb.ColumnTypeDateTime, "2026-11-24T10:00:00", true},
		{ingitdb.ColumnTypeDateTime, "2026-11-18", false},
		{ingitdb.ColumnTypeTime, "07:30", true},
		{ingitdb.ColumnTypeTime, "9:00", true},
		{ingitdb.ColumnTypeTime, "07:30:15", true},
		{ingitdb.ColumnTypeTime, "10:00:00.123", true},
		{ingitdb.ColumnTypeTime, "7h30", false},
		{ingitdb.ColumnTypeInt, "42", true},
		{ingitdb.ColumnTypeInt, "-7", true},
		{ingitdb.ColumnTypeInt, "0", true},
		{ingitdb.ColumnTypeInt, "4.2", false},
		{ingitdb.ColumnTypeInt, "007", false},
		{ingitdb.ColumnTypeInt, "+5", false},
		{ingitdb.ColumnTypeFloat, "4.2", true},
		{ingitdb.ColumnTypeFloat, "1e3", true},
		{ingitdb.ColumnTypeFloat, "abc", false},
		{ingitdb.ColumnTypeFloat, "NaN", false},
		{ingitdb.ColumnTypeFloat, "Inf", false},
		{ingitdb.ColumnTypeFloat, "0x1p2", false},
		{ingitdb.ColumnTypeBool, "true", true},
		{ingitdb.ColumnTypeBool, "false", true},
		{ingitdb.ColumnTypeBool, "yes", false},
		{ingitdb.ColumnTypeString, "anything", true},
		{ingitdb.ColumnTypeAny, "anything", true},
	}
	for _, tt := range tests {
		errs := ValidateRecordData(keyTypedColDef(tt.keyType), tt.key, map[string]any{})
		if tt.valid && len(errs) != 0 {
			t.Errorf("$ID %s %q: expected valid, got %v", tt.keyType, tt.key, errs)
		}
		if !tt.valid {
			if len(errs) != 1 {
				t.Errorf("$ID %s %q: expected 1 finding, got %v", tt.keyType, tt.key, errs)
				continue
			}
			msg := errs[0].Message
			if errs[0].FieldName != "$ID" || !strings.Contains(msg, "wrong type") || !strings.Contains(msg, tt.key) {
				t.Errorf("$ID %s %q: unexpected finding %+v", tt.keyType, tt.key, errs[0])
			}
		}
	}
}

// A typed key that parses feeds the parsed value to enum and range checks, so
// `$ID: {type: int, max_value: 10}` compares numbers, not text.
func TestRecordKeyType_ParsedValueFeedsRangeAndEnum(t *testing.T) {
	t.Parallel()
	maxValue := 10.0
	colDef := &ingitdb.CollectionDef{
		Columns: map[string]*ingitdb.ColumnDef{
			"$ID": {Type: ingitdb.ColumnTypeInt, MaxValue: &maxValue, Enum: []any{3, 11}},
		},
	}
	if errs := ValidateRecordData(colDef, "3", map[string]any{}); len(errs) != 0 {
		t.Fatalf("key 3 is in range and in the enum, got %v", errs)
	}
	if errs := ValidateRecordData(colDef, "11", map[string]any{}); len(errs) != 1 || !strings.Contains(errs[0].Message, "max_value") {
		t.Fatalf("key 11 is above max_value, got %v", errs)
	}
	if errs := ValidateRecordData(colDef, "4", map[string]any{}); len(errs) != 1 || !strings.Contains(errs[0].Message, "permitted values") {
		t.Fatalf("key 4 is not in the enum, got %v", errs)
	}
}

// A non-string $ID value (a list-of-records row storing "$ID": 5) keeps the
// plain value type check, unchanged.
func TestRecordKeyType_NonStringValueKeepsValueTypeCheck(t *testing.T) {
	t.Parallel()
	errs := ValidateRecordData(keyTypedColDef(ingitdb.ColumnTypeInt), "5", map[string]any{"$ID": 5})
	if len(errs) != 0 {
		t.Fatalf("an int $ID value must pass an int column, got %v", errs)
	}
	errs = ValidateRecordData(keyTypedColDef(ingitdb.ColumnTypeDate), "5", map[string]any{"$ID": 5})
	if len(errs) != 1 || !strings.Contains(errs[0].Message, "expected date, got int") {
		t.Fatalf("an int $ID value must fail a date column as before, got %v", errs)
	}
}

// Only $ID is parsed as a key: an ordinary string column typed date keeps the
// existing check (any string is accepted), so no stored field changes outcome.
func TestRecordKeyType_OrdinaryDateColumnUnchanged(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		Columns: map[string]*ingitdb.ColumnDef{
			"day": {Type: ingitdb.ColumnTypeDate},
		},
	}
	if errs := ValidateRecordData(colDef, "k", map[string]any{"day": "notadate"}); len(errs) != 0 {
		t.Fatalf("ordinary date columns are unchanged by key checking, got %v", errs)
	}
}
