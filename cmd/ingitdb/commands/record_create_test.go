package commands

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/urfave/cli/v3"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// testDef returns a Definition with a single SingleRecord YAML collection at dirPath.
func testDef(dirPath string) *ingitdb.Definition {
	return &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.items": {
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
			},
		},
	}
}

// runCLICommand runs a cli.Command with the given arguments (without the program name).
func runCLICommand(cmd *cli.Command, args ...string) error {
	app := &cli.Command{
		Commands: []*cli.Command{cmd},
	}
	argv := append([]string{"app", cmd.Name}, args...)
	return app.Run(context.Background(), argv)
}

func TestCreate_Success(t *testing.T) {
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

	cmd := Create(homeDir, getWd, readDef, newDB, logf)
	err := runCLICommand(cmd, "--path="+dir, "--id=test/items/hello", "--data={name: Hello}")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(dir, "hello.yaml")); statErr != nil {
		t.Fatalf("expected file hello.yaml to be created: %v", statErr)
	}
}

func TestCreate_MissingID(t *testing.T) {
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

	cmd := Create(homeDir, getWd, readDef, newDB, logf)
	err := runCLICommand(cmd, "--path="+dir, "--data={name: Hello}")
	if err == nil {
		t.Fatal("expected error for missing --id flag")
	}
}

func TestCreate_InvalidYAML(t *testing.T) {
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

	cmd := Create(homeDir, getWd, readDef, newDB, logf)
	err := runCLICommand(cmd, "--path="+dir, "--id=test/items/x", "--data=: invalid: yaml: :")
	if err == nil {
		t.Fatal("expected error for invalid YAML in --data")
	}
}

func TestCreate_CollectionNotFound(t *testing.T) {
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

	cmd := Create(homeDir, getWd, readDef, newDB, logf)
	err := runCLICommand(cmd, "--path="+dir, "--id=no/such/thing", "--data={name: X}")
	if err == nil {
		t.Fatal("expected error for unknown collection")
	}
}

func TestCreate_ReadDefinitionError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("boom")
	}
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2ingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := Create(homeDir, getWd, readDef, newDB, logf)
	err := runCLICommand(cmd, "--path="+dir, "--id=test/items/x", "--data={name: X}")
	if err == nil {
		t.Fatal("expected error when readDefinition fails")
	}
}
