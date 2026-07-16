package datavalidator

import (
	"context"
	"strings"
	"testing"

	ingitdb "github.com/ingitdb/ingitdb-go/ingitdb"
)

// recordCountViolations returns the collection-level record-count errors in a
// result — the ones with no field/record location that mention a records-count
// bound. Used to assert record-count enforcement without tripping on unrelated
// schema violations a fixture might carry.
func recordCountViolations(errs []ingitdb.ValidationError) []ingitdb.ValidationError {
	var out []ingitdb.ValidationError
	for _, e := range errs {
		if strings.Contains(e.Message, "records_count") {
			out = append(out, e)
		}
	}
	return out
}

// Verifies record-count-constraints#ac:min-records-count-rejects-too-few.
func TestRecordCount_MinRejectsTooFew(t *testing.T) {
	dir := t.TempDir()
	widgets := writeMapCollection(t, dir, "widgets",
		"w1:\n  name: One\n",
		map[string]*ingitdb.ColumnDef{"name": {Type: ingitdb.ColumnTypeString}})
	widgets.MinRecordsCount = ip(2)

	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"widgets": widgets}}
	res, err := NewValidator().Validate(context.Background(), dir, def)
	if err != nil {
		t.Fatal(err)
	}
	viol := recordCountViolations(res.Errors())
	if len(viol) != 1 {
		t.Fatalf("expected 1 record-count violation, got %d: %v", len(viol), res.Errors())
	}
	msg := viol[0].Error()
	for _, want := range []string{"widgets", "min_records_count", "2", "1"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error must mention %q, got: %s", want, msg)
		}
	}
	if viol[0].FieldName != "" || viol[0].RecordKey != "" {
		t.Errorf("record-count error must be collection-level (no field/record), got field=%q record=%q",
			viol[0].FieldName, viol[0].RecordKey)
	}
	if viol[0].CollectionID != "widgets" {
		t.Errorf("record-count error must name the collection, got %q", viol[0].CollectionID)
	}
}

// Verifies record-count-constraints#ac:max-records-count-rejects-too-many.
func TestRecordCount_MaxRejectsTooMany(t *testing.T) {
	dir := t.TempDir()
	widgets := writeMapCollection(t, dir, "widgets",
		"w1:\n  name: One\nw2:\n  name: Two\n",
		map[string]*ingitdb.ColumnDef{"name": {Type: ingitdb.ColumnTypeString}})
	widgets.MaxRecordsCount = ip(1)

	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"widgets": widgets}}
	res, err := NewValidator().Validate(context.Background(), dir, def)
	if err != nil {
		t.Fatal(err)
	}
	viol := recordCountViolations(res.Errors())
	if len(viol) != 1 {
		t.Fatalf("expected 1 record-count violation, got %d: %v", len(viol), res.Errors())
	}
	msg := viol[0].Error()
	for _, want := range []string{"widgets", "max_records_count", "1", "2"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error must mention %q, got: %s", want, msg)
		}
	}
}

// Verifies record-count-constraints#ac:record-count-within-bounds-passes.
func TestRecordCount_WithinBoundsPasses(t *testing.T) {
	dir := t.TempDir()
	widgets := writeMapCollection(t, dir, "widgets",
		"w1:\n  name: One\nw2:\n  name: Two\nw3:\n  name: Three\n",
		map[string]*ingitdb.ColumnDef{"name": {Type: ingitdb.ColumnTypeString}})
	widgets.MinRecordsCount = ip(1)
	widgets.MaxRecordsCount = ip(5)

	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"widgets": widgets}}
	res, err := NewValidator().Validate(context.Background(), dir, def)
	if err != nil {
		t.Fatal(err)
	}
	if viol := recordCountViolations(res.Errors()); len(viol) != 0 {
		t.Fatalf("3 records within [1,5] must pass, got: %v", viol)
	}
}

// Verifies record-count-constraints#ac:max-records-count-zero-enforced — a
// declared zero max means "must be empty" and is enforced, not read as unset.
func TestRecordCount_MaxZeroEnforced(t *testing.T) {
	dir := t.TempDir()
	widgets := writeMapCollection(t, dir, "widgets",
		"w1:\n  name: One\n",
		map[string]*ingitdb.ColumnDef{"name": {Type: ingitdb.ColumnTypeString}})
	widgets.MaxRecordsCount = ip(0)

	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"widgets": widgets}}
	res, err := NewValidator().Validate(context.Background(), dir, def)
	if err != nil {
		t.Fatal(err)
	}
	viol := recordCountViolations(res.Errors())
	if len(viol) != 1 {
		t.Fatalf("max_records_count: 0 with 1 record must fail, got %d: %v", len(viol), res.Errors())
	}
	if msg := viol[0].Error(); !strings.Contains(msg, "max_records_count") || !strings.Contains(msg, "0") {
		t.Errorf("error must name max_records_count and the bound 0, got: %s", msg)
	}
}

// A collection with the record count exactly on each bound passes — the bounds
// are inclusive.
func TestRecordCount_BoundsAreInclusive(t *testing.T) {
	dir := t.TempDir()
	widgets := writeMapCollection(t, dir, "widgets",
		"w1:\n  name: One\nw2:\n  name: Two\n",
		map[string]*ingitdb.ColumnDef{"name": {Type: ingitdb.ColumnTypeString}})
	widgets.MinRecordsCount = ip(2)
	widgets.MaxRecordsCount = ip(2)

	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"widgets": widgets}}
	res, err := NewValidator().Validate(context.Background(), dir, def)
	if err != nil {
		t.Fatal(err)
	}
	if viol := recordCountViolations(res.Errors()); len(viol) != 0 {
		t.Fatalf("count exactly equal to both bounds must pass, got: %v", viol)
	}
}

// A collection declaring no bounds is unconstrained by this rule.
func TestRecordCount_NoBoundsUnconstrained(t *testing.T) {
	dir := t.TempDir()
	widgets := writeMapCollection(t, dir, "widgets",
		"w1:\n  name: One\n",
		map[string]*ingitdb.ColumnDef{"name": {Type: ingitdb.ColumnTypeString}})

	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"widgets": widgets}}
	res, err := NewValidator().Validate(context.Background(), dir, def)
	if err != nil {
		t.Fatal(err)
	}
	if viol := recordCountViolations(res.Errors()); len(viol) != 0 {
		t.Fatalf("no bounds declared, so record count is unconstrained, got: %v", viol)
	}
}
