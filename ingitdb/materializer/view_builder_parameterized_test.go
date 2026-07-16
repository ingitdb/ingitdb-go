package materializer

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

// allCallsCapturingWriter accumulates all calls to WriteView.
type allCallsCapturingWriter struct {
	calls []capturingWriterCall
}

type capturingWriterCall struct {
	outPath string
	records []ingitdb.IRecordEntry
}

func (w *allCallsCapturingWriter) WriteView(
	ctx context.Context,
	col *ingitdb.CollectionDef,
	view *ingitdb.ViewDef,
	records []ingitdb.IRecordEntry,
	outPath string,
) (WriteOutcome, error) {
	_ = ctx
	_ = col
	_ = view
	recs := make([]ingitdb.IRecordEntry, len(records))
	copy(recs, records)
	w.calls = append(w.calls, capturingWriterCall{outPath: outPath, records: recs})
	return WriteOutcomeCreated, nil
}

func TestSimpleViewBuilder_BuildViews_ParameterizedView_Partitions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:      "items",
		DirPath: filepath.Join(dir, "items"),
	}
	view := &ingitdb.ViewDef{
		ID:      "by_category_{category}",
		Columns: []string{"title"},
	}
	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("1", map[string]any{"category": "books", "title": "A"}),
		ingitdb.NewMapRecordEntry("2", map[string]any{"category": "books", "title": "B"}),
		ingitdb.NewMapRecordEntry("3", map[string]any{"category": "games", "title": "C"}),
	}
	writer := &allCallsCapturingWriter{}
	builder := SimpleViewBuilder{
		DefReader:     fakeViewDefReader{views: map[string]*ingitdb.ViewDef{"by_category_{category}": view}},
		RecordsReader: fakeRecordsReader{records: records},
		Writer:        writer,
	}

	result, err := builder.BuildViews(context.Background(), dir, dir, col, &ingitdb.Definition{})
	if err != nil {
		t.Fatalf("BuildViews: %v", err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if len(writer.calls) != 2 {
		t.Fatalf("expected writer called 2 times, got %d", len(writer.calls))
	}

	paths := make([]string, 0, len(writer.calls))
	for _, call := range writer.calls {
		paths = append(paths, call.outPath)
	}

	hasBooks := false
	hasGames := false
	for _, p := range paths {
		if strings.Contains(p, "by_category_books") {
			hasBooks = true
		}
		if strings.Contains(p, "by_category_games") {
			hasGames = true
		}
		if strings.Contains(p, "{") || strings.Contains(p, "}") {
			t.Errorf("output path contains literal braces: %q", p)
		}
	}
	if !hasBooks {
		t.Errorf("expected output path containing 'by_category_books', got: %v", paths)
	}
	if !hasGames {
		t.Errorf("expected output path containing 'by_category_games', got: %v", paths)
	}

	// Verify each partition has only the records for that category.
	for _, call := range writer.calls {
		var expectedTitle string
		if strings.Contains(call.outPath, "by_category_books") {
			// books partition should have 2 records
			if len(call.records) != 2 {
				t.Errorf("books partition: expected 2 records, got %d", len(call.records))
			}
		} else if strings.Contains(call.outPath, "by_category_games") {
			// games partition should have 1 record with title "C"
			if len(call.records) != 1 {
				t.Errorf("games partition: expected 1 record, got %d", len(call.records))
			}
			d := call.records[0].GetData()
			expectedTitle = "C"
			if d["title"] != expectedTitle {
				t.Errorf("games partition: expected title %q, got %q", expectedTitle, d["title"])
			}
		}
	}
}

func TestSimpleViewBuilder_BuildViews_ParameterizedView_SkipsRecordsWithMissingField(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:      "items",
		DirPath: filepath.Join(dir, "items"),
	}
	view := &ingitdb.ViewDef{
		ID:      "by_category_{category}",
		Columns: []string{"title"},
	}
	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("1", map[string]any{"category": "books", "title": "A"}),
		ingitdb.NewMapRecordEntry("2", map[string]any{"title": "NoCategory"}),
	}
	writer := &allCallsCapturingWriter{}
	builder := SimpleViewBuilder{
		DefReader:     fakeViewDefReader{views: map[string]*ingitdb.ViewDef{"by_category_{category}": view}},
		RecordsReader: fakeRecordsReader{records: records},
		Writer:        writer,
	}

	result, err := builder.BuildViews(context.Background(), dir, dir, col, &ingitdb.Definition{})
	if err != nil {
		t.Fatalf("BuildViews: %v", err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if len(writer.calls) != 1 {
		t.Fatalf("expected writer called 1 time (only books partition), got %d", len(writer.calls))
	}
	if !strings.Contains(writer.calls[0].outPath, "by_category_books") {
		t.Errorf("expected output path containing 'by_category_books', got: %q", writer.calls[0].outPath)
	}
}

func TestExtractParameterField(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input     string
		wantField string
		wantFound bool
	}{
		{"by_class_{equivalenceClass}", "equivalenceClass", true},
		{"README", "", false},
		{"by_{a}_{b}", "a", true},
		{"", "", false},
	}
	for _, tc := range cases {
		field, found := extractParameterField(tc.input)
		if found != tc.wantFound {
			t.Errorf("extractParameterField(%q): found=%v, want %v", tc.input, found, tc.wantFound)
		}
		if field != tc.wantField {
			t.Errorf("extractParameterField(%q): field=%q, want %q", tc.input, field, tc.wantField)
		}
	}
}
