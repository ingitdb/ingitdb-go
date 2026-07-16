package validator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
	"github.com/ingitdb/ingitdb-go/ingitdb/datavalidator"
)

// writeRecordCountDB builds a real single-collection inGitDB database in a temp
// dir: a `widgets` MapOfRecords collection holding recordKeys, whose definition
// carries the given verbatim record-count bound lines (e.g.
// "min_records_count: 2"). It returns the database root path so tests exercise
// the production reader + validator, never a hand-unmarshalled struct.
func writeRecordCountDB(t *testing.T, boundLines string, recordKeys ...string) string {
	t.Helper()
	dir := t.TempDir()

	ingitdbDir := filepath.Join(dir, ".ingitdb")
	if err := os.MkdirAll(ingitdbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ingitdbDir, "settings.yaml"),
		[]byte("languages:\n  - required: en\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ingitdbDir, "root-collections.yaml"),
		[]byte("widgets: ./widgets\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	schemaDir := filepath.Join(dir, "widgets", ".collection")
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	def := "record_file:\n  name: widgets.yaml\n  type: \"map[$record_id]map[$field_name]any\"\n  format: yaml\ncolumns:\n  name:\n    type: string\n"
	if boundLines != "" {
		def += boundLines
		if !strings.HasSuffix(def, "\n") {
			def += "\n"
		}
	}
	if err := os.WriteFile(filepath.Join(schemaDir, "definition.yaml"), []byte(def), 0o644); err != nil {
		t.Fatal(err)
	}

	var records strings.Builder
	for i, key := range recordKeys {
		fmt.Fprintf(&records, "%s:\n  name: Widget %d\n", key, i+1)
	}
	if err := os.WriteFile(filepath.Join(dir, "widgets", "widgets.yaml"), []byte(records.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func recordCountErrors(errs []ingitdb.ValidationError) []ingitdb.ValidationError {
	var out []ingitdb.ValidationError
	for _, e := range errs {
		if strings.Contains(e.Message, "records_count") {
			out = append(out, e)
		}
	}
	return out
}

// Verifies record-count-constraints#ac:min-records-count-rejects-too-few
// through the real reader + validator.
func TestRecordCountE2E_MinRejectsTooFew(t *testing.T) {
	dir := writeRecordCountDB(t, "min_records_count: 2", "w1")

	def, err := ReadDefinition(dir, ingitdb.Validate())
	if err != nil {
		t.Fatalf("ReadDefinition failed: %v", err)
	}
	res, err := datavalidator.NewValidator().Validate(context.Background(), dir, def)
	if err != nil {
		t.Fatal(err)
	}
	viol := recordCountErrors(res.Errors())
	if len(viol) != 1 {
		t.Fatalf("expected 1 record-count violation, got %d: %v", len(viol), res.Errors())
	}
	for _, want := range []string{"widgets", "min_records_count", "2", "1"} {
		if !strings.Contains(viol[0].Error(), want) {
			t.Errorf("error must mention %q, got: %s", want, viol[0].Error())
		}
	}
}

// Verifies record-count-constraints#ac:record-count-within-bounds-passes
// through the real reader + validator.
func TestRecordCountE2E_WithinBoundsPasses(t *testing.T) {
	dir := writeRecordCountDB(t, "min_records_count: 1\nmax_records_count: 5", "w1", "w2", "w3")

	def, err := ReadDefinition(dir, ingitdb.Validate())
	if err != nil {
		t.Fatalf("ReadDefinition failed: %v", err)
	}
	res, err := datavalidator.NewValidator().Validate(context.Background(), dir, def)
	if err != nil {
		t.Fatal(err)
	}
	if viol := recordCountErrors(res.Errors()); len(viol) != 0 {
		t.Fatalf("3 records within [1,5] must pass, got: %v", viol)
	}
}

// Verifies record-count-constraints#ac:max-records-count-rejects-too-many
// through the real reader + validator.
func TestRecordCountE2E_MaxRejectsTooMany(t *testing.T) {
	dir := writeRecordCountDB(t, "max_records_count: 1", "w1", "w2")

	def, err := ReadDefinition(dir, ingitdb.Validate())
	if err != nil {
		t.Fatalf("ReadDefinition failed: %v", err)
	}
	res, err := datavalidator.NewValidator().Validate(context.Background(), dir, def)
	if err != nil {
		t.Fatal(err)
	}
	viol := recordCountErrors(res.Errors())
	if len(viol) != 1 {
		t.Fatalf("expected 1 record-count violation, got %d: %v", len(viol), res.Errors())
	}
	for _, want := range []string{"widgets", "max_records_count", "1", "2"} {
		if !strings.Contains(viol[0].Error(), want) {
			t.Errorf("error must mention %q, got: %s", want, viol[0].Error())
		}
	}
}

// An impossible bound is a definition-load error surfaced by the real reader,
// not a validation-time finding. Verifies
// record-count-constraints#ac:negative-min-rejected-at-load and
// #ac:min-exceeds-max-rejected-at-load through ReadDefinition.
func TestRecordCountE2E_InvalidBoundsRejectedAtLoad(t *testing.T) {
	cases := []struct {
		name       string
		boundLines string
		wantSubstr []string
	}{
		{"negative min", "min_records_count: -1", []string{"min_records_count", "negative"}},
		{"negative max", "max_records_count: -1", []string{"max_records_count", "negative"}},
		{"min exceeds max", "min_records_count: 10\nmax_records_count: 5", []string{"min_records_count", "max_records_count", "exceeds"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeRecordCountDB(t, tc.boundLines, "w1")
			_, err := ReadDefinition(dir, ingitdb.Validate())
			if err == nil {
				t.Fatalf("an invalid record-count bound must fail ReadDefinition under validation")
			}
			for _, want := range tc.wantSubstr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("load error must mention %q, got: %v", want, err)
				}
			}
		})
	}
}

// Verifies record-count-constraints#ac:known-databases-validate-clean. None of
// the loadable workspace databases declares a record-count bound, so the new
// check must contribute no error to any of them — asserted specifically for
// record-count errors, since some of these databases carry unrelated
// pre-existing schema violations.
func TestRecordCountE2E_KnownDatabasesHaveNoRecordCountErrors(t *testing.T) {
	dbs := []string{
		"/Users/alex/projects/ingitdb/demo-ingitdb",
		"/Users/alex/projects/ingitdb/demo-commerce-ingitdb",
		"/Users/alex/projects/ingitdb/e2e-test-ingitdb",
		"/Users/alex/projects/bots-go-framework/can-i-use",
	}
	for _, db := range dbs {
		if _, err := os.Stat(db); os.IsNotExist(err) {
			t.Logf("%s: not present in this workspace, skipping", db)
			continue
		}
		def, err := ReadDefinition(db, ingitdb.Validate())
		if err != nil {
			t.Errorf("%s: load error: %v", db, err)
			continue
		}
		res, err := datavalidator.NewValidator().Validate(context.Background(), db, def)
		if err != nil {
			t.Errorf("%s: validate error: %v", db, err)
			continue
		}
		if viol := recordCountErrors(res.Errors()); len(viol) != 0 {
			t.Errorf("%s: expected no record-count violations, got: %v", db, viol)
		}
	}
}
