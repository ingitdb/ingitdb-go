package datavalidator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ingitdb "github.com/ingitdb/ingitdb-go/ingitdb"
)

// writeMapCollection writes a MapOfRecords YAML collection and returns its
// CollectionDef pointed at the temp dir.
func writeMapCollection(t *testing.T, dir, id, recordsYAML string, cols map[string]*ingitdb.ColumnDef) *ingitdb.CollectionDef {
	t.Helper()
	colDir := filepath.Join(dir, id)
	if err := os.MkdirAll(colDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(colDir, "data.yaml"), []byte(recordsYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	return &ingitdb.CollectionDef{
		ID:      id,
		DirPath: colDir,
		RecordFile: &ingitdb.RecordFileDef{
			Name: "data.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.MapOfRecords,
		},
		Columns: cols,
	}
}

// REQ:foreign-key-enforced (record level) — an FK value with no matching key in
// the target collection is an error naming the field, the value, and the target.
func TestForeignKeyReferences_RejectsDanglingValue(t *testing.T) {
	dir := t.TempDir()
	authors := writeMapCollection(t, dir, "authors",
		"ada:\n  name: Ada\ngrace:\n  name: Grace\n",
		map[string]*ingitdb.ColumnDef{"name": {Type: ingitdb.ColumnTypeString}})
	books := writeMapCollection(t, dir, "books",
		"b1:\n  title: T1\n  author: ada\nb2:\n  title: T2\n  author: nobody\n",
		map[string]*ingitdb.ColumnDef{
			"title":  {Type: ingitdb.ColumnTypeString},
			"author": {Type: ingitdb.ColumnTypeString, ForeignKey: "authors"},
		})

	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{
		"authors": authors, "books": books,
	}}
	res, err := NewValidator().Validate(context.Background(), dir, def)
	if err != nil {
		t.Fatal(err)
	}
	errs := res.Errors()
	if len(errs) != 1 {
		t.Fatalf("expected 1 dangling-FK error, got %d: %v", len(errs), errs)
	}
	msg := errs[0].Error()
	for _, want := range []string{"author", "nobody", "authors"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error must mention %q, got: %v", want, msg)
		}
	}
}

// Every FK value pointing at a real key validates clean.
func TestForeignKeyReferences_AcceptsValidValues(t *testing.T) {
	dir := t.TempDir()
	authors := writeMapCollection(t, dir, "authors",
		"ada:\n  name: Ada\n",
		map[string]*ingitdb.ColumnDef{"name": {Type: ingitdb.ColumnTypeString}})
	books := writeMapCollection(t, dir, "books",
		"b1:\n  title: T1\n  author: ada\n",
		map[string]*ingitdb.ColumnDef{
			"title":  {Type: ingitdb.ColumnTypeString},
			"author": {Type: ingitdb.ColumnTypeString, ForeignKey: "authors"},
		})
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"authors": authors, "books": books}}
	res, err := NewValidator().Validate(context.Background(), dir, def)
	if err != nil {
		t.Fatal(err)
	}
	if errs := res.Errors(); len(errs) != 0 {
		t.Errorf("valid FK values must pass, got: %v", errs)
	}
}

// An absent FK value is not an integrity error — that is a required/optional
// concern owned by the schema pass.
func TestForeignKeyReferences_AbsentValueIsNotDangling(t *testing.T) {
	dir := t.TempDir()
	authors := writeMapCollection(t, dir, "authors", "ada:\n  name: Ada\n",
		map[string]*ingitdb.ColumnDef{"name": {Type: ingitdb.ColumnTypeString}})
	books := writeMapCollection(t, dir, "books",
		"b1:\n  title: T1\n", // no author
		map[string]*ingitdb.ColumnDef{
			"title":  {Type: ingitdb.ColumnTypeString},
			"author": {Type: ingitdb.ColumnTypeString, ForeignKey: "authors"},
		})
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"authors": authors, "books": books}}
	res, err := NewValidator().Validate(context.Background(), dir, def)
	if err != nil {
		t.Fatal(err)
	}
	if errs := res.Errors(); len(errs) != 0 {
		t.Errorf("an absent FK value must not be an integrity error, got: %v", errs)
	}
}

// Module-relative: a bare foreign_key resolves within the declaring module, and
// the value is checked against that module's collection — not a same-named
// collection in another module.
func TestForeignKeyReferences_ModuleRelativeTarget(t *testing.T) {
	dir := t.TempDir()
	// commerce.countries has 'us'; geo.countries has 'de'. books.country=us
	// must check commerce.countries (resolved module-relative), so 'us' is
	// valid and 'de' would not be.
	commerceCountries := writeMapCollection(t, dir, "commerce.countries", "us:\n  name: USA\n",
		map[string]*ingitdb.ColumnDef{"name": {Type: ingitdb.ColumnTypeString}})
	geoCountries := writeMapCollection(t, dir, "geo.countries", "de:\n  name: Germany\n",
		map[string]*ingitdb.ColumnDef{"name": {Type: ingitdb.ColumnTypeString}})
	addresses := writeMapCollection(t, dir, "commerce.addresses",
		"a1:\n  country: us\na2:\n  country: de\n",
		map[string]*ingitdb.ColumnDef{
			"country": {Type: ingitdb.ColumnTypeString, ForeignKey: "countries"},
		})
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{
		"commerce.countries": commerceCountries,
		"geo.countries":      geoCountries,
		"commerce.addresses": addresses,
	}}
	res, err := NewValidator().Validate(context.Background(), dir, def)
	if err != nil {
		t.Fatal(err)
	}
	errs := res.Errors()
	// a1 (us) valid against commerce.countries; a2 (de) dangling — de is only in
	// geo.countries, which module-relative resolution does not reach.
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 error (a2/de dangling against commerce.countries), got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "commerce.countries") || !strings.Contains(errs[0].Error(), "de") {
		t.Errorf("error must show de dangling against commerce.countries, got: %v", errs[0])
	}
}
