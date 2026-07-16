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

// --- list-column-type -------------------------------------------------------

// Verifies column-validation#ac:list-rejects-non-list.
func TestList_RejectsNonList(t *testing.T) {
	t.Parallel()

	col := colDefWith("docs", ingitdb.ColumnDef{Type: "[]string"})

	errs := validateRecordData("test", "f.json", "r1", col, map[string]any{"docs": "not-a-list"})
	if len(errs) == 0 {
		t.Fatal("expected an error for a non-list value in a []string column")
	}
	if msg := errorsJoined(errs); !strings.Contains(msg, "docs") {
		t.Errorf("error must name the field; got: %s", msg)
	}
}

// Verifies column-validation#ac:list-rejects-wrong-element-type.
func TestList_RejectsWrongElementType(t *testing.T) {
	t.Parallel()

	col := colDefWith("tags", ingitdb.ColumnDef{Type: "[]string"})

	errs := validateRecordData("test", "f.json", "r1", col, map[string]any{"tags": []any{"ok", 42}})
	if len(errs) == 0 {
		t.Fatal("expected an error for a non-string element in a []string column")
	}
	if msg := errorsJoined(errs); !strings.Contains(msg, "tags") {
		t.Errorf("error must name the field; got: %s", msg)
	}
}

func TestList_AcceptsHomogeneousList(t *testing.T) {
	t.Parallel()

	col := colDefWith("docs", ingitdb.ColumnDef{Type: "[]string"})

	if errs := validateRecordData("test", "f.json", "r1", col, map[string]any{"docs": []any{"a", "b"}}); len(errs) != 0 {
		t.Fatalf("expected no errors for a []string of strings; got: %s", errorsJoined(errs))
	}
}

// Verifies column-validation#ac:list-any-still-requires-a-list.
func TestList_AnyStillRequiresAList(t *testing.T) {
	t.Parallel()

	col := colDefWith("misc", ingitdb.ColumnDef{Type: "[]any"})

	if errs := validateRecordData("test", "f.json", "r1", col, map[string]any{"misc": []any{"x", 42, true}}); len(errs) != 0 {
		t.Fatalf("[]any accepts any element; got: %s", errorsJoined(errs))
	}
	if errs := validateRecordData("test", "f.json", "r1", col, map[string]any{"misc": "not-a-list"}); len(errs) == 0 {
		t.Fatal("[]any must still require the value to BE a list")
	}
}

// --- value-range-constraints ------------------------------------------------

func f64(v float64) *float64 { return &v }

// Verifies column-validation#ac:min-value-rejects-below-bound — and pins the
// declared-zero semantics: min_value: 0 is exactly what geo-ingitdb declares,
// so a plain float64 + "!= 0" guard would silently drop this constraint.
func TestValueRange_MinValueRejectsBelowBound_IncludingDeclaredZero(t *testing.T) {
	t.Parallel()

	col := colDefWith("population", ingitdb.ColumnDef{Type: ingitdb.ColumnTypeInt, MinValue: f64(0)})

	errs := validateRecordData("test", "f.json", "r1", col, map[string]any{"population": -5})
	if len(errs) == 0 {
		t.Fatal("min_value: 0 must be enforced; a declared zero is not 'unset'")
	}
	msg := errorsJoined(errs)
	for _, want := range []string{"population", "-5"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error must name %q; got: %s", want, msg)
		}
	}
}

// Verifies column-validation#ac:max-value-rejects-above-bound.
func TestValueRange_MaxValueRejectsAboveBound(t *testing.T) {
	t.Parallel()

	col := colDefWith("percent", ingitdb.ColumnDef{Type: ingitdb.ColumnTypeInt, MaxValue: f64(100)})

	errs := validateRecordData("test", "f.json", "r1", col, map[string]any{"percent": 101})
	if len(errs) == 0 {
		t.Fatal("expected an error for a value above max_value")
	}
	if msg := errorsJoined(errs); !strings.Contains(msg, "percent") || !strings.Contains(msg, "101") {
		t.Errorf("error must name the field and value; got: %s", msg)
	}
}

func TestValueRange_AcceptsInclusiveBounds(t *testing.T) {
	t.Parallel()

	col := colDefWith("n", ingitdb.ColumnDef{Type: ingitdb.ColumnTypeInt, MinValue: f64(0), MaxValue: f64(10)})

	for _, v := range []any{0, 5, 10} {
		if errs := validateRecordData("test", "f.json", "r1", col, map[string]any{"n": v}); len(errs) != 0 {
			t.Errorf("bounds are inclusive; %v should pass, got: %s", v, errorsJoined(errs))
		}
	}
}

func TestValueRange_UnsetBoundsDoNotConstrain(t *testing.T) {
	t.Parallel()

	col := colDefWith("n", ingitdb.ColumnDef{Type: ingitdb.ColumnTypeInt})

	if errs := validateRecordData("test", "f.json", "r1", col, map[string]any{"n": -99999}); len(errs) != 0 {
		t.Fatalf("no bounds declared, so any int passes; got: %s", errorsJoined(errs))
	}
}
