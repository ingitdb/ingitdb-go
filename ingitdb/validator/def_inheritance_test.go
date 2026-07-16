package validator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
	"github.com/ingitdb/ingitdb-go/ingitdb/datavalidator"
)

// writeInheritanceDB writes a fixture database rooted at a fresh temp dir.
// files maps a slash-separated relative path to file content; parent dirs are
// created as needed. Returns the root path.
func writeInheritanceDB(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const rootStates = "states: states\n"

// mapRecordFile is a MapOfRecords json record_file: all records in one file,
// keeping fixtures free of the $records/ subdirectory layout.
const mapRecordFile = "record_file:\n  name: records.json\n  format: json\n  type: \"map[$record_id]map[$field_name]any\"\n"

// validateDB loads the database with the real reader and runs data validation,
// returning the merged definition and the list of record-level violations.
func validateDB(t *testing.T, dir string) (*ingitdb.Definition, []ingitdb.ValidationError) {
	t.Helper()
	def, err := ReadDefinition(dir, ingitdb.Validate())
	if err != nil {
		t.Fatalf("ReadDefinition: %v", err)
	}
	res, err := datavalidator.NewValidator().Validate(context.Background(), dir, def)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return def, res.Errors()
}

// AC: base-only-column-is-inherited — a column only the base declares appears in
// the merged collection.
func TestInheritance_BaseOnlyColumnIsInherited(t *testing.T) {
	dir := writeInheritanceDB(t, map[string]string{
		".ingitdb/root-collections.yaml": rootStates,
		"states/.collection/$base.yaml":  "columns:\n  population:\n    type: int\n",
		"states/.collection/definition.yaml": "inherits: $base.yaml\n" + mapRecordFile +
			"columns:\n  name:\n    type: string\n",
		"states/records.json": `{"r1": {"name": "X", "population": 100}}` + "\n",
	})
	def, viols := validateDB(t, dir)
	cols := def.Collections["states"].Columns
	if _, ok := cols["population"]; !ok {
		t.Errorf("merged collection must inherit base column 'population'; got columns %v", keysOf(cols))
	}
	if _, ok := cols["name"]; !ok {
		t.Errorf("merged collection must keep its own column 'name'; got columns %v", keysOf(cols))
	}
	if len(viols) != 0 {
		t.Errorf("expected no violations, got %v", viols)
	}
}

// AC: child-column-overrides-base-column — a column the child redeclares wholly
// replaces the base's column of the same name.
func TestInheritance_ChildColumnOverridesBase(t *testing.T) {
	dir := writeInheritanceDB(t, map[string]string{
		".ingitdb/root-collections.yaml": rootStates,
		"states/.collection/$base.yaml":  "columns:\n  code:\n    type: string\n    max_length: 2\n",
		"states/.collection/definition.yaml": "inherits: $base.yaml\n" + mapRecordFile +
			"columns:\n  code:\n    type: string\n    max_length: 5\n",
		"states/records.json": `{"r1": {"code": "abcd"}}` + "\n",
	})
	_, viols := validateDB(t, dir)
	if len(viols) != 0 {
		t.Errorf("child max_length:5 must win over base max_length:2 for a 4-char value; got %v", viols)
	}
}

// AC: scalar-field-inherited-when-child-omits-it — a child with no record_file
// inherits the base's.
func TestInheritance_ScalarFieldInheritedWhenChildOmits(t *testing.T) {
	dir := writeInheritanceDB(t, map[string]string{
		".ingitdb/root-collections.yaml":     rootStates,
		"states/.collection/$base.yaml":      mapRecordFile,
		"states/.collection/definition.yaml": "inherits: $base.yaml\ncolumns:\n  name:\n    type: string\n",
		"states/records.json":                `{"r1": {"name": "X"}}` + "\n",
	})
	def, viols := validateDB(t, dir)
	if def.Collections["states"].RecordFile == nil {
		t.Error("child must inherit base record_file")
	}
	if len(viols) != 0 {
		t.Errorf("expected no violations, got %v", viols)
	}
}

// AC: titles-merge-by-locale — base and child titles combine per locale.
func TestInheritance_TitlesMergeByLocale(t *testing.T) {
	dir := writeInheritanceDB(t, map[string]string{
		".ingitdb/root-collections.yaml": rootStates,
		"states/.collection/$base.yaml":  "titles:\n  en: Divisions\n",
		"states/.collection/definition.yaml": "inherits: $base.yaml\ntitles:\n  ru: Штаты\n" + mapRecordFile +
			"columns:\n  name:\n    type: string\n",
	})
	def, _ := validateDB(t, dir)
	titles := def.Collections["states"].Titles
	if titles["en"] != "Divisions" {
		t.Errorf("expected inherited en title 'Divisions', got %q", titles["en"])
	}
	if titles["ru"] != "Штаты" {
		t.Errorf("expected own ru title 'Штаты', got %q", titles["ru"])
	}
}

// AC: missing-base-is-load-error — an unresolvable inherits path fails loudly.
func TestInheritance_MissingBaseIsLoadError(t *testing.T) {
	dir := writeInheritanceDB(t, map[string]string{
		".ingitdb/root-collections.yaml": rootStates,
		"states/.collection/definition.yaml": "inherits: $nonexistent.yaml\n" + mapRecordFile +
			"columns:\n  name:\n    type: string\n",
	})
	_, err := ReadDefinition(dir, ingitdb.Validate())
	if err == nil {
		t.Fatal("an unresolvable inherits base must be a load error, not silently discarded")
	}
	if !strings.Contains(err.Error(), "$nonexistent.yaml") {
		t.Errorf("error must name the unresolvable base, got: %v", err)
	}
}

// AC: self-inheritance-is-a-cycle.
func TestInheritance_SelfInheritanceIsCycle(t *testing.T) {
	dir := writeInheritanceDB(t, map[string]string{
		".ingitdb/root-collections.yaml": rootStates,
		"states/.collection/$a.yaml":     "inherits: $a.yaml\ncolumns:\n  x:\n    type: string\n",
		"states/.collection/definition.yaml": "inherits: $a.yaml\n" + mapRecordFile +
			"columns:\n  name:\n    type: string\n",
	})
	_, err := ReadDefinition(dir, ingitdb.Validate())
	if err == nil {
		t.Fatal("self-inheritance must be a cycle error, not an infinite loop")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error must mention a cycle, got: %v", err)
	}
}

// AC: mutual-inheritance-is-a-cycle.
func TestInheritance_MutualInheritanceIsCycle(t *testing.T) {
	dir := writeInheritanceDB(t, map[string]string{
		".ingitdb/root-collections.yaml": rootStates,
		"states/.collection/$a.yaml":     "inherits: $b.yaml\ncolumns:\n  x:\n    type: string\n",
		"states/.collection/$b.yaml":     "inherits: $a.yaml\ncolumns:\n  y:\n    type: string\n",
		"states/.collection/definition.yaml": "inherits: $a.yaml\n" + mapRecordFile +
			"columns:\n  name:\n    type: string\n",
	})
	_, err := ReadDefinition(dir, ingitdb.Validate())
	if err == nil {
		t.Fatal("mutual inheritance must be a cycle error")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error must mention a cycle, got: %v", err)
	}
}

// AC: multi-level-chain-merges-far-base — a grandparent's column reaches the child.
func TestInheritance_MultiLevelChain(t *testing.T) {
	dir := writeInheritanceDB(t, map[string]string{
		".ingitdb/root-collections.yaml":     rootStates,
		"states/.collection/$grandbase.yaml": "columns:\n  area:\n    type: int\n",
		"states/.collection/$base.yaml":      "inherits: $grandbase.yaml\ncolumns:\n  population:\n    type: int\n",
		"states/.collection/definition.yaml": "inherits: $base.yaml\n" + mapRecordFile +
			"columns:\n  name:\n    type: string\n",
	})
	def, _ := validateDB(t, dir)
	cols := def.Collections["states"].Columns
	for _, want := range []string{"area", "population", "name"} {
		if _, ok := cols[want]; !ok {
			t.Errorf("merged collection must contain column %q from the chain; got %v", want, keysOf(cols))
		}
	}
}

// AC: unknown-key-in-base-partial-rejected — strict decoding applies to bases.
func TestInheritance_UnknownKeyInBaseRejected(t *testing.T) {
	dir := writeInheritanceDB(t, map[string]string{
		".ingitdb/root-collections.yaml": rootStates,
		// record_labels is a genuinely unmodelled key (min_records_count, once a
		// candidate here, became a modelled key when ingitdb-go#8 merged).
		"states/.collection/$base.yaml": "record_labels: x\ncolumns:\n  x:\n    type: string\n",
		"states/.collection/definition.yaml": "inherits: $base.yaml\n" + mapRecordFile +
			"columns:\n  name:\n    type: string\n",
	})
	_, err := ReadDefinition(dir, ingitdb.Validate())
	if err == nil {
		t.Fatal("an unrecognised key in a base partial must be rejected")
	}
	if !strings.Contains(err.Error(), "record_labels") {
		t.Errorf("error must name the unrecognised base key, got: %v", err)
	}
}

// AC: merged-definition-still-missing-record-file-fails — inheritance does not
// mask a gap nothing supplied.
func TestInheritance_MergedStillMissingRecordFileFails(t *testing.T) {
	dir := writeInheritanceDB(t, map[string]string{
		".ingitdb/root-collections.yaml":     rootStates,
		"states/.collection/$base.yaml":      "columns:\n  x:\n    type: string\n",
		"states/.collection/definition.yaml": "inherits: $base.yaml\ncolumns:\n  name:\n    type: string\n",
	})
	_, err := ReadDefinition(dir, ingitdb.Validate())
	if err == nil {
		t.Fatal("a merged definition still lacking record_file must fail validation")
	}
	if !strings.Contains(err.Error(), "record_file") {
		t.Errorf("error must name the missing record_file, got: %v", err)
	}
}

// AC: nearer-base-wins-over-farther-base — in a chain, the nearer base's column
// overrides the farther base's, and the child inherits the nearer one.
func TestInheritance_NearerBaseWins(t *testing.T) {
	dir := writeInheritanceDB(t, map[string]string{
		".ingitdb/root-collections.yaml":     rootStates,
		"states/.collection/$grandbase.yaml": "columns:\n  code:\n    type: string\n    max_length: 2\n",
		"states/.collection/$base.yaml":      "inherits: $grandbase.yaml\ncolumns:\n  code:\n    type: string\n    max_length: 5\n",
		"states/.collection/definition.yaml": "inherits: $base.yaml\n" + mapRecordFile +
			"columns:\n  name:\n    type: string\n",
		"states/records.json": `{"r1": {"name": "X", "code": "abcd"}}` + "\n",
	})
	_, viols := validateDB(t, dir)
	if len(viols) != 0 {
		t.Errorf("nearer base max_length:5 must win over farther max_length:2 for a 4-char value; got %v", viols)
	}
}

func keysOf(m map[string]*ingitdb.ColumnDef) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
