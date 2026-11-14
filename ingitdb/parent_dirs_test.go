package ingitdb

// specscore: feature/subcollection-record-validation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mkdirs(t *testing.T, root string, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// Verifies subcollection-record-validation#ac:parent-dirs-one-rule.
func TestScanParentDirs_OneRule(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outsideDir := t.TempDir()
	base := filepath.Join(root, "spaces")
	mkdirs(t, base,
		"p1/ext", "p2/ext", "$x/ext", // "$x" is a Firestore-style id: a parent
		"$records/ext", "$views/ext", "$ingitdb/ext", "$data/ext", // inGitDB's own names
		".hidden/ext", "old-1/ext", "p3/other", "p4",
	)
	mkdirs(t, root, "elsewhere/ext")
	mkdirs(t, outsideDir, "ext")
	if err := os.WriteFile(filepath.Join(base, "p4", "ext"), nil, 0o644); err != nil { // a file, not a directory
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "file.json"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "elsewhere"), filepath.Join(base, "inside-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideDir, filepath.Join(base, "outside-link")); err != nil {
		t.Fatal(err)
	}
	recordsDir := "$data"
	rf := &RecordFileDef{Name: "{key}.json", RecordsDir: &recordsDir, ExcludeRegex: "^old-"}

	keys, outside := ScanParentDirs(root, base, rf).KeysWith("ext")
	if got := strings.Join(keys, " "); got != "$x inside-link p1 p2" {
		t.Errorf("keys = %q, want %q", got, "$x inside-link p1 p2")
	}
	if got := strings.Join(outside, " "); got != "outside-link" {
		t.Errorf("outside = %q, want %q", got, "outside-link")
	}

	// Without a root (or with one that cannot be resolved) links are followed.
	for _, r := range []string{"", filepath.Join(root, "absent")} {
		keys, outside = ScanParentDirs(r, base, rf).KeysWith("ext")
		if got := strings.Join(keys, " "); got != "$x inside-link outside-link p1 p2" || outside != nil {
			t.Errorf("root %q: keys = %q, outside = %v", r, got, outside)
		}
	}
	// A nil record file applies no exclusion; a missing base yields nothing.
	if keys, _ = ScanParentDirs(root, base, nil).KeysWith("ext"); !strings.Contains(strings.Join(keys, " "), "old-1") {
		t.Errorf("nil record file: old-1 must be a parent, got %v", keys)
	}
	if keys, outside = ScanParentDirs(root, filepath.Join(root, "absent"), rf).KeysWith("ext"); keys != nil || outside != nil {
		t.Errorf("missing base: got %v, %v", keys, outside)
	}
}

// The first segment of a nested records directory is not a parent, and
// "$records" never is.
func TestScanParentDirs_RecordsDirHead(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mkdirs(t, root, "data/items/x", "$records/items", "p/items")
	nested := "data/items"
	if keys, _ := ScanParentDirs(root, root, &RecordFileDef{Name: "{key}.json", RecordsDir: &nested}).KeysWith("items"); strings.Join(keys, " ") != "p" {
		t.Errorf("records_dir data/items: keys = %v", keys)
	}
	if keys, _ := ScanParentDirs(root, root, &RecordFileDef{Name: "{key}.json"}).KeysWith("items"); strings.Join(keys, " ") != "data p" {
		t.Errorf("default records_dir: keys = %v", keys)
	}
}

func TestIsWithin(t *testing.T) {
	t.Parallel()
	sep := string(filepath.Separator)
	for p, want := range map[string]bool{sep + "db": true, sep + "db" + sep + "a": true, sep + "dbx": false, sep: false, "relative": false} {
		if got := isWithin(sep+"db", p); got != want {
			t.Errorf("isWithin(/db, %q) = %v, want %v", p, got, want)
		}
	}
}
