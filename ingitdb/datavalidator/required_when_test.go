package datavalidator

import (
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

func requiredWhenColDef() *ingitdb.CollectionDef {
	return &ingitdb.CollectionDef{
		Columns: map[string]*ingitdb.ColumnDef{
			"state": {Type: ingitdb.ColumnTypeString},
			// Mirrors the can-i-use pilot: a capability has no native concept to
			// name when it is absent or unresearched.
			"name": {Type: ingitdb.ColumnTypeString, RequiredWhen: `state != "absent" and state != "unknown"`},
		},
	}
}

// REQ:conditional-required — the column is required when the condition holds.
func TestRequiredWhen_FiresWhenConditionHolds(t *testing.T) {
	errs := ValidateRecordData(requiredWhenColDef(), "k", map[string]any{"state": "native"})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for missing conditionally-required field, got %d: %v", len(errs), errs)
	}
	if errs[0].FieldName != "name" {
		t.Errorf("error must name the field, got FieldName=%q (%v)", errs[0].FieldName, errs[0])
	}
}

// REQ:conditional-required — silent when the condition does not hold.
func TestRequiredWhen_SilentWhenConditionFails(t *testing.T) {
	for _, state := range []string{"absent", "unknown"} {
		t.Run(state, func(t *testing.T) {
			errs := ValidateRecordData(requiredWhenColDef(), "k", map[string]any{"state": state})
			if len(errs) != 0 {
				t.Errorf("state=%q must not require 'name', got: %v", state, errs)
			}
		})
	}
}

// Present value satisfies the requirement regardless of the condition.
func TestRequiredWhen_SatisfiedWhenValuePresent(t *testing.T) {
	errs := ValidateRecordData(requiredWhenColDef(), "k", map[string]any{"state": "native", "name": "CallbackQuery"})
	if len(errs) != 0 {
		t.Errorf("expected no errors when the field is present, got: %v", errs)
	}
}

// A sibling the record omits binds as None rather than exploding. Without this
// every sparse record would fail on "predeclared variable state is
// uninitialized" — the field is declared, the record just omits it.
func TestRequiredWhen_AbsentSiblingBindsAsNone(t *testing.T) {
	errs := ValidateRecordData(requiredWhenColDef(), "k", map[string]any{})
	// state is absent -> None; None != "absent" and None != "unknown" -> name required.
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 error (name required), got %d: %v", len(errs), errs)
	}
	if errs[0].FieldName != "name" {
		t.Errorf("error must name the field, got FieldName=%q (%v)", errs[0].FieldName, errs[0])
	}
}

// REQ:formula-load-time-resolution — an expression evaluating to a non-boolean
// is an error, not a silent truthiness coercion. `required_when: 'name'` must
// NOT mean "required when name is non-empty".
func TestRequiredWhen_RejectsNonBooleanResult(t *testing.T) {
	colDef := &ingitdb.CollectionDef{
		Columns: map[string]*ingitdb.ColumnDef{
			"name": {Type: ingitdb.ColumnTypeString},
			"note": {Type: ingitdb.ColumnTypeString, RequiredWhen: `name`},
		},
	}
	errs := ValidateRecordData(colDef, "k", map[string]any{"name": "x"})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for non-boolean required_when, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "required_when") {
		t.Errorf("error must mention required_when, got: %v", errs[0])
	}
}

// Starlark truthiness must not leak in via a numeric result either.
func TestRequiredWhen_RejectsNumericResult(t *testing.T) {
	colDef := &ingitdb.CollectionDef{
		Columns: map[string]*ingitdb.ColumnDef{
			"count": {Type: ingitdb.ColumnTypeInt},
			"note":  {Type: ingitdb.ColumnTypeString, RequiredWhen: `count`},
		},
	}
	errs := ValidateRecordData(colDef, "k", map[string]any{"count": 1})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for numeric required_when, got %d: %v", len(errs), errs)
	}
}

// A computed sibling is not visible to required_when at evaluation, matching
// the load-time rule that required_when may reference only stored fields.
func TestRequiredWhen_DoesNotSeeComputedSiblings(t *testing.T) {
	colDef := &ingitdb.CollectionDef{
		Columns: map[string]*ingitdb.ColumnDef{
			"state":   {Type: ingitdb.ColumnTypeString},
			"derived": {Type: ingitdb.ColumnTypeString, Formula: `state + "!"`},
			"name":    {Type: ingitdb.ColumnTypeString, RequiredWhen: `state == "native"`},
		},
	}
	errs := ValidateRecordData(colDef, "k", map[string]any{"state": "absent"})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}
