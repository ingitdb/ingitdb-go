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

// REQ:foreign-key-list-elements — a `type: any` or list-typed FK column
// (event_ids: [a, b]) is checked element by element: before this, the whole
// list stringified to "[a b]" (fmt.Sprintf("%v", raw)), which never matched
// any real key, so every such record failed validation even when every
// element referenced a real record. Table-driven per founder direction.
func TestForeignKeyReferences_ListValuedColumn(t *testing.T) {
	writeEvents := func(t *testing.T, dir string) *ingitdb.CollectionDef {
		return writeMapCollection(t, dir, "events",
			"e1:\n  name: E1\ne2:\n  name: E2\n",
			map[string]*ingitdb.ColumnDef{"name": {Type: ingitdb.ColumnTypeString}})
	}

	tests := []struct {
		name           string
		plotlinesYAML  string
		wantErrCount   int
		wantErrSubstrs []string // checked against the single error when wantErrCount == 1
	}{
		{
			name:          "all elements ok",
			plotlinesYAML: "p1:\n  title: T1\n  event_ids: [e1, e2]\n",
			wantErrCount:  0,
		},
		{
			name:           "one missing element names only that element",
			plotlinesYAML:  "p1:\n  title: T1\n  event_ids: [e1, no-such-event]\n",
			wantErrCount:   1,
			wantErrSubstrs: []string{"event_ids", "no-such-event", "events"},
		},
		{
			name:          "empty list is clean",
			plotlinesYAML: "p1:\n  title: T1\n  event_ids: []\n",
			wantErrCount:  0,
		},
		{
			name:          "list with a nil element skips the nil",
			plotlinesYAML: "p1:\n  title: T1\n  event_ids: [e1, null]\n",
			wantErrCount:  0,
		},
		{
			name:           "nested non-scalar element is an error",
			plotlinesYAML:  "p1:\n  title: T1\n  event_ids: [e1, [x, y]]\n",
			wantErrCount:   1,
			wantErrSubstrs: []string{"event_ids", "foreign key element must be a scalar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			events := writeEvents(t, dir)
			plotlines := writeMapCollection(t, dir, "plotlines", tt.plotlinesYAML,
				map[string]*ingitdb.ColumnDef{
					"title":     {Type: ingitdb.ColumnTypeString},
					"event_ids": {Type: ingitdb.ColumnTypeAny, ForeignKey: "events"},
				})
			def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{
				"events": events, "plotlines": plotlines,
			}}
			res, err := NewValidator().Validate(context.Background(), dir, def)
			if err != nil {
				t.Fatal(err)
			}
			errs := res.Errors()
			if len(errs) != tt.wantErrCount {
				t.Fatalf("expected %d error(s), got %d: %v", tt.wantErrCount, len(errs), errs)
			}
			if tt.wantErrCount == 1 {
				msg := errs[0].Error()
				for _, want := range tt.wantErrSubstrs {
					if !strings.Contains(msg, want) {
						t.Errorf("error must mention %q, got: %v", want, msg)
					}
				}
			}
		})
	}
}
