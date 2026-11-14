package materializer

// specscore: feature/record-key-encoding

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

// Verifies record-key-encoding#ac:readers-use-decoded-keys for views: the
// reader yields decoded keys and fails on a name the encoding never writes.
func TestReadRecords_KeyEncodingTildeV1(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(recordsDir, "a~2Fb.yaml"), []byte("title: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	col := &ingitdb.CollectionDef{ID: "c", DirPath: dir, RecordFile: &ingitdb.RecordFileDef{
		Name: "{key}.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.SingleRecord, KeyEncoding: ingitdb.KeyEncodingTildeV1}}
	var keys []string
	err := NewFileRecordsReader().ReadRecords(context.Background(), dir, col, func(e ingitdb.IRecordEntry) error {
		keys = append(keys, e.GetID())
		return nil
	})
	if err != nil || len(keys) != 1 || keys[0] != "a/b" {
		t.Fatalf("keys = %q, %v", keys, err)
	}
	if err := os.WriteFile(filepath.Join(recordsDir, "a b.yaml"), []byte("title: y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = NewFileRecordsReader().ReadRecords(context.Background(), dir, col, func(ingitdb.IRecordEntry) error { return nil })
	if !errors.Is(err, ingitdb.ErrInvalidKeyName) {
		t.Fatalf("expected ErrInvalidKeyName, got %v", err)
	}
}
