package materializer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
	"github.com/ingitdb/ingitdb-go/ingitdb/validator"
)

// Verifies materialized-view-md-extension#ac:md-formats-view-gets-md-extension:
// a template-less named view whose formats include "md" resolves to a ".md"
// output path (not ".ingr").
func TestResolveViewOutputPath_MdFormatsGetsMdExtension(t *testing.T) {
	t.Parallel()

	col := &ingitdb.CollectionDef{DirPath: "/db/collection"}
	view := &ingitdb.ViewDef{ID: "status_new", Formats: []string{"md"}}

	got := resolveViewOutputPath(col, view, "/db", "/db")
	want := filepath.Join("/db", ingitdb.IngitdbDir, "collection", "status_new.md")
	if got != want {
		t.Errorf("resolveViewOutputPath = %q, want %q", got, want)
	}
}

// Verifies materialized-view-md-extension#ac:non-md-view-keeps-ingr-extension:
// a template-less named view without "md" in formats keeps the ".ingr"
// data-export extension, unchanged from prior behaviour.
func TestResolveViewOutputPath_NoMdFormatsKeepsIngrExtension(t *testing.T) {
	t.Parallel()

	col := &ingitdb.CollectionDef{DirPath: "/db/collection"}
	view := &ingitdb.ViewDef{ID: "export"}

	got := resolveViewOutputPath(col, view, "/db", "/db")
	want := filepath.Join("/db", ingitdb.IngitdbDir, "collection", "export.ingr")
	if got != want {
		t.Errorf("resolveViewOutputPath = %q, want %q", got, want)
	}
}

// writeViewFixtureDB builds a real single-collection inGitDB database in a temp
// dir: a `tasks` SingleRecord collection with a parameterized markdown view
// `status_{status}.yaml`, exercised through the production reader + view
// builder rather than a hand-built CollectionDef.
func writeViewFixtureDB(t *testing.T, records map[string]string) string {
	t.Helper()
	dir := t.TempDir()

	ingitdbDir := filepath.Join(dir, ".ingitdb")
	mustMkdir(t, ingitdbDir)
	mustWrite(t, filepath.Join(ingitdbDir, "settings.yaml"), "languages:\n  - required: en\n")
	mustWrite(t, filepath.Join(ingitdbDir, "root-collections.yaml"), "tasks: ./tasks\n")

	schemaDir := filepath.Join(dir, "tasks", ".collection")
	mustMkdir(t, schemaDir)
	mustWrite(t, filepath.Join(schemaDir, "definition.yaml"),
		"record_file:\n  name: \"{key}.json\"\n  type: \"map[string]any\"\n  format: json\n"+
			"columns:\n  title:\n    type: string\n    required: true\n"+
			"  status:\n    type: string\n    required: true\n")

	viewsDir := filepath.Join(schemaDir, "views")
	mustMkdir(t, viewsDir)
	mustWrite(t, filepath.Join(viewsDir, "status_{status}.yaml"),
		"titles:\n  en: \"Status: {status}\"\norder_by: title\nformats:\n  - md\ncolumns:\n  - title\n  - status\n")

	recordsDir := filepath.Join(dir, "tasks", "$records")
	mustMkdir(t, recordsDir)
	for key, content := range records {
		mustWrite(t, filepath.Join(recordsDir, key+".json"), content)
	}
	return dir
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Verifies
// materialized-view-md-extension#ac:parameterized-md-view-materializes-md-files
// and #ac:view-db-materializes-at-least-one-file: a database shipping a
// formats:[md] parameterized view, read with the real reader, materializes one
// ".md" file per distinct status value, each holding a Markdown table.
func TestBuildViews_ParameterizedMdView_MaterializesMdFiles(t *testing.T) {
	t.Parallel()

	dir := writeViewFixtureDB(t, map[string]string{
		"t1": `{"title":"A","status":"new"}`,
		"t2": `{"title":"B","status":"done"}`,
	})

	def, err := validator.ReadDefinition(dir, ingitdb.Validate())
	if err != nil {
		t.Fatalf("ReadDefinition failed: %v", err)
	}
	col := def.Collections["tasks"]
	if col == nil {
		t.Fatalf("collection %q not loaded; have %v", "tasks", def.Collections)
	}

	builder := NewViewBuilder(NewFileRecordsReader(), nil)
	res, err := builder.BuildViews(context.Background(), dir, dir, col, def)
	if err != nil {
		t.Fatalf("BuildViews failed: %v", err)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("unexpected view errors: %v", res.Errors)
	}
	if res.FilesCreated < 1 {
		t.Fatalf("expected at least one materialized file, got %d created", res.FilesCreated)
	}

	viewsOut := filepath.Join(dir, ingitdb.IngitdbDir, "tasks")
	for _, status := range []string{"new", "done"} {
		outPath := filepath.Join(viewsOut, "status_"+status+".md")
		content, readErr := os.ReadFile(outPath)
		if readErr != nil {
			t.Fatalf("expected materialized markdown file %s: %v", outPath, readErr)
		}
		text := string(content)
		if !strings.Contains(text, "| title |") || !strings.Contains(text, "---|") {
			t.Errorf("file %s is not a markdown table:\n%s", outPath, text)
		}
	}

	// No ".ingr" file must be produced for a formats:[md] view.
	if entries, _ := os.ReadDir(viewsOut); entries != nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".ingr") {
				t.Errorf("unexpected .ingr output for a formats:[md] view: %s", e.Name())
			}
		}
	}
}
