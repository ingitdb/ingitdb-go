package todo

import (
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestRecords_Golden pins every record at the fixed now ovdb's own
// internal/setup/demo/demo_test.go uses (time.Date(2026, 9, 17, 10, 0, 0, 0,
// time.UTC)), so a change to this package that would change what ovdb or
// ingitdb-cli install is caught here first.
func TestRecords_Golden(t *testing.T) {
	now := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)
	want := []Record{
		{Key: "lists/to-buy", Data: map[string]any{"title": "To buy"}},
		{Key: "lists/to-buy/items/milk", Data: map[string]any{
			"title": "Milk", "done": false, "added_at": "2026-09-17T09:59:56Z",
		}},
		{Key: "lists/to-buy/items/bananas", Data: map[string]any{
			"title": "Bananas", "done": false, "added_at": "2026-09-17T09:59:57Z",
		}},
		{Key: "lists/to-buy/items/coffee", Data: map[string]any{
			"title": "Coffee", "done": false, "added_at": "2026-09-17T09:59:58Z",
		}},
		{Key: "lists/to-watch", Data: map[string]any{"title": "To watch"}},
		{Key: "lists/to-watch/items/the-matrix", Data: map[string]any{
			"title": "The Matrix", "done": false, "added_at": "2026-09-17T09:59:59Z",
		}},
		{Key: "lists/to-watch/items/interstellar", Data: map[string]any{
			"title": "Interstellar", "done": false, "added_at": "2026-09-17T10:00:00Z",
		}},
	}
	got := Records(now)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Records(%v) =\n%#v\nwant\n%#v", now, got, want)
	}
	if len(got) != 7 {
		t.Fatalf("len(Records()) = %d, want 7", len(got))
	}
}

// TestRecords_TruncatesToSecond checks the added_at rule against a now with
// sub-second precision: the last item (interstellar) still lands exactly on
// now truncated to the second, never in the future.
func TestRecords_TruncatesToSecond(t *testing.T) {
	now := time.Date(2026, 9, 17, 10, 0, 0, 999_999_999, time.UTC)
	got := Records(now)
	last := got[len(got)-1]
	if last.Key != "lists/to-watch/items/interstellar" {
		t.Fatalf("last record key = %q", last.Key)
	}
	if last.Data["added_at"] != "2026-09-17T10:00:00Z" {
		t.Errorf("interstellar added_at = %v, want 2026-09-17T10:00:00Z", last.Data["added_at"])
	}
}

// TestRecords_NonUTCNow checks a non-UTC now converts to UTC before the
// added_at rule applies.
func TestRecords_NonUTCNow(t *testing.T) {
	loc := time.FixedZone("UTC+2", 2*60*60)
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, loc) // 10:00:00 UTC
	got := Records(now)
	last := got[len(got)-1]
	if last.Data["added_at"] != "2026-09-17T10:00:00Z" {
		t.Errorf("interstellar added_at = %v, want 2026-09-17T10:00:00Z", last.Data["added_at"])
	}
}

func TestApp(t *testing.T) {
	if App != "todo" {
		t.Errorf("App = %q, want %q", App, "todo")
	}
}

func TestLists(t *testing.T) {
	want := []string{"lists/to-buy", "lists/to-watch"}
	if !reflect.DeepEqual(Lists, want) {
		t.Errorf("Lists = %v, want %v", Lists, want)
	}
}

// TestOnlyStandardLibraryImports proves the package, cli/demo#REQ:shared-demo-data
// and todo-demo#REQ:seed-from-shared-package's shared contract, imports
// nothing beyond the Go standard library: no dot in an import path's first
// segment, the same heuristic goimports uses to tell a standard package from
// a module path (github.com/..., golang.org/x/..., ...). It checks every
// non-test file in this directory (the shipped package), parsed file by
// file with parser.ParseFile rather than the deprecated parser.ParseDir.
func TestOnlyStandardLibraryImports(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	checked := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		checked++
		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if !isStandardLibraryImport(path) {
				t.Errorf("file %s: non-standard-library import %q", name, path)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no non-test .go files found in .")
	}
}

// TestOnlyStandardLibraryImports_FindsAViolation proves the check itself
// catches a non-standard-library import, using a file parsed from a string
// rather than one written to this directory (which would break the
// stdlib-only guarantee it verifies).
func TestOnlyStandardLibraryImports_FindsAViolation(t *testing.T) {
	fset := token.NewFileSet()
	src := "package todo\n\nimport \"github.com/ingitdb/ingitdb-go/ingitdb\"\n"
	file, err := parser.ParseFile(fset, "violation.go", src, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if !isStandardLibraryImport(path) {
			found = true
		}
	}
	if !found {
		t.Fatal("expected the non-standard-library import to be detected")
	}
}

// isStandardLibraryImport reports whether path looks like a standard
// library import path rather than a module path: a module path's first
// path segment contains a dot (a domain), such as github.com/x/y.
func isStandardLibraryImport(path string) bool {
	first := path
	if i := strings.Index(path, "/"); i >= 0 {
		first = path[:i]
	}
	return !strings.Contains(first, ".")
}

// TestIsStandardLibraryImport_Self proves the heuristic itself against a
// known standard and a known module path, so a broken heuristic cannot make
// TestOnlyStandardLibraryImports pass vacuously.
func TestIsStandardLibraryImport_Self(t *testing.T) {
	cases := map[string]bool{
		"time":                                  true,
		"go/parser":                             true,
		"github.com/ingitdb/ingitdb-go/ingitdb": false,
		"golang.org/x/tools/go/packages":        false,
	}
	for path, want := range cases {
		if got := isStandardLibraryImport(path); got != want {
			t.Errorf("isStandardLibraryImport(%q) = %v, want %v", path, got, want)
		}
	}
}
