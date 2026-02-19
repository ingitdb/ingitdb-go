package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2fsingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/materializer"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/validator"
)

func TestCreateRecord_UpdatesTagsReadme(t *testing.T) {
	t.Parallel()

	repoRoot := findRepoRoot(t)
	tmpDir := t.TempDir()
	dstTagsDir := filepath.Join(tmpDir, "test-ingitdb", "todo", "tags")
	srcTagsDir := filepath.Join(repoRoot, "test-ingitdb", "todo", "tags")
	if err := copyDir(srcTagsDir, dstTagsDir); err != nil {
		t.Fatalf("copy tags dir: %v", err)
	}
	rootConfig := []byte("rootCollections:\n  todo.tags: test-ingitdb/todo/tags\n")
	if err := os.WriteFile(filepath.Join(tmpDir, ".ingitdb.yaml"), rootConfig, 0o644); err != nil {
		t.Fatalf("write root config: %v", err)
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return tmpDir, nil }
	readDef := validator.ReadDefinition
	newDB := func(root string, def *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, def)
	}
	viewBuilder := materializer.NewViewBuilder(materializer.NewFileRecordsReader())
	logf := func(...any) {}

	cmd := Create(homeDir, getWd, readDef, newDB, viewBuilder, logf)
	if err := runCLICommand(cmd, "record", "--path="+tmpDir, "--id=todo.tags/urgent", "--data={title: Urgent}"); err != nil {
		t.Fatalf("Create record: %v", err)
	}

	readmePath := filepath.Join(dstTagsDir, "README.md")
	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	if !strings.Contains(string(content), "**Urgent**") {
		t.Fatalf("expected README to include Urgent tag, got:\n%s", string(content))
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 6; i++ {
		if _, statErr := os.Stat(filepath.Join(dir, "test-ingitdb")); statErr == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("failed to locate repo root from %s", dir)
	return ""
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
}
