package materializer

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

type fakeViewDefReader struct {
	views map[string]*ingitdb.ViewDef
}

func (f fakeViewDefReader) ReadViewDefs(string) (map[string]*ingitdb.ViewDef, error) {
	return f.views, nil
}

type fakeRecordsReader struct {
	records []ingitdb.RecordEntry
}

func (f fakeRecordsReader) ReadRecords(
	ctx context.Context,
	dbPath string,
	col *ingitdb.CollectionDef,
	yield func(ingitdb.RecordEntry) error,
) error {
	_ = ctx
	_ = dbPath
	_ = col
	for _, record := range f.records {
		if err := yield(record); err != nil {
			return err
		}
	}
	return nil
}

type capturingWriter struct {
	lastOutPath string
	lastRecords []ingitdb.RecordEntry
	called      int
}

func (w *capturingWriter) WriteView(
	ctx context.Context,
	col *ingitdb.CollectionDef,
	view *ingitdb.ViewDef,
	records []ingitdb.RecordEntry,
	outPath string,
) (bool, error) {
	_ = ctx
	_ = col
	_ = view
	w.called++
	w.lastOutPath = outPath
	w.lastRecords = make([]ingitdb.RecordEntry, len(records))
	copy(w.lastRecords, records)
	return true, nil
}

func TestSimpleViewBuilder_BuildViewsOrdersRecords(t *testing.T) {
	t.Parallel()

	col := &ingitdb.CollectionDef{
		ID:      "todo.tags",
		DirPath: "/tmp/tags",
	}
	view := &ingitdb.ViewDef{
		ID:       "README",
		OrderBy:  "title desc",
		Top:      2,
		Columns:  []string{"title"},
		FileName: "README.md",
	}
	writer := &capturingWriter{}
	builder := SimpleViewBuilder{
		DefReader: fakeViewDefReader{views: map[string]*ingitdb.ViewDef{"README": view}},
		RecordsReader: fakeRecordsReader{records: []ingitdb.RecordEntry{
			{Key: "a", Data: map[string]any{"title": "Alpha", "extra": "x"}},
			{Key: "c", Data: map[string]any{"title": "Charlie", "extra": "y"}},
			{Key: "b", Data: map[string]any{"title": "Bravo", "extra": "z"}},
		}},
		Writer: writer,
	}

	result, err := builder.BuildViews(context.Background(), "/db", col, &ingitdb.Definition{})
	if err != nil {
		t.Fatalf("BuildViews: %v", err)
	}
	if result.FilesWritten != 1 {
		t.Fatalf("expected 1 file written, got %d", result.FilesWritten)
	}
	if writer.called != 1 {
		t.Fatalf("expected writer called once, got %d", writer.called)
	}
	expectedPath := filepath.Join(col.DirPath, "README.md")
	if writer.lastOutPath != expectedPath {
		t.Fatalf("expected out path %q, got %q", expectedPath, writer.lastOutPath)
	}
	if len(writer.lastRecords) != 2 {
		t.Fatalf("expected 2 records after top filter, got %d", len(writer.lastRecords))
	}
	order := []string{
		writer.lastRecords[0].Data["title"].(string),
		writer.lastRecords[1].Data["title"].(string),
	}
	if !reflect.DeepEqual(order, []string{"Charlie", "Bravo"}) {
		t.Fatalf("unexpected order: %v", order)
	}
	for _, record := range writer.lastRecords {
		if _, ok := record.Data["extra"]; ok {
			t.Fatalf("expected extra column to be filtered out")
		}
		if _, ok := record.Data["title"]; !ok {
			t.Fatalf("expected title column to remain")
		}
	}
}

func TestSimpleViewBuilder_MissingDependencies(t *testing.T) {
	t.Parallel()

	builder := SimpleViewBuilder{}
	_, err := builder.BuildViews(context.Background(), "/db", &ingitdb.CollectionDef{}, &ingitdb.Definition{})
	if err == nil {
		t.Fatalf("expected error for missing dependencies")
	}
}
