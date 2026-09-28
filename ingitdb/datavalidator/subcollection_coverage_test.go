package datavalidator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

func TestValidateSubCollections_Coverage(t *testing.T) {
	// 1. def == nil
	resNil := &ingitdb.ValidationResult{}
	validateSubCollections(nil, resNil)
	if len(resNil.Errors()) != 0 {
		t.Error("expected 0 errors for nil def")
	}

	// 2. parent collection loadCollectionRecords error
	badDir := t.TempDir()
	badFile := filepath.Join(badDir, "bad.yaml")
	if err := os.WriteFile(badFile, []byte("bad: [invalid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	badParent := &ingitdb.CollectionDef{
		ID:      "bad",
		DirPath: badDir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "bad.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.MapOfRecords,
		},
		SubCollections: map[string]*ingitdb.CollectionDef{
			"sub": {ID: "sub"},
		},
	}
	walkSubCollectionInstances("bad", badParent, func(inst subCollectionInstance) {
		t.Error("expected no callback on loadCollectionRecords error")
	})

	// 3. Full pass with subcollection records + record count violation + validation error
	dir := t.TempDir()
	parent := writeMapCollection(t, dir, "orders", "o1:\n  name: Order1\n", map[string]*ingitdb.ColumnDef{
		"name": {Type: ingitdb.ColumnTypeString},
	})

	subDataDir := filepath.Join(dir, "orders", "o1", "items")
	if err := os.MkdirAll(subDataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// qty is string "not_a_number" to cause validation error
	if err := os.WriteFile(filepath.Join(subDataDir, "data.yaml"), []byte("i1:\n  qty: not_a_number\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	minRecords := 2
	parent.SubCollections = map[string]*ingitdb.CollectionDef{
		"items": {
			ID:      "items",
			DirPath: subDataDir,
			RecordFile: &ingitdb.RecordFileDef{
				Name:       "data.yaml",
				Format:     ingitdb.RecordFormatYAML,
				RecordType: ingitdb.MapOfRecords,
			},
			Columns: map[string]*ingitdb.ColumnDef{
				"qty": {Type: ingitdb.ColumnTypeInt},
			},
			MinRecordsCount: &minRecords,
		},
	}

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"orders": parent,
		},
	}

	result := &ingitdb.ValidationResult{}
	validateSubCollections(def, result)

	errs := result.Errors()
	if len(errs) < 2 {
		t.Fatalf("expected at least 2 errors (type error + min_records error), got %d: %+v", len(errs), errs)
	}
}
