package validator

// specscore: feature/subcollection-record-validation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
	"github.com/ingitdb/ingitdb-go/ingitdb/datavalidator"
)

// firestoreStyleParent is a parent collection whose records sit directly in
// the collection directory (records_dir: "."), the layout of a Firestore
// mirror: `ext/commitius.json` next to the `ext/commitius/` directory.
const firestoreStyleParent = `record_file:
  name: "{key}.json"
  type: "map[string]any"
  format: json
  records_dir: .
columns:
  name:
    type: string
`

const firestoreStyleChild = `record_file:
  name: "{key}.json"
  type: "map[string]any"
  format: json
  records_dir: .
columns:
  minutes:
    type: int
    required: true
`

func validateResult(t *testing.T, root string) *ingitdb.ValidationResult {
	t.Helper()
	def, err := ReadDefinition(root, ingitdb.Validate())
	if err != nil {
		t.Fatalf("ReadDefinition failed: %v", err)
	}
	res, err := datavalidator.NewValidator().Validate(context.Background(), root, def)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	return res
}

// Verifies subcollection-record-validation#ac:parentless-subcollection-records-validated.
// `ext/commitius/sessions/s1.json` exists, `ext/commitius.json` does not.
func TestSubcolE2E_ParentlessRecordsValidated(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"ext/.collection/definition.yaml":                         firestoreStyleParent,
		"ext/.collection/subcollections/sessions/definition.yaml": firestoreStyleChild,
		"ext/commitius/sessions/s1.json":                          `{"minutes": "thirty"}`,
		"ext/commitius/sessions/s2.json":                          `{"minutes": 30}`,
	}
	res := validateResult(t, writeDB(t, map[string]string{"ext": "./ext"}, files))

	errs := findingsFor(res.Errors(), "ext/sessions")
	if len(errs) != 1 {
		t.Fatalf("expected 1 finding below the missing parent, got %d: %v", len(errs), res.Errors())
	}
	if !strings.Contains(errs[0].Message, "wrong type") || !strings.HasSuffix(errs[0].FilePath, filepath.Join("commitius", "sessions", "s1.json")) {
		t.Errorf("unexpected finding: %+v", errs[0])
	}
	passed, total := res.GetRecordCounts("ext/sessions")
	if passed != 1 || total != 2 {
		t.Errorf("record counts for ext/sessions = %d/%d, want 1/2", passed, total)
	}
	if _, rootTotal := res.GetRecordCounts("ext"); rootTotal != 0 {
		t.Errorf("a missing parent document must not be counted as a record, got %d", rootTotal)
	}
}

// Verifies subcollection-record-validation#ac:parentless-nested-records-validated.
// Two missing documents on the way down: spaces/s1/ext/commitius/checkIns/c1
// with neither spaces/s1/ext/commitius.json nor a sessions parent document.
func TestSubcolE2E_ParentlessNestedRecordsValidated(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"spaces/.collection/definition.yaml":                                            firestoreStyleParent,
		"spaces/.collection/subcollections/ext/definition.yaml":                         firestoreStyleParent,
		"spaces/.collection/subcollections/ext/subcollections/checkIns/definition.yaml": firestoreStyleChild,
		"spaces/s1.json": `{"name": "Space 1"}`,
		"spaces/s1/ext/commitius/checkIns/c1.json": `{"minutes": "late"}`,
		"spaces/s1/ext/calendarius.json":           `{"name": "cal"}`,
		"spaces/s2/ext/commitius/checkIns/c2.json": `{"minutes": 5}`,
	}
	res := validateResult(t, writeDB(t, map[string]string{"spaces": "./spaces"}, files))

	errs := findingsFor(res.Errors(), "spaces/ext/checkIns")
	if len(errs) != 1 || !strings.Contains(errs[0].Message, "minutes") {
		t.Fatalf("expected the wrong-type finding two missing parents down, got: %v", res.Errors())
	}
	if passed, total := res.GetRecordCounts("spaces/ext/checkIns"); passed != 1 || total != 2 {
		t.Errorf("record counts for spaces/ext/checkIns = %d/%d, want 1/2", passed, total)
	}
	if passed, total := res.GetRecordCounts("spaces/ext"); passed != 1 || total != 1 {
		t.Errorf("record counts for spaces/ext = %d/%d, want 1/1 (only calendarius.json exists)", passed, total)
	}
}

// Verifies subcollection-record-validation#ac:parentless-foreign-key-checked.
func TestSubcolE2E_ParentlessForeignKeyChecked(t *testing.T) {
	t.Parallel()
	child := firestoreStyleChild + "  groupID:\n    type: string\n    foreign_key: groups\n"
	files := map[string]string{
		"ext/.collection/definition.yaml":                         firestoreStyleParent,
		"ext/.collection/subcollections/sessions/definition.yaml": child,
		"ext/commitius/sessions/s1.json":                          `{"minutes": 30, "groupID": "ghost"}`,
		"groups/.collection/definition.yaml":                      firestoreStyleParent,
		"groups/g1.json":                                          `{"name": "G1"}`,
	}
	res := validateResult(t, writeDB(t, map[string]string{"ext": "./ext", "groups": "./groups"}, files))
	errs := findingsFor(res.Errors(), "ext/sessions")
	if len(errs) != 1 || !strings.Contains(errs[0].Message, "ghost") {
		t.Fatalf("expected the dangling foreign key below the missing parent, got: %v", res.Errors())
	}
}

// Verifies subcollection-record-validation#ac:parentless-discovery-ignores-other-entries.
// Only a directory that holds the subcollection's own directory is a parent:
// hidden entries, `$records`, excluded names, unrelated directories and a file
// named like the subcollection are not, so they add no instance and no
// record-count finding.
func TestSubcolE2E_ParentlessDiscoveryIgnoresOtherEntries(t *testing.T) {
	t.Parallel()
	parent := strings.Replace(firestoreStyleParent, "  records_dir: .\n", "  records_dir: .\n  exclude_regex: \"^skip\"\n", 1)
	child := firestoreStyleChild + "min_records_count: 1\n"
	files := map[string]string{
		"ext/.collection/definition.yaml":                         parent,
		"ext/.collection/subcollections/sessions/definition.yaml": child,
		"ext/real/sessions/s1.json":                               `{"minutes": 1}`,
		"ext/.hidden/sessions/s1.json":                            `{"minutes": "x"}`,
		"ext/$records/sessions/s1.json":                           `{"minutes": "x"}`,
		"ext/$views/sessions/s1.json":                             `{"minutes": "x"}`,
		"ext/skipme/sessions/s1.json":                             `{"minutes": "x"}`,
		"ext/other/unrelated/u1.json":                             `{"minutes": "x"}`,
		"ext/fileonly/sessions":                                   "not a directory",
	}
	res := validateResult(t, writeDB(t, map[string]string{"ext": "./ext"}, files))
	if errs := res.Errors(); len(errs) != 0 {
		t.Fatalf("only ext/real is a parent instance and it is valid, got: %v", errs)
	}
	if passed, total := res.GetRecordCounts("ext/sessions"); passed != 1 || total != 1 {
		t.Errorf("record counts for ext/sessions = %d/%d, want 1/1", passed, total)
	}
}

// A parent stored as one map-of-records file that fails to parse still has
// its directory-discovered instances validated (previously the whole level was
// dropped); the parse error itself is reported by the root pass as before.
func TestSubcolE2E_ParentlessWhenParentFileUnparsable(t *testing.T) {
	t.Parallel()
	parent := "record_file:\n  name: all.yaml\n  type: \"map[$record_id]map[$field_name]any\"\n  format: yaml\ncolumns:\n  name:\n    type: string\n"
	files := map[string]string{
		"ext/.collection/definition.yaml":                         parent,
		"ext/.collection/subcollections/sessions/definition.yaml": firestoreStyleChild,
		"ext/all.yaml":           "a: [unclosed\n",
		"ext/a/sessions/s1.json": `{"minutes": "x"}`,
	}
	res := validateResult(t, writeDB(t, map[string]string{"ext": "./ext"}, files))
	if len(findingsFor(res.Errors(), "ext")) != 1 {
		t.Errorf("the parent parse error must still be reported once, got: %v", res.Errors())
	}
	if len(findingsFor(res.Errors(), "ext/sessions")) != 1 {
		t.Errorf("the record below the unparsable parent must be validated, got: %v", res.Errors())
	}
}

// A parent record that exists AND owns the directory yields one instance, not
// two: counts are not doubled.
func TestSubcolE2E_ParentRecordAndDirectoryNotDoubled(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"ext/.collection/definition.yaml":                         firestoreStyleParent,
		"ext/.collection/subcollections/sessions/definition.yaml": firestoreStyleChild,
		"ext/commitius.json":                                      `{}`,
		"ext/commitius/sessions/s1.json":                          `{"minutes": 1}`,
	}
	res := validateResult(t, writeDB(t, map[string]string{"ext": "./ext"}, files))
	if passed, total := res.GetRecordCounts("ext/sessions"); passed != 1 || total != 1 {
		t.Errorf("record counts for ext/sessions = %d/%d, want 1/1", passed, total)
	}
}

// The parent records directory is missing entirely: no instance, no panic.
func TestSubcolE2E_ParentlessMissingParentDir(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"ext/.collection/definition.yaml":                         strings.Replace(firestoreStyleParent, "records_dir: .", "records_dir: data", 1),
		"ext/.collection/subcollections/sessions/definition.yaml": firestoreStyleChild,
	}
	root := writeDB(t, map[string]string{"ext": "./ext"}, files)
	if _, err := os.Stat(filepath.Join(root, "ext", "data")); !os.IsNotExist(err) {
		t.Fatalf("fixture must not have the records dir")
	}
	if errs := validateResult(t, root).Errors(); len(errs) != 0 {
		t.Fatalf("expected no findings, got %v", errs)
	}
}
