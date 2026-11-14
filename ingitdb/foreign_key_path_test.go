package ingitdb

// specscore: feature/column-validation

import (
	"strings"
	"testing"
)

func fkPathDefinition(fk string) *Definition {
	str := &ColumnDef{Type: ColumnTypeString}
	agreements := &CollectionDef{ID: "agreements", Columns: map[string]*ColumnDef{"title": str}}
	grants := &CollectionDef{ID: "grants", Columns: map[string]*ColumnDef{
		"spaceID":     str,
		"agreementID": {Type: ColumnTypeString, ForeignKey: fk},
	}}
	ext := &CollectionDef{ID: "ext", Columns: map[string]*ColumnDef{"title": str},
		SubCollections: map[string]*CollectionDef{"agreements": agreements, "grants": grants}}
	spaces := &CollectionDef{ID: "spaces", Columns: map[string]*ColumnDef{"title": str},
		SubCollections: map[string]*CollectionDef{"ext": ext}}
	other := &CollectionDef{ID: "other", Columns: map[string]*ColumnDef{"title": str}}
	return &Definition{Collections: map[string]*CollectionDef{"spaces": spaces, "other": other}}
}

// Verifies column-validation#ac:subcollection-foreign-key-checked-at-load.
func TestValidateForeignKeys_SubcollectionPaths(t *testing.T) {
	t.Parallel()
	valid := []string{
		"spaces/s1/ext/calendarius/agreements",
		"spaces/$same/ext/$same/agreements",
		"spaces/{spaceID}/ext/$same/agreements",
	}
	for _, fk := range valid {
		if err := ValidateForeignKeys(fkPathDefinition(fk)); err != nil {
			t.Errorf("%s: expected valid, got %v", fk, err)
		}
	}
	invalid := map[string]string{
		"spaces/s1":                                "must alternate collection ids and parent keys",
		"spaces//ext/x/agreements":                 "has an empty segment",
		"nowhere/s1/ext":                           `root collection "nowhere" does not resolve`,
		"spaces/s1/ext/x/missing":                  `"spaces/ext" has no subcollection "missing"`,
		"other/$same/ext":                          "$same after \"other\" is only allowed",
		"spaces/{nope}/ext/calendarius/agreements": `field "nope" is not a column of "spaces/ext/grants"`,
	}
	for fk, want := range invalid {
		err := ValidateForeignKeys(fkPathDefinition(fk))
		if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "column 'agreementID'") {
			t.Errorf("%s: expected error containing %q, got %v", fk, want, err)
		}
	}
}

func TestResolveForeignKeyPath_Steps(t *testing.T) {
	t.Parallel()
	def := fkPathDefinition("x")
	grants := def.Collections["spaces"].SubCollections["ext"].SubCollections["grants"]
	path, err := ResolveForeignKeyPath("spaces/ext/grants", "spaces/{spaceID}/ext/$same/agreements", grants, def.Collections)
	if err != nil {
		t.Fatal(err)
	}
	if path.Root != "spaces" || len(path.Steps) != 2 || path.Steps[0].Field != "spaceID" || !path.Steps[1].Same || path.Steps[1].Collection != "agreements" {
		t.Fatalf("unexpected path %+v", path)
	}
	if path.String() != "spaces/{spaceID}/ext/$same/agreements" {
		t.Fatalf("String() = %q", path.String())
	}
}
