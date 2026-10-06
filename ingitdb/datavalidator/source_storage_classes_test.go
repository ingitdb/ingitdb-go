package datavalidator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ingitdb "github.com/ingitdb/ingitdb-go/ingitdb"
)

func TestStorageClassSidecarBindsToRecordIDs(t *testing.T) {
	dir := t.TempDir()
	col := writeMapCollectionJSON(t, dir, "amounts", `{"a":{"amount":"1.5"},"b":{"amount":"2"}}`, map[string]*ingitdb.ColumnDef{"amount": {Type: ingitdb.ColumnTypeString}})
	col.SourceSchema = &ingitdb.SourceSchemaDef{Fields: []ingitdb.SourceFieldDef{{Name: "amount", Type: "decimal"}}, StorageClassFiles: []string{"source-storage-0001.jsonl"}}
	path := filepath.Join(col.DirPath, col.SourceSchema.StorageClassFiles[0])
	if err := os.WriteFile(path, []byte("{\"id\":\"a\",\"classes\":{\"amount\":\"real\"}}\n{\"id\":\"b\",\"classes\":{\"amount\":\"integer\"}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if findings := validateStorageClassFiles("amounts", col); len(findings) != 0 {
		t.Fatalf("valid sidecar rejected: %v", findings)
	}
	if err := os.WriteFile(path, []byte("{\"id\":\"a\",\"classes\":{\"amount\":\"real\"}}\n{\"id\":\"a\",\"classes\":{\"amount\":\"real\"}}\n{\"id\":\"missing\",\"classes\":{\"amount\":\"integer\"}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	findings := validateStorageClassFiles("amounts", col)
	if len(findings) != 3 {
		t.Fatalf("expected duplicate, dangling, and missing entry; got %v", findings)
	}
	joined := findings[0].Message + findings[1].Message + findings[2].Message
	for _, want := range []string{"duplicate", "no matching", "missing"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %v", want, findings)
		}
	}
}

func TestSourceSchemaRejectsUnsafeSidecarPath(t *testing.T) {
	col := &ingitdb.CollectionDef{ID: "x", Columns: map[string]*ingitdb.ColumnDef{"amount": {Type: ingitdb.ColumnTypeString}}, RecordFile: &ingitdb.RecordFileDef{Name: "data.json", Format: ingitdb.RecordFormatJSON, RecordType: ingitdb.MapOfRecords}, SourceSchema: &ingitdb.SourceSchemaDef{StorageClassFiles: []string{"../outside.jsonl"}}}
	if err := col.Validate(); err == nil || !strings.Contains(err.Error(), "storage class file") {
		t.Fatalf("unsafe path accepted: %v", err)
	}
}

func TestStorageClassSidecarMalformedInputs(t *testing.T) {
	dir := t.TempDir()
	col := writeMapCollectionJSON(t, dir, "amounts", `{"a":{"amount":"1.5","other":"x"}}`, map[string]*ingitdb.ColumnDef{"amount": {Type: ingitdb.ColumnTypeString}, "other": {Type: ingitdb.ColumnTypeString}})
	col.SourceSchema = &ingitdb.SourceSchemaDef{Fields: []ingitdb.SourceFieldDef{{Name: "amount", Type: "decimal"}}, StorageClassFiles: []string{"classes.jsonl"}}
	path := filepath.Join(col.DirPath, "classes.jsonl")
	if got := validateStorageClassFiles("amounts", col); len(got) < 1 || !strings.Contains(got[0].Message, "cannot read") {
		t.Fatalf("missing file: %v", got)
	}
	bad := `not-json
{"id":"","classes":{}}
{"id":"a","classes":{"other":"text","amount":"unknown"}}
`
	if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	got := validateStorageClassFiles("amounts", col)
	joined := ""
	for _, f := range got {
		joined += f.Message + "\n"
	}
	for _, want := range []string{"invalid source storage class line", "duplicate or empty", "non-decimal field", "invalid source storage class"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q: %v", want, got)
		}
	}
	if err := os.WriteFile(path, []byte(`{"id":"a","classes":{"amount":"real"}}`+"\n"+strings.Repeat("x", 4*1024*1024+1)), 0o644); err != nil {
		t.Fatal(err)
	}
	got = validateStorageClassFiles("amounts", col)
	found := false
	for _, f := range got {
		if strings.Contains(f.Message, "read source storage classes") {
			found = true
		}
	}
	if !found {
		t.Fatalf("oversize sidecar line: %v", got)
	}
	if err := os.WriteFile(filepath.Join(col.DirPath, "data.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := validateStorageClassFiles("amounts", col); got != nil {
		t.Fatalf("ordinary record parser owns error: %v", got)
	}
}

func TestStorageClassSidecarRejectsNull(t *testing.T) {
	dir := t.TempDir()
	col := writeMapCollectionJSON(t, dir, "amounts", `{"a":{"amount":null}}`, map[string]*ingitdb.ColumnDef{"amount": {Type: ingitdb.ColumnTypeString}})
	col.SourceSchema = &ingitdb.SourceSchemaDef{Fields: []ingitdb.SourceFieldDef{{Name: "amount", Type: "decimal"}}, StorageClassFiles: []string{"classes.jsonl"}}
	if err := os.WriteFile(filepath.Join(col.DirPath, "classes.jsonl"), []byte(`{"id":"a","classes":{"amount":"real"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := validateStorageClassFiles("amounts", col)
	if len(got) != 1 || !strings.Contains(got[0].Message, "null or absent") {
		t.Fatalf("null sidecar: %v", got)
	}
}

func TestValidatorReportsStorageClassSidecarError(t *testing.T) {
	dir := t.TempDir()
	col := writeMapCollectionJSON(t, dir, "amounts", `{"a":{"amount":"1.5"}}`, map[string]*ingitdb.ColumnDef{"amount": {Type: ingitdb.ColumnTypeString}})
	col.SourceSchema = &ingitdb.SourceSchemaDef{Fields: []ingitdb.SourceFieldDef{{Name: "amount", Type: "decimal"}}, StorageClassFiles: []string{"missing.jsonl"}}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"amounts": col}}
	result, err := NewValidator().Validate(context.Background(), dir, def)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors()) == 0 {
		t.Fatal("missing storage-class file must be reported")
	}
}
