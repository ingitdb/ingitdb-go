package datavalidator

import (
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

func TestCollectionForRecordFile_EdgeCases(t *testing.T) {
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"skip": {
				RecordFile: nil, // shouldSkipRecordParsing == true
			},
			"bad_pattern": {
				RecordFile: &ingitdb.RecordFileDef{
					RecordType: ingitdb.SingleRecord,
					Format:     "unknown_format",
				},
			},
		},
	}
	id, col := CollectionForRecordFile(def, "/any/path.yaml")
	if id != "" || col != nil {
		t.Errorf("expected empty result, got id=%q, col=%v", id, col)
	}
}

// A changed file resolves into a subcollection instance only through a
// declared subcollection id below a parent name; a nil collection, an
// unknown record type and a relative data directory resolve nothing.
func TestResolveRecordFile_Edges(t *testing.T) {
	t.Parallel()
	def := plainDB("/db")
	def.Collections["nil"] = nil
	def.Collections["odd"] = &ingitdb.CollectionDef{ID: "odd", DirPath: "/db/odd", RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: ingitdb.RecordFormatYAML, RecordType: "odd"}}
	def.Collections["rel"] = &ingitdb.CollectionDef{ID: "rel", DirPath: "rel", RecordFile: plainRecordFile()}
	for path, want := range map[string]string{
		"/db/c/$records/a.yaml":                       "c",
		"/db/c/$records/p/notes/$records/n.yaml":      "c/notes",
		"/db/c/$records/p/undeclared/$records/n.yaml": "",
		"/db/c/$records/p/notes":                      "",
		"/db/odd/$records/a.yaml":                     "",
		"/elsewhere/a.yaml":                           "",
	} {
		levels := resolveRecordFile(def, path)
		got := ""
		if levels != nil {
			got = levels[len(levels)-1].id
		}
		if got != want {
			t.Errorf("%s: %q, want %q", path, got, want)
		}
	}
	if id, col := CollectionForRecordFile(def, "/db/c/$records/a.yaml"); id != "c" || col == nil {
		t.Errorf("root record: %q", id)
	}
}
