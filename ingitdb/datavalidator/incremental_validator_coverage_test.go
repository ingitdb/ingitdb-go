package datavalidator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

type fakeResolver struct {
	records []AffectedRecord
	err     error
}

func (r fakeResolver) Resolve(_ string, _ *ingitdb.Definition, _ []ingitdb.ChangedFile) ([]AffectedRecord, error) {
	return r.records, r.err
}

func TestIncrementalValidator_ErrorsAndBranches(t *testing.T) {
	ctx := context.Background()

	// 1. differ error
	ivDiffErr := NewIncrementalValidator(fakeDiffer{err: errors.New("diff fail")}, fakeResolver{}, nil)
	if _, err := ivDiffErr.ValidateChanges(ctx, "/repo", nil, "from", "to"); err == nil {
		t.Error("expected error on differ failure")
	}

	// 2. resolver error
	ivResErr := NewIncrementalValidator(fakeDiffer{}, fakeResolver{err: errors.New("resolver fail")}, nil)
	if _, err := ivResErr.ValidateChanges(ctx, "/repo", nil, "from", "to"); err == nil {
		t.Error("expected error on resolver failure")
	}

	// 3. changedDefinitionFile with sub/.ingitdb/
	if !changedDefinitionFile([]ingitdb.ChangedFile{{Path: "sub/.ingitdb/settings.yaml"}}) {
		t.Error("expected true for nested .ingitdb path")
	}

	// 4. validateWholeRecordFile with unknown record type
	unknownCol := &ingitdb.CollectionDef{
		RecordFile: &ingitdb.RecordFileDef{
			RecordType: "unknown_type",
		},
	}
	p, tot, errs := validateWholeRecordFile("test", unknownCol)
	if p != 0 || tot != 0 || len(errs) != 0 {
		t.Errorf("expected 0,0,nil for unknown type, got %d, %d, %v", p, tot, errs)
	}

	// 5. MapOfRecords and ListOfRecords affected, plus unknown collection ID and duplicate in same collection
	dir := t.TempDir()
	mapCol := writeMapCollection(t, dir, "mapcol", "k1:\n  name: V1\n", map[string]*ingitdb.ColumnDef{
		"name": {Type: ingitdb.ColumnTypeString},
	})
	listPath := filepath.Join(dir, "listcol", "data.json")
	if err := os.MkdirAll(filepath.Dir(listPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(listPath, []byte(`[{"id": "1", "name": "A"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	listCol := &ingitdb.CollectionDef{
		ID:      "listcol",
		DirPath: filepath.Join(dir, "listcol"),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "data.json",
			Format:     ingitdb.RecordFormatJSON,
			RecordType: ingitdb.ListOfRecords,
		},
		Columns: map[string]*ingitdb.ColumnDef{"name": {Type: ingitdb.ColumnTypeString}},
	}

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"mapcol":  mapCol,
			"listcol": listCol,
		},
	}

	affected := []AffectedRecord{
		{CollectionID: "missing_col"},
		{CollectionID: "mapcol", FilePath: filepath.Join(mapCol.DirPath, "data.yaml")},
		{CollectionID: "mapcol", FilePath: filepath.Join(mapCol.DirPath, "data.yaml")}, // duplicate triggers line 74 continue
		{CollectionID: "listcol", FilePath: listPath},
	}

	iv := NewIncrementalValidator(fakeDiffer{files: []ingitdb.ChangedFile{{Path: "mapcol/data.yaml"}}}, fakeResolver{records: affected}, nil)
	res, err := iv.ValidateChanges(ctx, dir, def, "from", "to")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}
}
