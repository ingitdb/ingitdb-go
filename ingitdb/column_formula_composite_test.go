package ingitdb

import (
	"strings"
	"testing"
)

// List and map columns must be usable from an expression. Found by pointing
// required_when at the can-i-use pilot: every record failed with "unsupported
// field type []interface {}" because goToStarlark had no composite cases, even
// though the expression (state == "absent") never referenced the list column.
// Every stored column is bound, so one unconvertible column broke every
// expression in the collection.
func TestEvaluateFormula_CompositeFields(t *testing.T) {
	cases := []struct {
		name    string
		formula string
		fields  map[string]any
		want    any
	}{
		{
			name:    "len of a []any list (JSON-decoded shape)",
			formula: `len(docs) > 0`,
			fields:  map[string]any{"docs": []any{"https://example.test"}},
			want:    true,
		},
		{
			name:    "len of a []string list (Go-native shape)",
			formula: `len(tags) > 0`,
			fields:  map[string]any{"tags": []string{"a", "b"}},
			want:    true,
		},
		{
			name:    "empty list is falsy by length",
			formula: `len(docs) > 0`,
			fields:  map[string]any{"docs": []any{}},
			want:    false,
		},
		{
			name:    "index into a map[string]any column",
			formula: `constraints["maxButtons"] == 3`,
			fields:  map[string]any{"constraints": map[string]any{"maxButtons": 3}},
			want:    true,
		},
		{
			name:    "map membership",
			formula: `"grid" in constraints`,
			fields:  map[string]any{"constraints": map[string]any{"grid": false}},
			want:    true,
		},
		{
			name:    "map[string]string column",
			formula: `titles["en"] == "Countries"`,
			fields:  map[string]any{"titles": map[string]string{"en": "Countries"}},
			want:    true,
		},
		{
			name:    "list membership",
			formula: `"pick-one-of-n" in jobs`,
			fields:  map[string]any{"jobs": []any{"pick-one-of-n", "confirm-action"}},
			want:    true,
		},
		{
			name:    "nested composite",
			formula: `constraints["headerTypes"][0] == "text"`,
			fields:  map[string]any{"constraints": map[string]any{"headerTypes": []any{"text", "image"}}},
			want:    true,
		},
		{
			name:    "an unreferenced composite column does not break the expression",
			formula: `state == "absent"`,
			fields: map[string]any{
				"state":       "absent",
				"docs":        []any{"https://example.test"},
				"constraints": map[string]any{"maxButtons": 3},
			},
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := EvaluateFormula(tc.formula, tc.fields)
			if err != nil {
				t.Fatalf("EvaluateFormula(%q): %v", tc.formula, err)
			}
			if got != tc.want {
				t.Errorf("EvaluateFormula(%q) = %v (%T), want %v", tc.formula, got, got, tc.want)
			}
		})
	}
}

// Composites are bound as shared predeclared values, so they must be frozen:
// otherwise one evaluation could mutate a value a later one observes, and the
// sandbox stops being side-effect-free.
func TestEvaluateFormula_CompositesAreFrozen(t *testing.T) {
	_, err := EvaluateFormula(`docs.append("x") or True`, map[string]any{"docs": []any{"a"}})
	if err == nil {
		t.Fatal("expected mutating a bound list to fail (frozen), got nil")
	}
	if !strings.Contains(err.Error(), "frozen") && !strings.Contains(err.Error(), "immutable") {
		t.Errorf("expected a frozen/immutable error, got: %v", err)
	}
}

// A value with no Starlark equivalent is still an error rather than a silent
// None: the element type is named so the offending column can be found.
func TestEvaluateFormula_RejectsUnconvertibleElement(t *testing.T) {
	_, err := EvaluateFormula(`len(xs)`, map[string]any{"xs": []any{struct{ A int }{1}}})
	if err == nil {
		t.Fatal("expected an error for an unconvertible list element")
	}
	if !strings.Contains(err.Error(), "element 0") {
		t.Errorf("error must locate the element, got: %v", err)
	}
}
