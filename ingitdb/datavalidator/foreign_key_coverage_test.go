package datavalidator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

func TestForeignKeyCheck_RemainingCoverage(t *testing.T) {
	// 1. def == nil
	if errs := validateForeignKeyReferences(nil); len(errs) != 0 {
		t.Errorf("expected nil errors for nil def, got %v", errs)
	}

	dir := t.TempDir()

	// 2. loadCollectionRecords errors for index & checkCollectionForeignKeys
	brokenFile := filepath.Join(dir, "broken", "data.yaml")
	_ = os.MkdirAll(filepath.Dir(brokenFile), 0o755)
	_ = os.WriteFile(brokenFile, []byte("bad: [yaml\n"), 0o644)
	brokenCol := &ingitdb.CollectionDef{
		ID:      "broken",
		DirPath: filepath.Join(dir, "broken"),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "data.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.MapOfRecords,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"ref": {ForeignKey: "other"},
		},
	}

	defWithBroken := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"broken": brokenCol,
			"other": {
				ID:      "other",
				DirPath: filepath.Join(dir, "other"),
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "data.yaml",
					Format:     ingitdb.RecordFormatYAML,
					RecordType: ingitdb.MapOfRecords,
				},
			},
		},
	}
	_ = validateForeignKeyReferences(defWithBroken)

	// 3. Unresolvable foreign key target (!ok in ResolveForeignKey)
	unresolvedCol := &ingitdb.CollectionDef{
		ID: "unresolved",
		Columns: map[string]*ingitdb.ColumnDef{
			"bad_fk": {ForeignKey: "missing_target"},
		},
	}
	errs := checkCollectionForeignKeys("unresolved", unresolvedCol, &ingitdb.Definition{}, nil)
	if len(errs) != 0 {
		t.Errorf("expected 0 errors for unresolved target, got %v", errs)
	}

	// 4. Subcollection foreign key check
	parent := writeMapCollection(t, dir, "orders", "o1:\n  name: Ord1\n", nil)
	subDir := filepath.Join(dir, "orders", "o1", "items")
	_ = os.MkdirAll(subDir, 0o755)
	_ = os.WriteFile(filepath.Join(subDir, "data.yaml"), []byte("i1:\n  order_id: o1\n"), 0o644)
	parent.SubCollections = map[string]*ingitdb.CollectionDef{
		"items": {
			ID:      "items",
			DirPath: subDir,
			RecordFile: &ingitdb.RecordFileDef{
				Name:       "data.yaml",
				Format:     ingitdb.RecordFormatYAML,
				RecordType: ingitdb.MapOfRecords,
			},
			Columns: map[string]*ingitdb.ColumnDef{
				"order_id": {ForeignKey: "orders"},
			},
		},
	}
	defWithSub := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"orders": parent,
		},
	}
	_ = validateForeignKeyReferences(defWithSub)

	// 5. loadCollectionRecords: ListOfRecords & default case
	listCol := &ingitdb.CollectionDef{
		ID:      "listcol",
		DirPath: filepath.Join(dir, "listcol"),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "data.json",
			Format:     ingitdb.RecordFormatJSON,
			RecordType: ingitdb.ListOfRecords,
		},
	}
	// Missing list file (!ok)
	recs, err := loadCollectionRecords(listCol)
	if err != nil || len(recs) != 0 {
		t.Errorf("expected nil for missing list file, got %v, %v", recs, err)
	}

	// Default case (unknown record type)
	recs, err = loadCollectionRecords(&ingitdb.CollectionDef{
		RecordFile: &ingitdb.RecordFileDef{Format: ingitdb.RecordFormatJSON, RecordType: "unknown"},
	})
	if err != nil || recs != nil {
		t.Errorf("expected nil, nil for unknown record type, got %v, %v", recs, err)
	}

	// 6. loadSingleRecords branches
	singleDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(singleDir, "subfolder.yaml"), 0o755) // is directory
	_ = os.WriteFile(filepath.Join(singleDir, ".hidden.yaml"), []byte("hidden: 1"), 0o644)
	_ = os.WriteFile(filepath.Join(singleDir, "bad.yaml"), []byte("bad: [yaml"), 0o644)
	_ = os.WriteFile(filepath.Join(singleDir, "good.yaml"), []byte("ok: 1"), 0o644)

	// Unreadable file: causes os.ReadFile to fail
	unreadable := filepath.Join(singleDir, "unreadable.yaml")
	_ = os.WriteFile(unreadable, []byte("ok: 2"), 0o000)
	defer func() { _ = os.Chmod(unreadable, 0o644) }()

	singleCol := &ingitdb.CollectionDef{
		ID:      "singles",
		DirPath: singleDir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "*.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
		Columns: map[string]*ingitdb.ColumnDef{"ok": {Type: ingitdb.ColumnTypeInt}},
	}
	singleRecs, err := loadSingleRecords(singleCol)
	if err != nil {
		t.Fatalf("loadSingleRecords error: %v", err)
	}
	if len(singleRecs) != 1 {
		t.Errorf("expected 1 good record, got %d", len(singleRecs))
	}

	// Malformed glob pattern in loadSingleRecords
	badGlobCol := &ingitdb.CollectionDef{
		DirPath: singleDir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "[a-",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	if _, err := loadSingleRecords(badGlobCol); err == nil {
		t.Error("expected error for malformed glob pattern")
	}

	// 7. loadMapRecords: missing file (!ok)
	missingMapCol := &ingitdb.CollectionDef{
		DirPath: filepath.Join(dir, "missing_map"),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "missing.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.MapOfRecords,
		},
	}
	if mapRecs, err := loadMapRecords(missingMapCol); err != nil || len(mapRecs) != 0 {
		t.Errorf("expected nil for missing map file, got %v, %v", mapRecs, err)
	}

	// 8. loadListRecords: invalid content, unresolvable key, and valid record
	listFile := filepath.Join(dir, "listcol", "data.json")
	_ = os.MkdirAll(filepath.Dir(listFile), 0o755)
	// Invalid JSON content
	_ = os.WriteFile(listFile, []byte("invalid json"), 0o644)
	if _, err := loadListRecords(listCol); err == nil {
		t.Error("expected error for invalid JSON list content")
	}

	// List with unresolvable key (no $ID, no id) and valid key
	_ = os.WriteFile(listFile, []byte(`[{"no_key": 1}, {"$ID": "k1", "val": 2}]`), 0o644)
	validListRecs, err := loadListRecords(listCol)
	if err != nil {
		t.Fatalf("unexpected error loading list records: %v", err)
	}
	if len(validListRecs) != 1 {
		t.Errorf("expected 1 record with key, got %d", len(validListRecs))
	}
}
