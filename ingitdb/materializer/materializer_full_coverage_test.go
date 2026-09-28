package materializer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

type nilDataRecord struct{}

func (nilDataRecord) GetID() string            { return "nil-rec" }
func (nilDataRecord) GetData() map[string]any { return nil }

type outcomeWriter struct {
	outcome WriteOutcome
	err     error
}

func (w outcomeWriter) WriteView(
	_ context.Context,
	_ *ingitdb.CollectionDef,
	_ *ingitdb.ViewDef,
	_ []ingitdb.IRecordEntry,
	_ string,
) (WriteOutcome, error) {
	return w.outcome, w.err
}

func TestExtractParameterField_EdgeCases(t *testing.T) {
	if _, ok := extractParameterField("foo{"); ok {
		t.Error("expected false for unclosed brace")
	}
	if _, ok := extractParameterField("foo{}bar"); ok {
		t.Error("expected false for empty parameter name")
	}
}

func TestBuildParameterizedViews_OutcomesAndErrors(t *testing.T) {
	dir := t.TempDir()
	col := &ingitdb.CollectionDef{ID: "items", DirPath: filepath.Join(dir, "items")}
	view := &ingitdb.ViewDef{
		ID:     "by_{category}",
		Titles: map[string]string{"en": "Category {category}"},
	}
	records := []ingitdb.IRecordEntry{
		nilDataRecord{},
		ingitdb.NewMapRecordEntry("1", map[string]any{"category": ""}),
		ingitdb.NewMapRecordEntry("2", map[string]any{"category": nil}),
		ingitdb.NewMapRecordEntry("3", map[string]any{"category": "catA"}),
	}

	// Test error outcome
	resErr := &ingitdb.MaterializeResult{}
	buildParameterizedViews(
		context.Background(), col, view, records, "category", dir, dir,
		outcomeWriter{err: errors.New("write failed")}, nil, resErr,
	)
	if len(resErr.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(resErr.Errors))
	}

	// Test WriteOutcomeUpdated and logging
	resUpdated := &ingitdb.MaterializeResult{}
	var logged bool
	logf := func(format string, args ...any) { logged = true }
	buildParameterizedViews(
		context.Background(), col, view, records, "category", dir, dir,
		outcomeWriter{outcome: WriteOutcomeUpdated}, logf, resUpdated,
	)
	if resUpdated.FilesUpdated != 1 || !logged {
		t.Errorf("expected 1 updated and logged, got filesUpdated=%d, logged=%v", resUpdated.FilesUpdated, logged)
	}

	// Test WriteOutcomeUnchanged (default)
	resUnchanged := &ingitdb.MaterializeResult{}
	buildParameterizedViews(
		context.Background(), col, view, records, "category", dir, dir,
		outcomeWriter{outcome: WriteOutcomeUnchanged}, nil, resUnchanged,
	)
	if resUnchanged.FilesUnchanged != 1 {
		t.Errorf("expected 1 unchanged, got %d", resUnchanged.FilesUnchanged)
	}
}

func TestSimpleViewBuilder_BuildView_Parameterized(t *testing.T) {
	dir := t.TempDir()
	col := &ingitdb.CollectionDef{ID: "items", DirPath: filepath.Join(dir, "items")}
	view := &ingitdb.ViewDef{
		ID:      "by_{category}",
		OrderBy: "title",
	}
	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("1", map[string]any{"category": "catA", "title": "B"}),
		ingitdb.NewMapRecordEntry("2", map[string]any{"category": "catA", "title": "A"}),
	}
	builder := SimpleViewBuilder{
		RecordsReader: fakeRecordsReader{records: records},
		Writer:        outcomeWriter{outcome: WriteOutcomeCreated},
	}
	res, err := builder.BuildView(context.Background(), dir, dir, col, nil, view)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.FilesCreated != 1 {
		t.Errorf("expected 1 file created, got %d", res.FilesCreated)
	}
}

func TestFuncViewWriter_BuiltinError(t *testing.T) {
	w := NewFuncViewWriter(func(content []byte) error {
		return errors.New("write failure")
	})
	col := &ingitdb.CollectionDef{ID: "c"}
	view := &ingitdb.ViewDef{
		Formats: []string{"md"},
		Columns: []string{"id"},
	}
	recs := []ingitdb.IRecordEntry{ingitdb.NewMapRecordEntry("1", map[string]any{"id": "1"})}
	_, err := w.WriteView(context.Background(), col, view, recs, "out.md")
	if err == nil {
		t.Fatal("expected error from FuncViewWriter with failing write")
	}
}

func TestFileViewWriter_BuiltinView_Branches(t *testing.T) {
	col := &ingitdb.CollectionDef{ID: "c"}
	view := &ingitdb.ViewDef{
		Formats: []string{"md"},
		Columns: []string{"id"},
	}
	recs := []ingitdb.IRecordEntry{ingitdb.NewMapRecordEntry("1", map[string]any{"id": "1"})}
	rendered, err := renderBuiltinView(view, recs)
	if err != nil {
		t.Fatalf("renderBuiltinView: %v", err)
	}

	// 1. Existing file equals content -> WriteOutcomeUnchanged
	wUnchanged := FileViewWriter{
		readFile: func(string) ([]byte, error) { return rendered, nil },
	}
	outcome, err := wUnchanged.WriteView(context.Background(), col, view, recs, "/tmp/out.md")
	if err != nil || outcome != WriteOutcomeUnchanged {
		t.Errorf("expected unchanged, got outcome=%v, err=%v", outcome, err)
	}

	// 2. MkdirAll fails
	wMkdirFail := FileViewWriter{
		readFile: func(string) ([]byte, error) { return nil, errors.New("not found") },
		mkdirAll: func(string, os.FileMode) error { return errors.New("mkdir fail") },
	}
	_, err = wMkdirFail.WriteView(context.Background(), col, view, recs, "/tmp/out.md")
	if err == nil {
		t.Error("expected error from failed mkdirAll")
	}

	// 3. WriteFile fails
	wWriteFail := FileViewWriter{
		readFile:  func(string) ([]byte, error) { return nil, errors.New("not found") },
		mkdirAll:  func(string, os.FileMode) error { return nil },
		writeFile: func(string, []byte, os.FileMode) error { return errors.New("write fail") },
	}
	_, err = wWriteFail.WriteView(context.Background(), col, view, recs, "/tmp/out.md")
	if err == nil {
		t.Error("expected error from failed writeFile")
	}

	// 4. File existed with different content -> WriteOutcomeUpdated
	wUpdated := FileViewWriter{
		readFile:  func(string) ([]byte, error) { return []byte("different"), nil },
		mkdirAll:  func(string, os.FileMode) error { return nil },
		writeFile: func(string, []byte, os.FileMode) error { return nil },
	}
	outcome, err = wUpdated.WriteView(context.Background(), col, view, recs, "/tmp/out.md")
	if err != nil || outcome != WriteOutcomeUpdated {
		t.Errorf("expected updated, got outcome=%v, err=%v", outcome, err)
	}
}

func TestRenderBuiltinMDTable_DeriveColumnsFromRecords(t *testing.T) {
	view := &ingitdb.ViewDef{
		// No explicit Columns
	}
	recs := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("1", map[string]any{"b": 2, "a": 1}),
	}
	table := renderBuiltinMDTable(view, recs)
	str := string(table)
	if str == "" {
		t.Error("expected non-empty markdown table")
	}
}
