package commands

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func newTestApp(cmds ...*cli.Command) *cli.Command {
	return &cli.Command{Commands: cmds}
}

func testContext() context.Context {
	return context.Background()
}

func TestDeleteRecord_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)
	content, err := yaml.Marshal(map[string]any{"name": "Bye"})
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	path := filepath.Join(dir, "bye.yaml")
	if err = os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2ingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	delCmd := Delete(homeDir, getWd, readDef, newDB, logf)
	app := newTestApp(delCmd)
	runErr := app.Run(testContext(), []string{"app", "delete", "record", "--path=" + dir, "--id=test/items/bye"})
	if runErr != nil {
		t.Fatalf("delete record: %v", runErr)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("expected file to be deleted")
	}
}

func TestDeleteRecord_NotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2ingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	delCmd := Delete(homeDir, getWd, readDef, newDB, logf)
	app := newTestApp(delCmd)
	err := app.Run(testContext(), []string{"app", "delete", "record", "--path=" + dir, "--id=test/items/ghost"})
	if err == nil {
		t.Fatal("expected error for not-found record")
	}
}
