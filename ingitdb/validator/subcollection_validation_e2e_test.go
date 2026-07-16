package validator

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
	"github.com/ingitdb/ingitdb-go/ingitdb/datavalidator"
)

// writeDB materialises a real inGitDB database in a temp dir: a `.ingitdb/`
// with settings + the given root-collection map, plus every file in `files`
// (paths relative to the database root). Tests exercise the production reader
// and validator over on-disk data, never a hand-unmarshalled struct.
func writeDB(t *testing.T, rootCollections map[string]string, files map[string]string) string {
	t.Helper()
	root := t.TempDir()

	writeFile := func(rel, content string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeFile(".ingitdb/settings.yaml", "languages:\n  - required: en\n")

	ids := make([]string, 0, len(rootCollections))
	for id := range rootCollections {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var rc strings.Builder
	for _, id := range ids {
		rc.WriteString(id)
		rc.WriteString(": ")
		rc.WriteString(rootCollections[id])
		rc.WriteString("\n")
	}
	writeFile(".ingitdb/root-collections.yaml", rc.String())

	for rel, content := range files {
		writeFile(rel, content)
	}
	return root
}

// loadAndValidate runs the real reader + validator and returns all findings.
func loadAndValidate(t *testing.T, root string) []ingitdb.ValidationError {
	t.Helper()
	def, err := ReadDefinition(root, ingitdb.Validate())
	if err != nil {
		t.Fatalf("ReadDefinition failed: %v", err)
	}
	res, err := datavalidator.NewValidator().Validate(context.Background(), root, def)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	return res.Errors()
}

// findingsFor returns the findings whose CollectionID equals id.
func findingsFor(errs []ingitdb.ValidationError, id string) []ingitdb.ValidationError {
	var out []ingitdb.ValidationError
	for _, e := range errs {
		if e.CollectionID == id {
			out = append(out, e)
		}
	}
	return out
}

const parentDefYAML = `titles:
  en: Parent
record_file:
  name: "{key}.yaml"
  type: "map[string]any"
  format: yaml
columns:
  name:
    type: string
    required: true
`

// childDef builds an order_details-style subcollection definition (a
// []map[string]any in details.json) with the given columns block and optional
// trailing collection-level lines (e.g. a record-count bound).
func childDef(columns, trailing string) string {
	d := `titles:
  en: Child
record_file:
  name: "details.json"
  type: "[]map[string]any"
  format: json
` + columns
	if trailing != "" {
		if !strings.HasSuffix(d, "\n") {
			d += "\n"
		}
		d += trailing
		if !strings.HasSuffix(d, "\n") {
			d += "\n"
		}
	}
	return d
}

// parentChildFiles wires one parent collection with a `child` subcollection.
// parents maps a parent record key to its child details.json content.
func parentChildFiles(childDefYAML string, parents map[string]string) map[string]string {
	files := map[string]string{
		"parent/.collection/definition.yaml":                      parentDefYAML,
		"parent/.collection/subcollections/child/definition.yaml": childDefYAML,
	}
	for pkey, detailsJSON := range parents {
		files["parent/$records/"+pkey+".yaml"] = "name: Parent " + pkey + "\n"
		if detailsJSON != "" {
			files["parent/$records/"+pkey+"/child/details.json"] = detailsJSON
		}
	}
	return files
}

// Verifies subcollection-record-validation#ac:subcollection-record-type-error-surfaced.
func TestSubcolE2E_TypeErrorSurfaced(t *testing.T) {
	cols := "columns:\n  qty:\n    type: int\n    required: true\n"
	files := parentChildFiles(childDef(cols, ""), map[string]string{
		"p1": `[{"$ID":"c1","qty":"lots"}]`,
	})
	errs := loadAndValidate(t, writeDB(t, map[string]string{"parent": "./parent"}, files))

	childErrs := findingsFor(errs, "parent/child")
	if len(childErrs) != 1 {
		t.Fatalf("expected 1 finding for parent/child, got %d: %v", len(childErrs), errs)
	}
	e := childErrs[0]
	if !strings.Contains(e.Message, "qty") || !strings.Contains(e.Message, "wrong type") {
		t.Errorf("expected wrong-type message for qty, got: %s", e.Message)
	}
	wantSuffix := filepath.Join("parent", "$records", "p1", "child", "details.json")
	if !strings.HasSuffix(e.FilePath, wantSuffix) {
		t.Errorf("FilePath must identify the parent instance record file (…/%s), got: %s", wantSuffix, e.FilePath)
	}
}

// Verifies subcollection-record-validation#ac:subcollection-required-field-enforced.
func TestSubcolE2E_RequiredFieldEnforced(t *testing.T) {
	cols := "columns:\n  qty:\n    type: int\n    required: true\n  note:\n    type: string\n    required: true\n"
	files := parentChildFiles(childDef(cols, ""), map[string]string{
		"p1": `[{"$ID":"c1","qty":3}]`, // note omitted
	})
	errs := loadAndValidate(t, writeDB(t, map[string]string{"parent": "./parent"}, files))

	childErrs := findingsFor(errs, "parent/child")
	if len(childErrs) != 1 {
		t.Fatalf("expected 1 missing-required finding, got %d: %v", len(childErrs), errs)
	}
	if !strings.Contains(childErrs[0].Message, "missing required field") || childErrs[0].FieldName != "note" {
		t.Errorf("expected missing-required for note, got field=%q msg=%q", childErrs[0].FieldName, childErrs[0].Message)
	}
}

// Verifies subcollection-record-validation#ac:subcollection-undeclared-field-surfaced.
func TestSubcolE2E_UndeclaredFieldSurfaced(t *testing.T) {
	cols := "columns:\n  qty:\n    type: int\n    required: true\n"
	files := parentChildFiles(childDef(cols, ""), map[string]string{
		"p1": `[{"$ID":"c1","qty":3,"bogus":"x"}]`,
	})
	errs := loadAndValidate(t, writeDB(t, map[string]string{"parent": "./parent"}, files))

	childErrs := findingsFor(errs, "parent/child")
	if len(childErrs) != 1 {
		t.Fatalf("expected 1 undeclared-field finding, got %d: %v", len(childErrs), errs)
	}
	if !strings.Contains(childErrs[0].Message, "undeclared field") || !strings.Contains(childErrs[0].Message, "bogus") {
		t.Errorf("expected undeclared-field for bogus, got: %s", childErrs[0].Message)
	}
}

// Verifies subcollection-record-validation#ac:subcollection-foreign-key-checked.
func TestSubcolE2E_ForeignKeyChecked(t *testing.T) {
	cols := "columns:\n  qty:\n    type: int\n    required: true\n  product_id:\n    type: string\n    required: true\n    foreign_key: products\n"
	files := parentChildFiles(childDef(cols, ""), map[string]string{
		"p1": `[{"$ID":"c1","qty":1,"product_id":"prod001"},{"$ID":"c2","qty":1,"product_id":"ghost"}]`,
	})
	// products target: has prod001, not ghost.
	files["products/.collection/definition.yaml"] = "titles:\n  en: Products\nrecord_file:\n  name: products.yaml\n  type: \"map[$record_id]map[$field_name]any\"\n  format: yaml\ncolumns:\n  name:\n    type: string\n"
	files["products/products.yaml"] = "prod001:\n  name: Widget\n"

	errs := loadAndValidate(t, writeDB(t, map[string]string{"parent": "./parent", "products": "./products"}, files))

	childErrs := findingsFor(errs, "parent/child")
	if len(childErrs) != 1 {
		t.Fatalf("expected 1 dangling-FK finding, got %d: %v", len(childErrs), errs)
	}
	for _, want := range []string{"product_id", "ghost", "products"} {
		if !strings.Contains(childErrs[0].Message, want) {
			t.Errorf("dangling-FK message must mention %q, got: %s", want, childErrs[0].Message)
		}
	}
}

// Verifies subcollection-record-validation#ac:valid-subcollection-records-pass.
func TestSubcolE2E_ValidRecordsPass(t *testing.T) {
	cols := "columns:\n  qty:\n    type: int\n    required: true\n  product_id:\n    type: string\n    required: true\n    foreign_key: products\n"
	files := parentChildFiles(childDef(cols, ""), map[string]string{
		"p1": `[{"$ID":"c1","qty":1,"product_id":"prod001"}]`,
		"p2": `[{"$ID":"c2","qty":2,"product_id":"prod002"}]`,
	})
	files["products/.collection/definition.yaml"] = "titles:\n  en: Products\nrecord_file:\n  name: products.yaml\n  type: \"map[$record_id]map[$field_name]any\"\n  format: yaml\ncolumns:\n  name:\n    type: string\n"
	files["products/products.yaml"] = "prod001:\n  name: Widget\nprod002:\n  name: Gadget\n"

	errs := loadAndValidate(t, writeDB(t, map[string]string{"parent": "./parent", "products": "./products"}, files))
	if childErrs := findingsFor(errs, "parent/child"); len(childErrs) != 0 {
		t.Fatalf("valid subcollection records must pass, got: %v", childErrs)
	}
}

// Verifies subcollection-record-validation#ac:nested-subcollection-records-validated.
// parent -> child -> grandchild, with the type error two levels below root.
func TestSubcolE2E_NestedRecordsValidated(t *testing.T) {
	childCols := "columns:\n  qty:\n    type: int\n    required: true\n"
	grandCols := "columns:\n  score:\n    type: int\n    required: true\n"

	files := map[string]string{
		"parent/.collection/definition.yaml":                                                parentDefYAML,
		"parent/.collection/subcollections/child/definition.yaml":                           childDef(childCols, ""),
		"parent/.collection/subcollections/child/subcollections/grandchild/definition.yaml": childDef(grandCols, ""),
		"parent/$records/p1.yaml":                                                           "name: P1\n",
		"parent/$records/p1/child/details.json":                                             `[{"$ID":"c1","qty":1}]`,
		// grandchild lives under the child record's own directory (child is a
		// list-in-one-file collection: records-base-path is empty, so the
		// per-record dir is <child dir>/<childKey>/<subId>/).
		"parent/$records/p1/child/c1/grandchild/details.json": `[{"$ID":"g1","score":"high"}]`,
	}
	errs := loadAndValidate(t, writeDB(t, map[string]string{"parent": "./parent"}, files))

	gErrs := findingsFor(errs, "parent/child/grandchild")
	if len(gErrs) != 1 {
		t.Fatalf("expected 1 finding two levels deep (parent/child/grandchild), got %d: %v", len(gErrs), errs)
	}
	if !strings.Contains(gErrs[0].Message, "score") || !strings.Contains(gErrs[0].Message, "wrong type") {
		t.Errorf("expected wrong-type for score, got: %s", gErrs[0].Message)
	}
}

// Verifies subcollection-record-validation#ac:subcollection-min-records-count-enforced-per-instance.
func TestSubcolE2E_MinRecordsCountPerInstance(t *testing.T) {
	cols := "columns:\n  qty:\n    type: int\n    required: true\n"
	def := childDef(cols, "min_records_count: 1")
	files := parentChildFiles(def, map[string]string{
		"p1": `[{"$ID":"c1","qty":1}]`, // ok: 1 record
		"p2": ``,                       // empty instance: no details.json => 0 records
	})
	// p2 still needs a parent record file with no child data — parentChildFiles
	// writes it because the map key exists.
	errs := loadAndValidate(t, writeDB(t, map[string]string{"parent": "./parent"}, files))

	var countErrs []ingitdb.ValidationError
	for _, e := range findingsFor(errs, "parent/child") {
		if strings.Contains(e.Message, "records_count") {
			countErrs = append(countErrs, e)
		}
	}
	if len(countErrs) != 1 {
		t.Fatalf("expected 1 per-instance min-records-count finding (p2 empty), got %d: %v", len(countErrs), errs)
	}
	if !strings.Contains(countErrs[0].Message, "min_records_count") {
		t.Errorf("expected min_records_count finding, got: %s", countErrs[0].Message)
	}
	if !strings.Contains(countErrs[0].FilePath, filepath.Join("p2", "child")) {
		t.Errorf("record-count finding must identify the offending parent instance (p2), got FilePath=%s", countErrs[0].FilePath)
	}
}

// Verifies subcollection-record-validation#ac:root-findings-unchanged-by-subcollection-walk.
// A parent record with a root-level defect plus a clean subcollection: the root
// finding is present and the subcollection contributes none.
func TestSubcolE2E_RootFindingsUnchanged(t *testing.T) {
	cols := "columns:\n  qty:\n    type: int\n    required: true\n"
	files := parentChildFiles(childDef(cols, ""), map[string]string{
		"p1": `[{"$ID":"c1","qty":1}]`,
	})
	// Introduce a root-level defect: p1's parent record carries an undeclared field.
	files["parent/$records/p1.yaml"] = "name: P1\nbogus_root: 1\n"

	errs := loadAndValidate(t, writeDB(t, map[string]string{"parent": "./parent"}, files))

	rootErrs := findingsFor(errs, "parent")
	if len(rootErrs) != 1 || !strings.Contains(rootErrs[0].Message, "bogus_root") {
		t.Fatalf("expected exactly the root undeclared-field finding, got: %v", rootErrs)
	}
	if childErrs := findingsFor(errs, "parent/child"); len(childErrs) != 0 {
		t.Fatalf("clean subcollection must contribute no finding, got: %v", childErrs)
	}
}

// Verifies subcollection-record-validation#ac:demo-ingitdb-subcollections-validate-clean.
func TestSubcolE2E_DemoIngitdbSubcollectionsClean(t *testing.T) {
	db := "/Users/alex/projects/ingitdb/demo-ingitdb"
	if _, err := os.Stat(db); os.IsNotExist(err) {
		t.Skipf("%s not present", db)
	}
	def, err := ReadDefinition(db, ingitdb.Validate())
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	res, err := datavalidator.NewValidator().Validate(context.Background(), db, def)
	if err != nil {
		t.Fatalf("validate error: %v", err)
	}
	errs := res.Errors()
	// The newly-walked orders/order_details subcollection is clean — and since
	// the root commerce.addresses postal_code data was fixed (ingitdb-go#6), the
	// whole database now validates clean. This asserts both at once.
	if len(errs) != 0 {
		t.Errorf("demo-ingitdb must validate clean (root + subcollections), got %d findings: %v", len(errs), errs)
	}
}
