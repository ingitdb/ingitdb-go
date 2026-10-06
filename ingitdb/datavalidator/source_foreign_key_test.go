package datavalidator

import (
	"context"
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
	children.SourceSchema = &ingitdb.SourceSchemaDef{ForeignKeys: []ingitdb.SourceForeignKeyDef{{
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
