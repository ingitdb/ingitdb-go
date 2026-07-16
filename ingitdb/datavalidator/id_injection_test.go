package datavalidator

import (
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

func idInjectionColDef() *ingitdb.CollectionDef {
	return &ingitdb.CollectionDef{
		Columns: map[string]*ingitdb.ColumnDef{
			"$ID":  {Type: ingitdb.ColumnTypeString, Required: true},
			"name": {Type: ingitdb.ColumnTypeString},
		},
	}
}

// #10 — $ID was injected only on the INGR and CSV paths. A per-file record
// legitimately omits $ID (it is the filename), so a declared-required $ID was
// always "missing" — 226 errors each in demo-ingitdb and demo-commerce-ingitdb.
// The key is known; bind it.
func TestIDInjection_BoundFromRecordKeyWhenDeclaredButAbsent(t *testing.T) {
	errs := ValidateRecordData(idInjectionColDef(), "product-42", map[string]any{"name": "x"})
	if len(errs) != 0 {
		t.Fatalf("$ID must be bound from the record key, got: %v", errs)
	}
}

// The binding must NOT mutate the caller's record. The exported entry point is
// used by the CLI's merge resolver, which serialises r.Fields after validating
// — writing $ID into it would leak the synthetic key into merged files.
func TestIDInjection_DoesNotMutateRecord(t *testing.T) {
	data := map[string]any{"name": "x"}
	_ = ValidateRecordData(idInjectionColDef(), "k", data)
	if _, present := data["$ID"]; present {
		t.Error("$ID must not be written into the caller's record map")
	}
}

// An explicitly stored $ID still wins — binding only fills an absent value.
func TestIDInjection_StoredValueIsUsed(t *testing.T) {
	// Declared max_length 3 so a too-long stored $ID is caught; the bound key
	// (which would be "k", length 1) would pass, proving the stored value flows
	// through validation rather than being shadowed.
	three := 3
	colDef := &ingitdb.CollectionDef{
		Columns: map[string]*ingitdb.ColumnDef{
			"$ID": {Type: ingitdb.ColumnTypeString, Required: true, MaxLength: &three},
		},
	}
	errs := ValidateRecordData(colDef, "k", map[string]any{"$ID": "toolong"})
	if len(errs) != 1 {
		t.Fatalf("stored $ID must be validated, expected 1 length error, got: %v", errs)
	}
}

// When the key is genuinely unknown (empty) and $ID is absent, it is still a
// missing required field: nothing can be bound. INGR/CSV inject $ID into the
// data before validation, so this only bites a caller passing an empty key.
func TestIDInjection_StillMissingWhenKeyUnknown(t *testing.T) {
	errs := ValidateRecordData(idInjectionColDef(), "", map[string]any{"name": "x"})
	if len(errs) != 1 {
		t.Fatalf("expected 1 missing-required error when key is unknown, got: %v", errs)
	}
	if errs[0].FieldName != "$ID" {
		t.Errorf("error must name $ID, got: %v", errs[0])
	}
}

// The bound value flows through type validation. A $ID declared as int cannot
// be satisfied by a string key — surfacing the mismatch rather than hiding it.
func TestIDInjection_BoundValueIsTypeChecked(t *testing.T) {
	colDef := &ingitdb.CollectionDef{
		Columns: map[string]*ingitdb.ColumnDef{
			"$ID": {Type: ingitdb.ColumnTypeInt, Required: true},
		},
	}
	errs := ValidateRecordData(colDef, "not-an-int", map[string]any{})
	if len(errs) != 1 {
		t.Fatalf("a string key bound to an int $ID must fail type check, got: %v", errs)
	}
}
