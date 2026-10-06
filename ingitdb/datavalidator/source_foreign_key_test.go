package datavalidator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ingitdb "github.com/ingitdb/ingitdb-go/ingitdb"
)

func TestSourceForeignKeysValidateCompositeTargetColumns(t *testing.T) {
	dir := t.TempDir()
	parents := writeMapCollectionJSON(t, dir, "parents", `{"p1":{"code":"A","revision":1},"p2":{"code":"B","revision":2}}`, map[string]*ingitdb.ColumnDef{
		"code": {Type: ingitdb.ColumnTypeString}, "revision": {Type: ingitdb.ColumnTypeInt},
	})
	children := writeMapCollectionJSON(t, dir, "children", `{"c1":{"parent_code":"A","parent_revision":1},"c2":{"parent_code":"A","parent_revision":2},"c3":{"parent_code":null,"parent_revision":9}}`, map[string]*ingitdb.ColumnDef{
		"parent_code": {Type: ingitdb.ColumnTypeString}, "parent_revision": {Type: ingitdb.ColumnTypeInt},
	})
	children.SourceSchema = &ingitdb.SourceSchemaDef{RelationshipComparison: "raw", ForeignKeys: []ingitdb.SourceForeignKeyDef{{
		Fields: []string{"parent_code", "parent_revision"}, ReferencedCollection: "parents",
		ReferencedFields: []string{"code", "revision"}, SourceEnforcement: "disabled",
	}}}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"parents": parents, "children": children}}
	result, err := NewValidator().Validate(context.Background(), dir, def)
	if err != nil {
		t.Fatal(err)
	}
	if errs := result.Errors(); len(errs) != 1 || errs[0].RecordKey != "c2" || !strings.Contains(errs[0].Error(), "parents") {
		t.Fatalf("want only composite orphan c2; got %v", errs)
	}
}

func TestSourceForeignKeysDoNotMisjudgeNativeSQLCollation(t *testing.T) {
	dir := t.TempDir()
	parent := writeMapCollectionJSON(t, dir, "p", `{"p1":{"k":"A"}}`, map[string]*ingitdb.ColumnDef{"k": {Type: ingitdb.ColumnTypeString}})
	child := writeMapCollectionJSON(t, dir, "c", `{"c1":{"v":"a"}}`, map[string]*ingitdb.ColumnDef{"v": {Type: ingitdb.ColumnTypeString}})
	child.SourceSchema = &ingitdb.SourceSchemaDef{RelationshipComparison: "provider-native", SourceDefinitionJSON: `{"dialect":"sqlite","createSql":"CREATE TABLE c(v TEXT REFERENCES p(k))"}`,
		ForeignKeys: []ingitdb.SourceForeignKeyDef{{Fields: []string{"v"}, ReferencedCollection: "p", ReferencedFields: []string{"k"}}}}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"p": parent, "c": child}}
	result, err := NewValidator().Validate(context.Background(), dir, def)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors()) != 0 {
		t.Fatalf("source SQL semantics must not be reinterpreted as raw native equality: %v", result.Errors())
	}
}

func TestSourceForeignKeyValidationErrorsAndTuples(t *testing.T) {
	if got := validateSourceForeignKeys(nil); got != nil {
		t.Fatalf("nil definition: %v", got)
	}
	dir := t.TempDir()
	parent := writeMapCollectionJSON(t, dir, "p", `{"p1":{"k":1}}`, map[string]*ingitdb.ColumnDef{"k": {Type: ingitdb.ColumnTypeInt}})
	child := writeMapCollectionJSON(t, dir, "c", `{"c1":{"v":1}}`, map[string]*ingitdb.ColumnDef{"v": {Type: ingitdb.ColumnTypeInt}})
	child.SourceSchema = &ingitdb.SourceSchemaDef{RelationshipComparison: "raw", ForeignKeys: []ingitdb.SourceForeignKeyDef{{Name: "missing", Fields: []string{"v"}, ReferencedCollection: "nope"}, {Name: "arity", Fields: []string{"v"}, ReferencedCollection: "p"}, {Name: "match", Fields: []string{"v"}, ReferencedCollection: "p", ReferencedFields: []string{"k"}}}}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"p": parent, "c": child}}
	findings := validateSourceForeignKeys(def)
	if len(findings) != 2 {
		t.Fatalf("expected missing target and unresolved tuple: %v", findings)
	}
	parent.PrimaryKey = []string{"k"}
	findings = validateSourceForeignKeys(def)
	if len(findings) != 1 {
		t.Fatalf("PK fallback should resolve tuple: %v", findings)
	}
	// Bad child and target records are handled by the ordinary record parser.
	if err := os.WriteFile(filepath.Join(child.DirPath, "data.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := validateSourceForeignKeys(def); len(got) != 0 {
		t.Fatalf("bad child records: %v", got)
	}
	if err := os.WriteFile(filepath.Join(child.DirPath, "data.json"), []byte(`{"c1":{"v":1}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent.DirPath, "data.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := validateSourceForeignKeys(def); len(got) != 1 {
		t.Fatalf("bad target records: %v", got)
	}
}

func TestRelationalTupleTypes(t *testing.T) {
	cases := []struct {
		v      any
		schema *ingitdb.SourceSchemaDef
		valid  bool
	}{
		{nil, nil, false},
		{int(2), nil, true},
		{int64(2), nil, true},
		{float64(2.5), nil, true},
		{"2.5", &ingitdb.SourceSchemaDef{Fields: []ingitdb.SourceFieldDef{{Name: "x", Type: "decimal"}}}, true},
		{"text", nil, true},
		{func() {}, nil, false},
	}
	for _, tc := range cases {
		_, valid := relationalTuple(map[string]any{"x": tc.v}, []string{"x"}, tc.schema)
		if valid != tc.valid {
			t.Fatalf("%T valid=%v want %v", tc.v, valid, tc.valid)
		}
	}
	if _, valid := relationalTuple(map[string]any{}, []string{"x"}, nil); valid {
		t.Fatal("missing field must skip tuple")
	}
}
