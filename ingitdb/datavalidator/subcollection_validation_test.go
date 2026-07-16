package datavalidator

import (
	"path/filepath"
	"testing"

	ingitdb "github.com/ingitdb/ingitdb-go/ingitdb"
)

// subCollectionDataDir implements the documented storage convention:
//
//	<parent DirPath>/<parent records-base-path>/<parentKey>/<subID>/
//
// The records-base-path is "$records" when the parent's record_file.name
// contains "{key}" (per-key files) and empty otherwise (all records in one
// file). Verifies subcollection-record-validation#req:subcollection-storage-convention.
func TestSubCollectionDataDir_Convention(t *testing.T) {
	t.Run("per-key-file parent uses $records base", func(t *testing.T) {
		parent := &ingitdb.CollectionDef{
			DirPath:    "/db/orders",
			RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.SingleRecord},
		}
		got := subCollectionDataDir(parent, "ord001", "order_details")
		want := filepath.Join("/db/orders", "$records", "ord001", "order_details")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("single-file parent uses empty base", func(t *testing.T) {
		parent := &ingitdb.CollectionDef{
			DirPath:    "/db/orders",
			RecordFile: &ingitdb.RecordFileDef{Name: "orders.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.MapOfRecords},
		}
		got := subCollectionDataDir(parent, "ord001", "order_details")
		want := filepath.Join("/db/orders", "ord001", "order_details")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

// walkSubCollectionInstances yields one instance per (subcollection, parent
// record), repointing each instance's DirPath at its on-disk data directory,
// and recurses to arbitrary depth. Here a parent map-collection with two
// records and one declared subcollection must yield two instances with distinct
// data dirs. Verifies subcollection-record-validation#req:subcollection-records-schema-validated.
func TestWalkSubCollectionInstances_PerParentInstances(t *testing.T) {
	dir := t.TempDir()
	// Parent: a map-of-records collection with two records p1, p2.
	parent := writeMapCollection(t, dir, "orders",
		"p1:\n  name: One\np2:\n  name: Two\n",
		map[string]*ingitdb.ColumnDef{"name": {Type: ingitdb.ColumnTypeString}})
	// One declared subcollection "lines" (definition only; its loaded DirPath is
	// irrelevant — the walk repoints it per parent record).
	parent.SubCollections = map[string]*ingitdb.CollectionDef{
		"lines": {
			ID:         "lines",
			DirPath:    "/wrong/schema/path",
			RecordFile: &ingitdb.RecordFileDef{Name: "lines.json", Format: ingitdb.RecordFormatJSON, RecordType: ingitdb.ListOfRecords},
			Columns:    map[string]*ingitdb.ColumnDef{"qty": {Type: ingitdb.ColumnTypeInt}},
		},
	}

	var got []subCollectionInstance
	walkSubCollectionInstances("orders", parent, func(inst subCollectionInstance) {
		got = append(got, inst)
	})

	if len(got) != 2 {
		t.Fatalf("expected 2 instances (one per parent record), got %d", len(got))
	}
	// Deterministic order: parents visited sorted by key.
	wantP1 := filepath.Join(dir, "orders", "p1", "lines") // map parent => empty records-base
	wantP2 := filepath.Join(dir, "orders", "p2", "lines")
	if got[0].colDef.DirPath != wantP1 || got[0].parentKey != "p1" {
		t.Errorf("instance[0]: got dir=%q key=%q, want dir=%q key=p1", got[0].colDef.DirPath, got[0].parentKey, wantP1)
	}
	if got[1].colDef.DirPath != wantP2 || got[1].parentKey != "p2" {
		t.Errorf("instance[1]: got dir=%q key=%q, want dir=%q key=p2", got[1].colDef.DirPath, got[1].parentKey, wantP2)
	}
	for _, inst := range got {
		if inst.fullID != "orders/lines" {
			t.Errorf("instance fullID must be the schema path orders/lines, got %q", inst.fullID)
		}
	}
}
