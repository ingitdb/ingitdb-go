package datavalidator

import (
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

// colDefWith builds a minimal single-column CollectionDef for constraint tests.
func colDefWith(name string, col ingitdb.ColumnDef) *ingitdb.CollectionDef {
	return &ingitdb.CollectionDef{
		ID:      "test",
		Columns: map[string]*ingitdb.ColumnDef{name: &col},
	}
}

func errorsJoined(errs []ingitdb.ValidationError) string {
	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		parts = append(parts, e.Error())
	}
	return strings.Join(parts, " | ")
}

// Verifies capability-record#ac:rejects-unknown-state-value via
// column-validation#req:enum-membership.
func TestEnum_RejectsNonMember(t *testing.T) {
	t.Parallel()

	col := colDefWith("state", ingitdb.ColumnDef{
		Type: ingitdb.ColumnTypeString,
		Enum: []any{"native", "partial", "absent", "unknown"},
	})

	errs := validateRecordData("test", "f.json", "r1", col, map[string]any{"state": "emulatable"})

	if len(errs) == 0 {
		t.Fatal("expected a validation error for a value outside the enum, got none")
	}
	msg := errorsJoined(errs)
	for _, want := range []string{"state", "emulatable", "native", "partial", "absent", "unknown"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error must name %q; got: %s", want, msg)
		}
	}
}

// A value inside the enum must pass.
func TestEnum_AcceptsMember(t *testing.T) {
	t.Parallel()

	col := colDefWith("state", ingitdb.ColumnDef{
		Type: ingitdb.ColumnTypeString,
		Enum: []any{"native", "partial", "absent", "unknown"},
	})

	if errs := validateRecordData("test", "f.json", "r1", col, map[string]any{"state": "absent"}); len(errs) != 0 {
		t.Fatalf("expected no errors for an enum member, got: %s", errorsJoined(errs))
	}
}

// A column without an enum must be unconstrained by this rule.
func TestEnum_AbsentEnumDoesNotConstrain(t *testing.T) {
	t.Parallel()

	col := colDefWith("state", ingitdb.ColumnDef{Type: ingitdb.ColumnTypeString})

	if errs := validateRecordData("test", "f.json", "r1", col, map[string]any{"state": "anything"}); len(errs) != 0 {
		t.Fatalf("no enum declared, so any string is valid; got: %s", errorsJoined(errs))
	}
}
