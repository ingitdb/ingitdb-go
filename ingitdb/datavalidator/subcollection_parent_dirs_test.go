package datavalidator

// specscore: feature/subcollection-record-validation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

// Verifies subcollection-record-validation#ac:parentless-discovery-ignores-other-entries
// at the unit level: only directories holding a directory named after the
// subcollection are parents; excluded, hidden, `$`-prefixed and plain entries
// are not, and a missing base directory gives none.
func TestParentKeysWithSubCollectionDir(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	for _, dir := range []string{"p1/ext", "p2/ext", "skip-me/ext", ".hidden/ext", "$views/ext", "p3/other"} {
		if err := os.MkdirAll(filepath.Join(base, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(base, "p2", "ext2"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(base, "p4"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "p4", "ext"), nil, 0o644); err != nil { // a file, not a dir
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "file.json"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	recordsDir := "."
	colDef := &ingitdb.CollectionDef{
		DirPath:    base,
		RecordFile: &ingitdb.RecordFileDef{Name: "{key}.json", RecordsDir: &recordsDir, ExcludeRegex: "^skip-"},
	}
	got := parentKeysWithSubCollectionDir(colDef, "ext")
	if len(got) != 2 || got[0] != "p1" || got[1] != "p2" {
		t.Fatalf("expected [p1 p2], got %v", got)
	}
	if got := parentKeysWithSubCollectionDir(&ingitdb.CollectionDef{DirPath: filepath.Join(base, "absent")}, "ext"); got != nil {
		t.Fatalf("missing base dir: expected none, got %v", got)
	}
}
