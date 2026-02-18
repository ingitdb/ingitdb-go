package dalgo2ingitdb

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// makeTestDef builds a minimal Definition with one SingleRecord YAML collection
// rooted at dirPath.
func makeTestDef(t *testing.T, dirPath string) *ingitdb.Definition {
	t.Helper()
	colDef := &ingitdb.CollectionDef{
		ID:      "test.items",
		DirPath: dirPath,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     "yaml",
			RecordType: ingitdb.SingleRecord,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"name": {Type: ingitdb.ColumnTypeString},
		},
	}
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.items": colDef,
		},
	}
	return def
}

func openTestDB(t *testing.T, dirPath string, def *ingitdb.Definition) dal.DB {
	t.Helper()
	db, err := NewLocalDBWithDef(dirPath, def)
	if err != nil {
		t.Fatalf("NewLocalDBWithDef: %v", err)
	}
	return db
}

func writeYAMLFixture(t *testing.T, path string, data map[string]any) {
	t.Helper()
	content, err := yaml.Marshal(data)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	if err = os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}
}

func TestGet_SingleRecord_Found(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeTestDef(t, dir)
	writeYAMLFixture(t, filepath.Join(dir, "abc.yaml"), map[string]any{"name": "Alpha"})

	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "abc")
	data := map[string]any{}
	record := dal.NewRecordWithData(key, data)

	err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !record.Exists() {
		t.Fatal("expected record to exist")
	}
	if data["name"] != "Alpha" {
		t.Fatalf("expected name=Alpha, got %v", data["name"])
	}
}

func TestGet_SingleRecord_NotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeTestDef(t, dir)
	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "missing")
	data := map[string]any{}
	record := dal.NewRecordWithData(key, data)

	err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if record.Exists() {
		t.Fatal("expected record to not exist")
	}
}

func TestInsert_SingleRecord(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeTestDef(t, dir)
	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "new")
	data := map[string]any{"name": "New Item"}
	record := dal.NewRecordWithData(key, data)

	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, record)
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	content, readErr := os.ReadFile(filepath.Join(dir, "new.yaml"))
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	var got map[string]any
	if err = yaml.Unmarshal(content, &got); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if got["name"] != "New Item" {
		t.Fatalf("expected name=New Item, got %v", got["name"])
	}
}

func TestInsert_SingleRecord_KeySubdirPattern(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	colDef := &ingitdb.CollectionDef{
		ID:      "test.items",
		DirPath: dir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}/{key}.yaml",
			Format:     "yaml",
			RecordType: ingitdb.SingleRecord,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"name": {Type: ingitdb.ColumnTypeString},
		},
	}
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{"test.items": colDef},
	}
	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "de")
	data := map[string]any{"name": "Germany"}
	record := dal.NewRecordWithData(key, data)

	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, record)
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	content, readErr := os.ReadFile(filepath.Join(dir, "de", "de.yaml"))
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	var got map[string]any
	if err = yaml.Unmarshal(content, &got); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if got["name"] != "Germany" {
		t.Fatalf("expected name=Germany, got %v", got["name"])
	}
}

func TestInsert_SingleRecord_AlreadyExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeTestDef(t, dir)
	writeYAMLFixture(t, filepath.Join(dir, "dup.yaml"), map[string]any{"name": "Existing"})

	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "dup")
	data := map[string]any{"name": "Duplicate"}
	record := dal.NewRecordWithData(key, data)

	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, record)
	})
	if err == nil {
		t.Fatal("expected error for duplicate insert, got nil")
	}
}

func TestSet_SingleRecord(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeTestDef(t, dir)
	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "upsert")
	data := map[string]any{"name": "Upserted"}
	record := dal.NewRecordWithData(key, data)

	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Set(ctx, record)
	})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	content, readErr := os.ReadFile(filepath.Join(dir, "upsert.yaml"))
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	var got map[string]any
	if err = yaml.Unmarshal(content, &got); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if got["name"] != "Upserted" {
		t.Fatalf("expected name=Upserted, got %v", got["name"])
	}
}

func TestDelete_SingleRecord(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeTestDef(t, dir)
	path := filepath.Join(dir, "del.yaml")
	writeYAMLFixture(t, path, map[string]any{"name": "ToDelete"})

	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "del")

	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, key)
	})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("expected file to be deleted")
	}
}

func TestDelete_SingleRecord_NotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeTestDef(t, dir)
	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "ghost")

	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, key)
	})
	if !errors.Is(err, dal.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}
