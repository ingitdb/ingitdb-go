package validator

import (
	"errors"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

func TestAbsInheritPath_Fallback(t *testing.T) {
	orig := filepathAbs
	defer func() { filepathAbs = orig }()
	filepathAbs = func(p string) (string, error) {
		return "", errors.New("simulated abs error")
	}

	got := absInheritPath("rel/file.yaml")
	if got != "rel/file.yaml" {
		t.Errorf("absInheritPath = %q, want %q", got, "rel/file.yaml")
	}
}

func TestOverlayCollectionDef_NilColumnsAndTitles(t *testing.T) {
	child := &ingitdb.CollectionDef{}
	base := &ingitdb.CollectionDef{
		Columns: map[string]*ingitdb.ColumnDef{
			"col1": {Type: "string"},
		},
		Titles: map[string]string{
			"en": "Title",
		},
	}
	overlayCollectionDef(child, base)
	if child.Columns == nil || child.Columns["col1"] == nil {
		t.Error("expected child.Columns to be populated")
	}
	if child.Titles == nil || child.Titles["en"] != "Title" {
		t.Error("expected child.Titles to be populated")
	}
}

func TestReadDefinition_ForeignKeyValidationError(t *testing.T) {
	dir := writeInheritanceDB(t, map[string]string{
		".ingitdb/root-collections.yaml": rootStates,
		"states/.collection/definition.yaml": mapRecordFile +
			"columns:\n  country_id:\n    type: string\n    foreign_key: non_existent_collection\n",
	})
	_, err := ReadDefinition(dir, ingitdb.Validate())
	if err == nil {
		t.Fatal("expected error on invalid foreign key in ReadDefinition with Validate()")
	}
}

func TestReadCollectionDefShared_InheritanceError(t *testing.T) {
	dl := defLoader{
		readFile: func(path string) ([]byte, error) {
			return []byte("inherits: non_existent_base.yaml\n"), nil
		},
	}
	_, err := dl.readCollectionDefShared("/schema/mycol", "/data", "", "mycol", ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("expected error when shared collection definition inherits non-existent file")
	}
}
