package datavalidator

import (
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

func undeclaredFieldsColDef() *ingitdb.CollectionDef {
	return &ingitdb.CollectionDef{
		Columns: map[string]*ingitdb.ColumnDef{
			"name": {Type: ingitdb.ColumnTypeString},
		},
	}
}

// REQ:reject-undeclared-record-fields — a record field with no corresponding
// column is an error naming the field. Validation must examine the record's
// keys, not only the schema's columns: iterating columns alone can never see a
// field the schema does not mention.
func TestUndeclaredFields_RejectsUndeclaredField(t *testing.T) {
	errs := ValidateRecordData(undeclaredFieldsColDef(), "k", map[string]any{
		"name":    "x",
		"typoed":  "y",
		"another": 1,
	})
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors for 2 undeclared fields, got %d: %v", len(errs), errs)
	}
	seen := map[string]bool{}
	for _, e := range errs {
		seen[e.FieldName] = true
		if !strings.Contains(e.Message, "undeclared") && !strings.Contains(e.Message, "no column") {
			t.Errorf("message must explain the field is undeclared, got: %q", e.Message)
		}
	}
	for _, f := range []string{"typoed", "another"} {
		if !seen[f] {
			t.Errorf("error must name undeclared field %q; got %v", f, errs)
		}
	}
}

// A record using only declared fields is clean.
func TestUndeclaredFields_AcceptsDeclaredFields(t *testing.T) {
	errs := ValidateRecordData(undeclaredFieldsColDef(), "k", map[string]any{"name": "x"})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// REQ:reject-undeclared-record-fields — $-prefixed keys are library-reserved
// and exempt. validator.go sets record["$ID"] for INGR and parse.go sets
// row["$ID"] for CSV, so without this exemption every list/INGR collection
// would fail on a field the library itself added.
func TestUndeclaredFields_ExemptsDollarPrefixedKeys(t *testing.T) {
	errs := ValidateRecordData(undeclaredFieldsColDef(), "k", map[string]any{
		"name":      "x",
		"$ID":       "k",
		"$anything": "reserved",
	})
	if len(errs) != 0 {
		t.Errorf("$-prefixed keys are library-reserved and must be exempt, got: %v", errs)
	}
}

// The exemption holds whether or not the definition declares the $ column —
// declaring it MUST NOT change the outcome. demo-ingitdb's order_details
// declares "$ID" explicitly.
func TestUndeclaredFields_DollarExemptionHoldsWhenDeclared(t *testing.T) {
	colDef := &ingitdb.CollectionDef{
		Columns: map[string]*ingitdb.ColumnDef{
			"$ID":  {Type: ingitdb.ColumnTypeString},
			"name": {Type: ingitdb.ColumnTypeString},
		},
	}
	errs := ValidateRecordData(colDef, "k", map[string]any{"name": "x", "$ID": "k"})
	if len(errs) != 0 {
		t.Errorf("a declared $ column must behave identically, got: %v", errs)
	}
}

// A computed column must not be reported as undeclared. It is declared — the
// separate "must not be stored" rule owns that case, and reporting both would
// double-count one mistake.
func TestUndeclaredFields_ComputedColumnIsNotUndeclared(t *testing.T) {
	colDef := &ingitdb.CollectionDef{
		Columns: map[string]*ingitdb.ColumnDef{
			"name":    {Type: ingitdb.ColumnTypeString},
			"derived": {Type: ingitdb.ColumnTypeString, Formula: `name + "!"`},
		},
	}
	errs := ValidateRecordData(colDef, "k", map[string]any{"name": "x", "derived": "x!"})
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 error (computed must not be stored), got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, "computed") {
		t.Errorf("expected the computed-column message, got: %q", errs[0].Message)
	}
}
