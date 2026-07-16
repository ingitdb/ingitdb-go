package ingitdb

import (
	"strings"
	"testing"
)

// requiredWhenCollection validates a minimal collection whose only interesting
// content is its columns. record_file is supplied because CollectionDef.Validate
// requires one; it is otherwise irrelevant to these tests.
func requiredWhenCollection(t *testing.T, columns map[string]*ColumnDef) error {
	t.Helper()
	def := &CollectionDef{
		ID:         "c",
		RecordFile: &RecordFileDef{Name: "{key}.json", Format: RecordFormatJSON, RecordType: "map[string]any"},
		Columns:    columns,
	}
	return def.Validate()
}

// REQ:conditional-required — required_when reuses the formula grammar, so an
// undeclared identifier is rejected at definition-load time, exactly as it is
// for formula.
func TestRequiredWhen_RejectsUndeclaredIdentifierAtLoad(t *testing.T) {
	err := requiredWhenCollection(t, map[string]*ColumnDef{
		"name":  {Type: ColumnTypeString, RequiredWhen: `nosuchfield == 1`},
		"state": {Type: ColumnTypeString},
	})
	if err == nil {
		t.Fatal("expected load error for undeclared identifier in required_when")
	}
	if !strings.Contains(err.Error(), "nosuchfield") {
		t.Errorf("error must name the undeclared identifier, got: %v", err)
	}
}

// REQ:conditional-required — required_when may not reference a computed column.
func TestRequiredWhen_RejectsComputedColumnReference(t *testing.T) {
	err := requiredWhenCollection(t, map[string]*ColumnDef{
		"total": {Type: ColumnTypeInt, Formula: "1 + 1"},
		"note":  {Type: ColumnTypeString, RequiredWhen: `total > 0`},
	})
	if err == nil {
		t.Fatal("expected load error referencing a computed column")
	}
	if !strings.Contains(err.Error(), "total") {
		t.Errorf("error must name the computed column, got: %v", err)
	}
}

// REQ:conditional-required — declaring both required and required_when on one
// column is a definition-load error.
func TestRequiredWhen_RejectsRequiredAndRequiredWhenTogether(t *testing.T) {
	err := requiredWhenCollection(t, map[string]*ColumnDef{
		"state": {Type: ColumnTypeString},
		"name":  {Type: ColumnTypeString, Required: true, RequiredWhen: `state != "absent"`},
	})
	if err == nil {
		t.Fatal("expected load error for required + required_when on one column")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error must name the offending column, got: %v", err)
	}
}

// A well-formed required_when over stored siblings loads cleanly. These mirror
// the real declarations in the can-i-use pilot database.
func TestRequiredWhen_AcceptsWellFormedExpressions(t *testing.T) {
	for _, expr := range []string{
		`state != "absent" and state != "unknown"`,
		`state == "absent"`,
		`state != "unknown"`,
		`equivalenceClass != None and (state == "native" or state == "partial")`,
		`len(tags) > 0`,
	} {
		t.Run(expr, func(t *testing.T) {
			err := requiredWhenCollection(t, map[string]*ColumnDef{
				"name":             {Type: ColumnTypeString, RequiredWhen: expr},
				"state":            {Type: ColumnTypeString},
				"equivalenceClass": {Type: ColumnTypeString},
				"tags":             {Type: ColumnType("[]string")},
			})
			if err != nil {
				t.Errorf("required_when %q must load cleanly, got: %v", expr, err)
			}
		})
	}
}

// REQ:formula-load-time-resolution — an expression that is syntactically
// invalid is rejected at load.
func TestRequiredWhen_RejectsUnparseableExpression(t *testing.T) {
	err := requiredWhenCollection(t, map[string]*ColumnDef{
		"name":  {Type: ColumnTypeString, RequiredWhen: `state ==`},
		"state": {Type: ColumnTypeString},
	})
	if err == nil {
		t.Fatal("expected load error for unparseable required_when")
	}
}
