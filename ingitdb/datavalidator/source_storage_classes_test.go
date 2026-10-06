package datavalidator

import (
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
	col := &ingitdb.CollectionDef{ID: "x", Columns: map[string]*ingitdb.ColumnDef{"amount": {Type: ingitdb.ColumnTypeString}}, RecordFile: &ingitdb.RecordFileDef{Name: "records.json", Format: ingitdb.RecordFormatJSON, RecordType: ingitdb.MapOfRecords}, SourceSchema: &ingitdb.SourceSchemaDef{StorageClassFiles: []string{"../outside.jsonl"}}}
	if err := col.Validate(); err == nil || !strings.Contains(err.Error(), "storage class file") {
		t.Fatalf("unsafe path accepted: %v", err)
	}
}
