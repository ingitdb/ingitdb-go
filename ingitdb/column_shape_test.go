package ingitdb

// specscore: feature/column-validation

import (
	"strings"
	"testing"
)

// Verifies column-validation#ac:nested-shape-rejected-where-meaningless.
func TestColumnShape_RejectedAtLoad(t *testing.T) {
	t.Parallel()
	str := &ColumnDef{Type: ColumnTypeString}
	tests := []struct {
		name string
		def  ColumnDef
		want string
	}{
		{"columns on string", ColumnDef{Type: ColumnTypeString, Columns: map[string]*ColumnDef{"a": str}}, "'columns' requires a map type or any"},
		{"columns on l10n", ColumnDef{Type: ColumnTypeL10N, Columns: map[string]*ColumnDef{"a": str}}, "'columns' requires a map type or any"},
		{"values on list", ColumnDef{Type: "[]any", Values: str}, "'values' requires a map type"},
		{"items on map", ColumnDef{Type: "map[string]any", Items: str}, "'items' requires a list type"},
		{"nil nested column", ColumnDef{Type: "map[string]any", Columns: map[string]*ColumnDef{"a": nil}}, "invalid nested column 'a': missing definition"},
		{"bad nested type", ColumnDef{Type: "map[string]any", Columns: map[string]*ColumnDef{"a": {Type: "number"}}}, "unknown column type 'number'"},
		{"nested formula", ColumnDef{Type: "map[string]any", Values: &ColumnDef{Type: ColumnTypeInt, Formula: "1"}}, "invalid 'values': 'formula' is not supported"},
		{"nested required_when", ColumnDef{Type: "[]any", Items: &ColumnDef{Type: ColumnTypeInt, RequiredWhen: "True"}}, "invalid 'items': 'required_when' is not supported"},
		{"nested foreign_key", ColumnDef{Type: "[]any", Items: &ColumnDef{Type: ColumnTypeString, ForeignKey: "users"}}, "'foreign_key' is not supported"},
		{"nested locale", ColumnDef{Type: "any", Columns: map[string]*ColumnDef{"t": {Type: ColumnTypeString, Locale: "en"}}}, "'locale' is not supported"},
		{"deep invalid", ColumnDef{Type: "map[string]any", Values: &ColumnDef{Type: "map[string]any", Columns: map[string]*ColumnDef{"x": {Type: ColumnTypeInt, Items: str}}}}, "'items' requires a list type"},
	}
	for _, tt := range tests {
		err := tt.def.Validate()
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: expected error containing %q, got %v", tt.name, tt.want, err)
		}
	}
}

// Verifies column-validation#ac:nested-shape-declared.
func TestColumnShape_AcceptedAtLoad(t *testing.T) {
	t.Parallel()
	slot := &ColumnDef{Type: "map[string]any", Columns: map[string]*ColumnDef{
		"durationInMinutes": {Type: ColumnTypeInt, Required: true},
		"weekdays":          {Type: "[]string", Items: &ColumnDef{Type: ColumnTypeString, Enum: []any{"mo", "tu"}}},
	}}
	defs := []ColumnDef{
		{Type: "map[string]any", Values: slot},
		{Type: ColumnTypeAny, Columns: map[string]*ColumnDef{"a": {Type: ColumnTypeString}}},
		{Type: "[]any", Items: slot},
		{Type: "map[string]string", Values: &ColumnDef{Type: ColumnTypeString, Enum: []any{"attended"}}},
	}
	for i, def := range defs {
		if err := def.Validate(); err != nil {
			t.Errorf("def %d: %v", i, err)
		}
	}
}

func TestForeignKeyPath_StringAndIsPath(t *testing.T) {
	t.Parallel()
	path := ForeignKeyPath{Root: "spaces", Steps: []ForeignKeyPathStep{
		{Same: true, Collection: "ext"},
		{Field: "extID", Collection: "happenings"},
		{Literal: "h1", Collection: "priceRevisions"},
	}}
	if got := path.String(); got != "spaces/$same/ext/{extID}/happenings/h1/priceRevisions" {
		t.Errorf("String() = %q", got)
	}
	if IsForeignKeyPath("users") || !IsForeignKeyPath("ext/commitius/cmSessions") {
		t.Error("IsForeignKeyPath must be true exactly for values with a slash")
	}
	if !hasPrefixPath([]string{"a", "b"}, []string{"a"}) || hasPrefixPath([]string{"a"}, []string{"a", "b"}) || hasPrefixPath([]string{"a", "b"}, []string{"c"}) {
		t.Error("hasPrefixPath")
	}
}
