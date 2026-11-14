package datavalidator

// specscore: feature/subcollection-record-validation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ingitdb "github.com/ingitdb/ingitdb-go/ingitdb"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// spacesWithNotes is spaces (records_dir per argument, {key}.yaml) with a
// notes subcollection whose title must be a string.
func spacesWithNotes(root string, recordsDir *string) *ingitdb.Definition {
	str := &ingitdb.ColumnDef{Type: ingitdb.ColumnTypeString}
	rf := func(dir *string) *ingitdb.RecordFileDef {
		return &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.SingleRecord, RecordsDir: dir}
	}
	dot := "."
	notes := &ingitdb.CollectionDef{ID: "notes", RecordFile: rf(&dot), Columns: map[string]*ingitdb.ColumnDef{"title": str}}
	spaces := &ingitdb.CollectionDef{ID: "spaces", DirPath: filepath.Join(root, "spaces"), RecordFile: rf(recordsDir),
		Columns: map[string]*ingitdb.ColumnDef{"title": str}, SubCollections: map[string]*ingitdb.CollectionDef{"notes": notes}}
	return &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"spaces": spaces}}
}

func findingTexts(errs []ingitdb.ValidationError) string {
	var out []string
	for _, e := range errs {
		out = append(out, e.Message)
	}
	return strings.Join(out, "\n")
}

// Verifies subcollection-record-validation#ac:parent-dirs-one-rule for
// validation: a "$"-prefixed Firestore-style parent is validated, a symlinked
// parent inside the database is followed, and one that leads outside is not
// read but reported as a warning (exit code unchanged: no error).
func TestValidate_ParentDirsOneRule(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := t.TempDir()
	dot := "."
	writeTree(t, root, map[string]string{
		"spaces/$x/notes/n1.yaml":         "title: 1\n", // invalid: found and reported
		"spaces/$views/notes/v.yaml":      "title: 2\n", // inGitDB's own name: not a parent
		"elsewhere/notes/n2.yaml":         "title: 3\n", // reached via an inside symlink: reported
		"spaces/ok/notes/n3.yaml":         "title: fine\n",
		"spaces/.hidden/notes/n4.yaml":    "title: 4\n",
		"spaces/$records/other/notes.yml": "",
	})
	writeTree(t, outside, map[string]string{"notes/evil.yaml": "title: 5\n"})
	if err := os.Symlink(filepath.Join(root, "elsewhere"), filepath.Join(root, "spaces", "inlink")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "spaces", "outlink")); err != nil {
		t.Fatal(err)
	}
	res, err := NewValidator().Validate(context.Background(), root, spacesWithNotes(root, &dot))
	if err != nil {
		t.Fatal(err)
	}
	errs := findingTexts(res.Errors())
	for _, want := range []string{`spaces/$x/notes/n1.yaml`, `spaces/inlink/notes/n2.yaml`} {
		found := false
		for _, e := range res.Errors() {
			if strings.HasSuffix(filepath.ToSlash(e.FilePath), want) {
				found = true
			}
		}
		if !found {
			t.Errorf("expected a finding for %s, got:\n%s", want, errs)
		}
	}
	if len(res.Errors()) != 2 {
		t.Errorf("expected exactly 2 findings ($x and inlink), got:\n%s", errs)
	}
	warnings := res.Warnings()
	if len(warnings) != 1 || !strings.Contains(warnings[0].Message, "spaces/outlink/notes resolves outside the database through a symbolic link") {
		t.Fatalf("expected one outside-link warning, got %+v", warnings)
	}
	if passed, total := res.GetRecordCounts("spaces/notes"); passed != 1 || total != 3 {
		t.Errorf("spaces/notes counts = %d/%d, want 1/3", passed, total)
	}
}

// Verifies subcollection-record-validation#ac:driver-layout-records-reported:
// under the default records_dir, records the storage drivers wrote at
// spaces/<key>/notes/ are not validated and are reported as a warning naming
// both locations; findings and exit code are unchanged.
func TestValidate_DriverLayoutRecordsReported(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"spaces/$records/s1.yaml":         "title: S1\n",
		"spaces/$records/s1/notes/a.yaml": "title: validated\n",
		"spaces/family/notes/c1.yaml":     "title: 1\n", // driver layout: not validated
		"spaces/work/notes/c2.yaml":       "title: 2\n",
		"spaces/empty/notes/.keep":        "", // hidden-only directory is still non-empty: counted
		"spaces/README.md":                "",
		"spaces/$records/s2/other/x.yaml": "",
		"spaces/$views/notes/v.yaml":      "",
		"spaces/nothing/notes-but-no-dir": "",
	})
	if err := os.MkdirAll(filepath.Join(root, "spaces", "bare", "notes"), 0o755); err != nil { // empty: not counted
		t.Fatal(err)
	}
	res, err := NewValidator().Validate(context.Background(), root, spacesWithNotes(root, nil))
	if err != nil {
		t.Fatal(err)
	}
	if res.HasErrors() {
		t.Fatalf("driver-layout records must not become errors, got:\n%s", findingTexts(res.Errors()))
	}
	w := res.Warnings()
	if len(w) != 1 {
		t.Fatalf("expected one warning, got %+v", w)
	}
	want := `spaces/notes: 3 instance(s) found in spaces/<key>/notes/ (e.g. spaces/empty/notes), the layout the storage drivers and ` +
		"`ingitdb select`" + ` use, are not validated; validation reads spaces/$records/<key>/notes/. Which layout applies under records_dir "$records" is an open decision`
	if w[0].Message != want || w[0].CollectionID != "spaces/notes" {
		t.Fatalf("warning:\n got %s\nwant %s", w[0].Message, want)
	}
	if passed, total := res.GetRecordCounts("spaces/notes"); passed != 1 || total != 1 {
		t.Errorf("validated counts = %d/%d, want 1/1", passed, total)
	}

	// records_dir "." is one layout: no warning. Nil definitions give none.
	dot := "."
	if got := driverLayoutWarnings(root, spacesWithNotes(root, &dot)); got != nil {
		t.Errorf("records_dir . must not warn, got %+v", got)
	}
	if driverLayoutWarnings(root, nil) != nil {
		t.Error("nil definition")
	}
}

func TestRelToRootAndLastSegment(t *testing.T) {
	t.Parallel()
	if got := relToRoot("", "/a/b"); got != "/a/b" {
		t.Errorf("no root: %q", got)
	}
	if got := relToRoot("/db", "/elsewhere/x"); got != "/elsewhere/x" {
		t.Errorf("outside root: %q", got)
	}
	if got := relToRoot("/db", "/db/a/b"); got != "a/b" {
		t.Errorf("inside root: %q", got)
	}
	if lastSegment("a/b/c") != "c" || lastSegment("a") != "a" {
		t.Error("lastSegment")
	}
	if recordsBaseDir(&ingitdb.CollectionDef{DirPath: "x"}) != "x" {
		t.Error("recordsBaseDir without a record file is the data directory")
	}
	empty := t.TempDir()
	if got := driverLayoutWarnings(empty, spacesWithNotes(empty, nil)); got != nil {
		t.Errorf("no driver-layout data must not warn, got %+v", got)
	}
}

// Verifies column-validation#ac:strict-temporal-opt-in: the opt-in pass
// reports text that is not a valid date/time/datetime, in root and
// subcollection records, while Validate keeps accepting it.
func TestStrictTemporalFindings(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dot := "."
	def := spacesWithNotes(root, &dot)
	spaces := def.Collections["spaces"]
	spaces.Columns["day"] = &ingitdb.ColumnDef{Type: ingitdb.ColumnTypeDate}
	spaces.SubCollections["notes"].Columns["at"] = &ingitdb.ColumnDef{Type: ingitdb.ColumnTypeTime}
	spaces.Columns["$ID"] = &ingitdb.ColumnDef{Type: ingitdb.ColumnTypeString}
	writeTree(t, root, map[string]string{
		"spaces/s1.yaml":         "title: S1\nday: 2026-02-30\n",
		"spaces/s2.yaml":         "title: S2\nday: \"2026-02-28\"\n",
		"spaces/s3.yaml":         "title: S3\nday: 7\n",
		"spaces/s1/notes/n.yaml": "title: N\nat: \"25:00\"\n",
	})
	res, err := NewValidator().Validate(context.Background(), root, def)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range res.Errors() {
		if strings.Contains(e.Message, "2026-02-30") || strings.Contains(e.Message, "25:00") {
			t.Fatalf("Validate must keep accepting any text in a temporal column, got %s", e.Message)
		}
	}
	got := findingTexts(StrictTemporalFindings(root, def))
	want := "strict time check for field \"at\": \"25:00\" is not a valid time (H:MM or H:MM:SS (hour of one or two digits), optional fractional seconds)\n" +
		"strict date check for field \"day\": \"2026-02-30\" is not a valid date (YYYY-MM-DD)"
	if strings.Count(got, "\n") != 1 || !strings.Contains(got, `"2026-02-30" is not a valid date (YYYY-MM-DD)`) || !strings.Contains(got, `"25:00" is not a valid time`) {
		t.Fatalf("strict findings:\n got %s\nwant %s", got, want)
	}
	if StrictTemporalFindings(root, nil) != nil {
		t.Error("nil definition")
	}
}
