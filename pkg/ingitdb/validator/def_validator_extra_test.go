package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/config"
)

func writeCollectionDef(t *testing.T, dir string, content string) {
	t.Helper()

	err := os.MkdirAll(dir, 0777)
	if err != nil {
		t.Fatalf("failed to create dir: %s", err)
	}
	path := filepath.Join(dir, ingitdb.CollectionDefFileName)
	err = os.WriteFile(path, []byte(content), 0666)
	if err != nil {
		t.Fatalf("failed to write file: %s", err)
	}
}

func TestReadRootCollections_WildcardError(t *testing.T) {
	t.Parallel()

	rootConfig := config.RootConfig{
		RootCollections: map[string]string{
			"todo": "missing/*",
		},
	}

	_, err := readRootCollections(t.TempDir(), rootConfig, ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "failed to validate root collections def (todo @ missing/*)") {
		t.Fatalf("unexpected error: %s", errMsg)
	}
}

func TestReadRootCollections_SingleError(t *testing.T) {
	t.Parallel()

	rootConfig := config.RootConfig{
		RootCollections: map[string]string{
			"countries": "missing",
		},
	}

	_, err := readRootCollections(t.TempDir(), rootConfig, ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "failed to validate root collection def ID=countries") {
		t.Fatalf("unexpected error: %s", errMsg)
	}
}

func TestReadCollectionDef_FileMissing(t *testing.T) {
	t.Parallel()

	_, err := readCollectionDef(t.TempDir(), "missing", "id", ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "failed to read file") {
		t.Fatalf("unexpected error: %s", errMsg)
	}
}

func TestReadCollectionDef_InvalidYAML(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := filepath.Join(root, "bad")
	writeCollectionDef(t, dir, "a: [1,2\n")

	_, err := readCollectionDef(root, "bad", "id", ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "failed to parse YAML file") {
		t.Fatalf("unexpected error: %s", errMsg)
	}
}

func TestReadCollectionDef_InvalidDefinitionWithValidation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := filepath.Join(root, "invalid")
	writeCollectionDef(t, dir, "columns: {}\n")

	_, err := readCollectionDef(root, "invalid", "id", ingitdb.NewReadOptions(ingitdb.Validate()))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "not valid definition of collection") {
		t.Fatalf("unexpected error: %s", errMsg)
	}
}

func TestReadCollectionDefs_ReadDirError(t *testing.T) {
	t.Parallel()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{},
	}

	_, err := readCollectionDefs(def, t.TempDir(), "missing/*", "root", ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "failed to read dir") {
		t.Fatalf("unexpected error: %s", errMsg)
	}
}

func TestReadCollectionDefs_SkipMissingAndNonDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	collectionsDir := filepath.Join(root, "collections")
	err := os.MkdirAll(collectionsDir, 0777)
	if err != nil {
		t.Fatalf("failed to create collections dir: %s", err)
	}

	filePath := filepath.Join(collectionsDir, "note.txt")
	err = os.WriteFile(filePath, []byte("note"), 0666)
	if err != nil {
		t.Fatalf("failed to write file: %s", err)
	}

	missingDir := filepath.Join(collectionsDir, "missing")
	err = os.MkdirAll(missingDir, 0777)
	if err != nil {
		t.Fatalf("failed to create dir: %s", err)
	}

	validDir := filepath.Join(collectionsDir, "valid")
	writeCollectionDef(t, validDir, "record_file:\n  format: json\n  name: \"{key}.json\"\ncolumns:\n  name:\n    type: string\n")

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{},
	}

	_, err = readCollectionDefs(def, root, "collections/*", "root", ingitdb.NewReadOptions())
	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}

	if def.Collections["root.valid"] == nil {
		t.Fatal("expected root.valid to be present")
	}
	if _, ok := def.Collections["root.missing"]; ok {
		t.Fatal("expected root.missing to be skipped")
	}
}

func TestReadCollectionDefs_PrefixWithDot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	collectionsDir := filepath.Join(root, "collections")
	dir := filepath.Join(collectionsDir, "alpha")
	writeCollectionDef(t, dir, "record_file:\n  format: json\n  name: \"{key}.json\"\ncolumns:\n  name:\n    type: string\n")

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{},
	}

	_, err := readCollectionDefs(def, root, "collections/*", "root.", ingitdb.NewReadOptions())
	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}
	if def.Collections["root.alpha"] == nil {
		t.Fatal("expected root.alpha to be present")
	}
}

func TestReadCollectionDefs_ErrorWrap(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	collectionsDir := filepath.Join(root, "collections")
	dir := filepath.Join(collectionsDir, "bad")
	writeCollectionDef(t, dir, "a: [1,2\n")

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{},
	}

	_, err := readCollectionDefs(def, root, "collections/*", "root", ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "failed to read collection def 'bad'") {
		t.Fatalf("unexpected error: %s", errMsg)
	}
}
