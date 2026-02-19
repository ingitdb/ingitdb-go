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
	if !strings.Contains(errMsg, "wildcard root collection paths are not supported") {
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
