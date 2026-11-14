package datavalidator

// specscore: feature/column-validation

import (
	"slices"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

func shapeColDef() *ingitdb.CollectionDef {
	two := 2
	one := 1.0
	slot := &ingitdb.ColumnDef{Type: "map[string]any", Columns: map[string]*ingitdb.ColumnDef{
		"durationInMinutes": {Type: ingitdb.ColumnTypeInt, Required: true, MinValue: &one},
		"weekdays":          {Type: "[]string", MaxLength: &two, Items: &ingitdb.ColumnDef{Type: ingitdb.ColumnTypeString, Enum: []any{"mo", "tu", "we"}}},
		"canceled":          {Type: "map[string]any", Columns: map[string]*ingitdb.ColumnDef{"at": {Type: ingitdb.ColumnTypeDateTime, Required: true}}},
	}}
	return &ingitdb.CollectionDef{Columns: map[string]*ingitdb.ColumnDef{
		"slots":   {Type: "map[string]any", Values: slot},
		"members": {Type: "[]any", Items: &ingitdb.ColumnDef{Type: "map[string]any", Columns: map[string]*ingitdb.ColumnDef{"role": {Type: ingitdb.ColumnTypeString, Enum: []any{"mentor", "member"}}}}},
		"ref":     {Type: ingitdb.ColumnTypeAny, Columns: map[string]*ingitdb.ColumnDef{"spaceID": {Type: ingitdb.ColumnTypeString, MinLength: &two}}},
		"plain":   {Type: "map[string]any"},
	}}
}

func findingMessages(errs []ingitdb.ValidationError) []string {
	var out []string
	for _, e := range errs {
		out = append(out, e.FieldName+": "+e.Message)
	}
	slices.Sort(out)
	return out
}

// Verifies column-validation#ac:nested-values-validated.
func TestColumnShape_ValuesChecked(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"slots": map[string]any{
			"sl1": map[string]any{"durationInMinutes": "x"},
			"sl2": map[string]any{"durationInMinutes": 0},
			"sl3": map[string]any{"weekdays": []any{"mo", "fr", "tu"}},
			"sl4": map[string]any{"durationInMinutes": 30, "weekdays": []any{"sa"}, "canceled": map[string]any{}, "extra": 1, "$meta": 1},
			"sl5": "not an object",
		},
		"members": []any{map[string]any{"role": "mentor"}, map[string]any{"role": "boss"}, nil},
		"ref":     map[string]any{"spaceID": "k"},
		"plain":   map[string]any{"anything": []any{1, "x"}},
	}
	got := findingMessages(ValidateRecordData(shapeColDef(), "r1", data))
	want := []string{
		`members[1].role: value boss for field "members[1].role" is not one of the permitted values: mentor, member`,
		`ref.spaceID: length 1 for field "ref.spaceID" is below min_length 2`,
		`slots.sl1.durationInMinutes: wrong type for field "slots.sl1.durationInMinutes": expected int, got string`,
		`slots.sl2.durationInMinutes: value 0 for field "slots.sl2.durationInMinutes" is below min_value 1`,
		`slots.sl3.durationInMinutes: missing required field`,
		`slots.sl3.weekdays: length 3 for field "slots.sl3.weekdays" is above max_length 2`,
		`slots.sl4.canceled.at: missing required field`,
		`slots.sl4.extra: undeclared field "slots.sl4.extra": no nested column of that name in the definition of "slots.sl4"`,
		`slots.sl4.weekdays[0]: value sa for field "slots.sl4.weekdays[0]" is not one of the permitted values: mo, tu, we`,
		`slots.sl5: wrong type for field "slots.sl5": expected map[string]any, got string`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("findings:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// A null list element is not a missing required field unless the item
// definition says required; a non-required nil nested value is skipped.
func TestColumnShape_NilValues(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{Columns: map[string]*ingitdb.ColumnDef{
		"tags": {Type: "[]any", Items: &ingitdb.ColumnDef{Type: ingitdb.ColumnTypeString, Required: true}},
		"obj":  {Type: "map[string]any", Columns: map[string]*ingitdb.ColumnDef{"opt": {Type: ingitdb.ColumnTypeString}}},
	}}
	got := findingMessages(ValidateRecordData(colDef, "r", map[string]any{"tags": []any{"a", nil}, "obj": map[string]any{"opt": nil}}))
	if len(got) != 1 || got[0] != "tags[1]: missing required field" {
		t.Errorf("got %v", got)
	}
}

// Verifies column-validation#ac:undeclared-shape-unchanged.
// A column without Columns/Values/Items checks exactly what it did before.
func TestColumnShape_NoShapeNoChange(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{Columns: map[string]*ingitdb.ColumnDef{
		"slots": {Type: "map[string]any"},
		"list":  {Type: "[]any"},
		"obj":   {Type: ingitdb.ColumnTypeAny, Columns: map[string]*ingitdb.ColumnDef{"a": {Type: ingitdb.ColumnTypeInt}}},
	}}
	data := map[string]any{"slots": map[string]any{"sl1": map[string]any{"durationInMinutes": "x"}}, "list": []any{1, "x"}, "obj": "a string, not an object"}
	if errs := ValidateRecordData(colDef, "r", data); len(errs) != 0 {
		t.Errorf("expected no findings, got %v", errs)
	}
}
