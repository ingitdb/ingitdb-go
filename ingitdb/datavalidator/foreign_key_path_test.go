package datavalidator

// specscore: feature/column-validation

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	ingitdb "github.com/ingitdb/ingitdb-go/ingitdb"
)

func writeMapRecords(t *testing.T, dir, recordsYAML string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data.yaml"), []byte(recordsYAML), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mapRecordFile() *ingitdb.RecordFileDef {
	return &ingitdb.RecordFileDef{Name: "data.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.MapOfRecords}
}

// Verifies column-validation#ac:subcollection-foreign-key-literal-parent,
// #ac:subcollection-foreign-key-same-parent and
// #ac:subcollection-foreign-key-parent-from-field within this package.
func TestForeignKeyReferences_SubcollectionPaths(t *testing.T) {
	dir := t.TempDir()
	str := &ingitdb.ColumnDef{Type: ingitdb.ColumnTypeString}
	anyCol := func(fk string) *ingitdb.ColumnDef {
		return &ingitdb.ColumnDef{Type: ingitdb.ColumnTypeAny, ForeignKey: fk}
	}

	writeMapRecords(t, filepath.Join(dir, "spaces"), "s1:\n  title: S1\ns2:\n  title: S2\n")
	writeMapRecords(t, filepath.Join(dir, "spaces", "s1", "agreements"), "a1:\n  title: A1\n")
	writeMapRecords(t, filepath.Join(dir, "spaces", "s1", "grants"), "g1:\n  agreementID: a1\ng2:\n  agreementID: [a1, ghost, [nested]]\ng3:\n  agreementID: []\ng4:\n  agreementID: null\n")
	writeMapRecords(t, filepath.Join(dir, "spaces", "s2", "grants"), "g5:\n  agreementID: a1\n")
	writeMapRecords(t, filepath.Join(dir, "bookings"), "b1:\n  spaceID: s1\n  agreementID: a1\nb2:\n  spaceID: s2\n  agreementID: a1\nb3:\n  agreementID: a1\nb4:\n  spaceID: s1\n  literal: a2\nb5:\n  spaceID: ../../evil\n  agreementID: a1\n")

	agreements := &ingitdb.CollectionDef{ID: "agreements", RecordFile: mapRecordFile(), Columns: map[string]*ingitdb.ColumnDef{"title": str}}
	grants := &ingitdb.CollectionDef{ID: "grants", RecordFile: mapRecordFile(), Columns: map[string]*ingitdb.ColumnDef{
		"agreementID": anyCol("spaces/$same/agreements"),
	}}
	spaces := &ingitdb.CollectionDef{ID: "spaces", DirPath: filepath.Join(dir, "spaces"), RecordFile: mapRecordFile(),
		Columns:        map[string]*ingitdb.ColumnDef{"title": str},
		SubCollections: map[string]*ingitdb.CollectionDef{"agreements": agreements, "grants": grants}}
	bookings := &ingitdb.CollectionDef{ID: "bookings", DirPath: filepath.Join(dir, "bookings"), RecordFile: mapRecordFile(),
		Columns: map[string]*ingitdb.ColumnDef{
			"spaceID":     str,
			"agreementID": {Type: ingitdb.ColumnTypeString, ForeignKey: "spaces/{spaceID}/agreements"},
			"literal":     {Type: ingitdb.ColumnTypeString, ForeignKey: "spaces/s1/agreements"},
			"unresolved":  {Type: ingitdb.ColumnTypeString, ForeignKey: "spaces/s1/missing"},
		}}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"spaces": spaces, "bookings": bookings}}

	res, err := NewValidator().Validate(context.Background(), dir, def)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range res.Errors() {
		got = append(got, e.Message)
	}
	slices.Sort(got)
	want := []string{
		`foreign key "agreementID" = "a1" has no matching record in collection "spaces/s2/agreements"`,
		`foreign key "agreementID" = "a1" has no matching record in collection "spaces/s2/agreements"`,
		`foreign key "agreementID" = "a1": field "spaceID", which gives the parent key in "spaces/{spaceID}/agreements", is empty`,
		`foreign key "agreementID" = "a1": field "spaceID", which gives the parent key in "spaces/{spaceID}/agreements", is "../../evil", which is not a valid record key`,
		`foreign key "agreementID" = "ghost" has no matching record in collection "spaces/s1/agreements"`,
		`foreign key "agreementID" has a list element that is not a scalar ([nested]): foreign key element must be a scalar`,
		`foreign key "literal" = "a2" has no matching record in collection "spaces/s1/agreements"`,
	}
	slices.Sort(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("findings:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}	// Every path foreign-key finding names the record file it is about.
	for _, e := range res.Errors() {
		if filepath.Base(e.FilePath) != "data.yaml" || !strings.HasPrefix(e.FilePath, dir) {
			t.Errorf("finding %q has FilePath %q, want the record file", e.Message, e.FilePath)
		}
	}
}

// A {field} value becomes a parent key only when it is a plain record key;
// numbers are written as they read, not in exponent form.
func TestParentKeyFromField(t *testing.T) {
	t.Parallel()
	tests := []struct {
		raw     any
		key     string
		problem string
	}{
		{"s1", "s1", ""},
		{float64(1000000), "1000000", ""},
		{1.5, "1.5", ""},
		{7, "7", ""},
		{true, "true", ""},
		{nil, "", "is empty"},
		{"", "", "is empty"},
		{[]any{"s1"}, "", "must be a scalar"},
		{map[string]any{"a": 1}, "", "must be a scalar"},
		{"../../evil", "", `is "../../evil", which is not a valid record key`},
		{"s1/items/i1", "", `is "s1/items/i1", which is not a valid record key`},
		{"..", "", `is "..", which is not a valid record key`},
		{`a\b`, "", `is "a\\b", which is not a valid record key`},
	}
	for _, tt := range tests {
		key, problem := parentKeyFromField(tt.raw)
		if key != tt.key || problem != tt.problem {
			t.Errorf("%#v: got (%q, %q), want (%q, %q)", tt.raw, key, problem, tt.key, tt.problem)
		}
	}
}
